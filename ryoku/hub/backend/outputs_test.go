package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// TestPublishGreeterPrimary: the Displays apply records the layout's main output
// where the login greeter can read it. The page's "Set as main" re-bases the
// layout onto the chosen output, so the origin entry wins; a layout that
// predates that convention falls back to the top-left-most enabled output, and
// a disabled output at the origin is not the main.
func TestPublishGreeterPrimary(t *testing.T) {
	file := filepath.Join(t.TempDir(), "greeter-primary")
	t.Setenv("RYOKU_GREETER_PRIMARY_FILE", file)

	read := func() string {
		b, err := os.ReadFile(file)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}

	cases := []struct {
		name   string
		layout []wm.OutputLayout
		want   string
	}{
		{
			name: "origin is main",
			layout: []wm.OutputLayout{
				{Name: "HDMI-A-1", Enabled: true, X: 0, Y: 0},
				{Name: "eDP-1", Enabled: true, X: 1920, Y: 0},
			},
			want: "HDMI-A-1",
		},
		{
			name: "top-left without an origin",
			layout: []wm.OutputLayout{
				{Name: "DP-2", Enabled: true, X: 2560, Y: 0},
				{Name: "DP-1", Enabled: true, X: 1280, Y: 0},
			},
			want: "DP-1",
		},
		{
			name: "a disabled output at the origin is skipped",
			layout: []wm.OutputLayout{
				{Name: "DP-9", Enabled: false, X: 0, Y: 0},
				{Name: "eDP-1", Enabled: true, X: 0, Y: 0},
			},
			want: "eDP-1",
		},
		{
			name:   "nothing enabled publishes nothing",
			layout: []wm.OutputLayout{{Name: "DP-1", Enabled: false}},
			want:   "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Remove(file)
			publishGreeterPrimary(tc.layout)
			if got := read(); got != tc.want {
				t.Fatalf("published %q, want %q", got, tc.want)
			}
		})
	}
}
