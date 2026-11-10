package desktop

import (
	"image"
	"testing"
)

func desk(own seamWindow, others ...seamWindow) seamState {
	return seamState{
		Outputs: []seamOutput{{Name: "left", Width: 1920, Height: 1080}, {Name: "right", X: 1920, Width: 1440, Height: 900}},
		Windows: append([]seamWindow{own}, others...),
	}
}

// Only a floating Rashin covers what it opens. Tiled, the new window lands
// beside it and is left alone.
func TestAWindowIsPlacedOnlyWhenThisAppFloats(t *testing.T) {
	tiled := seamWindow{ID: "1", Pid: 42, Output: "right", Width: 1280, Height: 820}
	if seen := floatingAmong(desk(tiled), 42); seen != nil {
		t.Fatal("a tiled app asked for the windows it opens to be placed")
	}
	floating := tiled
	floating.Floating, floating.X, floating.Y = true, 2000, 40
	if seen := floatingAmong(desk(floating), 7); seen != nil {
		t.Fatal("another program's floating window was taken for this app's")
	}
	seen := floatingAmong(desk(floating), 42)
	if seen == nil || seen.out.Name != "right" || seen.self.ID != "1" {
		t.Fatalf("a floating app was read as %#v", seen)
	}
}

// The window to place is the one that was not there before the launch. This
// app's own later windows are never it, and of several arrivals the focused
// one is what the user just asked for.
func TestTheArrivalIsTheNewWindowInFocus(t *testing.T) {
	own := seamWindow{ID: "1", Pid: 42, Output: "left", Floating: true, Width: 1280, Height: 820}
	old := seamWindow{ID: "2", Pid: 9, FocusOrder: 1}
	seen := floatingAmong(desk(own, old), 42)
	if _, found := seen.arrival(desk(own, old)); found {
		t.Fatal("a window that stood before the launch was taken for a new one")
	}
	later := desk(own, old,
		seamWindow{ID: "3", Pid: 42, FocusOrder: 0},
		seamWindow{ID: "4", Pid: 11, FocusOrder: 3},
		seamWindow{ID: "5", Pid: 12, FocusOrder: 2},
	)
	arrived, found := seen.arrival(later)
	if !found || arrived.ID != "5" {
		t.Fatalf("the arrival was read as %#v (%v)", arrived, found)
	}
}

// The new window is centred on this app's window, in the coordinates of the
// screen that window is on, and never hangs off that screen.
func TestAnArrivalIsCentredOnThisAppAndKeptOnItsScreen(t *testing.T) {
	own := seamWindow{ID: "1", Pid: 42, Output: "right", Floating: true, X: 1920 + 80, Y: 40, Width: 1280, Height: 820}
	seen := floatingAmong(desk(own), 42)
	if got, want := seen.place(image.Pt(900, 560)), image.Rect(270, 170, 1170, 730); got != want {
		t.Fatalf("placed at %v, want %v", got, want)
	}
	// larger than the screen allows: it shrinks to fit inside the margin
	if got, want := seen.place(image.Pt(3000, 2000)), image.Rect(24, 24, 1416, 876); got != want {
		t.Fatalf("an oversized window was placed at %v, want %v", got, want)
	}
	// this app pushed into a corner: the window stays whole on the screen
	own.X, own.Y, own.Width, own.Height = 1920+1100, 600, 400, 300
	seen = floatingAmong(desk(own), 42)
	if got, want := seen.place(image.Pt(900, 560)), image.Rect(516, 316, 1416, 876); got != want {
		t.Fatalf("a window near the corner was placed at %v, want %v", got, want)
	}
}
