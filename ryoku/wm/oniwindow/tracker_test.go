package window

import (
	"slices"
	"testing"

	"ryoku-rashin/internal/oni/spec"
)

func twoOutputs() []Output {
	return []Output{
		{Name: "eDP-1", W: 1920, H: 1200, Scale: 1.5},
		{Name: "DP-1", X: 1920, W: 2560, H: 1440, Scale: 1},
	}
}

func perched() Window {
	return Window{ID: "7", App: "org.gnome.Calculator", Title: "Calculator", Output: "eDP-1",
		Box: &Box{X: 100, Y: 100, W: 400, H: 300}, Floating: true, Visible: true, FocusOrder: 1}
}

func other(focused bool) Window {
	w := Window{ID: "9", App: "kitty", Output: "eDP-1", Box: &Box{X: 0, Y: 0, W: 960, H: 1200}, Visible: true, FocusOrder: 1}
	if focused {
		w.Focused, w.FocusOrder = true, 0
	}
	return w
}

func snap(ws ...Window) Snapshot { return Snapshot{Outputs: twoOutputs(), Windows: ws} }

func TestTrackerClassifiesChanges(t *testing.T) {
	cases := []struct {
		name string
		edit func(w *Window)
		want []string
	}{
		{"unchanged", func(*Window) {}, nil},
		{"move", func(w *Window) { w.Box = &Box{X: 140, Y: 90, W: 400, H: 300} }, []string{ChangeMoved}},
		{"resize", func(w *Window) { w.Box = &Box{X: 100, Y: 100, W: 500, H: 320} }, []string{ChangeResized}},
		{"move and resize", func(w *Window) { w.Box = &Box{X: 0, Y: 0, W: 1, H: 1} }, []string{ChangeMoved, ChangeResized}},
		{"output change", func(w *Window) { w.Output = "DP-1" }, []string{ChangeOutput}},
		{"workspace hide", func(w *Window) { w.Visible = false }, []string{ChangeHidden}},
		{"minimized", func(w *Window) { w.Minimized = true }, []string{ChangeHidden}},
		{"fullscreen", func(w *Window) { w.Fullscreen = true; w.Box = &Box{W: 1920, H: 1200} }, []string{ChangeFullscreen, ChangeMoved, ChangeResized}},
		{"lost placement", func(w *Window) { w.Box, w.Unplaced = nil, UnplacedTiled }, []string{ChangeMoved}},
		{"title only", func(w *Window) { w.Title = "Calculator — Scientific" }, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tr Tracker
			tr.Track("7")
			if got := tr.Update(snap(perched(), other(true))); got != nil {
				t.Fatalf("baseline reported %v", kinds(got))
			}
			w := perched()
			c.edit(&w)
			got := tr.Update(snap(w, other(true)))
			if !slices.Equal(kinds(got), c.want) {
				t.Fatalf("changes %v, want %v", kinds(got), c.want)
			}
			for _, ch := range got {
				if ch.Window.ID != "7" {
					t.Fatalf("change names %q", ch.Window.ID)
				}
			}
		})
	}
}

func TestTrackerFocusAndClose(t *testing.T) {
	var tr Tracker
	tr.Track("7")
	tr.Update(snap(perched(), other(true)))

	focusedPerch := perched()
	focusedPerch.Focused, focusedPerch.FocusOrder = true, 0
	got := tr.Update(snap(focusedPerch, other(false)))
	if !slices.Equal(kinds(got), []string{ChangeFocus}) || got[0].Window.ID != "7" {
		t.Fatalf("focus transition %+v", got)
	}

	// Nothing focused (a layer surface or an empty workspace has focus).
	got = tr.Update(snap(perched(), other(false)))
	if !slices.Equal(kinds(got), []string{ChangeFocus}) || got[0].Window.ID != "" {
		t.Fatalf("focus to nothing %+v", got)
	}

	got = tr.Update(snap(other(true)))
	if !slices.Equal(kinds(got), []string{ChangeClosed, ChangeFocus}) || got[0].Window.ID != "7" || got[0].Window.Title != "Calculator" {
		t.Fatalf("close %+v", got)
	}
	// Closed ends tracking: later snapshots say nothing more about it.
	if got := tr.Update(snap(other(true))); got != nil {
		t.Fatalf("after close %v", kinds(got))
	}

	// Tracking a window that is already gone reports it closed at once.
	tr.Track("404")
	if got := tr.Update(snap(other(true))); !slices.Equal(kinds(got), []string{ChangeClosed}) || got[0].Window.ID != "404" {
		t.Fatalf("missing window %+v", got)
	}

	// The zero tracker reports focus only, from its second snapshot on.
	var idle Tracker
	if got := idle.Update(snap(other(true))); got != nil {
		t.Fatalf("first snapshot %v", kinds(got))
	}
	if got := idle.Update(snap(perched(), other(false))); !slices.Equal(kinds(got), []string{ChangeFocus}) {
		t.Fatalf("idle tracker %v", kinds(got))
	}
}

func TestHitTestPrecedence(t *testing.T) {
	tiledA := Window{ID: "a", Output: "eDP-1", Box: &Box{X: 0, Y: 0, W: 960, H: 1200}, Visible: true, FocusOrder: 2}
	tiledB := Window{ID: "b", Output: "eDP-1", Box: &Box{X: 0, Y: 0, W: 960, H: 1200}, Visible: true, FocusOrder: 1}
	float := Window{ID: "f", Output: "eDP-1", Box: &Box{X: 200, Y: 200, W: 300, H: 200}, Visible: true, Floating: true, FocusOrder: 5}
	second := Window{ID: "s", Output: "DP-1", Box: &Box{X: 0, Y: 0, W: 400, H: 400}, Visible: true, Floating: true, FocusOrder: 3}
	hidden := Window{ID: "h", Output: "eDP-1", Box: &Box{X: 0, Y: 0, W: 1920, H: 1200}, Floating: true, FocusOrder: 0}
	mini := Window{ID: "m", Output: "eDP-1", Box: &Box{X: 0, Y: 0, W: 1920, H: 1200}, Visible: true, Minimized: true, Floating: true}
	gone := Window{ID: "g", Output: "HDMI-A-1", Box: &Box{X: 0, Y: 0, W: 1920, H: 1200}, Visible: true, Floating: true}
	tiledNoBox := Window{ID: "n", Output: "eDP-1", Visible: true, Unplaced: UnplacedTiled}
	s := snap(tiledA, tiledB, float, second, hidden, mini, gone, tiledNoBox)

	var tr Tracker
	cases := []struct {
		at   spec.Point
		want ID
		ok   bool
	}{
		{spec.Point{X: 250, Y: 250}, "f", true}, // floating over tiled, despite older focus
		{spec.Point{X: 50, Y: 50}, "b", true},   // most recently focused tiled window
		{spec.Point{X: 1925, Y: 5}, "s", true},  // global point on the second output
		{spec.Point{X: 959.9, Y: 1199.9}, "b", true},
		{spec.Point{X: 960, Y: 600}, "", false}, // right edge is exclusive
		{spec.Point{X: 2400, Y: 10}, "", false}, // empty desktop on the second output
		{spec.Point{X: -1, Y: 10}, "", false},
	}
	for _, c := range cases {
		w, ok := tr.HitTest(c.at, s)
		if ok != c.ok || w.ID != c.want {
			t.Fatalf("hit at %+v: %q %v, want %q %v", c.at, w.ID, ok, c.want, c.ok)
		}
	}
}
