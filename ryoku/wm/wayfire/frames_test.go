package main

import (
	"testing"

	wm "ryoku-wm"
)

// The cell derivation is the only place this provider invents a handle the
// compositor never sent, so its arithmetic is pinned against the values seen
// live: a view reads x=173 on its home cell and x=-1107 after the current
// workspace moves to (1,0) on a 1280-wide output.
func TestCellOf(t *testing.T) {
	out := wayfireOutput{
		Name:     "HEADLESS-1",
		Geometry: wayfireGeometry{X: 0, Y: 0, Width: 1280, Height: 720},
	}
	grid := wayfireWorkspace{GridWidth: 3, GridHeight: 3}

	cases := []struct {
		name string
		geo  wayfireGeometry
		cur  wayfireWorkspace
		want [2]int
	}{
		{"at home", wayfireGeometry{X: 173, Y: 0}, wayfireWorkspace{X: 0, Y: 0}, [2]int{0, 0}},
		{"translated by navigation", wayfireGeometry{X: -1107, Y: 0}, wayfireWorkspace{X: 1, Y: 0}, [2]int{0, 0}},
		{"on the current cell", wayfireGeometry{X: 100, Y: 50}, wayfireWorkspace{X: 1, Y: 0}, [2]int{1, 0}},
		{"cell to the right", wayfireGeometry{X: 1300, Y: 0}, wayfireWorkspace{X: 0, Y: 0}, [2]int{1, 0}},
		{"below", wayfireGeometry{X: 0, Y: 721}, wayfireWorkspace{X: 0, Y: 0}, [2]int{0, 1}},
		{"translated down", wayfireGeometry{X: 0, Y: -719}, wayfireWorkspace{X: 0, Y: 1}, [2]int{0, 0}},
		{"straddles the left edge", wayfireGeometry{X: -1, Y: 0}, wayfireWorkspace{X: 1, Y: 0}, [2]int{0, 0}},
		{"corner outside the grid clamps in", wayfireGeometry{X: -99999, Y: 99999}, wayfireWorkspace{X: 1, Y: 1}, [2]int{0, 2}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			x, y := cellOf(wayfireView{Geometry: c.geo}, out, c.cur, grid.GridWidth, grid.GridHeight)
			if x != c.want[0] || y != c.want[1] {
				t.Errorf("cellOf(%v at %v) = (%d,%d), want (%d,%d)", c.geo, c.cur, x, y, c.want[0], c.want[1])
			}
		})
	}
}

// The output geometry is the anchor, so a window on a second monitor derives
// against that monitor's origin rather than the global one.
func TestCellOfMultiOutput(t *testing.T) {
	right := wayfireOutput{
		Name:     "HDMI-A-1",
		Geometry: wayfireGeometry{X: 1920, Y: 0, Width: 1920, Height: 1080},
	}
	v := wayfireView{Geometry: wayfireGeometry{X: 1920 + 1921, Y: 10}}
	x, y := cellOf(v, right, wayfireWorkspace{X: 0, Y: 0}, 3, 3)
	if x != 1 || y != 0 {
		t.Errorf("got (%d,%d), want (1,0)", x, y)
	}
}

