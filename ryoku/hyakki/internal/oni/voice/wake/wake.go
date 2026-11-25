// Package wake decides which continuous-microphone speech may reach Oni.
//
// Wake listens, while armed, for the configured phrase at the start of a
// short local burst transcript. The phrase opens a session that listens for
// at most voice.wake.sessionS; wake itself stays armed for at most
// voice.wake.armedMaxS and then disarms until the user arms it again. Open
// lets every utterance through for at most voice.press.openMicS. Speech that
// is not let through is discarded: nothing returned carries it and nothing
// is logged about it. Every ceiling is absolute and read from an injected
// clock; nothing here arms a timer, the caller arms one at Deadline.
package wake

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ryoku-rashin/internal/oni/spec"
)

// DefaultPhrase is the wake phrase until the user changes it.
const DefaultPhrase = "hey oni"

// Phrase limits: one word fires on ordinary speech, and a long phrase is
// rarely transcribed whole.
const (
	MinPhraseWords = 2
	MaxPhraseWords = 4
	MinWordRunes   = 2
	MaxPhraseRunes = 40
)

// ErrPhrase is returned for a phrase outside the limits.
var ErrPhrase = errors.New("wake: unusable wake phrase")

// fillers may come before the phrase in a burst: a decoder often keeps the
// sound someone makes before speaking.
var fillers = []string{"um", "uh", "er", "erm", "hm", "hmm", "mm", "oh", "ah"}

// maxFillers is how many fillers may come before the phrase.
const maxFillers = 2

// Phrase is a wake phrase as compared: its folded words.
type Phrase struct{ words []string }

// ParsePhrase checks a wake phrase; an empty one is DefaultPhrase.
func ParsePhrase(s string) (Phrase, error) {
	if strings.TrimSpace(s) == "" {
		s = DefaultPhrase
	}
	if strings.ContainsAny(s, "[(*") {
		return Phrase{}, fmt.Errorf("%w: brackets are not words", ErrPhrase)
	}
	norm := Normalize(s)
	words := strings.Fields(norm)
	switch {
	case len(words) < MinPhraseWords || len(words) > MaxPhraseWords:
		return Phrase{}, fmt.Errorf("%w: use %d to %d words", ErrPhrase, MinPhraseWords, MaxPhraseWords)
	case utf8.RuneCountInString(norm) > MaxPhraseRunes:
		return Phrase{}, fmt.Errorf("%w: longer than %d characters", ErrPhrase, MaxPhraseRunes)
	case slices.Contains(fillers, words[0]):
		return Phrase{}, fmt.Errorf("%w: it may not start with a filler sound", ErrPhrase)
	}
	for _, w := range words {
		if utf8.RuneCountInString(w) < MinWordRunes {
			return Phrase{}, fmt.Errorf("%w: every word needs %d or more letters", ErrPhrase, MinWordRunes)
		}
	}
	return Phrase{words: words}, nil
}

// String is the phrase as compared.
func (p Phrase) String() string { return strings.Join(p.words, " ") }

// prefix reports how many of toks the phrase covers when the burst starts
// with it: up to maxFillers filler sounds, then the phrase's words in order,
// each word whole. A decoder that runs the words together ("heyoni") still
// matches. Anything else, a word that only sounds or starts the same
// included, does not.
func (p Phrase) prefix(toks []token) (int, bool) {
	if len(p.words) == 0 {
		return 0, false
	}
	i := 0
	for i < len(toks) && i < maxFillers && slices.Contains(fillers, toks[i].word) {
		i++
	}
	if i < len(toks) && toks[i].word == strings.Join(p.words, "") {
		return i + 1, true
	}
	for _, w := range p.words {
		if i >= len(toks) || toks[i].word != w {
			return 0, false
		}
		i++
	}
	return i, true
}

