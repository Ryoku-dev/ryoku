package shell

import (
	"sync"
	"testing"
	"time"
)

// busyWindow is a window the way the toolkit has it: a redraw asked for while
// a frame is being drawn is ignored, and it can be asked again only once that
// frame is over.
type busyWindow struct {
	mu      sync.Mutex
	drawing bool
	woken   int
}

func (w *busyWindow) Invalidate() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.drawing {
		w.woken++
	}
}

func (w *busyWindow) set(drawing bool) {
	w.mu.Lock()
	w.drawing = drawing
	w.mu.Unlock()
}

func (w *busyWindow) wakes() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.woken
}

// The last event of an answer often lands while the frame for the one before
// it is still being drawn. The window ignored that redraw, so the end of a
// turn, or a daemon coming back, stayed off screen until the pointer moved.
// It is now asked again until a frame has started after the change.
func TestAChangeThatLandsMidFrameIsStillDrawn(t *testing.T) {
	window := &busyWindow{}
	redraw := &redrawer{window: window}

	redraw.ask()
	if window.wakes() != 1 {
		t.Fatalf("a change woke the window %d times", window.wakes())
	}
	window.set(true)
	redraw.frame()
	redraw.ask()
	window.set(false)
	deadline := time.Now().Add(time.Second)
	for window.wakes() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("the change that landed mid-frame was never drawn")
		}
		time.Sleep(2 * time.Millisecond)
	}

	// a frame that starts after the change settles it: nothing is asked again
	redraw.frame()
	time.Sleep(4 * redrawCheck)
	if got := window.wakes(); got != 2 {
		t.Fatalf("the window was woken %d times for two changes", got)
	}
}

// A hidden window draws nothing however often it is asked. It is asked a few
// times and then left alone, so a minimized app does not wake every 20 ms for
// as long as something is waiting to be shown.
func TestAWindowThatNeverDrawsIsLeftAlone(t *testing.T) {
	window := &busyWindow{}
	redraw := &redrawer{window: window}
	redraw.ask()
	time.Sleep((redrawTries + 6) * redrawCheck)
	settled := window.wakes()
	if settled > redrawTries+1 {
		t.Fatalf("a window that drew nothing was woken %d times", settled)
	}
	time.Sleep(6 * redrawCheck)
	if got := window.wakes(); got != settled {
		t.Fatalf("it was still being woken: %d, then %d", settled, got)
	}
	// the next change starts the asking again
	redraw.ask()
	if got := window.wakes(); got != settled+1 {
		t.Fatalf("a later change woke it %d times", got-settled)
	}
}
