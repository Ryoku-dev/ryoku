package daemon

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/reflex/decide"
	"ryoku-rashin/internal/oni/routine"
	"ryoku-rashin/internal/oni/voice/route"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/reflex/act"
)

// fakeExec records the actions it is given; block, when set, holds each
// action until it is closed, after telling started.
type fakeExec struct {
	mu      sync.Mutex
	actions []reflex.Action
	block   chan struct{}
	started chan struct{}
}

func (f *fakeExec) Caps() map[reflex.Intent]bool {
	caps := map[reflex.Intent]bool{}
	for _, in := range reflex.Registry() {
		caps[in.ID] = true
	}
	caps[reflex.Minimize] = false
	return caps
}

func (f *fakeExec) Do(_ context.Context, a reflex.Action) (act.Outcome, error) {
	f.mu.Lock()
	f.actions = append(f.actions, a)
	block, started := f.block, f.started
	f.mu.Unlock()
	if block != nil {
		started <- struct{}{}
		<-block
	}
	switch a.Intent {
	case reflex.OpenApp:
		return act.Outcome{Done: true, Line: "Opening " + a.Arg + "."}, nil
	case reflex.Mute:
		return act.Outcome{Done: true, Line: act.LineMuted}, nil
	case reflex.VolumeUp:
		return act.Outcome{Done: true, Line: act.LineVolumeUp}, nil
	case reflex.Minimize:
		return act.Outcome{Line: act.LineCantHere}, act.ErrUnsupported
	}
	return act.Outcome{Done: true, Line: "Done."}, nil
}

func (f *fakeExec) calls() []reflex.Action {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.actions)
}

// fakeCatalog resolves only exact names, ignoring case.
type fakeCatalog struct{ names []string }

func (c fakeCatalog) Resolve(spoken string) (string, string, float64, bool) {
	for _, n := range c.names {
		if strings.EqualFold(strings.TrimSpace(spoken), n) {
			return n, strings.ToLower(n), 1, true
		}
	}
	return "", "", 0, false
}

// fakeDecider answers every decision with answer and records what it was
// asked.
type fakeDecider struct {
	mu     sync.Mutex
	asked  []protocol.FastDecisionParams
	answer func(protocol.FastDecisionParams) protocol.FastDecisionResult
}

func (f *fakeDecider) Decide(_ context.Context, p protocol.FastDecisionParams) (protocol.FastDecisionResult, error) {
	f.mu.Lock()
	f.asked = append(f.asked, p)
	answer := f.answer
	f.mu.Unlock()
	if answer == nil {
		return protocol.FastDecisionResult{Route: protocol.DecisionRules}, nil
	}
	return answer(p), nil
}

func (f *fakeDecider) calls() []protocol.FastDecisionParams {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.asked)
}

// decided is a resolved answer naming intent from route.
func decided(route string, intent reflex.Intent, p float64) protocol.FastDecisionResult {
	probabilities := map[string]float64{}
	for _, l := range decide.Labels() {
		probabilities[l] = 0
	}
	probabilities[string(intent)] = p
	probabilities[decide.None] += 1 - p
	return protocol.FastDecisionResult{Route: route, Resolved: true, Decision: &protocol.DecisionResponse{
		Model: "fixture", Answers: map[string]protocol.DecisionAnswer{decide.QuestionID: {Type: "choice", Choice: string(intent), Probabilities: probabilities}},
	}}
}

// noteMemory records memory calls and answers them with err.
type noteMemory struct {
	mu    sync.Mutex
	calls []string
	from  []core.MemoryCaller
	err   error
}

func (m *noteMemory) Bind(core.MemorySessions) {}
func (m *noteMemory) Handle(_ context.Context, from core.MemoryCaller, method string, raw json.RawMessage) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, method+" "+string(raw))
	m.from = append(m.from, from)
	return protocol.MemoryChange{}, m.err
}
func (m *noteMemory) Status(context.Context) protocol.MemoryStatus { return protocol.MemoryStatus{} }
func (m *noteMemory) Recall(context.Context, protocol.SessionInfo, string) core.Recall {
	return core.Recall{}
}
func (m *noteMemory) Brief(protocol.SessionInfo) string { return "" }
func (m *noteMemory) TeamBrief(protocol.Team) string    { return "" }
func (m *noteMemory) TurnEnded(protocol.SessionInfo)    {}
func (m *noteMemory) SessionDeleted(string)             {}
func (m *noteMemory) Settle()                           {}
func (m *noteMemory) Busy() bool                        { return false }
func (m *noteMemory) Release()                          {}
func (m *noteMemory) Close() error                      { return nil }

