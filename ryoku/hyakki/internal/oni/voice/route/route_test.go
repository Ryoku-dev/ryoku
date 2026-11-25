package route

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/characters"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/voice/turn"
	"ryoku-rashin/internal/protocol"
)

// t0 is a Sunday afternoon.
var t0 = time.Date(2026, 11, 22, 15, 45, 0, 0, time.UTC)

func newTurn(t *testing.T) *turn.Turn {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return turn.New(sp, nil)
}

func newRouter(t *testing.T, c Config) (*Router, *turn.Turn) {
	t.Helper()
	if c.Turn == nil {
		c.Turn = newTurn(t)
	}
	if c.Location == nil {
		c.Location = time.UTC
	}
	r, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	return r, c.Turn
}

func TestRouteOrder(t *testing.T) {
	r, tn := newRouter(t, Config{Name: "Oni", Lines: Lines{Ack: []string{"On it."}}})
	id := tn.Barge()
	at := func(d time.Duration) time.Time { return t0.Add(d) }
	say := func(text string, when time.Time) Decision {
		return r.Route(Utterance{Text: text, Source: Chord, Turn: id, At: when})
	}

	// Oni's own line heard back is dropped before anything else, even when
	// it reads like a card answer; the user's own question in the same
	// window is answered.
	tn.Said("It's 3:45.", t0)
	if d := say("its 3:45", at(200*time.Millisecond)); d.Kind != Dropped || d.Why != WhyHeardBack {
		t.Fatalf("heard back: %+v", d)
	}
	if d := say("What time is it?", at(300*time.Millisecond)); d.Kind != Answered || d.Card != CardTime || d.Line != "It's 3:45." {
		t.Fatalf("user question inside the echo window: %+v", d)
	}
	if d := r.Route(Utterance{Text: "what time is it", Source: Chord, Turn: id - 1, At: t0}); d.Why != WhyStale {
		t.Fatalf("older turn: %+v", d)
	}
	for _, empty := range []string{"", "   ", "[BLANK_AUDIO]", "..."} {
		if d := say(empty, at(time.Minute)); d.Kind != Dropped || d.Why != WhyEmpty {
			t.Fatalf("empty %q: %+v", empty, d)
		}
	}
	if d := r.Route(Utterance{Text: "what time is it", Turn: id, At: t0}); d.Why != WhySource {
		t.Fatalf("no source: %+v", d)
	}

	cases := []struct {
		text string
		when time.Time
		kind Kind
		card Card
		line string
	}{
		{"what time is it", at(time.Minute), Answered, CardTime, "It's 3:46."},
		{"Hey Oni, what's the time?", at(0), Answered, CardTime, "It's 3:45."},
		{"um, time please", at(0), Answered, CardTime, "It's 3:45."},
		{"what time is it", time.Date(2026, 11, 22, 0, 5, 0, 0, time.UTC), Answered, CardTime, "It's 12:05."},
		{"what time is it", time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC), Answered, CardTime, "It's 9:00."},
		{"What's today's date?", at(0), Answered, CardDate, "It's Sunday, November 22nd."},
		{"what day is it", time.Date(2026, 3, 21, 9, 0, 0, 0, time.UTC), Answered, CardDate, "It's Saturday, March 21st."},
		{"date", time.Date(2026, 3, 11, 9, 0, 0, 0, time.UTC), Answered, CardDate, "It's Wednesday, March 11th."},
		{"the date", time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC), Answered, CardDate, "It's Thursday, March 12th."},
		{"what is the date", time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC), Answered, CardDate, "It's Monday, March 23rd."},
		{"Set a timer for five minutes.", at(0), Answered, CardTimer, "Timer set, 5 minutes."},
		{"set a 1-minute timer", at(0), Answered, CardTimer, "Timer set, 1 minute."},
		{"start a timer for twenty five seconds", at(0), Answered, CardTimer, "Timer set, 25 seconds."},
		{"timer for an hour please", at(0), Answered, CardTimer, "Timer set, 1 hour."},
		{"set a timer for 99 hours", at(0), Answered, CardTimer, "Timers stop at 24 hours."},
		{"cancel the timer", at(0), Answered, CardRemindCancel, ""},
		{"what are my reminders", at(0), Answered, CardRemindList, ""},
		{"Never mind.", at(0), Dismissed, CardNone, ""},
		{"okay, never mind, thanks", at(0), Dismissed, CardNone, ""},
		{"stop", at(0), Dismissed, CardNone, ""},
		{"No thanks.", at(0), Dismissed, CardNone, ""},
		{"Oni, that's all.", at(0), Dismissed, CardNone, ""},
		{"Hello!", at(0), Answered, CardNone, "Hello."},
		{"hey there", at(0), Answered, CardNone, "Hi, I'm here."},
		{"Thank you, Oni.", at(0), Answered, CardNone, "Anytime."},
		{"how are you doing", at(0), Answered, CardNone, "All good here."},
		{"Who are you?", at(0), Answered, CardNone, "I'm Oni."},
	}
	for _, c := range cases {
		d := say(c.text, c.when)
		if d.Kind != c.kind || d.Card != c.card || d.Line != c.line || d.Ask != nil || d.Ack != "" {
			t.Errorf("Route(%q) = %+v, want %s %d %q", c.text, d, c.kind, c.card, c.line)
		}
		if set := c.card == CardTimer && !strings.HasPrefix(c.line, "Timers stop"); set != (d.Reminder != nil) {
			t.Errorf("Route(%q) keeps %+v", c.text, d.Reminder)
		}
	}
	timers := map[string]time.Duration{
		"set a timer for five minutes":          5 * time.Minute,
		"set a 1 minute timer":                  time.Minute,
		"start a timer for twenty five seconds": 25 * time.Second,
		"set a timer for an hour":               time.Hour,
		"set a timer for 90 seconds":            90 * time.Second,
	}
	for text, want := range timers {
		d := say(text, t0)
		if d.Reminder == nil || d.Reminder.Kind != remind.KindTimer || d.Reminder.Due.Sub(t0) != want || !d.Reminder.Created.Equal(t0) {
			t.Errorf("Route(%q) keeps %+v, want a timer of %s", text, d.Reminder, want)
		}
	}
	// a reminder keeps its own words, as said, and when it falls; one with
	// no time goes on to be asked
	if d := say("Remind me in 20 minutes to call Sam", t0); d.Card != CardReminder || d.Reminder == nil || d.Reminder.Text != "call Sam" || d.Reminder.Due.Sub(t0) != 20*time.Minute || d.Line == "" {
		t.Errorf("reminder: %+v %+v", d, d.Reminder)
	}
	if d := say("Oni, remind me in 20 minutes to call Sam", t0); d.Card != CardReminder || d.Reminder == nil || strings.Contains(strings.ToLower(d.Reminder.Text), "oni") {
		t.Errorf("a reminder said to Oni by name: %+v %+v", d, d.Reminder)
	}
	if d := say("cancel all my reminders", t0); d.Card != CardRemindCancel || d.Cancel != remind.CancelReminders {
		t.Errorf("cancel: %+v", d)
	}

	// What reads close to a local answer but is not one goes to the slow
	// path with the words as said.
	for _, text := range []string{
		"never mind the weather, what is it like",
		"set a timer for a while",
		"set a timer for 0 minutes",
		"remind me to call Sam",
		"what time is it in Tokyo",
		"what's the capital of Peru?",
		"hello can you summarise my last chat",
		"yes",
	} {
		d := say(text, t0)
		if text == "yes" {
			if d.Kind != Dropped || d.Why != WhyNoCard {
				t.Errorf("a yes with no card: %+v", d)
			}
			continue
		}
		if d.Kind != Asked || d.Ask == nil || d.Ask.Question != text {
			t.Errorf("Route(%q) = %+v, want the slow path with the text as said", text, d)
		}
	}
}

