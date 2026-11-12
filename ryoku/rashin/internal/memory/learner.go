package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/rank"
	"ryoku-rashin/internal/redact"
)

// shownFacts is how many stored facts the learner is shown to reconcile
// against.
const shownFacts = 20

// learnReplyTokens bounds the learner's reply.
const learnReplyTokens = 2048

// learner learns from conversations once they have gone quiet. One timer
// stands for the earliest conversation due; nothing runs and no timer is set
// while none waits.
type learner struct {
	e *Engine

	mu sync.Mutex
	// due is when each waiting conversation goes quiet.
	due map[string]time.Time
	// waiting are conversations put off until a model can learn them, with
	// why.
	waiting map[string]string
	timer   *time.Timer
	// running counts learning passes under way; work tracks their
	// goroutines for close.
	running int
	work    sync.WaitGroup
	// learning are the conversations a pass is reading now, so two passes
	// never learn the same turns.
	learning map[string]bool
	// scanned is set once the conversations left behind by an earlier run of
	// the daemon were queued.
	scanned bool
	last    *time.Time
	model   learnModel
	// rewrote is when the model last rewrote the card.
	rewrote time.Time
	ctx     context.Context
	cancel  context.CancelFunc
	// chats is the past-conversation search index, built on first search and
	// dropped by release.
	chats *chatIndex
	// now is the clock, replaced in tests.
	now func() time.Time
}

// newLearner costs nothing: no timer, goroutine or file until a conversation
// is due.
func newLearner(e *Engine) *learner {
	ctx, cancel := context.WithCancel(context.Background())
	return &learner{
		e:        e,
		due:      map[string]time.Time{},
		waiting:  map[string]string{},
		learning: map[string]bool{},
		ctx:      ctx,
		cancel:   cancel,
		now:      time.Now,
	}
}

// learns reports whether memory learns at all now.
func (l *learner) learns() bool {
	return l.e.on() && l.e.settings().Learn
}

// quiet is how long a conversation rests before it is learned from.
func (l *learner) quiet() time.Duration {
	return time.Duration(l.e.settings().QuietMinutes) * time.Minute
}

// learnable reports a conversation memory may learn from: not temporary, not
// told to only use memory or none, and not a team task's.
func learnable(info protocol.SessionInfo) bool {
	return !info.Temporary && info.Memory == protocol.MemoryRemember && info.Team == ""
}

// turnEnded makes the conversation due once it has rested.
func (l *learner) turnEnded(info protocol.SessionInfo) {
	if !l.learns() || !learnable(info) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.scanLocked()
	delete(l.waiting, info.ID)
	l.due[info.ID] = l.now().Add(l.quiet())
	l.armLocked()
}

// scanLocked queues, once, the conversations an earlier run of the daemon
// left behind their watermark, each due when it has rested since its last
// turn.
func (l *learner) scanLocked() {
	sessions := l.e.sessionsOf()
	if l.scanned || sessions == nil {
		return
	}
	store, err := l.e.store()
	if err != nil {
		return
	}
	l.scanned = true
	for _, kind := range []protocol.SessionKind{protocol.KindAsk, protocol.KindAgent} {
		for _, info := range sessions.List(kind) {
			if learnable(info) && info.LastSeq > store.Watermark(info.ID) {
				_, queued := l.due[info.ID]
				_, waits := l.waiting[info.ID]
				if !queued && !waits {
					l.due[info.ID] = info.Updated.Add(l.quiet())
				}
			}
		}
	}
}

// armLocked sets the one timer for the earliest conversation due.
func (l *learner) armLocked() {
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	if len(l.due) == 0 || l.ctx.Err() != nil {
		return
	}
	var first time.Time
	for _, at := range l.due {
		if first.IsZero() || at.Before(first) {
			first = at
		}
	}
	l.timer = time.AfterFunc(max(first.Sub(l.now()), 0), l.fire)
}

// fire learns every conversation that has gone quiet, and gives the
// conversations waiting for a model another try with them: a pass that
// runs anyway costs no extra wake-up.
func (l *learner) fire() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.timer = nil
	ids := l.takeLocked(false)
	if len(ids) > 0 {
		for id := range l.waiting {
			ids = append(ids, id)
		}
		clear(l.waiting)
	}
	l.startLocked(ids)
	l.armLocked()
}

