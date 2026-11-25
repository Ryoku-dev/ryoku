package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/note"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sensitive"
)

// Voice memory goes through memory.Handle like the app's own calls, as the
// user. Nothing is forgotten or changed by voice without a card that names
// exactly what: a forget or a correction shows the facts and waits for a
// yes from the talk key or the app, never from the wake phrase or open mic.
// What Oni knows is read back masked, as every memory read is.

// oniMemory is the voice memory card waiting on its answer and the note the
// voice kept last, which "no, I meant" may still correct.
type oniMemory struct {
	mu   sync.Mutex
	last *voiceNote
	card *memoryCard
}

// voiceNote is the note a spoken "remember that" kept.
type voiceNote struct {
	id, text string
	at       time.Time
}

// memoryCard is one forget or correction waiting on its answer: the facts
// it forgets, or the fact it changes and the text it changes it to.
type memoryCard struct {
	id    string
	ids   []string
	edit  string
	text  string
	timer presence.Timer
}

var oniPhase16Spec = sync.OnceValues(spec.Load)

// memoryLimits is the spec's voice memory section.
func memoryLimits() spec.Memory {
	if s, err := oniPhase16Spec(); err == nil {
		return s.Memory
	}
	return spec.Memory{}
}

// Lines said for voice memory.
const (
	lineMemoryKnowNone   = "I don't remember anything about that."
	lineMemoryKnowYou    = "I don't remember anything about you yet."
	lineMemoryNone       = "I don't remember anything like that."
	lineMemoryNoLast     = "I haven't kept a note just now."
	lineMemoryLate       = "That note is too old to change by voice. Say what to correct."
	lineMemoryCard       = "Check the card, then say yes with the key."
	lineMemoryNoCard     = "Open Oni to see what I'd change."
	lineMemoryForgot     = "Forgotten."
	lineMemoryChanged    = "Changed it."
	lineMemoryUnreadable = "I couldn't reach memory."
	lineMemoryKept       = "Nothing changed."
)

// voiceMemoryOn reports whether spoken memory commands are on: the shared
// notes switch with quick commands.
func (d *Daemon) voiceMemoryOn() bool {
	return feature.Decide(feature.Request{Feature: feature.MemoryForget}, protocol.EffectiveOni(d.Settings())).Allow
}

// memoryNoted keeps the note a spoken "remember that" just kept, so
// "no, I meant" may correct it within the spec's window.
func (d *Daemon) memoryNoted(change protocol.MemoryChange) {
	for _, item := range change.Items {
		if item.Op != protocol.MemoryOpRemembered && item.Op != protocol.MemoryOpReplaced {
			continue
		}
		m := &d.oni.memory
		m.mu.Lock()
		m.last = &voiceNote{id: item.ID, text: item.Text, at: d.reflexClock().Now()}
		m.mu.Unlock()
		return
	}
}

