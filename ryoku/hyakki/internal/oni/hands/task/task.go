// Package task keeps the Hands task journal and the projects the person
// chose to keep. A row holds a task's state, its scope, the app id it
// works in, how many steps it took, why it ended and when: nothing else.
// It never holds the goal, a window title, a URL, page or screen text, or
// any word typed into a field, and the file's strict shape refuses any
// field that could carry one. After a restart the person restates the
// goal.
//
// One task runs at a time, and at most QueueMax more wait behind it, first
// in, first out. A task that waits, and any task a restart interrupted,
// starts again only from a button in the app or on the presenter. A task
// that ended, failed included, never starts again: a Hands error is not
// retried.
//
// The journal is pure state on disk: it starts no process, reads no clock
// but the one it is given, and acts on no window.
package task

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/hands/discipline"
	"ryoku-rashin/internal/oni/hands/loop"
	"ryoku-rashin/internal/oni/hands/proto"
	"ryoku-rashin/internal/oni/spec"
)

// State is where a task stands.
type State string

// The states a task moves through.
const (
	// UpNext waits in the queue for a button.
	UpNext State = "upnext"
	// Working is the running task taking a step.
	Working State = "working"
	// Checking is the running task checking what a step did.
	Checking State = "checking"
	// NeedsYou is the running task paused on the person: a handoff, a
	// question or a pause card.
	NeedsYou State = "needsyou"
	// Done ended with the goal reached.
	Done State = "done"
	// Failed ended on an error. It is never retried.
	Failed State = "failed"
	// Stopped ended on a stop from any source.
	Stopped State = "stopped"
	// Interrupted was running or waiting when the daemon went away. It
	// starts again only from a button, with the goal restated.
	Interrupted State = "interrupted"
)

// Active says the task is the running one.
func (s State) Active() bool { return s == Working || s == Checking || s == NeedsYou }

// Ended says the task is over: done, failed, stopped or interrupted.
func (s State) Ended() bool { return s == Done || s == Failed || s == Stopped || s == Interrupted }

func (s State) known() bool { return s == UpNext || s.Active() || s.Ended() }

// Where a task came from.
const (
	OriginApp       = "app"       // the app's Start
	OriginPresenter = "presenter" // the presenter's Start
	OriginVoice     = "voice"     // a Press the Hands slot claimed
	OriginHarness   = "harness"   // a hosted harness's approved request
)

var origins = []string{OriginApp, OriginPresenter, OriginVoice, OriginHarness}

// ByButton is the only source that starts a waiting or interrupted task:
// the app's or the presenter's Start button. A Press, wake, open mic, the
// CLI and a harness never do.
const ByButton = loop.ByButton

// ReasonRestart is the journal's own reason: the daemon went away while
// the task ran or waited.
const ReasonRestart = "restart"

// reasons is every reason a row may hold: a closed set, so a reason never
// carries free text. It is exactly what the loop and the window discipline
// emit, from their own constants, and ReasonRestart.
var reasons = []string{
	ReasonRestart,

	// How the loop ends a task.
	loop.EndDone, loop.EndAsked, loop.EndAnswered, loop.EndStopped, loop.EndBudget,
	loop.EndRepeat, loop.EndUnconfirmed, loop.EndRefused, loop.EndExpired, loop.EndAway,
	loop.EndTimeout, loop.EndFailed, loop.EndClosed, loop.EndUnread,

	// Why the loop pauses a task on its own.
	loop.PauseClosed, loop.PauseNavigation, loop.PauseWindow, loop.PauseFocus,

	// Why the window discipline pauses a task; the loop passes these on.
	string(discipline.PauseClosed), string(discipline.PauseFocus), string(discipline.PauseWindow),
	string(discipline.PauseHidden), string(discipline.PauseOutput), string(discipline.PauseTitle),
	string(discipline.PauseGeometry), string(discipline.PauseCovered), string(discipline.PauseSelf),
	string(discipline.PausePrivate), string(discipline.PauseKey), string(discipline.PausePresenter),
}