// The frames are the contract: ids round-trip, the active cell is the one an
// output shows, and occupancy counts windows by their derived cell.
func TestWorkspaceAndWindowFrames(t *testing.T) {
	s := &stage{
		outputs: []wayfireOutput{{
			ID: 1, Name: "HEADLESS-1",
			Geometry:  wayfireGeometry{X: 0, Y: 0, Width: 1280, Height: 720},
			WsetIndex: 1,
			Workspace: wayfireWorkspace{X: 1, Y: 0, GridWidth: 3, GridHeight: 3},
		}},
		wsets: []wayfireWset{{
			Index: 1, Name: "set", OutputID: 1, OutputName: "HEADLESS-1",
			Workspace: wayfireWorkspace{X: 1, Y: 0, GridWidth: 3, GridHeight: 3},
		}},
		views: []wayfireView{
			// Translated off to the left: home sits on cell (0,0) while the
			// set shows (1,0).
			{ID: 1, Title: "home", OutputName: "HEADLESS-1", WsetIndex: 1,
				Geometry:           wayfireGeometry{X: -1107, Y: 10, Width: 400, Height: 300},
				LastFocusTimestamp: 200},
			// At x=10 the view sits on the current cell (1,0).
			{ID: 2, Title: "right", OutputName: "HEADLESS-1", WsetIndex: 1,
				Geometry:  wayfireGeometry{X: 10, Y: 10, Width: 400, Height: 300},
				Activated: true, LastFocusTimestamp: 300},
			{ID: 3, Title: "full", OutputName: "HEADLESS-1", WsetIndex: 1,
				Geometry:   wayfireGeometry{X: 10, Y: 10, Width: 1280, Height: 720},
				Fullscreen: true, TiledEdges: 15, LastFocusTimestamp: 100},
		},
		focused: "HEADLESS-1",
	}

	ws := s.workspaceFrame()
	if len(ws) != 9 {
		t.Fatalf("got %d cells, want 9", len(ws))
	}
	active := 0
	counts := map[string]int{}
	for _, w := range ws {
		if w.Active {
			active++
			if w.ID != "1:1:0" {
				t.Errorf("active cell is %q, want 1:1:0", w.ID)
			}
		}
		counts[w.ID] += w.Windows
	}
	if active != 1 {
		t.Errorf("got %d active cells, want exactly 1", active)
	}
	if counts["1:0:0"] != 1 {
		t.Errorf("cell 1:0:0 holds %d windows, want 1 (home)", counts["1:0:0"])
	}
	if counts["1:1:0"] != 2 {
		t.Errorf("cell 1:1:0 holds %d windows, want 2 (right + full)", counts["1:1:0"])
	}
	if ws[0].Name != "1" || ws[1].Name != "2" {
		t.Errorf("cell names are %q,%q, want row-major 1,2", ws[0].Name, ws[1].Name)
	}
	for _, w := range ws {
		if w.Fullscreen && w.ID != "1:1:0" {
			t.Errorf("fullscreen reported on %q, want the cell the view sits on", w.ID)
		}
	}

	wins := s.windowFrame()
	if len(wins) != 3 {
		t.Fatalf("got %d windows, want 3", len(wins))
	}
	if wins[0].ID != "2" || wins[0].FocusOrder != 0 {
		t.Errorf("focused window is %q at order %d, want id 2 at 0", wins[0].ID, wins[0].FocusOrder)
	}
	if wins[1].ID != "1" || wins[2].ID != "3" {
		t.Errorf("focus order is %s,%s want 1,3 by timestamp", wins[1].ID, wins[2].ID)
	}
	if wins[1].Floating != true || wins[2].Floating {
		t.Errorf("floating flags wrong: an untilled window must float, tiled fullscreen must not")
	}
	if wins[0].Workspace != "2" {
		t.Errorf("current-cell window maps to workspace %q, want 2", wins[0].Workspace)
	}
}

// wayfire reports the focused output straight, and an unknown keyboard
// layout reads as no layout rather than the literal word.
func TestFocusedAndKeyboard(t *testing.T) {
	s := &stage{focused: "eDP-1"}
	for _, o := range s.outputsFrame() {
		if !o.Focused {
			t.Errorf("output %s not marked focused", o.Name)
		}
	}
	current, all := wayfireKeyboard{Layout: "unknown", PossibleLayouts: nil}.keyboardPair()
	if current != "" {
		t.Errorf("unknown layout rendered as %q, want empty", current)
	}
	if all == nil {
		t.Error("layouts list must be non-nil so a bar renders an empty menu")
	}
}

// The frame kinds must all exist in the shared contract, or a consumer would
// read a frame the shell never gates on.
func TestFrameKindsAreShared(t *testing.T) {
	for _, k := range []wm.FrameKind{wm.FrameFocus, wm.FrameKeyboard, wm.FrameOutputs, wm.FrameWorkspaces, wm.FrameWindows, wm.FrameReady} {
		if k == "" {
			t.Fatal("empty frame kind")
		}
	}
}
