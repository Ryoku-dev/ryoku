package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	wm "ryoku-wm"
)

func TestParseRandrKeepsExactRefresh(t *testing.T) {
	text := `DP-1 "Acme" "Panel"
  Enabled: yes
  Position: 1920,0
  Transform: normal
  Modes:
    2560x1440 px, 143.999001 Hz (current, preferred)
    1920x1080 px, 60.000000 Hz
`
	detail, ok := parseRandr(text, "DP-1")
	if !ok {
		t.Fatal("DP-1 was not parsed")
	}
	want := []string{"2560x1440@143.999001", "1920x1080@60"}
	if len(detail.modes) != len(want) {
		t.Fatalf("modes = %v, want %v", detail.modes, want)
	}
	for i := range want {
		if detail.modes[i] != want[i] {
			t.Fatalf("modes = %v, want %v", detail.modes, want)
		}
	}
	if detail.mode != want[0] {
		t.Errorf("current mode = %q, want %q", detail.mode, want[0])
	}
	if !parseAdvertisedModes(text)["DP-1"][want[0]] {
		t.Errorf("advertised modes lost exact refresh: %v", parseAdvertisedModes(text))
	}
}

func TestMonitorsConfOmitsZeroRefresh(t *testing.T) {
	conf := string(monitorsConf([]wm.OutputLayout{
		{Name: "DP-1", Enabled: true, Mode: "1920x1080@144"},
		{Name: "HDMI-A-1", Enabled: true, Mode: "1920x1080@0"},
		{Name: "eDP-1", Enabled: true},
	}))
	if !strings.Contains(conf, "name:^DP-1$,width:1920,height:1080,refresh:144") {
		t.Fatalf("valid refresh was not emitted:\n%s", conf)
	}
	for _, line := range strings.Split(conf, "\n") {
		if (strings.Contains(line, "HDMI-A-1") || strings.Contains(line, "eDP-1")) && strings.Contains(line, "refresh:") {
			t.Errorf("automatic mode emitted a refresh field: %s", line)
		}
	}
	if strings.Contains(conf, "refresh:0") {
		t.Errorf("zero refresh was emitted:\n%s", conf)
	}
}