// Config holds the wake phrase and the wake ceilings.
type Config struct {
	Phrase   Phrase
	Session  time.Duration // voice.wake.sessionS
	ArmedMax time.Duration // voice.wake.armedMaxS
}

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// FromSpec returns the wake ceilings from the spec with phrase, or
// DefaultPhrase when phrase is empty.
func FromSpec(sp *spec.OniSpec, phrase string) (Config, error) {
	p, err := ParsePhrase(phrase)
	if err != nil {
		return Config{}, err
	}
	return Config{Phrase: p, Session: seconds(sp.Voice.Wake.SessionS), ArmedMax: seconds(sp.Voice.Wake.ArmedMaxS)}, nil
}

// OpenLimit is how long open mic lasts: voice.press.openMicS.
func OpenLimit(sp *spec.OniSpec) time.Duration { return seconds(sp.Voice.Press.OpenMicS) }

// Notice is a ceiling reached, for the caller to tell the user.
type Notice uint8

const (
	NoticeNone Notice = iota
	// NoticeSessionEnded: a wake session reached voice.wake.sessionS.
	NoticeSessionEnded
	// NoticeDisarmed: wake reached voice.wake.armedMaxS and disarmed; it
	// stays off until armed again.
	NoticeDisarmed
	// NoticeOpenEnded: open mic reached voice.press.openMicS and stopped.
	NoticeOpenEnded
)

func (n Notice) String() string {
	switch n {
	case NoticeSessionEnded:
		return "session-ended"
	case NoticeDisarmed:
		return "disarmed"
	case NoticeOpenEnded:
		return "open-ended"
	}
	return "none"
}

// Heard is what a burst let through. The zero Heard, apart from Notice,
// means the burst was discarded.
type Heard struct {
	// Woke is set when the burst began with the wake phrase.
	Woke bool
	// Text is the request: what followed the wake phrase, or the whole
	// burst inside a session or open mic. Empty after a bare wake phrase.
	Text string
	// Notice is a ceiling the call found reached before it heard the burst.
	Notice Notice
}

// Logf receives the notes wake and open mic keep. It is never given a
// transcript.
type Logf func(format string, args ...any)

func quiet(string, ...any) {}

// Wake is the wake phrase listener's state. It is safe for concurrent use.
type Wake struct {
	c    Config
	now  func() time.Time
	logf Logf

	mu      sync.Mutex
	armed   bool
	armedAt time.Time
	session bool
	opened  time.Time
}

// New returns a disarmed listener. now is the clock every ceiling reads;
// logf, when not nil, receives notes that never hold speech.
func New(c Config, now func() time.Time, logf Logf) (*Wake, error) {
	if len(c.Phrase.words) == 0 {
		return nil, fmt.Errorf("%w: none set", ErrPhrase)
	}
	if c.Session <= 0 || c.ArmedMax <= 0 || c.Session > c.ArmedMax {
		return nil, errors.New("wake: ceilings must be positive with the session inside the armed time")
	}
	if now == nil {
		now = time.Now
	}
	if logf == nil {
		logf = quiet
	}
	return &Wake{c: c, now: now, logf: logf}, nil
}

// Arm starts listening for the phrase; the armed ceiling runs from now. Arm
// while armed leaves the ceiling where it is.
func (w *Wake) Arm() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.armed {
		return
	}
	w.armed, w.armedAt, w.session = true, w.now(), false
	w.logf("oni wake: armed for at most %s", w.c.ArmedMax)
}

// Disarm stops listening and ends any session.
func (w *Wake) Disarm() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.armed {
		w.armed, w.session = false, false
		w.logf("oni wake: disarmed")
	}
}

// EndSession ends a session before its ceiling, as after a dismissal.
func (w *Wake) EndSession() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.session {
		w.session = false
		w.logf("oni wake: session ended")
	}
}

// Armed reports whether wake is listening, after applying the ceilings.
func (w *Wake) Armed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.tick()
	return w.armed
}

// InSession reports whether a wake session is open, after applying the
// ceilings.
func (w *Wake) InSession() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.tick()
	return w.session
}

// Tick applies the ceilings at the clock's now and reports the one reached.
// The caller runs it at Deadline.
func (w *Wake) Tick() Notice {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.tick()
}