func (m *noteMemory) seen() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.calls)
}

// switchAccount is a signed-in account whose id can change.
type switchAccount struct {
	fakeAccount
	mu       sync.Mutex
	id       string
	onChange func()
}

func (a *switchAccount) Status() protocol.AccountState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.id == "" {
		return protocol.AccountState{State: "signedOut"}
	}
	return protocol.AccountState{State: "signedIn", Account: &protocol.AccountIdentity{ID: a.id, Display: "Kit"}}
}

func (a *switchAccount) OnChange(f func()) { a.onChange = f }

func (a *switchAccount) become(id string) {
	a.mu.Lock()
	a.id = id
	a.mu.Unlock()
	a.onChange()
}

type reflexRig struct {
	*voiceRig
	exec    *fakeExec
	decider *fakeDecider
	memory  *noteMemory
	account *switchAccount
	mu      sync.Mutex
	notes   []string
}

// serveReflex is serveVoice with reflex on, a fake executor, catalog,
// decision route, memory and account, edit applied to its settings.
func serveReflex(t *testing.T, edit func(*protocol.Settings)) *reflexRig {
	t.Helper()
	rr := &reflexRig{exec: &fakeExec{}, decider: &fakeDecider{}, memory: &noteMemory{}, account: &switchAccount{id: "ra_1"}}
	rr.voiceRig = serveVoice(t, func(s *protocol.Settings) {
		s.Oni.Reflex = true
		if edit != nil {
			edit(s)
		}
	}, nil, func(d *Deps) {
		d.FastDecisions = rr.decider
		d.Memory = rr.memory
		d.Account = rr.account
		d.OniReflex = OniReflexDeps{
			Exec:      rr.exec,
			Catalog:   fakeCatalog{names: []string{"Firefox", "Notepad", "Files"}},
			Routines:  routine.NewStore(t.TempDir() + "/routines.json"),
			Notify:    rr.notify,
			Wake:      func(bool) error { return nil },
			Clock:     d.OniVoice.Clock,
			Reminders: nil,
		}
	})
	return rr
}

func (r *reflexRig) notify(summary, body string) error {
	r.mu.Lock()
	r.notes = append(r.notes, body)
	r.mu.Unlock()
	return nil
}

func (r *reflexRig) notices() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.notes)
}

// utter routes text as if src had just said it, through the real router.
func (r *reflexRig) utter(t *testing.T, src route.Source, text string) {
	t.Helper()
	d := r.d
	v := &d.oni.voice
	v.mu.Lock()
	if err := d.voiceEnsureLocked(protocol.EffectiveOni(d.Settings())); err != nil {
		v.mu.Unlock()
		t.Fatal(err)
	}
	id := v.turn.Barge()
	v.mu.Unlock()
	d.voiceRoute(route.Utterance{Text: text, Source: src, Turn: id, At: r.clock.Now()}, voiceSourceOf(src))
}

// settle waits until the voice loop has nothing on its way.
func (r *reflexRig) settle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		v := &r.d.oni.voice
		v.mu.Lock()
		work := v.work
		v.mu.Unlock()
		if work == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the voice loop never settled")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func reflexLog(t *testing.T, p *socketPeer) (protocol.OniReflexLogResult, string) {
	t.Helper()
	var raw json.RawMessage
	if err := p.call(protocol.MOniReflexLog, protocol.Empty{}, &raw); err != nil {
		t.Fatalf("oni.reflex.log: %v", err)
	}
	var res protocol.OniReflexLogResult
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	return res, string(raw)
}

