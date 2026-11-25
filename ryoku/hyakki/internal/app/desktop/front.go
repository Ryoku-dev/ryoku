package desktop

import (
	"context"
	"encoding/json"
	"image"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// a floating window covers what it opens, so that window is asked in front through the ryoku wm door, which names no compositor
const (
	seamTimeout  = 2 * time.Second
	arrivalWait  = 6 * time.Second
	arrivalEvery = 150 * time.Millisecond
	frontMargin  = 24
)

type seamWindow struct {
	ID         string `json:"id"`
	Pid        int    `json:"pid"`
	Output     string `json:"output"`
	FocusOrder int    `json:"focusOrder"`
	Floating   bool   `json:"floating"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type seamOutput struct {
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type seamState struct {
	Outputs []seamOutput `json:"outputs"`
	Windows []seamWindow `json:"windows"`
}

// the windows that stood before a launch, kept only when this app floats among them
type windows struct {
	known map[string]bool
	self  seamWindow
	out   seamOutput
}

// the command that reaches the compositor, a variable so the tests never ask the desktop they run on
var seamCommand = "ryoku"

func seam(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, seamTimeout)
	defer cancel()
	return exec.CommandContext(ctx, seamCommand, append([]string{"wm"}, args...)...).Output()
}

func readState(ctx context.Context) (seamState, bool) {
	var state seamState
	raw, err := seam(ctx, "state")
	if err != nil || json.Unmarshal(raw, &state) != nil {
		return seamState{}, false
	}
	return state, true
}

// nil when there is nothing to do: no seam, or this app is not floating
func seenWindows(ctx context.Context) *windows {
	state, ok := readState(ctx)
	if !ok {
		return nil
	}
	return floatingAmong(state, os.Getpid())
}

func floatingAmong(state seamState, pid int) *windows {
	seen := &windows{known: make(map[string]bool, len(state.Windows))}
	found := false
	for _, window := range state.Windows {
		seen.known[window.ID] = true
		if window.Pid == pid && window.Floating && window.Width > 0 && window.Height > 0 {
			seen.self, found = window, true
		}
	}
	if !found {
		return nil
	}
	for _, output := range state.Outputs {
		if output.Name == seen.self.Output {
			seen.out = output
			return seen
		}
	}
	return nil
}

// the window that was not there before, the focused one when several arrived
func (seen *windows) arrival(state seamState) (seamWindow, bool) {
	var arrived seamWindow
	found := false
	for _, window := range state.Windows {
		if seen.known[window.ID] || window.Pid == seen.self.Pid {
			continue
		}
		if !found || window.FocusOrder < arrived.FocusOrder {
			arrived, found = window, true
		}
	}
	return arrived, found
}

// where a window of size goes: centred on this app's window, kept inside its screen
func (seen *windows) place(size image.Point) image.Rectangle {
	width := min(size.X, seen.out.Width-2*frontMargin)
	height := min(size.Y, seen.out.Height-2*frontMargin)
	x := seen.self.X - seen.out.X + (seen.self.Width-width)/2
	y := seen.self.Y - seen.out.Y + (seen.self.Height-height)/2
	x = max(frontMargin, min(x, seen.out.Width-frontMargin-width))
	y = max(frontMargin, min(y, seen.out.Height-frontMargin-height))
	return image.Rect(x, y, x+width, y+height)
}

func (seen *windows) front(ctx context.Context, size image.Point) {
	deadline := time.Now().Add(arrivalWait)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(arrivalEvery):
		}
		state, ok := readState(ctx)
		if !ok {
			return
		}
		arrived, found := seen.arrival(state)
		if !found {
			continue
		}
		// one the desktop already floats came up in front by itself
		if !arrived.Floating {
			at := seen.place(size)
			_, _ = seam(ctx, "act", "window.place", arrived.ID,
				strconv.Itoa(at.Min.X), strconv.Itoa(at.Min.Y), strconv.Itoa(at.Dx()), strconv.Itoa(at.Dy()), seen.out.Name)
		}
		_, _ = seam(ctx, "act", "window.focus", arrived.ID)
		return
	}
}