func TestSlowPathIsAReadOnlyVoiceAsk(t *testing.T) {
	r, tn := newRouter(t, Config{})
	id := tn.Barge()
	d := r.Route(Utterance{Text: "What is a tide?", Source: Toggle, Turn: id, At: t0})
	if d.Kind != Asked || d.Ask == nil {
		t.Fatalf("slow path: %+v", d)
	}
	a := *d.Ask
	if a.Question != "What is a tide?" || a.Source != Toggle || a.Turn != id || a.Long || a.Learn {
		t.Fatalf("ask: %+v", a)
	}
	got := a.Session()
	if got.Kind != protocol.KindAsk || got.Origin != "voice" || got.Mode != protocol.ApprovalsReadOnly || got.Memory != protocol.MemoryUseOnly ||
		got.Temporary || got.Model != "" || got.Approvals != "" || got.Project != "" || got.Team != "" || got.Options != nil {
		t.Fatalf("session = %+v, want a read-only voice Ask that uses memory without learning", got)
	}
	if got := a.Params("s1"); got.Question != a.Question || got.Session != "s1" || got.Origin != "voice" || got.Model != "" {
		t.Fatalf("params = %+v", got)
	}
	for text, long := range map[string]bool{
		"explain how tides work":          true,
		"tell me more about the moon":     true,
		"walk me through it step by step": true,
		"what is the whole story":         true,
		"how do tides work in detail":     true,
		"how do tides work":               false,
		"what is detail":                  false,
	} {
		if d := r.Route(Utterance{Text: text, Source: Chord, Turn: id, At: t0}); d.Ask == nil || d.Ask.Long != long {
			t.Errorf("Route(%q).Ask = %+v, want Long %v", text, d.Ask, long)
		}
	}
	learning, tl := newRouter(t, Config{Learn: true})
	d = learning.Route(Utterance{Text: "what is a tide", Source: Chord, Turn: tl.Barge(), At: t0})
	if d.Ask == nil || !d.Ask.Learn || d.Ask.Session().Memory != protocol.MemoryRemember {
		t.Fatalf("learning ask: %+v", d.Ask)
	}
}