func (w *Wake) tick() Notice {
	if !w.armed {
		return NoticeNone
	}
	now := w.now()
	if !now.Before(w.armedAt.Add(w.c.ArmedMax)) {
		w.armed, w.session = false, false
		w.logf("oni wake: disarmed at its %s ceiling", w.c.ArmedMax)
		return NoticeDisarmed
	}
	if w.session && !now.Before(w.opened.Add(w.c.Session)) {
		w.session = false
		w.logf("oni wake: session ended at its %s ceiling", w.c.Session)
		return NoticeSessionEnded
	}
	return NoticeNone
}

// Deadline is when the next ceiling falls: the session's end inside a
// session, else the armed ceiling. It is zero when disarmed.
func (w *Wake) Deadline() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.armed {
		return time.Time{}
	}
	end := w.armedAt.Add(w.c.ArmedMax)
	if s := w.opened.Add(w.c.Session); w.session && s.Before(end) {
		end = s
	}
	return end
}

// Hear takes one burst transcript. A burst that starts with the phrase
// opens a session (a session already open keeps its own ceiling) and
// returns what followed the phrase. Inside a session the whole burst is
// let through. Anything else, and everything while disarmed, is
// discarded: the result holds no text and nothing is logged.
func (w *Wake) Hear(burst string) Heard {
	w.mu.Lock()
	defer w.mu.Unlock()
	h := Heard{Notice: w.tick()}
	if !w.armed {
		return h
	}
	text := strip(burst)
	toks := tokens(text)
	if n, ok := w.c.Phrase.prefix(toks); ok {
		if !w.session {
			w.session, w.opened = true, w.now()
			w.logf("oni wake: session opened for at most %s", w.c.Session)
		}
		h.Woke, h.Text = true, after(text, toks, n)
		return h
	}
	if w.session && len(toks) > 0 {
		h.Text = after(text, toks, 0)
	}
	return h
}

// Open is open mic: every utterance is let through until it is stopped or
// reaches voice.press.openMicS. It is safe for concurrent use.
type Open struct {
	limit time.Duration
	now   func() time.Time
	logf  Logf

	mu      sync.Mutex
	live    bool
	started time.Time
}

// NewOpen returns open mic, stopped, ending limit after each Start.
func NewOpen(limit time.Duration, now func() time.Time, logf Logf) (*Open, error) {
	if limit <= 0 {
		return nil, errors.New("wake: open mic needs a positive limit")
	}
	if now == nil {
		now = time.Now
	}
	if logf == nil {
		logf = quiet
	}
	return &Open{limit: limit, now: now, logf: logf}, nil
}

// Start opens the mic; the limit runs from now. Start while live leaves the
// limit where it is.
func (o *Open) Start() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.live {
		return
	}
	o.live, o.started = true, o.now()
	o.logf("oni open mic: on for at most %s", o.limit)
}

// Stop closes the mic.
func (o *Open) Stop() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.live {
		o.live = false
		o.logf("oni open mic: off")
	}
}

// Live reports whether open mic is on, after applying the limit.
func (o *Open) Live() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.tick()
	return o.live
}

// Tick applies the limit at the clock's now.
func (o *Open) Tick() Notice {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.tick()
}

func (o *Open) tick() Notice {
	if o.live && !o.now().Before(o.started.Add(o.limit)) {
		o.live = false
		o.logf("oni open mic: off at its %s ceiling", o.limit)
		return NoticeOpenEnded
	}
	return NoticeNone
}

// Deadline is when open mic ends; zero when it is off.
func (o *Open) Deadline() time.Time {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.live {
		return time.Time{}
	}
	return o.started.Add(o.limit)
}

// Hear lets an utterance through while open mic is on and discards it
// otherwise, with no text kept and nothing logged.
func (o *Open) Hear(utterance string) Heard {
	o.mu.Lock()
	defer o.mu.Unlock()
	h := Heard{Notice: o.tick()}
	if o.live {
		text := strip(utterance)
		if toks := tokens(text); len(toks) > 0 {
			h.Text = after(text, toks, 0)
		}
	}
	return h
}
