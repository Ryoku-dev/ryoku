package discipline

import (
	"sync/atomic"

	"ryoku-rashin/internal/oni/window"
)

// Guard checks one live app task: its Baseline, plus the person's own keys.
// Any key that goes down after the task starts or resumes pauses it, since
// a person typing into the window Hands drives is a person taking it back.
//
// The key reader reports through KeyDown, which takes nothing: which key went
// down never reaches this package. Hands' own typing does not reach the key
// reader, which reads the physical keyboards only. Until Listening(true)
// says a key reader runs, and after Listening(false) says it stopped, no
// check passes: a key nobody could hear is never assumed unpressed.
//
// KeyDown and Listening may be called from the reader's goroutines while the
// daemon calls Check and Resume from its own; Check and Resume must not run
// concurrently with each other.
type Guard struct {
	base  Baseline
	self  []string
	tolPx int

	downs     atomic.Uint64
	listening atomic.Bool
	mark      uint64
}

// NewGuard starts guarding b: keys from now on count. self lists Hyakki's own
// app ids and tolPx is the box tolerance, as Baseline.Check takes them.
func NewGuard(b Baseline, self []string, tolPx int) *Guard {
	return &Guard{base: b, self: append([]string(nil), self...), tolPx: tolPx}
}

// KeyDown says the person pressed a key.
func (g *Guard) KeyDown() { g.downs.Add(1) }

// Listening says whether the key reader is running.
func (g *Guard) Listening(on bool) { g.listening.Store(on) }

// Resume starts counting keys afresh, after the person pressed Resume: keys
// pressed while the task was paused do not pause it again.
func (g *Guard) Resume() { g.mark = g.downs.Load() }

// Check returns the window verdict for s first, since it names what changed,
// then PauseKey when no key reader is listening or a key went down since the
// guard started or last resumed.
func (g *Guard) Check(s window.Snapshot) Pause {
	if p := g.base.Check(s, g.self, g.tolPx); p != OK {
		return p
	}
	if !g.listening.Load() || g.downs.Load() != g.mark {
		return PauseKey
	}
	return OK
}