func TestOutputsSurfacesReloadFailure(t *testing.T) {
	home := sandbox(t)
	layoutPath := filepath.Join(home, "layout.json")
	if err := os.WriteFile(layoutPath, []byte(`[{"name":"DP-1","enabled":true,"mode":"1920x1080@144"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	prevRandr := runRandr
	runRandr = func() ([]byte, error) {
		return []byte("DP-1\n  Modes:\n    1920x1080 px, 144.000000 Hz (current)\n"), nil
	}
	defer func() { runRandr = prevRandr }()

	restore := stubRequest(t, func(cmd string) (json.RawMessage, error) {
		if cmd != "dispatch reload_config" {
			t.Fatalf("reload command = %q", cmd)
		}
		return nil, errors.New("reload rejected")
	})
	defer restore()

	err := runOutputs([]string{layoutPath})
	if err == nil || !strings.Contains(err.Error(), "reload rejected") {
		t.Fatalf("reload failure was hidden: %v", err)
	}
}

func TestSnapshotHasOneFocusedWindowAndIPCFallbackGeometry(t *testing.T) {
	prevRandr := runRandr
	runRandr = func() ([]byte, error) { return nil, errors.New("wlr-randr unavailable") }
	defer func() { runRandr = prevRandr }()

	restore := stubRequest(t, func(cmd string) (json.RawMessage, error) {
		switch cmd {
		case "get all-monitors":
			return json.RawMessage(`{"monitors":[{"name":"DP-1","active":true,"x":320,"y":180,"width":1920,"height":1080,"scale":1.25,"active_client":{"id":8},"tags":[{"index":1,"is_active":true,"layout":"T","client_count":3}]}]}`), nil
		case "get all-clients":
			return json.RawMessage(`{"clients":[{"id":12,"monitor":"DP-1","tags":[1]},{"id":8,"monitor":"DP-1","tags":[1]},{"id":3,"monitor":"DP-1","tags":[1]}]}`), nil
		case "get layouts":
			return json.RawMessage(`{"layouts":[{"symbol":"T","name":"tile"}]}`), nil
		case "get keyboardlayout":
			return json.RawMessage(`{"layout":"English (US)"}`), nil
		default:
			t.Fatalf("unexpected request %q", cmd)
			return nil, nil
		}
	})
	defer restore()

	snap, err := snapshot()
	if err != nil {
		t.Fatal(err)
	}
	zeroes := 0
	orders := map[string]int{}
	for _, window := range snap.Windows {
		orders[window.ID] = window.FocusOrder
		if window.FocusOrder == 0 {
			zeroes++
		}
	}
	if zeroes != 1 || orders["8"] != 0 || orders["3"] != 1 || orders["12"] != 2 {
		t.Errorf("focus orders = %v, want client 8 focused and numeric-id fallback order", orders)
	}
	if len(snap.Outputs) != 1 {
		t.Fatalf("outputs = %v", snap.Outputs)
	}
	out := snap.Outputs[0]
	if out.X != 320 || out.Y != 180 || out.Width != 1920 || out.Height != 1080 || out.Scale != 1.25 {
		t.Errorf("IPC geometry fallback lost: %+v", out)
	}
}

func TestTagZeroUsesSpecialWorkspaceHandle(t *testing.T) {
	window := clientFrame(mangoClient{ID: 7, Monitor: "DP-1", Tags: []int{0}}, 1, map[string]int{"DP-1": 1})
	if window.Workspace != "0" {
		t.Errorf("tag-0 workspace = %q, want mango's special handle 0", window.Workspace)
	}
}

func TestSpecialWorkspacesMatchAcrossReadPaths(t *testing.T) {
	monitors := []mangoMonitor{
		{
			Name:         "DP-1",
			Active:       true,
			LayoutSymbol: "T",
			Tags: []mangoTag{
				{Index: 1, IsActive: true, Layout: "T", ClientCount: 2},
				{Index: 2, Layout: "S"},
				{Index: 3, Layout: "T", ClientCount: 1},
			},
		},
		{
			Name:         "HDMI-A-1",
			LayoutSymbol: "S",
			Tags: []mangoTag{
				{Index: 1, Layout: "S"},
				{Index: 2, IsActive: true, Layout: "S", ClientCount: 2},
				{Index: 3, Layout: "T"},
			},
		},
	}
	clients := []mangoClient{{
		ID:        7,
		Monitor:   "DP-1",
		Tags:      []int{0},
		IsVisible: true,
		IsUrgent:  true,
	}}
	layouts := map[string]string{"T": "tile", "S": "scroller"}
	watchFrames := (&session{monitors: monitors, clients: clients, layouts: layouts}).workspaceFrames()

	monitorJSON, err := json.Marshal(monitorsEnvelope{Monitors: monitors})
	if err != nil {
		t.Fatal(err)
	}
	clientJSON, err := json.Marshal(clientsEnvelope{Clients: clients})
	if err != nil {
		t.Fatal(err)
	}
	prevRandr := runRandr
	runRandr = func() ([]byte, error) { return nil, errors.New("wlr-randr unavailable") }
	defer func() { runRandr = prevRandr }()
	restore := stubRequest(t, func(cmd string) (json.RawMessage, error) {
		switch cmd {
		case "get all-monitors":
			return monitorJSON, nil
		case "get all-clients":
			return clientJSON, nil
		case "get layouts":
			return json.RawMessage(`{"layouts":[{"symbol":"T","name":"tile"},{"symbol":"S","name":"scroller"}]}`), nil
		case "get keyboardlayout":
			return json.RawMessage(`{"layout":"English (US)"}`), nil
		default:
			t.Fatalf("unexpected request %q", cmd)
			return nil, nil
		}
	})
	defer restore()

	snap, err := snapshot()
	if err != nil {
		t.Fatal(err)
	}
	want := []wm.Workspace{
		{ID: "1", Name: "1", Output: "DP-1", Active: true, Windows: 2, Layout: "tile"},
		{ID: "2", Name: "2", Output: "DP-1", Layout: "scroller"},
		{ID: "3", Name: "3", Output: "DP-1", Windows: 1, Layout: "tile"},
		{ID: "0", Name: "0", Output: "DP-1", Active: true, Windows: 1, Special: true, Layout: "tile"},
		{ID: "1", Name: "1", Output: "HDMI-A-1", Layout: "scroller"},
		{ID: "2", Name: "2", Output: "HDMI-A-1", Active: true, Windows: 2, Layout: "scroller"},
		{ID: "3", Name: "3", Output: "HDMI-A-1", Layout: "tile"},
		{ID: "0", Name: "0", Output: "HDMI-A-1", Special: true, Layout: "scroller"},
	}
	for _, tc := range []struct {
		name string
		got  []wm.Workspace
	}{
		{name: "watch", got: watchFrames},
		{name: "snapshot", got: snap.Workspaces},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !reflect.DeepEqual(tc.got, want) {
				t.Errorf("workspaces = %#v, want %#v", tc.got, want)
			}
		})
	}
}

func TestLayerRuleInheritDiffersFromOff(t *testing.T) {
	var inherit strings.Builder
	var off strings.Builder
	writeLayerRules(&inherit, []LayerRule{{Namespace: "panel", Blur: "inherit", Shadow: "inherit"}})
	writeLayerRules(&off, []LayerRule{{Namespace: "panel", Blur: "off", Shadow: "off"}})

	if strings.Contains(inherit.String(), "noblur") || strings.Contains(inherit.String(), "noshadow") {
		t.Errorf("inherit emitted disabling fields: %q", inherit.String())
	}
	if !strings.Contains(off.String(), "noblur:1") || !strings.Contains(off.String(), "noshadow:1") {
		t.Errorf("off did not emit disabling fields: %q", off.String())
	}
	if inherit.String() == off.String() {
		t.Errorf("inherit and off rendered identically: %q", inherit.String())
	}
}
