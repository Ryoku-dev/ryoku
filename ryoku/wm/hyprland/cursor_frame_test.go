package main

import (
	"bufio"
	"bytes"
	"errors"
	"testing"
	"time"
)

// cursorFrameStubs fakes the compositor with cursor:invisible reporting
// invisibleBefore, records every eval, and records every wait in slept.
func cursorFrameStubs(t *testing.T, invisibleBefore string, evalErr error) (evals *[]string, slept *[]time.Duration) {
	t.Helper()
	evals, slept = new([]string), new([]time.Duration)
	restoreCtl := stubCtl(t, func(args ...string) ([]byte, error) {
		switch {
		case len(args) >= 2 && args[0] == "getoption" && args[1] == "cursor:invisible":
			return []byte(invisibleBefore), nil
		case len(args) == 2 && args[0] == "eval":
			*evals = append(*evals, args[1])
			return nil, evalErr
		}
		return nil, nil
	})
	prevSleep, prevOut := cursorFrameSleep, stdout
	cursorFrameSleep = func(d time.Duration) { *slept = append(*slept, d) }
	stdout = bufio.NewWriter(&bytes.Buffer{})
	t.Cleanup(func() {
		restoreCtl()
		cursorFrameSleep, stdout = prevSleep, prevOut
	})
	return evals, slept
}

func wantHiddenAfterTick(t *testing.T, evals *[]string, slept *[]time.Duration) {
	t.Helper()
	if len(*evals) != 1 || (*evals)[0] != `hl.config({ cursor = { invisible = true } })` {
		t.Errorf("evals = %q, want one invisible = true eval", *evals)
	}
	if len(*slept) != 1 || (*slept)[0] != cursorTickWait {
		t.Errorf("slept %v, want one wait of %v", *slept, cursorTickWait)
	}
}

// Hyprland only applies cursor:invisible on its next 500 ms tick, and a
// software cursor is in the frame until then, so hiding waits the tick out.
func TestHideCursorWaitsATick(t *testing.T) {
	evals, slept := cursorFrameStubs(t, `{"int":0}`, nil)
	if err := runAct([]string{"cursor.visibility", "hidden"}); err != nil {
		t.Fatalf("hide cursor: %v", err)
	}
	wantHiddenAfterTick(t, evals, slept)
}

// A second hide that starts while an earlier one is still waiting for the tick
// finds cursor:invisible already set. It must wait too, or it takes the frame
// before Hyprland has repainted without the cursor.
func TestHideCursorWaitsWhenAlreadyHidden(t *testing.T) {
	evals, slept := cursorFrameStubs(t, `{"int":1}`, nil)
	if err := runAct([]string{"cursor.visibility", "hidden"}); err != nil {
		t.Fatalf("hide cursor: %v", err)
	}
	wantHiddenAfterTick(t, evals, slept)
}

// A hide Hyprland refused reports the error and does not wait for a tick that
// will hide nothing.
func TestHideCursorSkipsWaitWhenHideFails(t *testing.T) {
	_, slept := cursorFrameStubs(t, `{"int":0}`, errors.New("eval failed"))
	if err := runAct([]string{"cursor.visibility", "hidden"}); err == nil {
		t.Fatal("hide cursor succeeded, want the eval error")
	}
	if len(*slept) != 0 {
		t.Errorf("slept %v after a failed hide, want no wait", *slept)
	}
}

// Showing the cursor again needs no wait: the capture is already taken.
func TestShowCursorDoesNotWait(t *testing.T) {
	evals, slept := cursorFrameStubs(t, `{"int":1}`, nil)
	if err := runAct([]string{"cursor.visibility", "visible"}); err != nil {
		t.Fatalf("show cursor: %v", err)
	}
	if len(*evals) != 1 || (*evals)[0] != `hl.config({ cursor = { invisible = false } })` {
		t.Errorf("evals = %q, want one invisible = false eval", *evals)
	}
	if len(*slept) != 0 {
		t.Errorf("slept %v when showing the cursor, want no wait", *slept)
	}
}