// A pressed command runs its plan through the executor, says each action
// in a short line, writes one ledger row per action with no words in it,
// and asks no model.
func TestOniReflexPressRunsThePlanAnnouncesAndLogs(t *testing.T) {
	r := serveReflex(t, nil)
	surface, _ := r.presenting(t)
	cli := dialSocket(t, protocol.ClientCLI)
	var log oniLog
	r.ears.hear(nil, "open firefox and mute")
	talk(t, surface, protocol.OniVoiceStart)
	feed := r.mic.next(t)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceListening, ""))
	speak(feed)
	pause(feed)
	time.Sleep(50 * time.Millisecond)
	talk(t, surface, protocol.OniVoiceStop)
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, "Opening Firefox."))
	voiceUntil(t, surface, &log, voiceIs(feature.VoiceSpeaking, act.LineMuted))
	r.settle(t)

	want := []reflex.Action{{Intent: reflex.OpenApp, Arg: "Firefox"}, {Intent: reflex.Mute, Arg: reflex.MuteOn}}
	got := r.exec.calls()
	if len(got) != 2 || got[0].Intent != want[0].Intent || got[0].Arg != want[0].Arg || got[1].Intent != want[1].Intent || got[1].Arg != want[1].Arg {
		t.Fatalf("executed %+v", got)
	}
	if q := r.questions(); len(q) != 0 {
		t.Fatalf("a model was asked %q", q)
	}
	res, raw := reflexLog(t, cli)
	if len(res.Rows) != 2 {
		t.Fatalf("ledger %s", raw)
	}
	for i, row := range res.Rows {
		if w := want[len(want)-1-i]; row.Intent != string(w.Intent) || row.Route != protocol.DecisionRules || row.Outcome != protocol.OniReflexDone || row.Confidence <= 0 || row.Ms < 0 {
			t.Errorf("row %d = %+v", i, row)
		}
	}
	if strings.Contains(strings.ToLower(raw), "firefox") || strings.Contains(raw, "mute and") {
		t.Fatalf("the ledger holds words: %s", raw)
	}
	for _, v := range log.voices {
		if v.Phase == feature.VoiceSpeaking && !route.Fits(v.Text) {
			t.Errorf("%q breaks the voice discipline", v.Text)
		}
	}
	// an unsupported verb is said honestly and logged as such
	r.utter(t, route.Chord, "minimise this window")
	r.settle(t)
	res, _ = reflexLog(t, cli)
	if newest := res.Rows[0]; newest.Intent != string(reflex.Minimize) || newest.Outcome != protocol.OniReflexUnsupported {
		t.Fatalf("minimize row %+v", newest)
	}
}

// With reflex off nothing is built and every command is an ordinary ask.
func TestOniReflexOffBuildsNothing(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) { s.Oni.Reflex = false })
	cli := dialSocket(t, protocol.ClientCLI)
	r.utter(t, route.Chord, "open firefox")
	r.settle(t)
	if calls := r.exec.calls(); len(calls) != 0 {
		t.Fatalf("executed %+v with reflex off", calls)
	}
	if q := r.questions(); len(q) != 1 {
		t.Fatalf("the slow path was asked %d times", len(q))
	}
	rf := &r.d.oni.reflex
	rf.mu.Lock()
	built, exec, cat := rf.built, rf.exec, rf.cat
	rf.mu.Unlock()
	if built || exec != nil || cat != nil {
		t.Fatal("reflex off built its executors")
	}
	if code, reason := reasonOf(cli.call(protocol.MOniReflexLog, protocol.Empty{}, nil)); code != protocol.CodeDisabled || reason != feature.ReasonFeatureOff {
		t.Fatalf("oni.reflex.log with reflex off: %s %s", code, reason)
	}
	if len(r.decider.calls()) != 0 {
		t.Fatal("a decision was asked with reflex off")
	}
}

// While Oni is hidden every action is also posted as a notification.
func TestOniReflexHiddenAnnouncesThroughNotify(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) { s.Oni.Hidden = true })
	r.utter(t, route.Chord, "turn it up")
	r.settle(t)
	if n := r.notices(); !slices.Equal(n, []string{act.LineVolumeUp}) {
		t.Fatalf("notified %q", n)
	}
	// shown, nothing is notified
	r.patch(t, `{"oni":{"hidden":false}}`)
	r.presenting(t)
	r.utter(t, route.Chord, "mute")
	r.settle(t)
	if n := r.notices(); len(n) != 1 {
		t.Fatalf("notified while shown: %q", n)
	}
}

