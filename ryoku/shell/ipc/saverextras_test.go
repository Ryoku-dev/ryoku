package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	wm "ryoku-wm"
)

func TestBrightnessTarget(t *testing.T) {
	tests := []struct {
		name             string
		current, maximum int
		want             int
		change           bool
	}{
		{name: "sixty percent", current: 100, maximum: 100, want: 60, change: true},
		{name: "ten percent floor", current: 12, maximum: 100, want: 10, change: true},
		{name: "already at floor", current: 10, maximum: 100, want: 10, change: false},
		{name: "rounds", current: 99, maximum: 100, want: 59, change: true},
		{name: "invalid maximum", current: 50, maximum: 0, want: 50, change: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, change := brightnessTarget(tt.current, tt.maximum)
			if got != tt.want || change != tt.change {
				t.Fatalf("brightnessTarget(%d, %d) = %d, %v; want %d, %v", tt.current, tt.maximum, got, change, tt.want, tt.change)
			}
		})
	}
}

func TestKeyboardTarget(t *testing.T) {
	tests := []struct {
		current, maximum int
		want             int
		change           bool
	}{
		{current: 3, maximum: 3, want: 1, change: true},
		{current: 1, maximum: 3, want: 1, change: false},
		{current: 2, maximum: 1, want: 2, change: false},
	}
	for _, tt := range tests {
		got, change := keyboardTarget(tt.current, tt.maximum)
		if got != tt.want || change != tt.change {
			t.Fatalf("keyboardTarget(%d, %d) = %d, %v; want %d, %v", tt.current, tt.maximum, got, change, tt.want, tt.change)
		}
	}
}

func TestPanelModePick(t *testing.T) {
	output := wm.Output{
		Name: "eDP-1",
		Mode: "2560x1600@165.002",
		Modes: []string{
			"1920x1080@60",
			"2560x1600@59.940",
			"2560x1600@60.002",
		},
	}
	original, target, ok := panelModePick(output)
	if !ok || original != "165.002" || target != "60.002" {
		t.Fatalf("panelModePick() = %q, %q, %v", original, target, ok)
	}

	output.Name = "DP-1"
	if _, _, ok := panelModePick(output); ok {
		t.Fatal("external output must not be selected")
	}
	output.Name = "eDP-1"
	output.Mode = "2560x1600@75"
	if _, _, ok := panelModePick(output); ok {
		t.Fatal("75 Hz output must not be selected")
	}
	output.Mode = "2560x1600@165"
	output.Modes = []string{"1920x1080@60", "2560x1600@75"}
	if _, _, ok := panelModePick(output); ok {
		t.Fatal("output without a same-resolution 60 Hz mode must not be selected")
	}
}

func TestRestoreOnlyIfUnchanged(t *testing.T) {
	if !shouldRestore(60, 60) {
		t.Fatal("unchanged brightness should restore")
	}
	if shouldRestore(75, 60) {
		t.Fatal("user-changed brightness must not restore")
	}
	if !refreshUnchanged("60", "60.000") {
		t.Fatal("equivalent refresh strings should restore")
	}
	if refreshUnchanged("59.940", "60.000") {
		t.Fatal("changed refresh must not restore")
	}
}

func TestSaverExtrasBrightnessEnterLeave(t *testing.T) {
	root := t.TempDir()
	leds := filepath.Join(root, "leds")
	backlights := filepath.Join(root, "backlight")
	writeFakeBrightness(t, leds, "platform::kbd_backlight", 3, 3)
	writeFakeBrightness(t, leds, "status-led", 8, 8)
	writeFakeBrightness(t, backlights, "panel0", 80, 100)

	var calls []string
	controller := &saverExtrasController{
		path:          filepath.Join(root, "state", "saver-extras.json"),
		ledsRoot:      leds,
		backlightRoot: backlights,
		setBrightness: func(subsystem, name string, value int) error {
			calls = append(calls, subsystem+":"+name)
			deviceRoot := backlights
			if subsystem == "leds" {
				deviceRoot = leds
			}
			return os.WriteFile(filepath.Join(deviceRoot, name, "brightness"), []byte(intString(value)), 0o644)
		},
		outputs:    func() ([]wm.Output, error) { return nil, nil },
		setRefresh: func(context.Context, string, string) error { return nil },
	}

	controller.reconcile(context.Background(), true, saverExtrasFlags{Keyboard: true, Screen: true})
	assertFakeBrightness(t, leds, "platform::kbd_backlight", 1)
	assertFakeBrightness(t, leds, "status-led", 8)
	assertFakeBrightness(t, backlights, "panel0", 48)
	if len(calls) != 2 {
		t.Fatalf("enter made %d writes, want 2", len(calls))
	}

	body, err := os.ReadFile(controller.path)
	if err != nil {
		t.Fatal(err)
	}
	var saved saverExtrasState
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Keyboard["platform::kbd_backlight"] != (brightnessChange{Original: 3, Set: 1}) {
		t.Fatalf("keyboard state = %+v", saved.Keyboard)
	}
	if saved.Screen["panel0"] != (brightnessChange{Original: 80, Set: 48}) {
		t.Fatalf("screen state = %+v", saved.Screen)
	}

	controller.reconcile(context.Background(), false, saverExtrasFlags{})
	assertFakeBrightness(t, leds, "platform::kbd_backlight", 3)
	assertFakeBrightness(t, backlights, "panel0", 80)
	if len(calls) != 4 {
		t.Fatalf("leave made %d total writes, want 4", len(calls))
	}
	if _, err := os.Stat(controller.path); !os.IsNotExist(err) {
		t.Fatalf("empty state file still exists: %v", err)
	}
}

