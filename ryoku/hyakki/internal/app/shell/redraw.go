package shell

import (
	"sync/atomic"
	"time"
)

const (
	// how long after a change the window is asked whether it drew it
	redrawCheck = 20 * time.Millisecond
	// a window that draws nothing for this many checks is hidden, and is left alone until it shows again
	redrawTries = 5
)

// what a redraw is asked of, the window
type invalidator interface{ Invalidate() }

// the window ignores a redraw asked for while it is drawing, so a change that lands mid-frame is asked for again until a frame has started after it
type redrawer struct {
	window invalidator
	// changes asked for, and how many of them the last frame had seen when it started
	asked, drawn atomic.Uint64
	// checks left before giving up, zero when none is scheduled
	left atomic.Int32
}

func (r *redrawer) ask() {
	r.asked.Add(1)
	r.window.Invalidate()
	if r.left.Swap(redrawTries) == 0 {
		time.AfterFunc(redrawCheck, r.check)
	}
}

// called as a frame starts, before it reads any state
func (r *redrawer) frame() { r.drawn.Store(r.asked.Load()) }

func (r *redrawer) check() {
	if r.drawn.Load() == r.asked.Load() {
		r.left.Store(0)
		return
	}
	r.window.Invalidate()
	if r.left.Add(-1) > 0 {
		time.AfterFunc(redrawCheck, r.check)
	}
}
