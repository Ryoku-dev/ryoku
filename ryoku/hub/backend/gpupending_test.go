package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNextGpuModeMarker(t *testing.T) {
	cases := []struct {
		name           string
		old            gpuModeMarker
		have           bool
		prev, mode     string
		wantNil        bool
		wantFrom, want string
	}{
		{"first change is pending", gpuModeMarker{}, false, "hybrid", "performance", false, "hybrid", "performance"},
		{"choosing the running mode again is not", gpuModeMarker{}, false, "hybrid", "hybrid", true, "", ""},
		{"a second change keeps where the session started",
			gpuModeMarker{From: "hybrid", To: "performance", Session: "s1"}, true, "performance", "passthrough", false, "hybrid", "passthrough"},
		{"changing back to the running mode clears it",
			gpuModeMarker{From: "hybrid", To: "performance", Session: "s1"}, true, "performance", "hybrid", true, "", ""},
		{"a marker from an older session is ignored",
			gpuModeMarker{From: "passthrough", To: "hybrid", Session: "s0"}, true, "performance", "hybrid", false, "performance", "hybrid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := nextGpuModeMarker(c.old, c.have, c.prev, c.mode, "aq", "s1")
			if c.wantNil {
				if got != nil {
					t.Fatalf("got %+v, want nothing pending", *got)
				}
				return
			}
			if got == nil || got.From != c.wantFrom || got.To != c.want || got.Session != "s1" || got.AQ != "aq" {
				t.Fatalf("got %+v, want from %s to %s", got, c.wantFrom, c.want)
			}
		})
	}
}

func TestGpuModePending(t *testing.T) {
	m := gpuModeMarker{From: "hybrid", To: "performance", AQ: "/dev/dri/x", Session: "s1"}
	if !gpuModePending(m, true, "s1", "/dev/dri/x") {
		t.Error("same session and pin must be pending")
	}
	if gpuModePending(m, true, "s2", "/dev/dri/x") {
		t.Error("a new session has applied the mode")
	}
	if gpuModePending(m, true, "s1", "") {
		t.Error("a pin rewritten outside Settings makes the marker stale")
	}
	if gpuModePending(m, false, "s1", "/dev/dri/x") {
		t.Error("no marker, nothing pending")
	}
}

func TestRecordGpuModeChangeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("RYOKU_GPU_CONF", filepath.Join(dir, "gpu.lua"))
	t.Setenv("WAYLAND_DISPLAY", "")
	if err := recordGpuModeChange("hybrid", "performance"); err != nil {
		t.Fatal(err)
	}
	m, have := readGpuModeMarker(gpuModeMarkerPath())
	if !have || m.To != "performance" || m.From != "hybrid" {
		t.Fatalf("marker %+v", m)
	}
	if !gpuModePending(m, have, gpuSessionID(), readAQValue(gpuLuaPath())) {
		t.Fatal("a fresh change must read as pending")
	}
	if err := recordGpuModeChange("performance", "hybrid"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gpuModeMarkerPath()); !os.IsNotExist(err) {
		t.Fatal("returning to the running mode must remove the marker")
	}
}