// settle learns every conversation that is due, now, whatever its rest. A
// conversation waiting for a model is left waiting: it waits for a model,
// not for rest, so settling it again would only keep the daemon busy. It
// returns at once; busy reports the work until it is done.
func (l *learner) settle() {
	if !l.learns() {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.scanLocked()
	l.startLocked(l.takeLocked(true))
	l.armLocked()
}

// takeLocked removes and returns the conversations due now, or all of them.
func (l *learner) takeLocked(all bool) []string {
	now := l.now()
	var ids []string
	for id, at := range l.due {
		if all || !at.After(now) {
			ids = append(ids, id)
			delete(l.due, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// startLocked learns ids one after another on one goroutine. busy is true
// from this call until the last of them is done.
func (l *learner) startLocked(ids []string) {
	if len(ids) == 0 || l.ctx.Err() != nil {
		return
	}
	l.running++
	l.work.Add(1)
	go func() {
		defer l.work.Done()
		for _, id := range ids {
			if l.ctx.Err() != nil {
				break
			}
			_, _ = l.learn(l.ctx, id)
		}
		l.mu.Lock()
		l.running--
		l.mu.Unlock()
	}()
}

// busy reports learning under way.
func (l *learner) busy() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running > 0
}

// sessionDeleted forgets a deleted conversation from the queue.
func (l *learner) sessionDeleted(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.due, id)
	delete(l.waiting, id)
	if l.chats != nil {
		l.chats.drop(id)
	}
	l.armLocked()
}

// state is the learner as the status shows it.
func (l *learner) state() protocol.MemoryLearner {
	if !l.learns() {
		return protocol.MemoryLearner{State: protocol.LearnerOff}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	state := protocol.MemoryLearner{State: protocol.LearnerReady, Model: l.model.Model, Local: l.model.Local, Waiting: len(l.due) + len(l.waiting), Last: l.last}
	for _, why := range l.waiting {
		state.State, state.Detail = protocol.LearnerWaiting, why
		break
	}
	return state
}

// release drops the past-conversation index at rest.
func (l *learner) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.chats = nil
}

// close stops the timer and waits for learning under way to stop.
func (l *learner) close() {
	l.cancel()
	l.mu.Lock()
	if l.timer != nil {
		l.timer.Stop()
		l.timer = nil
	}
	l.mu.Unlock()
	l.work.Wait()
}

// wait puts a conversation off until a model can learn it.
func (l *learner) wait(id, why string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.waiting[id] = why
}

// learnNow learns a conversation at once, for memory.learn.
func (e *Engine) learnNow(ctx context.Context, session string) (protocol.MemoryChange, error) {
	if !e.learn.learns() {
		return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeDisabled, "%s", i18n.T("Learning is off in Settings > Memory."))
	}
	e.learn.mu.Lock()
	delete(e.learn.due, session)
	e.learn.running++
	e.learn.mu.Unlock()
	defer func() {
		e.learn.mu.Lock()
		e.learn.running--
		e.learn.armLocked()
		e.learn.mu.Unlock()
	}()
	return e.learn.learn(ctx, session)
}

// learn reads a conversation past its watermark and learns from it with one
// model call, retried once with the short prompt when the reply is not
// usable. The watermark moves only once a pass has finished; a pass that
// fails leaves the conversation to be read again with its next turns.
func (l *learner) learn(ctx context.Context, id string) (protocol.MemoryChange, error) {
	e := l.e
	l.mu.Lock()
	if l.learning[id] {
		l.mu.Unlock()
		return protocol.MemoryChange{}, nil
	}
	l.learning[id] = true
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		delete(l.learning, id)
		l.mu.Unlock()
	}()
	sessions := e.sessionsOf()
	if sessions == nil {
		return protocol.MemoryChange{}, nil
	}
	info, ok := sessions.Info(id)
	if !ok || !learnable(info) || !l.learns() {
		return protocol.MemoryChange{}, nil
	}
	// an Ask title is the raw first question: scrubbed once, used everywhere
	info.Title = redact.Strict(stripPrivate(info.Title))
	store, err := e.store()
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	events, err := sessions.Events(id)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	conv := readConversation(events, store.Watermark(id))
	if conv.last == 0 {
		return protocol.MemoryChange{}, nil
	}
	if len(conv.turns) == 0 || allTrivial(conv.turns) {
		return protocol.MemoryChange{}, store.SetWatermark(id, conv.last)
	}
	scrubTurns(conv.turns)
	settings := e.settings()
	scope, kept := e.sessionScope(info)
	answered := conv.model
	if answered == "" {
		answered = info.Model
	}
	engine, err := e.external(ctx)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	if engine != nil && engine.Caps().Learns {
		return protocol.MemoryChange{}, l.observe(ctx, engine, store, info, scope, kept, answered, conv)
	}
	model, why := e.chooseModel(ctx, answered)
	if why != "" {
		l.wait(id, why)
		return protocol.MemoryChange{}, protocol.Errorf(protocol.CodeUnavailable, "%s", why)
	}
	l.mu.Lock()
	l.model = model
	l.mu.Unlock()

	scopes := []string{protocol.MemoryYou}
	if kept {
		scopes = nil
	}
	if scope != "" {
		scopes = append(scopes, scope)
	}
	words := userWords(conv.turns)
	shown, _ := store.Similar(ctx, words, scopes, shownFacts)
	active, _ := store.Active(scopes...)
	r := &reconcile{
		shown:     shown,
		project:   scope,
		kept:      kept,
		words:     words,
		sensitive: settings.Sensitive,
		active:    len(active),
		suggest:   background(info),
		source:    protocol.MemorySource{Kind: sourceOf(info), Session: id, Turn: conv.turn, Title: info.Title, Model: model.Model},
		store:     store,
	}
	user := fmt.Sprintf(learnUser, id, info.Kind, info.Title, projectLine(e, scope), personalLine(kept),
		longDate(info.Updated), longDate(l.now()), listShown(shown, scope), renderTurns(conv.turns, model.budget()))
	reply, ops, err := l.ask(ctx, model, user, r)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	l.note(ctx, store, info, reply.Summary)
	change, err := store.Apply(Origin{By: protocol.MemoryByLearning, Session: id, Model: model.Model, Turn: conv.turn}, ops)
	if err != nil {
		return protocol.MemoryChange{}, err
	}
	if err := store.SetWatermark(id, conv.last); err != nil {
		return change, err
	}
	at := l.now()
	l.mu.Lock()
	l.last = &at
	l.mu.Unlock()
	if len(change.Items) > 0 {
		e.changed(change)
	}
	if faded, err := store.Fade(settings.FadeMonths); err == nil && len(faded.Items) > 0 {
		e.changed(faded)
	}
	if len(change.Items) > 0 {
		_, _ = e.buildCard(ctx, store, func() learnModel { return model }, false)
	}
	return change, nil
}

// Portions adapted from sipeed/picoclaw pkg/seahorse/short_compaction.go @
// bbf6893ca7af (MIT, Copyright (c) 2026 PicoClaw contributors). Modified by
// Ryoku: two rungs, the full prompt and the short one, and no raw fallback.

// ask makes the learning call, retrying once with the short prompt when the
// reply is not the JSON asked for or fails validation as a whole.
func (l *learner) ask(ctx context.Context, model learnModel, user string, r *reconcile) (learnReply, []Op, error) {
	var lastErr error
	for _, system := range []string{learnSystem, learnSystemShort} {
		zero := 0.0
		text, _, err := complete(ctx, l.e.deps.Client, llm.Request{
			Model:       model.Model,
			Strict:      true,
			Temperature: &zero,
			MaxTokens:   learnReplyTokens,
			Messages:    []llm.Message{{Role: llm.RoleSystem, Content: system}, {Role: llm.RoleUser, Content: user}},
			Extra:       jsonFormat(model.Local, "memory_learn", learnSchema),
		})
		if err != nil {
			return learnReply{}, nil, err
		}
		var reply learnReply
		if lastErr = parseReply(text, &reply); lastErr != nil {
			continue
		}
		ops, err := r.ops(ctx, reply)
		if lastErr = err; err != nil {
			continue
		}
		return reply, ops, nil
	}
	return learnReply{}, nil, lastErr
}

// note keeps the learner's summary of the conversation for searching it
// later; a summary holding a secret is kept without it, and one that reads
// as an instruction is not kept.
func (l *learner) note(ctx context.Context, store *Store, info protocol.SessionInfo, summary string) {
	summary = redact.Strict(stripPrivate(summary))
	if _, hit := scanStrict(summary); summary == "" || hit {
		return
	}
	note := ChatNote{Session: info.ID, Kind: info.Kind, Title: info.Title, Summary: summary, Updated: info.Updated}
	if vectors, err := store.Embed(ctx, []string{summary}); err == nil && len(vectors) == 1 {
		note.Vector = rank.Quantize(vectors[0])
	}
	if store.PutNote(note) == nil {
		l.mu.Lock()
		if l.chats != nil {
			l.chats.drop(info.ID)
		}
		l.mu.Unlock()
	}
}

// observe hands the new turns to an engine that learns by itself.
func (l *learner) observe(ctx context.Context, engine backend.Backend, store *Store, info protocol.SessionInfo, scope string, kept bool, answered string, conv conversation) error {
	if !kept {
		scope = protocol.MemoryYou
	}
	transcript := backend.Transcript{Session: info.ID, Kind: sourceOf(info), Title: info.Title, Scope: scope, Model: answered, Turns: conv.turns}
	if err := engine.Observe(ctx, transcript); err != nil {
		return err
	}
	at := l.now()
	l.mu.Lock()
	l.last = &at
	l.mu.Unlock()
	return store.SetWatermark(info.ID, conv.last)
}

// background reports a conversation the user did not hold themselves, such
// as one imported from another program: what is learned there waits for the
// user as a suggestion.
func background(info protocol.SessionInfo) bool {
	switch info.Origin {
	case protocol.OriginApp, protocol.OriginBar, protocol.OriginCLI, protocol.OriginTerminal, protocol.OriginFix, "":
		return false
	}
	return true
}

// sourceOf is the source kind of what is learned from a conversation.
func sourceOf(info protocol.SessionInfo) string {
	if info.Kind == protocol.KindAgent {
		return protocol.MemoryFromAgent
	}
	return protocol.MemoryFromAsk
}

// longDate is a day with its weekday, for grounding relative dates.
func longDate(at time.Time) string {
	at = at.Local()
	return at.Format("Monday, January 2, 2006") + " (" + at.Format("2006-01-02") + ")"
}

// projectLine names the conversation's project for the learner.
func projectLine(e *Engine, scope string) string {
	if scope == "" {
		return "none: there is no project, so keep no project facts"
	}
	return e.scopeName(scope)
}

// personalLine says whether personal facts may be learned.
func personalLine(kept bool) string {
	if kept {
		return "not kept: this project keeps to itself, so keep only project facts"
	}
	return "kept"
}