func TestSaverExtrasKeepsUserBrightnessChange(t *testing.T) {
	root := t.TempDir()
	backlights := filepath.Join(root, "backlight")
	writeFakeBrightness(t, backlights, "panel0", 100, 100)

	writes := 0
	controller := &saverExtrasController{
		path:          filepath.Join(root, "state", "saver-extras.json"),
		ledsRoot:      filepath.Join(root, "leds"),
		backlightRoot: backlights,
		setBrightness: func(_ string, name string, value int) error {
			writes++
			return os.WriteFile(filepath.Join(backlights, name, "brightness"), []byte(intString(value)), 0o644)
		},
	}

	controller.reconcile(context.Background(), true, saverExtrasFlags{Screen: true})
	assertFakeBrightness(t, backlights, "panel0", 60)
	if err := os.WriteFile(filepath.Join(backlights, "panel0", "brightness"), []byte("75"), 0o644); err != nil {
		t.Fatal(err)
	}
	controller.reconcile(context.Background(), true, saverExtrasFlags{Screen: true})
	controller.reconcile(context.Background(), true, saverExtrasFlags{Screen: false})
	assertFakeBrightness(t, backlights, "panel0", 75)
	if writes != 1 {
		t.Fatalf("user change caused another write: %d", writes)
	}
	if len(controller.state.Screen) != 0 {
		t.Fatalf("screen state not cleared: %+v", controller.state.Screen)
	}
}

func TestSaverExtrasPanelEnterLeave(t *testing.T) {
	root := t.TempDir()
	output := wm.Output{
		Name:  "eDP-1",
		Mode:  "2560x1600@165.002",
		Modes: []string{"2560x1600@165.002", "2560x1600@60.002"},
	}
	var calls []string
	controller := &saverExtrasController{
		path:          filepath.Join(root, "state", "saver-extras.json"),
		ledsRoot:      filepath.Join(root, "leds"),
		backlightRoot: filepath.Join(root, "backlight"),
		setBrightness: func(string, string, int) error { return nil },
		outputs: func() ([]wm.Output, error) {
			return []wm.Output{output}, nil
		},
		setRefresh: func(_ context.Context, name, hz string) error {
			calls = append(calls, name+"@"+hz)
			if hz == "60" {
				output.Mode = "2560x1600@60.002"
			} else {
				output.Mode = "2560x1600@" + hz
			}
			return nil
		},
	}

	controller.reconcile(context.Background(), true, saverExtrasFlags{Panel: true})
	if len(calls) != 1 || calls[0] != "eDP-1@60" {
		t.Fatalf("panel enter calls = %v", calls)
	}
	if controller.state.Outputs["eDP-1"] != (refreshChange{Original: "165.002", Set: "60.002"}) {
		t.Fatalf("panel state = %+v", controller.state.Outputs)
	}

	controller.reconcile(context.Background(), false, saverExtrasFlags{})
	if len(calls) != 2 || calls[1] != "eDP-1@165.002" {
		t.Fatalf("panel leave calls = %v", calls)
	}
	if len(controller.state.Outputs) != 0 {
		t.Fatalf("panel state not cleared: %+v", controller.state.Outputs)
	}
}

func writeFakeBrightness(t *testing.T, root, name string, current, maximum int) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brightness"), []byte(intString(current)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "max_brightness"), []byte(intString(maximum)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFakeBrightness(t *testing.T, root, name string, want int) {
	t.Helper()
	got, err := readBrightness(filepath.Join(root, name), "brightness")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s brightness = %d, want %d", name, got, want)
	}
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}