// An action that finishes after voice was turned off says nothing, and the
// plan's remaining actions never run.
func TestOniReflexLateRunAfterVoiceOffDoesNothing(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) { s.Oni.Hidden = true })
	r.exec.block, r.exec.started = make(chan struct{}), make(chan struct{}, 4)
	r.utter(t, route.Chord, "open firefox and mute")
	select {
	case <-r.exec.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the first action never started")
	}
	r.patch(t, `{"oni":{"voice":false}}`)
	close(r.exec.block)
	r.settle(t)
	if calls := r.exec.calls(); len(calls) != 1 {
		t.Fatalf("ran %+v after voice went off", calls)
	}
	if n := r.notices(); len(n) != 0 {
		t.Fatalf("a late line was said: %q", n)
	}
}

// Wake speech never starts a note, a routine or a settings flip, and voice
// can only narrow a setting: turning anything on is refused.
func TestOniReflexWakeNeverRunsPressOnlyAndVoiceNeverWidens(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) {
		s.Oni.Hidden = true
		s.Oni.Sounds = true
		s.Oni.ReflexSettings.Notes = true
	})
	before := r.d.Settings().Oni
	for _, say := range []string{"remember that the exam is on friday", "turn off your sounds", "run my routine"} {
		r.utter(t, route.Wake, say)
		r.settle(t)
	}
	if calls := r.memory.seen(); len(calls) != 0 {
		t.Fatalf("wake reached memory: %q", calls)
	}
	if n := r.notices(); len(n) != 0 {
		t.Fatalf("wake ran an action: %q", n)
	}
	for _, say := range []string{"turn on open mic", "enable hands", "start listening"} {
		r.utter(t, route.Chord, say)
		r.settle(t)
	}
	after := r.d.Settings().Oni
	if after.Sounds != before.Sounds || after.VoiceSettings.OpenMic || after.VoiceSettings.Wake.Enabled || after.Hands != before.Hands {
		t.Fatalf("voice changed settings: %+v", after)
	}
	if calls := r.exec.calls(); len(calls) != 0 {
		t.Fatalf("executed %+v", calls)
	}
	// a press narrows
	r.utter(t, route.Chord, "turn off your sounds")
	r.settle(t)
	if r.d.Settings().Oni.Sounds {
		t.Fatal("a pressed flip did not turn the sounds off")
	}
	if n := r.notices(); !slices.Contains(n, "Sounds off.") {
		t.Fatalf("the flip was not announced: %q", n)
	}
}

// A pressed note goes to memory.add as the user; memory off says so; a
// password or a card number is refused before memory sees it.
func TestOniReflexNotesGoToMemoryOnly(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) {
		s.Oni.Hidden = true
		s.Oni.ReflexSettings.Notes = true
	})
	r.utter(t, route.Chord, "remember that the exam is on friday")
	r.settle(t)
	calls := r.memory.seen()
	if len(calls) != 1 || !strings.HasPrefix(calls[0], protocol.MMemoryAdd+" ") || !strings.Contains(calls[0], `"text":"the exam is on friday"`) || !strings.Contains(calls[0], `"scope":"you"`) {
		t.Fatalf("memory calls %q", calls)
	}
	if from := r.memory.from[0]; from != (core.MemoryCaller{}) {
		t.Fatalf("the note was written as %+v", from)
	}
	for _, say := range []string{"remember that my password is hunter2", "remember that the visa reads 4111 1111 1111 1111"} {
		r.utter(t, route.Chord, say)
		r.settle(t)
	}
	if calls := r.memory.seen(); len(calls) != 1 {
		t.Fatalf("a secret reached memory: %q", calls)
	}
	r.memory.mu.Lock()
	r.memory.err = protocol.Errorf(protocol.CodeDisabled, "Memory is off.")
	r.memory.mu.Unlock()
	r.utter(t, route.Chord, "remember that the bins go out on tuesday")
	r.settle(t)
	n := r.notices()
	want := []string{lineNoteKept, lineNoteRefused, lineNoteRefused, lineNoteOff}
	if !slices.Equal(n, want) {
		t.Fatalf("said %q, want %q", n, want)
	}
	// notes off: memory is never asked
	r.patch(t, `{"oni":{"reflexSettings":{"notes":false}}}`)
	r.utter(t, route.Chord, "remember that the shop shuts at six")
	r.settle(t)
	if calls := r.memory.seen(); len(calls) != 2 {
		t.Fatalf("notes off reached memory: %q", calls)
	}
}

