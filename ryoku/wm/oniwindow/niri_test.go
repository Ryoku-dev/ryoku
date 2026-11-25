package window

import (
	"context"
	"slices"
	"testing"
)

func niriDesk() *fakeDesk {
	d := newDesk(map[string]string{
		"niri msg --json windows":    "niri/windows.json",
		"niri msg --json workspaces": "niri/workspaces.json",
		"niri msg --json outputs":    "niri/outputs.json",
	})
	d.streams["niri msg --json event-stream"] = "niri/event-stream.jsonl"
	return d
}

func TestNiriSnapshotPlacesFloatingWindowsOnly(t *testing.T) {
	d := niriDesk()
	p := newNiri(d.run, d.stream)
	s, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantOutputs := []Output{
		{Name: "eDP-1", X: 0, Y: 0, W: 1920, H: 1200, Scale: 1.5},
		{Name: "DP-1", X: 1920, Y: 0, W: 2560, H: 1440, Scale: 1},
	}
	if !slices.Equal(s.Outputs, wantOutputs) {
		t.Fatalf("outputs %+v, want %+v (a disabled output has no logical rect and is left out)", s.Outputs, wantOutputs)
	}

	calc := byID(t, s, "17")
	if !calc.Focused || calc.FocusOrder != 0 || !calc.Floating || !calc.Visible || calc.Output != "eDP-1" || calc.App != "org.gnome.Calculator" || calc.PID != 4377 {
		t.Fatalf("calculator %+v", calc)
	}
	// The window sits inside its tile by the tile's border offset.
	wantBox(t, calc, Box{X: 1182, Y: 212, W: 360, H: 620})
	// Workspace-view coordinates are already output-local on the second output.
	wantBox(t, byID(t, s, "30"), Box{X: 180, Y: 300, W: 420, H: 300})

	editor := byID(t, s, "12")
	wantUnplaced(t, editor, UnplacedTiled)
	if !editor.Visible || editor.Output != "eDP-1" {
		t.Fatalf("a tiled window is still shown and named on its output: %+v", editor)
	}
	wantUnplaced(t, byID(t, s, "21"), UnplacedTiled)

	pip := byID(t, s, "25")
	if pip.Visible {
		t.Fatalf("a window on an inactive workspace is not visible: %+v", pip)
	}
	wantUnplaced(t, pip, UnplacedOffView)

	order := []ID{}
	for _, w := range s.Windows {
		order = append(order, w.ID)
	}
	if want := []ID{"17", "30", "12", "21", "25"}; !slices.Equal(order, want) {
		t.Fatalf("focus history %v, want %v (focused first, then newest focus timestamp)", order, want)
	}
	if c := p.Caps(); c.Geometry != GeomFloating || c.Pointer || !c.Events || c.Provider != ProviderNiri {
		t.Fatalf("caps %+v", c)
	}
}

// A niri that reports a tiled window's place in the workspace view is used
// as reported, with no version check, and the provider then claims all
// geometry.
func TestNiriUsesTiledPositionWhenReported(t *testing.T) {
	d := niriDesk()
	d.set("niri msg --json windows", "niri/windows-tiled-placed.json")
	p := newNiri(d.run, d.stream)
	s, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantBox(t, byID(t, s, "12"), Box{X: 18, Y: 26, W: 940, H: 1148})
	if p.Caps().Geometry != GeomAll {
		t.Fatalf("caps %+v after a placed tiled window", p.Caps())
	}
}

func TestNiriWatchFoldsEventsIntoChanges(t *testing.T) {
	d := niriDesk()
	p := newNiri(d.run, d.stream)
	p.Geometry("12") // events carry layout changes; nothing to sample
	snaps := collect(t, p)
	if len(snaps) != 7 {
		for i, s := range snaps {
			t.Logf("%d: %+v", i, s.Windows)
		}
		t.Fatalf("%d snapshots, want 7: initial, move, resize, focus, hide, show, close (focus timestamps and unrelated events emit nothing)", len(snaps))
	}
	wantBox(t, byID(t, snaps[0], "17"), Box{X: 1182, Y: 212, W: 360, H: 620})

	var tr Tracker
	tr.Track("17")
	want := [][]string{nil, {ChangeMoved}, {ChangeResized}, {ChangeFocus}, {ChangeHidden, ChangeMoved}, {ChangeMoved}, {ChangeClosed}}
	for i, s := range snaps {
		got := tr.Update(s)
		if !slices.Equal(kinds(got), want[i]) {
			t.Fatalf("snapshot %d: changes %v, want %v", i, kinds(got), want[i])
		}
		if i == 3 && got[0].Window.ID != "12" {
			t.Fatalf("focus change names %q, want the newly focused window 12", got[0].Window.ID)
		}
	}
	wantBox(t, byID(t, snaps[2], "17"), Box{X: 1242, Y: 262, W: 400, H: 620})

	// Outputs are read before the stream and again when the workspace set
	// changes; that, the stream and nothing else.
	want2 := [][]string{
		{"niri", "msg", "--json", "outputs"},
		{"niri", "msg", "--json", "event-stream"},
		{"niri", "msg", "--json", "outputs"},
	}
	if !slices.EqualFunc(d.calls, want2, slices.Equal) {
		t.Fatalf("calls %v, want %v", d.calls, want2)
	}
}
