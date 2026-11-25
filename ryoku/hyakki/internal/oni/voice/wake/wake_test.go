package wake

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	c.t = t
	c.mu.Unlock()
}

type logs struct {
	mu    sync.Mutex
	lines []string
}

func (l *logs) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *logs) all() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

func (l *logs) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.lines)
}

var t0 = time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)

func loadSpec(t *testing.T) *spec.OniSpec {
	t.Helper()
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

func newWake(t *testing.T, phrase string) (*Wake, *clock, *logs) {
	t.Helper()
	c, err := FromSpec(loadSpec(t), phrase)
	if err != nil {
		t.Fatal(err)
	}
	clk, l := &clock{t: t0}, &logs{}
	w, err := New(c, clk.now, l.logf)
	if err != nil {
		t.Fatal(err)
	}
	return w, clk, l
}

func TestCeilingsComeFromTheSpec(t *testing.T) {
	sp := loadSpec(t)
	c, err := FromSpec(sp, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Session != 15*time.Second || c.ArmedMax != 60*time.Minute || OpenLimit(sp) != 180*time.Second {
		t.Fatalf("ceilings: session %s, armed %s, open mic %s; want 15s, 1h, 3m", c.Session, c.ArmedMax, OpenLimit(sp))
	}
	if c.Phrase.String() != DefaultPhrase {
		t.Fatalf("default phrase = %q", c.Phrase)
	}
}

func TestWakeAcceptsThePhraseAtTheStart(t *testing.T) {
	for _, c := range []struct{ burst, rest string }{
		{"hey oni", ""},
		{"Hey, Oni!", ""},
		{"HEY ONI.", ""},
		{"  hey   oni  ", ""},
		{"Hey Oni, what time is it?", "what time is it?"},
		{"hey oni what's the date", "what's the date"},
		{"Hey-Oni — set a timer for five minutes.", "set a timer for five minutes."},
		{"Heyoni, hello", "hello"},
		{"Héy Ōni", ""},
		{"Um, hey Oni. Thanks.", "Thanks."},
		{"uh um hey oni hi", "hi"},
		{"[BLANK_AUDIO] hey oni", ""},
		{"(wind blowing) Hey Oni, what day is it?", "what day is it?"},
		{"hey oni [BLANK_AUDIO]", ""},
		{"Hey Oni... tell me a joke", "tell me a joke"},
	} {
		w, _, _ := newWake(t, "")
		w.Arm()
		h := w.Hear(c.burst)
		if !h.Woke || h.Text != c.rest {
			t.Errorf("Hear(%q) = %+v, want woke with %q", c.burst, h, c.rest)
		}
		if !w.InSession() {
			t.Errorf("Hear(%q) opened no session", c.burst)
		}
	}
}

// nearMisses sound or read like the default phrase and must not open a
// session: a different word, the phrase cut short, a word that only starts
// the same, the phrase late in the burst, or too many sounds before it.
var nearMisses = []string{
	"",
	"   ",
	"...",
	"[BLANK_AUDIO]",
	"hey",
	"oni",
	"oni hey",
	"hey only what time is it",
	"hey honey",
	"hey onion soup",
	"hey onigiri please",
	"hey on it",
	"hey o n i",
	"hey annie",
	"hey ony",
	"hey oh nee",
	"hay oni",
	"they oni",
	"okay oni",
	"hi oni",
	"a oni",
	"heyonion",
	"hey, any news?",
	"I told her hey oni yesterday",
	"so hey oni",
	"um uh er hey oni",
	"hey hey oni",
	"(hey oni)",
	"[hey oni] what time is it",
	"*hey oni*",
}

func TestWakeRejectsNearMisses(t *testing.T) {
	w, _, l := newWake(t, "")
	w.Arm()
	armedLogs := l.count()
	for _, burst := range nearMisses {
		if h := w.Hear(burst); h != (Heard{}) {
			t.Errorf("Hear(%q) = %+v, want it discarded", burst, h)
		}
	}
	if w.InSession() {
		t.Fatal("a near miss opened a session")
	}
	if l.count() != armedLogs {
		t.Fatalf("near misses were logged: %q", l.all())
	}
}

func TestChangedPhrase(t *testing.T) {
	w, _, _ := newWake(t, "  Okay,  LANTERN ")
	w.Arm()
	for _, burst := range []string{"hey oni what time is it", "okay lanterns", "okay", "lantern okay"} {
		if h := w.Hear(burst); h != (Heard{}) {
			t.Errorf("with a changed phrase, Hear(%q) = %+v, want it discarded", burst, h)
		}
	}
	if h := w.Hear("Okay, Lantern: what's the date?"); !h.Woke || h.Text != "what's the date?" {
		t.Fatalf("changed phrase: %+v", h)
	}
	if got := w.c.Phrase.String(); got != "okay lantern" {
		t.Fatalf("phrase as compared = %q", got)
	}
}

func TestPhraseLimits(t *testing.T) {
	for _, bad := range []string{"oni", "hey", "a oni", "hey o", "um oni", "red tan sky ink jet", "hey (oni)", "[x] oni", strings.Repeat("long", 8) + " " + strings.Repeat("word", 4), "!!!"} {
		if _, err := ParsePhrase(bad); !errors.Is(err, ErrPhrase) {
			t.Errorf("ParsePhrase(%q) = %v, want ErrPhrase", bad, err)
		}
	}
	for _, good := range []string{"", "hey oni", "Hey, Oni", "okay lantern", "wake up little one"} {
		if _, err := ParsePhrase(good); err != nil {
			t.Errorf("ParsePhrase(%q) = %v", good, err)
		}
	}
	if _, err := New(Config{}, nil, nil); !errors.Is(err, ErrPhrase) {
		t.Fatalf("New with no phrase = %v", err)
	}
	p, _ := ParsePhrase("")
	if _, err := New(Config{Phrase: p, Session: time.Hour, ArmedMax: time.Minute}, nil, nil); err == nil {
		t.Fatal("a session longer than the armed time was accepted")
	}
}

func TestDisarmedHearsNothing(t *testing.T) {
	w, _, l := newWake(t, "")
	if h := w.Hear("hey oni what time is it"); h != (Heard{}) || w.InSession() {
		t.Fatalf("disarmed wake heard %+v", h)
	}
	if l.count() != 0 {
		t.Fatalf("disarmed wake logged %q", l.all())
	}
	if !w.Deadline().IsZero() {
		t.Fatal("disarmed wake has a deadline")
	}
}

func TestSessionCeiling(t *testing.T) {
	w, clk, _ := newWake(t, "")
	w.Arm()
	opened := t0.Add(time.Minute)
	clk.set(opened)
	if h := w.Hear("hey oni"); !h.Woke {
		t.Fatalf("wake: %+v", h)
	}
	if got := w.Deadline(); !got.Equal(opened.Add(15 * time.Second)) {
		t.Fatalf("session deadline = %s, want 15s after it opened", got)
	}
	clk.set(opened.Add(5 * time.Second))
	if h := w.Hear("what time is it"); h.Woke || h.Text != "what time is it" {
		t.Fatalf("inside the session: %+v", h)
	}
	// Saying the phrase again inside a session does not stretch it.
	clk.set(opened.Add(10 * time.Second))
	if h := w.Hear("hey oni, and the date?"); !h.Woke || h.Text != "and the date?" {
		t.Fatalf("again inside the session: %+v", h)
	}
	clk.set(opened.Add(15*time.Second - time.Millisecond))
	if h := w.Hear("still here"); h.Text != "still here" {
		t.Fatalf("just before the ceiling: %+v", h)
	}
	clk.set(opened.Add(15 * time.Second))
	if h := w.Hear("what is the weather"); h != (Heard{Notice: NoticeSessionEnded}) {
		t.Fatalf("at the ceiling: %+v, want it discarded with the session-ended notice", h)
	}
	if w.InSession() || !w.Armed() {
		t.Fatal("after the session ceiling wake should be armed with no session")
	}
	if h := w.Hear("what is the weather"); h != (Heard{}) {
		t.Fatalf("after the session: %+v", h)
	}
	if got := w.Deadline(); !got.Equal(t0.Add(time.Hour)) {
		t.Fatalf("armed deadline = %s, want an hour after arming", got)
	}
	// A new wake opens a new session.
	clk.set(opened.Add(20 * time.Second))
	if h := w.Hear("hey oni"); !h.Woke || !w.InSession() {
		t.Fatalf("new wake: %+v", h)
	}
	w.EndSession()
	if h := w.Hear("what is the weather"); h != (Heard{}) {
		t.Fatalf("after EndSession: %+v", h)
	}
}

func TestArmedCeiling(t *testing.T) {
	w, clk, l := newWake(t, "")
	w.Arm()
	clk.set(t0.Add(30 * time.Minute))
	w.Arm() // arming again does not move the ceiling
	clk.set(t0.Add(time.Hour - 5*time.Second))
	if h := w.Hear("hey oni"); !h.Woke {
		t.Fatalf("before the armed ceiling: %+v", h)
	}
	if got := w.Deadline(); !got.Equal(t0.Add(time.Hour)) {
		t.Fatalf("a session near the armed ceiling ends at %s, want the armed ceiling", got)
	}
	clk.set(t0.Add(time.Hour))
	if h := w.Hear("hey oni what time is it"); h != (Heard{Notice: NoticeDisarmed}) {
		t.Fatalf("at the armed ceiling: %+v, want it discarded with the disarmed notice", h)
	}
	if w.Armed() || w.InSession() || !w.Deadline().IsZero() {
		t.Fatal("wake is still armed after its ceiling")
	}
	if !strings.Contains(l.all(), "disarmed at its 1h0m0s ceiling") {
		t.Fatalf("no disarm note: %q", l.all())
	}
	clk.set(t0.Add(2 * time.Hour))
	if h := w.Hear("hey oni"); h != (Heard{}) {
		t.Fatalf("wake stayed live past its ceiling: %+v", h)
	}
	w.Arm()
	if h := w.Hear("hey oni"); !h.Woke {
		t.Fatalf("re-armed wake: %+v", h)
	}
	if got := w.Deadline(); !got.Equal(t0.Add(2*time.Hour + 15*time.Second)) {
		t.Fatalf("re-armed session deadline = %s", got)
	}
	clk.set(t0.Add(3 * time.Hour))
	if n := w.Tick(); n != NoticeDisarmed {
		t.Fatalf("Tick at the second ceiling = %s", n)
	}
	if n := w.Tick(); n != NoticeNone {
		t.Fatalf("a second Tick = %s", n)
	}
}

func TestOpenMicEndsAtItsCeiling(t *testing.T) {
	sp := loadSpec(t)
	clk, l := &clock{t: t0}, &logs{}
	o, err := NewOpen(OpenLimit(sp), clk.now, l.logf)
	if err != nil {
		t.Fatal(err)
	}
	if h := o.Hear("what time is it"); h != (Heard{}) || o.Live() {
		t.Fatalf("stopped open mic heard %+v", h)
	}
	o.Start()
	if got := o.Deadline(); !got.Equal(t0.Add(180 * time.Second)) {
		t.Fatalf("open mic deadline = %s", got)
	}
	clk.set(t0.Add(100 * time.Second))
	o.Start() // starting again does not stretch it
	clk.set(t0.Add(180*time.Second - time.Millisecond))
	if h := o.Hear("[BLANK_AUDIO] What time is it?"); h.Text != "What time is it?" || h.Woke {
		t.Fatalf("live open mic: %+v", h)
	}
	clk.set(t0.Add(180 * time.Second))
	if h := o.Hear("what time is it"); h != (Heard{Notice: NoticeOpenEnded}) {
		t.Fatalf("at the ceiling: %+v", h)
	}
	if o.Live() || !o.Deadline().IsZero() {
		t.Fatal("open mic still live after 180 s")
	}
	if !strings.Contains(l.all(), "off at its 3m0s ceiling") {
		t.Fatalf("no ceiling note: %q", l.all())
	}
	o.Start()
	o.Stop()
	if h := o.Hear("what time is it"); h != (Heard{}) {
		t.Fatalf("stopped open mic heard %+v", h)
	}
	if _, err := NewOpen(0, nil, nil); err == nil {
		t.Fatal("open mic with no limit was accepted")
	}
}

// Speech that is not let through leaves no trace: the result is the zero
// Heard and the log never holds it. Speech that is let through is never
// logged either.
func TestDiscardedSpeechLeavesNoTrace(t *testing.T) {
	const secret = "purple giraffe pays rent tomorrow"
	words := strings.Fields(secret)
	w, clk, l := newWake(t, "")
	o, err := NewOpen(OpenLimit(loadSpec(t)), clk.now, l.logf)
	if err != nil {
		t.Fatal(err)
	}
	var results []Heard
	results = append(results, w.Hear(secret), o.Hear(secret))
	w.Arm()
	results = append(results, w.Hear(secret), w.Hear("hey only "+secret), w.Hear("so hey oni "+secret))
	w.Hear("hey oni")
	clk.set(t0.Add(15 * time.Second))
	results = append(results, w.Hear(secret))
	o.Start()
	clk.set(t0.Add(15*time.Second + 180*time.Second))
	results = append(results, o.Hear(secret))
	clk.set(t0.Add(time.Hour))
	results = append(results, w.Hear("hey oni "+secret))
	for i, h := range results {
		if h.Woke || h.Text != "" {
			t.Errorf("result %d carries speech: %+v", i, h)
		}
		for _, word := range words {
			if strings.Contains(fmt.Sprintf("%+v %#v", h, h), word) {
				t.Errorf("result %d holds %q", i, word)
			}
		}
	}
	// Let-through speech is returned but never logged.
	w.Arm()
	o.Start()
	if h := w.Hear("hey oni " + secret); h.Text != secret {
		t.Fatalf("let-through wake text = %q", h.Text)
	}
	if h := o.Hear(secret); h.Text != secret {
		t.Fatalf("let-through open mic text = %q", h.Text)
	}
	log := strings.ToLower(l.all())
	for _, word := range append(words, "hey oni", "only") {
		if strings.Contains(log, word) {
			t.Fatalf("the log holds %q:\n%s", word, log)
		}
	}
	if l.count() == 0 {
		t.Fatal("the log captured nothing; the check above proves nothing")
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"What's the TIME?":             "whats the time",
		"[BLANK_AUDIO]":                "",
		"(music) hello *coughs* there": "hello there",
		"Café — déjà vu":               "cafe deja vu",
		"set a 3-minute timer":         "set a 3 minute timer",
		"it’s 3:45":                    "its 3 45",
		"open (never closed":           "open",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
