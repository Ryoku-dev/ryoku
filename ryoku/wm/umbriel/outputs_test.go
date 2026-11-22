package main

import (
	"strings"
	"testing"

	wm "ryoku-wm"
)

func TestMonitorsToml(t *testing.T) {
	layout := []wm.OutputLayout{
		{Name: "eDP-1", Enabled: true, Mode: "1920x1080@60010", Scale: 1, X: 0, Y: 0, VRR: false},
		{Name: "Virtual-1", Enabled: false, Mode: "1024x768@60004", Scale: 1.5, X: 1920, Y: 0, VRR: true},
	}
	got := string(monitorsToml(layout))
	for _, want := range []string{
		`[output."eDP-1"]`,
		`mode = "1920x1080@60.01"`,
		`scale = 1`,
		`position = [0, 0]`,
		`[output."Virtual-1"]`,
		`enabled = false`,
		`scale = 1.5`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered TOML missing %q:\n%s", want, got)
		}
	}
}

func TestOutputsUnhonored(t *testing.T) {
	layout := []wm.OutputLayout{{Name: "eDP-1", Mirror: "HDMI-A-1", ColorMode: "hdr", SdrBrightness: 200}}
	rows := outputsUnhonored(layout)
	if len(rows) != 3 {
		t.Fatalf("got %d unhonored rows, want 3: %+v", len(rows), rows)
	}
}