// voiceMemory carries out a spoken memory command and says how it went.
func (d *Daemon) voiceMemory(cmd note.Command, p route.Press, turn uint64, source string, gen uint64) {
	v := &d.oni.voice
	defer func() {
		v.mu.Lock()
		v.work--
		v.mu.Unlock()
		d.stateChanged()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	line := d.memoryCommand(ctx, cmd, p)
	v.mu.Lock()
	defer v.mu.Unlock()
	if gen != v.gen || !v.turn.Accept(turn) {
		return
	}
	settings := protocol.EffectiveOni(d.Settings())
	if !d.voiceSayLocked(settings, turn, source, route.Spoken(line, false)) {
		if svc := d.oni.service.Load(); svc != nil {
			_ = svc.Bubble(0, "answer", oniText(line, oniBubbleMax))
		}
	}
	d.voiceRestLocked(turn)
}

// memoryCommand runs cmd and returns the line to say. A forget or a
// correction only shows its card; the write waits for the answer.
func (d *Daemon) memoryCommand(ctx context.Context, cmd note.Command, p route.Press) string {
	if !d.voiceMemoryOn() {
		return i18n.T(lineNoteNotesOff)
	}
	if d.deps.Memory == nil {
		return i18n.T(lineNoteOff)
	}
	if cmd.Writes() && !p.Valid() {
		return i18n.T(route.ReflexNeedsPressLine)
	}
	switch cmd.Kind {
	case note.Know:
		facts, err := d.memoryFacts(ctx, cmd.Arg)
		if err != nil {
			return memoryErrLine(err)
		}
		return knowLine(facts, cmd.Arg)
	case note.Forget:
		return d.memoryForget(ctx, cmd.Arg)
	case note.Correct:
		return d.memoryCorrect(ctx, cmd.From, cmd.To)
	}
	return i18n.T(lineReflexCant)
}

// knowLine says what is remembered about topic, or that nothing is.
func knowLine(facts []protocol.Memory, topic string) string {
	if len(facts) == 0 {
		if topic == "" {
			return i18n.T(lineMemoryKnowYou)
		}
		return i18n.T(lineMemoryKnowNone)
	}
	texts := make([]string, len(facts))
	for i, f := range facts {
		texts[i] = strings.TrimRight(oniText(f.Text, 160), ".")
	}
	return i18n.Tf("I remember: %s.", strings.Join(texts, "; "))
}

// memoryFacts are the active facts about topic, best first, or the user's
// own when topic is empty: at most the spec's forgetMax, masked as every
// memory read is.
func (d *Daemon) memoryFacts(ctx context.Context, topic string) ([]protocol.Memory, error) {
	limit := max(memoryLimits().ForgetMax, 1)
	if topic == "" {
		var res protocol.MemoryListResult
		err := d.memoryAsUser(ctx, protocol.MMemoryList, protocol.MemoryListParams{Scope: protocol.MemoryYou, Limit: limit}, &res)
		return res.Memories, err
	}
	var res protocol.MemorySearchResult
	err := d.memoryAsUser(ctx, protocol.MMemorySearch, protocol.MemorySearchParams{Query: topic, Limit: limit}, &res)
	if len(res.Memories) > limit {
		res.Memories = res.Memories[:limit]
	}
	return res.Memories, err
}

// memoryAsUser calls one memory method as the user does from the command
// line, decoding its result into out.
func (d *Daemon) memoryAsUser(ctx context.Context, method string, params, out any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	res, err := d.deps.Memory.Handle(ctx, core.MemoryCaller{}, method, raw)
	if err != nil || out == nil {
		return err
	}
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// memoryErrLine is the line for a memory call that failed.
func memoryErrLine(err error) string {
	var perr *protocol.Error
	switch {
	case errors.As(err, &perr) && perr.Code == protocol.CodeDisabled:
		return i18n.T(lineNoteOff)
	case errors.As(err, &perr) && perr.Code == protocol.CodeBadRequest:
		return i18n.T(lineNoteWontKeep)
	}
	return i18n.T(lineMemoryUnreadable)
}

// lastNote is the note the voice kept within the correction window.
func (d *Daemon) lastNote() (voiceNote, bool, bool) {
	m := &d.oni.memory
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.last == nil {
		return voiceNote{}, false, false
	}
	fresh := d.reflexClock().Now().Sub(m.last.at) <= memoryLimits().Correct()
	return *m.last, true, fresh
}

// memoryForget shows the card that forgets what arg names, or the note just
// kept when arg is empty.
func (d *Daemon) memoryForget(ctx context.Context, arg string) string {
	if arg == "" {
		last, ok, fresh := d.lastNote()
		if !ok || !fresh {
			return i18n.T(lineMemoryNoLast)
		}
		return d.memoryCardOut(&memoryCard{ids: []string{last.id}}, i18n.Tf("Forget “%s”?", oniText(last.text, 160)))
	}
	facts, err := d.memoryFacts(ctx, arg)
	if err != nil {
		return memoryErrLine(err)
	}
	facts = naming(facts, arg)
	if len(facts) == 0 {
		return i18n.T(lineMemoryNone)
	}
	card := &memoryCard{}
	quoted := make([]string, len(facts))
	for i, f := range facts {
		card.ids = append(card.ids, f.ID)
		quoted[i] = "“" + oniText(f.Text, 120) + "”"
	}
	question := i18n.Tf("Forget %s?", strings.Join(quoted, "; "))
	if len(facts) > 1 {
		question = i18n.Tf("Forget these %d: %s?", len(facts), strings.Join(quoted, "; "))
	}
	return d.memoryCardOut(card, question)
}

// naming keeps the facts that hold a word of arg; when none does, the best
// match alone, so a forget never sweeps up facts that only rank near.
func naming(facts []protocol.Memory, arg string) []protocol.Memory {
	var words []string
	for _, w := range strings.Fields(strings.ToLower(arg)) {
		if w = strings.Trim(w, ".,!?;:\"'“”"); utf8.RuneCountInString(w) > 2 {
			words = append(words, w)
		}
	}
	var out []protocol.Memory
	for _, f := range facts {
		lower := strings.ToLower(f.Text)
		for _, w := range words {
			if strings.Contains(lower, w) {
				out = append(out, f)
				break
			}
		}
	}
	if len(out) == 0 && len(facts) > 0 {
		return facts[:1]
	}
	return out
}

// memoryCorrect shows the card that changes from to to in a remembered
// fact, or splices to into the note just kept when from is empty.
func (d *Daemon) memoryCorrect(ctx context.Context, from, to string) string {
	var id, old, text string
	if from == "" {
		last, ok, fresh := d.lastNote()
		switch {
		case !ok:
			return i18n.T(lineMemoryNoLast)
		case !fresh:
			return i18n.T(lineMemoryLate)
		}
		spliced, ok := note.Splice(last.text, to)
		if !ok {
			return i18n.T(lineReflexCant)
		}
		id, old, text = last.id, last.text, spliced
	} else {
		facts, err := d.memoryFacts(ctx, from)
		if err != nil {
			return memoryErrLine(err)
		}
		for _, f := range facts {
			if changed, ok := note.Replace(f.Text, from, to); ok {
				id, old, text = f.ID, f.Text, changed
				break
			}
		}
		if id == "" {
			return i18n.T(lineMemoryNone)
		}
	}
	// what memory would refuse is refused before any card shows it
	if sensitive.Note(text) {
		return i18n.T(lineNoteRefused)
	}
	return d.memoryCardOut(&memoryCard{edit: id, text: text}, i18n.Tf("Change “%s” to “%s”?", oniText(old, 150), oniText(text, 150)))
}

// memoryCardOut shows card with question, in place of any card still
// waiting, and returns the line that points at it.
func (d *Daemon) memoryCardOut(card *memoryCard, question string) string {
	svc := d.oni.service.Load()
	if svc == nil {
		return i18n.T(lineMemoryNoCard)
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return i18n.T(lineReflexFailed)
	}
	card.id = feature.ForgetCardPrefix + hex.EncodeToString(b[:])
	m := &d.oni.memory
	m.mu.Lock()
	d.memoryCardCloseLocked(false)
	card.timer = d.reflexClock().AfterFunc(feature.ConsentTTL, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.card == card {
			d.memoryCardCloseLocked(false)
		}
	})
	m.card = card
	m.mu.Unlock()
	if err := svc.Consent(*feature.ForgetCard(card.id, oniText(question, 400))); err != nil {
		m.mu.Lock()
		if m.card == card {
			d.memoryCardCloseLocked(false)
		}
		m.mu.Unlock()
		return i18n.T(lineMemoryNoCard)
	}
	return i18n.T(lineMemoryCard)
}

// memoryCardCloseLocked drops the card and says how it was answered on the
// frames. m.mu is held.
func (d *Daemon) memoryCardCloseLocked(yes bool) {
	m := &d.oni.memory
	card := m.card
	if card == nil {
		return
	}
	m.card = nil
	card.timer.Stop()
	answer := "no"
	if yes {
		answer = "yes"
	}
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.ConsentAnswer(feature.ConsentAnswerFrame{ID: card.id, Answer: answer})
	}
}