// Oni moves, goes home and changes size by voice.
func TestOniReflexSelfCommands(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) { s.Oni.Size = protocol.OniSizeNormal })
	r.utter(t, route.Wake, "hop over to the upper left corner")
	r.settle(t)
	if p, err := presence.ReadPlace(presence.PlacementFile()); err != nil || p.Corner != protocol.OniCornerTopLeft {
		t.Fatalf("place %+v, %v", p, err)
	}
	r.utter(t, route.Chord, "go home")
	r.settle(t)
	if p, err := presence.ReadPlace(presence.PlacementFile()); err != nil || p.Corner != presence.DefaultCorner {
		t.Fatalf("place %+v, %v", p, err)
	}
	r.utter(t, route.Chord, "make yourself smaller")
	r.settle(t)
	if size := r.d.Settings().Oni.Size; size != protocol.OniSizeSmall {
		t.Fatalf("size %q", size)
	}
}

// The local decision route classifies what the rules miss with no account
// and no press; its answer runs through the same executor and caps.
func TestOniReflexLocalDecisionNeedsNoAccount(t *testing.T) {
	const say = "i need the notepad thing now"
	if _, ok := reflex.Parse(say, time.Time{}, nil, fakeCatalog{names: []string{"Notepad"}}); ok {
		t.Fatalf("the rules claim %q; pick a phrase they miss", say)
	}
	r := serveReflex(t, func(s *protocol.Settings) {
		s.Oni.ReflexSettings.Decisions = true
		s.Providers.Decisions.Route = protocol.DecisionLocal
		s.Providers.Decisions.LocalModel = "fixture"
	})
	r.decider.answer = func(protocol.FastDecisionParams) protocol.FastDecisionResult {
		return decided(protocol.DecisionLocal, reflex.OpenApp, 0.9)
	}
	cli := dialSocket(t, protocol.ClientCLI)
	r.utter(t, route.Wake, say)
	r.settle(t)
	calls := r.exec.calls()
	if len(calls) != 1 || calls[0].Intent != reflex.OpenApp || calls[0].Arg != "Notepad" {
		t.Fatalf("executed %+v", calls)
	}
	asked := r.decider.calls()
	if len(asked) != 1 || asked[0].AccountID != "" || asked[0].Feature != decide.Feature {
		t.Fatalf("asked %+v", asked)
	}
	var said string
	if err := json.Unmarshal(asked[0].State, &said); err != nil || said != say {
		t.Fatalf("the request state is %s", asked[0].State)
	}
	res, _ := reflexLog(t, cli)
	if len(res.Rows) != 1 || res.Rows[0].Route != protocol.DecisionLocal || res.Rows[0].Confidence != 0.9 {
		t.Fatalf("ledger %+v", res.Rows)
	}
	if q := r.questions(); len(q) != 0 {
		t.Fatalf("the slow path was asked %q", q)
	}
	// a low-confidence answer, or the route down, falls to the slow path
	r.decider.answer = func(protocol.FastDecisionParams) protocol.FastDecisionResult {
		return decided(protocol.DecisionLocal, reflex.OpenApp, 0.5)
	}
	r.utter(t, route.Chord, "i need the files thing now")
	r.settle(t)
	if len(r.exec.calls()) != 1 || len(r.questions()) != 1 {
		t.Fatalf("a weak answer acted: %+v, asked %q", r.exec.calls(), r.questions())
	}
}

