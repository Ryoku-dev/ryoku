package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"
)

// cursorFrameStubs fakes the compositor for a visible cursor at 100,200 and a
// capture function the case controls.
// slept records every wait the code under test asked for.
var slept []time.Duration

// gaps counts the random pauses taken between probe captures.
var gaps int

// invisibleBefore is cursor:invisible as the stubbed compositor reports it
// before the hide.
var invisibleBefore = `{"int":0}`

func cursorFrameStubs(t *testing.T, capture func(x, y, w, h int, withCursor bool) ([]byte, error)) (*[]string, func()) {
	t.Helper()
	var evals []string
	slept = nil
	restoreCtl := stubCtl(t, func(args ...string) ([]byte, error) {
		switch {
		case len(args) >= 2 && args[0] == "getoption" && args[1] == "cursor:invisible":
			return []byte(invisibleBefore), nil
		case len(args) >= 1 && args[0] == "cursorpos":
			return []byte(`{"x":100,"y":200}`), nil
		case len(args) == 2 && args[0] == "eval":
			evals = append(evals, args[1])
		}
		return nil, nil
	})
	prevCapture, prevSleep, prevGap := captureBox, cursorFrameSleep, probeGap
	captureBox = capture
	cursorFrameSleep = func(d time.Duration) { slept = append(slept, d) }
	gaps = 0
	probeGap = func() { gaps++ }
	prevOut := stdout
	stdout = bufio.NewWriter(&bytes.Buffer{})
	return &evals, func() {
		restoreCtl()
		captureBox, cursorFrameSleep, probeGap = prevCapture, prevSleep, prevGap
		stdout = prevOut
	}
}

func hideAndReport(t *testing.T, evals *[]string) {
	t.Helper()
	if err := runAct([]string{"cursor.visibility", "hidden"}); err != nil {
		t.Fatalf("hide cursor: %v", err)
	}
	if len(*evals) != 1 || (*evals)[0] != `hl.config({ cursor = { invisible = true } })` {
		t.Errorf("evals = %q, want one invisible = true eval", *evals)
	}
}

func wantTickWait(t *testing.T) {
	t.Helper()
	if len(slept) != 1 || slept[0] != cursorTickWait {
		t.Errorf("slept %v, want one wait of %v", slept, cursorTickWait)
	}
}

// A software cursor is painted into the frame, so the overlay flag changes
// nothing. Hyprland only applies cursor:invisible on its next 500 ms tick, so
// hiding must wait the tick out.
func TestHideCursorWaitsATickForSoftwareCursor(t *testing.T) {
	var boxes []string
	var evals *[]string
	evals, restore := cursorFrameStubs(t, func(x, y, w, h int, withCursor bool) ([]byte, error) {
		boxes = append(boxes, fmt.Sprintf("%d,%d,%d,%d", x, y, w, h))
		return []byte("cursor"), nil
	})
	defer restore()

	hideAndReport(t, evals)
	wantTickWait(t)
	for _, b := range boxes {
		if b != "92,192,32,32" {
			t.Errorf("captured box %s, want the probe around the pointer 92,192,32,32", b)
		}
	}
}

// A hardware cursor never reaches the frame: the overlay capture differs from
// two plain captures that agree, so hiding returns at once.
func TestHideCursorSkipsWaitForHardwareCursor(t *testing.T) {
	var evals *[]string
	evals, restore := cursorFrameStubs(t, func(x, y, w, h int, withCursor bool) ([]byte, error) {
		if withCursor {
			return []byte("cursor"), nil
		}
		return []byte("clean"), nil
	})
	defer restore()

	hideAndReport(t, evals)
	if len(slept) != 0 {
		t.Errorf("slept %v for a hardware cursor, want no wait", slept)
	}
	if gaps != 4 {
		t.Errorf("took %d random gaps between the 5 probe captures, want 4", gaps)
	}
}

// Moving content under the pointer (a video, a live wallpaper) makes every
// capture differ, which would pass for a hardware cursor if the two plain
// captures were not compared. It must wait the tick instead of guessing.
func TestHideCursorWaitsATickWhenContentMoves(t *testing.T) {
	n := 0
	var evals *[]string
	evals, restore := cursorFrameStubs(t, func(x, y, w, h int, withCursor bool) ([]byte, error) {
		n++
		return []byte(fmt.Sprintf("frame%d", n)), nil
	})
	defer restore()

	hideAndReport(t, evals)
	wantTickWait(t)
}

// Content that changes for one include-cursor capture and changes back makes
// the plain frames agree and differ from that overlay frame, which is what a
// hardware cursor looks like on three captures. The second overlay capture must
// catch it, so hiding still waits.
func TestHideCursorWaitsWhenOneFrameFlickers(t *testing.T) {
	n := 0
	var evals *[]string
	evals, restore := cursorFrameStubs(t, func(x, y, w, h int, withCursor bool) ([]byte, error) {
		n++
		if n == 2 {
			return []byte("flicker"), nil
		}
		return []byte("cursor"), nil
	})
	defer restore()

	hideAndReport(t, evals)
	wantTickWait(t)
}

// A second hide that starts while an earlier one is still waiting for the tick
// finds cursor:invisible already set. It must wait too, or it takes the frame
// before Hyprland has repainted without the cursor.
func TestHideCursorWaitsWhenAlreadyHidden(t *testing.T) {
	invisibleBefore = `{"int":1}`
	defer func() { invisibleBefore = `{"int":0}` }()
	var evals *[]string
	evals, restore := cursorFrameStubs(t, func(x, y, w, h int, withCursor bool) ([]byte, error) {
		return []byte("cursor"), nil
	})
	defer restore()

	hideAndReport(t, evals)
	wantTickWait(t)
}

// A capture failure leaves the probe blind: a slower shot beats a pointer in it.
func TestHideCursorWaitsATickWhenProbeFails(t *testing.T) {
	evals, restore := cursorFrameStubs(t, func(x, y, w, h int, withCursor bool) ([]byte, error) {
		return nil, errors.New("no screencopy")
	})
	defer restore()

	hideAndReport(t, evals)
	wantTickWait(t)
}
