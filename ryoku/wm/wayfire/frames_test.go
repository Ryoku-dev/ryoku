package main

import (
	"encoding/json"
	"strconv"
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
	ids := map[string]bool{}
	for _, w := range ws {
		ids[w.ID] = true
		_, x, y, _ := parseWorkspaceHandle(w.ID)
		if want := y*3 + x + 1; w.Index != want {
			t.Errorf("workspace %s index = %d, want %d", w.ID, w.Index, want)
		}
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
	if got := s.outputFrame(s.outputs[0]).ActiveWorkspace; got != "1:1:0" {
		t.Errorf("active output workspace is %q, want handle 1:1:0", got)
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
	for _, win := range wins {
		if !ids[win.Workspace] {
			t.Errorf("window %s workspace %q does not match a workspace id", win.ID, win.Workspace)
		}
	}
	if wins[0].Workspace != "1:1:0" {
		t.Errorf("current-cell window maps to workspace %q, want handle 1:1:0", wins[0].Workspace)
	}
}

func TestScratchSetIsOneSpecialWorkspace(t *testing.T) {
	index := scratchWsetBase + 1
	s := &stage{
		outputs: []wayfireOutput{{
			ID: 1, Name: "HEADLESS-1", WsetIndex: index,
			Geometry:  wayfireGeometry{Width: 1280, Height: 720},
			Workspace: wayfireWorkspace{X: 2, Y: 1, GridWidth: 5, GridHeight: 2},
		}},
		wsets: []wayfireWset{{
			Index: index, OutputID: 1, OutputName: "HEADLESS-1",
			Workspace: wayfireWorkspace{X: 2, Y: 1, GridWidth: 5, GridHeight: 2},
		}},
		views: []wayfireView{{
			ID: 9, OutputName: "HEADLESS-1", WsetIndex: index,
			Geometry: wayfireGeometry{X: 10, Y: 10, Width: 400, Height: 300},
		}},
	}
	workspaces := s.workspaceFrame()
	if len(workspaces) != 1 {
		t.Fatalf("scratch set rendered %d workspace pills, want 1", len(workspaces))
	}
	got := workspaces[0]
	if got.ID != workspaceHandle(index, 0, 0) || got.Name != "scratchpad" ||
		!got.Special || !got.Active || got.Windows != 1 {
		t.Errorf("scratch workspace = %+v", got)
	}
	if active := s.outputFrame(s.outputs[0]).ActiveWorkspace; active != got.ID {
		t.Errorf("scratch output active workspace = %q, want %q", active, got.ID)
	}
	windows := s.windowFrame()
	if len(windows) != 1 || windows[0].Workspace != got.ID {
		t.Errorf("scratch window workspace = %+v, want %q", windows, got.ID)
	}
}

func TestDetachedSetsListOnlyWhileHoldingWindows(t *testing.T) {
	grid := wayfireWorkspace{GridWidth: 5, GridHeight: 2}
	s := &stage{
		outputs: []wayfireOutput{{
			ID: 3, Name: "eDP-1", WsetIndex: 3, Workspace: grid,
			Geometry: wayfireGeometry{Width: 2560, Height: 1600},
		}},
		wsets: []wayfireWset{
			{Index: 1, OutputID: -1, Workspace: grid},
			{Index: 2, OutputID: -1, Workspace: grid},
			{Index: 3, OutputID: 3, OutputName: "eDP-1", Workspace: grid},
		},
		views: []wayfireView{{
			ID: 7, WsetIndex: 2, Geometry: wayfireGeometry{X: 10, Y: 10, Width: 400, Height: 300},
		}},
	}
	sets := map[string]int{}
	for _, w := range s.workspaceFrame() {
		set, _, _, _ := parseWorkspaceHandle(w.ID)
		sets[strconv.FormatInt(set, 10)]++
	}
	if sets["1"] != 0 || sets["2"] != 10 || sets["3"] != 10 {
		t.Errorf("pills per set = %v, want the empty detached set dropped and the others whole", sets)
	}
}

func TestRectIgnoresDecorExtent(t *testing.T) {
	geometry := wayfireGeometry{X: 471, Y: 209, Width: 1509, Height: 1069}
	shadowed := wayfireGeometry{X: 426, Y: 169, Width: 1599, Height: 1159}
	if got := (wayfireView{Geometry: geometry, BBox: shadowed}).rect(); got != geometry {
		t.Errorf("rect = %+v, want the window geometry without its shadow", got)
	}
	if got := (wayfireView{Geometry: geometry, BBox: shadowed, Fullscreen: true}).rect(); got != shadowed {
		t.Errorf("fullscreen rect = %+v, want the rendered bbox", got)
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

func TestOutputFrameReadsMirrorAndHDR(t *testing.T) {
	s := &stage{
		outputs: []wayfireOutput{{
			ID: 1, Name: "HDMI-A-1", WsetIndex: 1,
			Geometry:  wayfireGeometry{Width: 1920, Height: 1080},
			Workspace: wayfireWorkspace{GridWidth: 5, GridHeight: 2},
		}},
		config: map[string]map[string]configOption{
			"output:HDMI-A-1": {
				"mode": {Value: json.RawMessage(`"mirror eDP-1"`)},
				"hdr":  {Value: json.RawMessage(`"true"`)},
			},
		},
	}
	got := s.outputFrame(s.outputs[0])
	if got.Mirror != "eDP-1" || got.Mode != "" || got.ColorMode != "hdr" {
		t.Errorf("mirror/HDR output frame = %+v", got)
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

func TestOutputDetailsFillDisplayEditorState(t *testing.T) {
	base := []wm.Output{{Name: "eDP-1", Width: 2560, Height: 1600, Scale: 1}}
	var detail randrOutput
	detail.Name = "eDP-1"
	detail.Make = "Thermotrex Corporation"
	detail.Model = "TL140ADXP02-0"
	detail.Enabled = true
	detail.PhysicalSize.Width = 300
	detail.Position.X = 20
	detail.Position.Y = 30
	detail.Transform = "flipped-90"
	detail.Scale = 1.25
	detail.AdaptiveSync = true
	detail.Modes = []randrMode{
		{Width: 2560, Height: 1600, Refresh: 60.001999, Preferred: true, Current: true},
		{Width: 2560, Height: 1600, Refresh: 165.001999},
	}
	got := enrichOutputs(base, []randrOutput{detail})
	if len(got) != 1 {
		t.Fatalf("outputs = %+v", got)
	}
	out := got[0]
	if out.Make != detail.Make || out.Model != detail.Model || out.PhysicalWidth != 300 {
		t.Errorf("identity = %+v", out)
	}
	if out.Mode != "2560x1600@60.001999" || len(out.Modes) != 2 {
		t.Errorf("modes = %q %+v", out.Mode, out.Modes)
	}
	if out.X != 20 || out.Y != 30 || out.Scale != 1.25 || out.Transform != 5 || !out.VRR {
		t.Errorf("layout = %+v", out)
	}
}