// The hosted route is reached only from a press, once a press answered the
// disclosure card for the account signed in; at most hostedPerMin a
// minute; and an account change drops the answer.
func TestOniReflexHostedDecisionNeedsPressCardAndAccount(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) {
		s.Oni.ReflexSettings.Decisions = true
		s.Providers.Decisions.Route = protocol.DecisionEmber
	})
	surface, _ := r.presenting(t)
	accounts := func() []string {
		var out []string
		for _, p := range r.decider.calls() {
			out = append(out, p.AccountID)
		}
		return out
	}
	r.utter(t, route.Chord, "could you sort the thing out")
	r.settle(t)
	card := awaitDecisionCard(t, surface)
	if card.Feature != feature.Decisions || card.Class != feature.ConsentDisclosure || card.Destination != "hosted" || !strings.Contains(card.Question, "Kit") {
		t.Fatalf("card %+v", card)
	}
	// only the app answers it
	cli := dialSocket(t, protocol.ClientCLI)
	if err := cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, nil); !forbidden(err) {
		t.Fatalf("the CLI answered the card: %v", err)
	}
	r.utter(t, route.Wake, "could you sort the thing out")
	r.settle(t)
	r.app.Store(true)
	app := dialSocket(t, protocol.ClientApp)
	if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "yes"}, nil); err != nil {
		t.Fatalf("the app's yes: %v", err)
	}
	if got := accounts(); !slices.Equal(got, []string{"", ""}) {
		t.Fatalf("an account rode before the card: %q", got)
	}
	lim := reflexLimits().Decisions.HostedPerMin
	for range lim + 2 {
		r.utter(t, route.Chord, "could you sort the thing out")
		r.settle(t)
	}
	r.utter(t, route.Wake, "could you sort the thing out")
	r.settle(t)
	got := accounts()[2:]
	hosted := 0
	for _, a := range got {
		if a == "ra_1" {
			hosted++
		} else if a != "" {
			t.Fatalf("account %q", a)
		}
	}
	if hosted != lim || got[len(got)-1] != "" {
		t.Fatalf("hosted %d times (cap %d): %q", hosted, lim, got)
	}
	// a new account drops the answer
	r.clock.mu.Lock()
	r.clock.now = r.clock.now.Add(2 * time.Minute)
	r.clock.mu.Unlock()
	r.account.become("ra_2")
	n := len(r.decider.calls())
	r.utter(t, route.Chord, "could you sort the thing out")
	r.settle(t)
	if after := r.decider.calls(); len(after) != n+1 || after[n].AccountID != "" {
		t.Fatalf("after the account changed: %+v", after[n:])
	}
	if next := awaitDecisionCard(t, surface); next.ID == card.ID {
		t.Fatal("no new card for the new account")
	}
}

// Running out of credits says so once and pins later decisions to the
// local route with no account.
func TestOniReflexCreditExhaustionDegradesOnce(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) {
		s.Oni.Hidden = true
		s.Oni.ReflexSettings.Decisions = true
		s.Providers.Decisions.Route = protocol.DecisionEmber
	})
	r.decider.answer = func(protocol.FastDecisionParams) protocol.FastDecisionResult {
		return protocol.FastDecisionResult{Route: protocol.DecisionRules, CreditsExhausted: true}
	}
	for range 2 {
		r.utter(t, route.Chord, "could you sort the thing out")
		r.settle(t)
	}
	if n := r.notices(); !slices.Equal(n, []string{"Credits ran out; rules only."}) {
		t.Fatalf("said %q", n)
	}
	asked := r.decider.calls()
	if len(asked) != 2 || asked[1].Route != protocol.DecisionLocal || asked[1].AccountID != "" {
		t.Fatalf("asked %+v", asked)
	}
	if len(r.exec.calls()) != 0 || len(r.questions()) != 2 {
		t.Fatalf("executed %+v, asked %q", r.exec.calls(), r.questions())
	}
}

// awaitDecisionCard reads p's oni frames until a decision card arrives.
func awaitDecisionCard(t *testing.T, p *socketPeer) feature.ConsentFrame {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-p.conn.Events():
			if !ok {
				t.Fatal("the connection closed")
			}
			if e.Event != protocol.EvtOni {
				continue
			}
			f, err := feature.DecodeFrame(e.Data)
			if err != nil {
				t.Fatalf("a frame the contract refuses: %v", err)
			}
			if c, ok := f.(*feature.ConsentFrame); ok && c.Feature == feature.Decisions {
				return *c
			}
		case <-timeout:
			t.Fatal("no decision card came")
		}
	}
}