// memoryCardWaiting is the id of the memory card waiting, or "".
func (d *Daemon) memoryCardWaiting() string {
	m := &d.oni.memory
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.card == nil {
		return ""
	}
	return m.card.id
}

// memoryCardTake answers the card id names: yes forgets or changes what it
// showed, through memory.Handle as the user; no changes nothing. The line
// saying how it went fills the bubble.
func (d *Daemon) memoryCardTake(id string, allow bool) (protocol.OniAskResult, error) {
	m := &d.oni.memory
	m.mu.Lock()
	card := m.card
	if card == nil || card.id != id {
		m.mu.Unlock()
		return protocol.OniAskResult{}, oniOfferGone()
	}
	d.memoryCardCloseLocked(allow)
	m.mu.Unlock()
	if !allow {
		return protocol.OniAskResult{Status: protocol.OniAskDeclined}, nil
	}
	line := d.memoryCardDo(card)
	if svc := d.oni.service.Load(); svc != nil {
		_ = svc.Bubble(0, "answer", oniText(line, oniBubbleMax))
	}
	return protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &protocol.OniAnswer{Text: line, Destination: "local"}}, nil
}

// memoryCardDo carries out a card said yes to and returns its line.
func (d *Daemon) memoryCardDo(card *memoryCard) string {
	if !d.voiceMemoryOn() {
		return i18n.T(lineNoteNotesOff)
	}
	if d.deps.Memory == nil {
		return i18n.T(lineNoteOff)
	}
	ctx, cancel := context.WithTimeout(context.Background(), oniAskTimeout)
	defer cancel()
	if card.edit != "" {
		text := card.text
		if err := d.memoryAsUser(ctx, protocol.MMemoryEdit, protocol.MemoryEditParams{ID: card.edit, Text: &text}, nil); err != nil {
			return memoryErrLine(err)
		}
		m := &d.oni.memory
		m.mu.Lock()
		if m.last != nil && m.last.id == card.edit {
			m.last.text = text
		}
		m.mu.Unlock()
		return i18n.T(lineMemoryChanged)
	}
	if err := d.memoryAsUser(ctx, protocol.MMemoryForget, protocol.MemoryForgetParams{IDs: card.ids}, nil); err != nil {
		return memoryErrLine(err)
	}
	m := &d.oni.memory
	m.mu.Lock()
	if m.last != nil && len(card.ids) > 0 && m.last.id == card.ids[0] {
		m.last = nil
	}
	m.mu.Unlock()
	return i18n.T(lineMemoryForgot)
}

// memoryCardAnswer serves oni.consent.answer for the memory card: only the
// app, which shows the facts, answers it; the talk key answers it by voice.
// It reports false when id is not the card.
func (d *Daemon) memoryCardAnswer(c *connection, p protocol.OniConsentAnswerParams) (protocol.OniAskResult, bool, error) {
	if !strings.HasPrefix(p.ID, feature.ForgetCardPrefix) {
		return protocol.OniAskResult{}, false, nil
	}
	if !c.app {
		return protocol.OniAskResult{}, true, protocol.Errorf(protocol.CodeForbidden, "%s", i18n.T("Only the Hyakki app, or the talk key, answers this card."))
	}
	res, err := d.memoryCardTake(p.ID, p.Answer == "yes")
	return res, true, err
}

// memorySettings follows the settings: with voice memory off, the card goes
// unanswered, nothing changes, and the note kept for correcting is let go.
func (d *Daemon) memorySettings(next protocol.Settings) {
	if feature.Decide(feature.Request{Feature: feature.MemoryForget}, protocol.EffectiveOni(next)).Allow {
		return
	}
	m := &d.oni.memory
	m.mu.Lock()
	defer m.mu.Unlock()
	d.memoryCardCloseLocked(false)
	m.last = nil
}