func TestOneAckPerPress(t *testing.T) {
	r, tn := newRouter(t, Config{Lines: Lines{Ack: []string{"On it.", "Got it."}}})
	slow := func(id uint64, at time.Time) string {
		d := r.Route(Utterance{Text: "what is the tallest tree", Source: Chord, Turn: id, At: at})
		if d.Kind != Asked {
			t.Fatalf("not the slow path: %+v", d)
		}
		return d.Ack
	}
	id := tn.Barge()
	if got := slow(id, t0); got != "On it." {
		t.Fatalf("first ack = %q", got)
	}
	if got := slow(id, t0.Add(500*time.Millisecond)); got != "" {
		t.Fatalf("a second ack in one press: %q", got)
	}
	// A new press 100 ms after the last ack gets none; one 300 ms after
	// does.
	id = tn.Barge()
	if got := slow(id, t0.Add(100*time.Millisecond)); got != "" {
		t.Fatalf("an ack 100 ms after the last: %q", got)
	}
	id = tn.Barge()
	if got := slow(id, t0.Add(299*time.Millisecond)); got != "" {
		t.Fatalf("an ack 299 ms after the last: %q", got)
	}
	id = tn.Barge()
	if got := slow(id, t0.Add(300*time.Millisecond)); got != "Got it." {
		t.Fatalf("an ack 300 ms after the last = %q", got)
	}
	// Local answers never spend the ack.
	id = tn.Barge()
	if d := r.Route(Utterance{Text: "what time is it", Source: Chord, Turn: id, At: t0.Add(time.Second)}); d.Ack != "" {
		t.Fatalf("a card answer carried an ack: %+v", d)
	}
	if got := slow(id, t0.Add(time.Second)); got != "On it." {
		t.Fatalf("ack after a card answer = %q", got)
	}
	// A character with no lines gets no acks at all.
	quiet, tq := newRouter(t, Config{})
	if d := quiet.Route(Utterance{Text: "what is the tallest tree", Source: Chord, Turn: tq.Barge(), At: t0}); d.Ack != "" {
		t.Fatalf("ack with no lines: %q", d.Ack)
	}
}