// Launched apps keep the session bus and the display, and lose the
// daemon's own markers.
func TestReflexEnvKeepsTheSession(t *testing.T) {
	env := reflexEnv([]string{"PATH=/usr/bin", "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus", "WAYLAND_DISPLAY=wayland-1", "HYAKKI_DAEMON=1", "NOTIFY_SOCKET=/run/x", "INVOCATION_ID=abc", "HOME=/home/u"})
	want := []string{"PATH=/usr/bin", "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus", "WAYLAND_DISPLAY=wayland-1", "HOME=/home/u"}
	if !slices.Equal(env, want) {
		t.Fatalf("env %q", env)
	}
}

// Every line the daemon's own actions say keeps a first sentence of at
// most five words.
func TestOniReflexLinesKeepTheVoiceDiscipline(t *testing.T) {
	lines := []string{lineReflexEnough, lineReflexPress, lineReflexCant, lineReflexMoved, lineReflexHome, lineReflexBigger,
		lineReflexSmaller, lineReflexBiggest, lineReflexSmallest, lineReflexFailed, lineNoteKept, lineNoteOff, lineNoteNotesOff,
		lineNoteRefused, lineNoteWontKeep}
	for _, f := range reflexFlips {
		lines = append(lines, f.line)
	}
	for _, l := range lines {
		if route.LeadWords(l) > route.MaxLeadWords || len(route.Sentences(l)) == 0 {
			t.Errorf("%q breaks the voice discipline", l)
		}
	}
}

// A no to the decision card spends nothing and the card is not shown for
// that account again.
func TestOniReflexDeclinedCardIsNotOfferedAgain(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) {
		s.Oni.ReflexSettings.Decisions = true
		s.Providers.Decisions.Route = protocol.DecisionAuto
		s.Providers.Decisions.AllowHosted = true
	})
	surface, _ := r.presenting(t)
	r.utter(t, route.Chord, "could you sort the thing out")
	r.settle(t)
	card := awaitDecisionCard(t, surface)
	r.app.Store(true)
	app := dialSocket(t, protocol.ClientApp)
	var res protocol.OniAskResult
	if err := app.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: "no"}, &res); err != nil || res.Status != protocol.OniAskDeclined {
		t.Fatalf("no = %+v, %v", res, err)
	}
	r.utter(t, route.Chord, "could you sort the thing out")
	r.settle(t)
	if id := r.d.reflexCardWaiting(); id != "" {
		t.Fatalf("the card came back: %s", id)
	}
	for _, p := range r.decider.calls() {
		if p.AccountID != "" {
			t.Fatalf("an account rode after a no: %+v", p)
		}
	}
	// without allowHosted on auto no card is shown at all
	r.patch(t, `{"providers":{"decisions":{"allowHosted":false}}}`)
	r.account.become("ra_3")
	r.utter(t, route.Chord, "could you sort the thing out")
	r.settle(t)
	if id := r.d.reflexCardWaiting(); id != "" {
		t.Fatalf("a card on auto without allowHosted: %s", id)
	}
}

// A command wake speech may not start leaves no mark on the session: the
// same words pressed straight after still run.
func TestOniReflexRefusedWakeCommandDoesNotCoolThePress(t *testing.T) {
	r := serveReflex(t, func(s *protocol.Settings) { s.Oni.ReflexSettings.Notes = true })
	r.utter(t, route.Wake, "remember that the exam is on friday")
	r.settle(t)
	r.utter(t, route.Chord, "remember that the exam is on friday")
	r.settle(t)
	if calls := r.memory.seen(); len(calls) != 1 {
		t.Fatalf("memory calls %q", calls)
	}
}

// oni.reflex.log serves the ledger newest first, as the wire contract says.
func TestOniReflexLogIsNewestFirst(t *testing.T) {
	r := serveReflex(t, nil)
	cli := dialSocket(t, protocol.ClientCLI)
	r.utter(t, route.Chord, "open firefox and mute and turn it up")
	r.settle(t)
	if calls := r.exec.calls(); len(calls) != 3 {
		t.Fatalf("executed %+v", calls)
	}
	res, raw := reflexLog(t, cli)
	var got []string
	for _, row := range res.Rows {
		got = append(got, row.Intent)
	}
	want := []string{string(reflex.VolumeUp), string(reflex.Mute), string(reflex.OpenApp)}
	if !slices.Equal(got, want) {
		t.Fatalf("ledger order %q, want newest first %q: %s", got, want, raw)
	}
}