// Bounds on what a row holds.
const (
	// IDBytes is the length of a task or project id: NewID's 16 random
	// bytes in hex.
	IDBytes = 32
	// MaxAppBytes is the longest app id kept.
	MaxAppBytes = 128
)

// Task is one journal row. It is never the goal, a title, a URL or a typed
// word: App is an app id, Reason one the loop or the discipline emits.
type Task struct {
	ID     string `json:"id"`
	State  State  `json:"state"`
	Scope  string `json:"scope"`
	App    string `json:"app,omitempty"`
	Origin string `json:"origin"`
	// Step counts the steps taken; it carries across a restart, so a
	// restart never refills the turn budget.
	Step int `json:"step"`
	// At is when the task was queued; Ended when it ended, zero while it
	// waits or runs.
	At     time.Time `json:"at"`
	Ended  time.Time `json:"ended,omitzero"`
	Reason string    `json:"reason,omitempty"`
}

// Project is a task the person kept, to continue later in the same scope
// and app with the goal restated. It keeps no name, goal, title or URL.
type Project struct {
	ID    string    `json:"id"`
	Scope string    `json:"scope"`
	App   string    `json:"app,omitempty"`
	Steps int       `json:"steps"`
	Kept  time.Time `json:"kept"`
	Used  time.Time `json:"used"`
}

// Config holds the journal's numbers; FromSpec fills them from the spec.
type Config struct {
	// QueueMax is how many tasks may wait behind the running one,
	// hands.task.queueMax.
	QueueMax int
	// Turns is the most steps one task takes, hands.task.turns.
	Turns int
	// Age is how long an ended task or an unused project is kept,
	// hands.retention.days.
	Age time.Duration
	// Rows is the most tasks, and the most projects, kept,
	// hands.retention.rows.
	Rows int
}

var limits = sync.OnceValue(func() Config {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return FromSpec(s)
})

// FromSpec returns the journal's numbers from the spec's hands section.
func FromSpec(s *spec.OniSpec) Config {
	h := s.Hands
	return Config{
		QueueMax: h.Task.QueueMax,
		Turns:    h.Task.Turns,
		Age:      h.Retention.Age(),
		Rows:     h.Retention.Rows,
	}
}

// normal holds c to the spec: a number past the spec's, or none, is the
// spec's. A caller may tighten a number, never loosen it.
func normal(c Config) Config {
	l := limits()
	clamp := func(v *int, max int) {
		if *v <= 0 || *v > max {
			*v = max
		}
	}
	clamp(&c.QueueMax, l.QueueMax)
	clamp(&c.Turns, l.Turns)
	clamp(&c.Rows, l.Rows)
	if c.Age <= 0 || c.Age > l.Age {
		c.Age = l.Age
	}
	if c.Rows <= c.QueueMax {
		c.Rows = c.QueueMax + 1
	}
	return c
}

// NewID returns a fresh random task or project id.
func NewID() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// validID says id has NewID's shape: IDBytes lowercase hex digits. A typed
// word, a title or a URL never has it.
func validID(id string) bool {
	if len(id) != IDBytes {
		return false
	}
	for i := range len(id) {
		if c := id[i]; !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// validApp says app is empty or shaped like an app id: ASCII letters and
// digits, joined by '.', '-', '_' or '+', starting with a letter or digit.
// No space, slash, colon, query or other rune a title or a URL needs.
func validApp(app string) bool {
	if app == "" {
		return true
	}
	if len(app) > MaxAppBytes {
		return false
	}
	for i := range len(app) {
		c := app[i]
		alnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if i == 0 && !alnum {
			return false
		}
		if !alnum && c != '.' && c != '-' && c != '_' && c != '+' {
			return false
		}
	}
	return true
}

func validScope(scope string, app string) bool {
	if !proto.KnownScope(scope) {
		return false
	}
	// A live window is known by its app id; Hands never starts on one it
	// cannot pin.
	return scope != proto.ScopeWindow || app != ""
}

func validReason(r string) bool { return r == "" || slices.Contains(reasons, r) }