type failingConsent struct{ t *testing.T }

func (f failingConsent) Pending() bool { return true }
func (f failingConsent) Answer(p Press, allow bool) error {
	f.t.Fatalf("consent answered from %s (valid %v)", p.Source(), p.Valid())
	return nil
}

type recordingConsent struct{ answers []Press }

func (r *recordingConsent) Pending() bool { return true }
func (r *recordingConsent) Answer(p Press, allow bool) error {
	if !allow {
		return errors.New("only allows are recorded")
	}
	r.answers = append(r.answers, p)
	return nil
}

var (
	consentWords = []string{"yes", "Yes please.", "allow it", "go ahead", "Hey Oni, yes.", "no", "deny", "don't"}
	handsWords   = []string{"click the send button", "type my password", "close this window", "press enter"}
)

// Wake and open-mic speech can never answer a consent card or reach Hands,
// whatever it says: the handlers below fail the test when called.
func TestWakeAndOpenNeverAnswerConsentOrReachHands(t *testing.T) {
	r, tn := newRouter(t, Config{
		Name:    "Oni",
		Consent: failingConsent{t},
		Hands: func(p Press, text string) bool {
			t.Fatalf("hands reached from %s with %q", p.Source(), text)
			return true
		},
	})
	for _, src := range []Source{Wake, Open} {
		id := tn.Barge()
		for _, text := range consentWords {
			d := r.Route(Utterance{Text: text, Source: src, Turn: id, At: t0})
			if d.Kind != NeedsPress || d.Line != NeedsPressLine || d.Allow {
				t.Errorf("%s %q = %+v, want NeedsPress", src, text, d)
			}
		}
		for _, text := range handsWords {
			d := r.Route(Utterance{Text: text, Source: src, Turn: id, At: t0})
			if d.Kind != Asked || d.Ask == nil || d.Ask.Session().Mode != protocol.ApprovalsReadOnly {
				t.Errorf("%s %q = %+v, want a read-only ask", src, text, d)
			}
		}
	}
	// A Press cannot be forged: the zero Press is never valid.
	if (Press{}).Valid() || (Press{source: Wake, turn: 1}).Valid() || (Press{source: Open, turn: 1}).Valid() {
		t.Fatal("a Press outside a pressed source is valid")
	}
}

// The same words from the chord or toggle do answer and do reach Hands,
// with a valid Press, so the branch above is the source and nothing else.
func TestPressedSpeechAnswersConsentAndReachesHands(t *testing.T) {
	consent := &recordingConsent{}
	var handed []string
	r, tn := newRouter(t, Config{
		Consent: consent,
		Hands: func(p Press, text string) bool {
			if !p.Valid() {
				t.Fatalf("hands given an invalid press for %q", text)
			}
			handed = append(handed, text)
			return true
		},
	})
	for _, src := range []Source{Chord, Toggle} {
		id := tn.Barge()
		d := r.Route(Utterance{Text: "Yes please.", Source: src, Turn: id, At: t0})
		if d.Kind != ConsentAnswered || !d.Allow {
			t.Fatalf("%s yes = %+v", src, d)
		}
		last := consent.answers[len(consent.answers)-1]
		if !last.Valid() || last.Source() != src || last.Turn() != id {
			t.Fatalf("%s press = %+v", src, last)
		}
		if d := r.Route(Utterance{Text: "click the send button", Source: src, Turn: id, At: t0}); d.Kind != HandsTook {
			t.Fatalf("%s hands = %+v", src, d)
		}
	}
	if len(consent.answers) != 2 || !slices.Equal(handed, []string{"click the send button", "click the send button"}) {
		t.Fatalf("answers %d, hands %q", len(consent.answers), handed)
	}
}

