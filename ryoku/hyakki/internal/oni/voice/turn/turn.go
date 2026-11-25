// Package turn owns who may speak in the voice loop: the current turn's id,
// the echo window after Oni speaks, the heard-back guard that keeps Oni's
// own lines from returning as commands, and the acknowledgement pacing. It
// is pure state with the spec's voice.turn numbers; it starts nothing.
package turn

import (
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/oni/spec"
)

// Turn is the voice loop's turn state. It is safe for concurrent use.
type Turn struct {
	echoMs func(chars int) float64
	ackGap time.Duration

	mu        sync.Mutex
	id        uint64
	echoUntil time.Time
	cancel    func()
	lines     []line
	ackedID   uint64
	acked     bool
	lastAck   time.Time
}

// line is one of Oni's spoken lines, as compared, and when its echo window
// ends.
type line struct {
	words []string
	until time.Time
}

// New returns the turn state with the spec's echo window and ack gap.
// cancel, when not nil, is called by Barge to cut whatever Oni is saying.
func New(sp *spec.OniSpec, cancel func()) *Turn {
	return &Turn{
		echoMs: sp.EchoMs,
		ackGap: time.Duration(sp.Voice.Turn.AckGapMs * float64(time.Millisecond)),
		cancel: cancel,
	}
}

// ID is the current turn. Work started for a turn carries its id and is
// kept only while Accept says so.
func (t *Turn) ID() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.id
}

// EchoUntil is when the current echo window ends; zero when there is none.
func (t *Turn) EchoUntil() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.echoUntil
}

func (t *Turn) window(chars int) time.Duration {
	return time.Duration(t.echoMs(chars) * float64(time.Millisecond))
}

// Spoke opens the echo window for a line of n characters that starts
// playing at now: until now + voice.turn.echoBaseMs + n·echoPerCharMs the
// mic is hearing Oni. A window already open for longer is kept.
func (t *Turn) Spoke(n int, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spoke(n, now)
}

func (t *Turn) spoke(n int, now time.Time) time.Time {
	until := now.Add(t.window(n))
	if until.After(t.echoUntil) {
		t.echoUntil = until
	}
	return until
}

// Said is Spoke for a line whose words are known: it opens the echo window
// by the line's length in characters and remembers the line for Heard
// until that line's own window ends.
func (t *Turn) Said(text string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prune(now)
	until := t.spoke(utf8.RuneCountInString(text), now)
	if w := words(text); len(w) > 0 {
		t.lines = append(t.lines, line{words: w, until: until})
	}
}

// Echoing reports whether now falls in the echo window.
func (t *Turn) Echoing(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return now.Before(t.echoUntil)
}

// Barge is the talk key going down: a new turn begins, the echo window and
// the remembered lines are cleared (the user, not Oni, speaks next), and the
// cancel hook cuts any playback. It returns the new turn's id.
func (t *Turn) Barge() uint64 {
	t.mu.Lock()
	t.id++
	id := t.id
	t.echoUntil = time.Time{}
	t.lines = nil
	cancel := t.cancel
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return id
}

// Accept reports whether work made for turn id is still current. A line
// rendered or an answer produced for an older turn is dropped.
func (t *Turn) Accept(id uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return id == t.id
}

// Heard reports whether text, transcribed at now, is Oni's own speech heard
// back: its words, compared without case or punctuation, are the whole or a
// run of Oni's lines whose echo windows are still open, taken in the order
// they were said, so a run may cross from one line into the next (the mic
// hears an acknowledgement and the reply after it as one utterance). Such
// text is never a command and the caller drops it. Anything else, including
// a user utterance that only shares words with a line, is the user's.
func (t *Turn) Heard(text string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prune(now)
	w := words(text)
	if len(w) == 0 {
		return false
	}
	var said []string
	for _, l := range t.lines {
		said = append(said, l.words...)
	}
	return run(said, w)
}

// prune forgets the lines whose echo window has ended by now.
func (t *Turn) prune(now time.Time) {
	kept := t.lines[:0]
	for _, l := range t.lines {
		if now.Before(l.until) {
			kept = append(kept, l)
		}
	}
	clear(t.lines[len(kept):])
	t.lines = kept
}

// Ack reports whether an acknowledgement may play at now, and takes the
// slot when it may: one per turn, and at least voice.turn.ackGapMs after the
// previous one.
func (t *Turn) Ack(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.acked && (t.ackedID == t.id || now.Sub(t.lastAck) < t.ackGap) {
		return false
	}
	t.acked, t.ackedID, t.lastAck = true, t.id, now
	return true
}

// words splits text into words as compared: lower case, letters and digits
// only, empty words dropped.
func words(text string) []string {
	var out []string
	for _, f := range strings.Fields(text) {
		k := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, f)
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

// run reports whether sub appears as a contiguous run of words in w.
func run(w, sub []string) bool {
	for i := 0; i+len(sub) <= len(w); i++ {
		match := true
		for j, s := range sub {
			if w[i+j] != s {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
