package window

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func seamCapsFrom(t *testing.T, file string) seamCaps {
	t.Helper()
	b, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var c seamCaps
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSeamOnNiriConvertsGlobalGeometryAndRefusesTiled(t *testing.T) {
	d := newDesk(map[string]string{"ryoku wm state": "seam/state-niri.json"})
	p := newSeam(seamCapsFrom(t, "seam/caps-niri.json"), d.run, d.stream)
	if c := p.Caps(); c.Geometry != GeomFloating || c.Provider != "ryoku" || c.Pointer {
		t.Fatalf("caps %+v: the niri seam places floating windows only and has no read-only pointer verb", c)
	}
	s, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	calc := byID(t, s, "17")
	if !calc.Focused || calc.FocusOrder != 0 {
		t.Fatalf("calculator %+v", calc)
	}
	wantBox(t, calc, Box{X: 1182, Y: 212, W: 360, H: 620})
	// Global x 2100 on an output whose origin is 1920.
	picker := byID(t, s, "30")
	wantBox(t, picker, Box{X: 180, Y: 300, W: 420, H: 300})
	if picker.Fullscreen {
		t.Fatalf("a small window on a workspace holding a fullscreen window is not itself fullscreen")
	}
	wantUnplaced(t, byID(t, s, "12"), UnplacedTiled)
	wantUnplaced(t, byID(t, s, "21"), UnplacedTiled)
	wantUnplaced(t, byID(t, s, "25"), UnplacedOffView)
	if g, ok := Global(picker, s.Outputs); !ok || g != (Box{X: 2100, Y: 300, W: 420, H: 300}) {
		t.Fatalf("global %+v %v", g, ok)
	}
}

func TestSeamOnHyprlandPlacesEveryWindow(t *testing.T) {
	d := newDesk(map[string]string{"ryoku wm state": "seam/state-hyprland.json"})
	p := newSeam(seamCapsFrom(t, "seam/caps-hyprland.json"), d.run, d.stream)
	if p.Caps().Geometry != GeomAll {
		t.Fatalf("caps %+v", p.Caps())
	}
	s, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	term := byID(t, s, "0x5a1e2b4f0a10")
	wantBox(t, term, Box{X: 0, Y: 0, W: 2560, H: 1440})
	if !term.Fullscreen || !term.Focused || term.Output != "DP-1" {
		t.Fatalf("terminal %+v", term)
	}
	wantBox(t, byID(t, s, "0x5a1e2b3c4d50"), Box{X: 12, Y: 52, W: 944, H: 1136})
}

func TestSeamWatch(t *testing.T) {
	d := newDesk(map[string]string{})
	d.streams["ryoku wm watch windows outputs workspaces focus"] = "seam/watch-niri.jsonl"
	p := newSeam(seamCapsFrom(t, "seam/caps-niri.json"), d.run, d.stream)
	snaps := collect(t, p)
	if len(snaps) != 3 {
		t.Fatalf("%d snapshots, want 3: nothing before ready, focus-output frames emit nothing", len(snaps))
	}
	var tr Tracker
	tr.Track("17")
	want := [][]string{nil, {ChangeOutput, ChangeMoved}, {ChangeClosed, ChangeFocus}}
	for i, s := range snaps {
		if got := kinds(tr.Update(s)); !slices.Equal(got, want[i]) {
			t.Fatalf("snapshot %d: %v, want %v", i, got, want[i])
		}
	}
	wantBox(t, byID(t, snaps[1], "17"), Box{X: 180, Y: 212, W: 360, H: 620})
	if len(d.calls) != 1 {
		t.Fatalf("calls %v: the seam stream is the only read", d.calls)
	}
}