// In Phase 11 the Reflex and Hands slots are nil: everything they would
// take goes to the read-only slow path.
func TestNilSlotsFallToTheSlowPath(t *testing.T) {
	r, tn := newRouter(t, Config{})
	if r.c.Reflex != nil || r.c.Hands != nil || r.c.Consent != nil {
		t.Fatal("a zero Config filled a slot")
	}
	id := tn.Barge()
	for _, text := range handsWords {
		if d := r.Route(Utterance{Text: text, Source: Chord, Turn: id, At: t0}); d.Kind != Asked {
			t.Errorf("%q = %+v, want the slow path", text, d)
		}
	}
	if d := r.Route(Utterance{Text: "yes", Source: Chord, Turn: id, At: t0}); d.Why != WhyNoCard {
		t.Errorf("yes with no consent handler = %+v", d)
	}
	reflexed, tr := newRouter(t, Config{Reflex: func(u Utterance, _ Press) (Decision, bool) {
		return Decision{Line: "Paused."}, u.Text == "pause the music"
	}})
	id = tr.Barge()
	if d := reflexed.Route(Utterance{Text: "pause the music", Source: Wake, Turn: id, At: t0}); d.Kind != Reflexed || d.Line != "Paused." {
		t.Fatalf("reflex: %+v", d)
	}
	if _, err := New(Config{}); err == nil {
		t.Fatal("New with no turn was accepted")
	}
}

// Dropped decisions carry nothing of what was said.
func TestDroppedDecisionsHoldNoSpeech(t *testing.T) {
	r, tn := newRouter(t, Config{Consent: failingConsent{t}})
	id := tn.Barge()
	tn.Said("The purple giraffe naps.", t0)
	for _, u := range []Utterance{
		{Text: "purple giraffe naps", Source: Wake, Turn: id, At: t0},
		{Text: "purple giraffe naps", Source: Open, Turn: id + 7, At: t0},
		{Text: "purple giraffe naps", Turn: id, At: t0},
	} {
		d := r.Route(u)
		if d.Kind != Dropped {
			t.Fatalf("%+v not dropped: %+v", u, d)
		}
		if s := fmt.Sprintf("%+v", d); strings.Contains(s, "giraffe") {
			t.Fatalf("a dropped decision holds speech: %s", s)
		}
	}
}

// Every line Oni says without a model keeps the voice discipline, and a
// character line that breaks it is never said.
func TestLocalLinesKeepTheVoiceDiscipline(t *testing.T) {
	data, err := characters.FS.ReadFile("oni/lines.json")
	if err != nil {
		t.Fatal(err)
	}
	var shipped struct{ Acknowledge []string }
	if err := json.Unmarshal(data, &shipped); err != nil {
		t.Fatal(err)
	}
	all := slices.Concat(shipped.Acknowledge, defaultGreet, defaultThanks, defaultFine, []string{NeedsPressLine})
	for _, at := range []time.Time{t0, time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)} {
		all = append(all, clockLine(at), dateLine(at))
	}
	for _, text := range []string{"set a timer for ninety nine minutes", "set a timer for 1 second", "set a timer for 30 hours"} {
		_, line, ok := remind.Parse(text, t0, time.UTC)
		if !ok {
			t.Fatalf("timer %q", text)
		}
		all = append(all, line)
	}
	for _, line := range all {
		if !Fits(line) {
			t.Errorf("%q breaks the voice discipline (%d lead words)", line, LeadWords(line))
		}
	}
	r, tn := newRouter(t, Config{Lines: Lines{
		Ack:   []string{"This acknowledgement is far too long to say.", "Sure. And more.", "Sure."},
		Greet: []string{"Well hello there, my very good friend."},
	}})
	id := tn.Barge()
	if d := r.Route(Utterance{Text: "what is a comet", Source: Chord, Turn: id, At: t0}); d.Ack != "Sure." {
		t.Fatalf("ack = %q, want the one fitting line", d.Ack)
	}
	if d := r.Route(Utterance{Text: "hello", Source: Chord, Turn: id, At: t0}); !slices.Contains(defaultGreet, d.Line) {
		t.Fatalf("greet = %q, want a default line when none fits", d.Line)
	}
}
