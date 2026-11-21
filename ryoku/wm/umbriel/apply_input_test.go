package main

import (
	"strings"
	"testing"
)

func TestKeyboardSettings(t *testing.T) {
	store := map[string]any{"desktop": map[string]any{"input": map[string]any{
		"kbLayout": "us,ara", "kbVariant": "", "kbOptions": "grp:alt_shift_toggle",
		"repeatRate": 30.0, "repeatDelay": 250.0, "numlockByDefault": true,
	}}}
	sections, unh, mapped := inputSettings(store)
	body := renderInputToml(sections)
	for _, want := range []string{
		"[input.keyboard]", `layout = "us,ara"`, `options = "grp:alt_shift_toggle"`,
		"repeat_rate = 30", "repeat_delay = 250", "numlock_toggle = true",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q:\n%s", want, body)
		}
	}
	if len(unh) != 0 {
		t.Fatalf("unexpected unhonoured: %+v", unh)
	}
	for _, key := range []string{"desktop.input.kbLayout", "desktop.input.kbVariant", "desktop.input.kbOptions", "desktop.input.repeatRate", "desktop.input.repeatDelay", "desktop.input.numlockByDefault"} {
		if !mapped[key] {
			t.Fatalf("key %q not consumed", key)
		}
	}
}

func TestInputSettingsMapping(t *testing.T) {
	store := map[string]any{
		"desktop": map[string]any{
			"input": map[string]any{
				"sensitivity":        0.5,
				"accelProfile":       "flat",
				"leftHanded":         true,
				"mouseNaturalScroll": false,
				"mouseScrollFactor":  1.0,
				"middleClickPaste":   true,
				"tapToClick":         true,
				"tapAndDrag":         false,
				"disableWhileTyping": true,
				"clickfinger":        true,
				"touchScrollFactor":  1.5,
			},
			"cursor": map[string]any{
				"hideOnKeyPress":  true,
				"inactiveTimeout": 5.0,
			},
		},
	}
	sections, unhonored, mapped := inputSettings(store)
	toml := renderInputToml(sections)
	for _, want := range []string{
		"[input.mouse]", "sensitivity = 0.5", "accel_profile = \"flat\"",
		"left_handed = true", "middle_click_paste = true",
		"[input.touchpad]", "tap = true", "click_method = \"clickfinger\"",
		"scroll_factor = 1.5", "[input.cursor]", "hide_when_typing = true",
		"hide_timeout_ms = 5000",
	} {
		if !strings.Contains(toml, want) {
			t.Fatalf("rendered config missing %q:\n%s", want, toml)
		}
	}
	if len(unhonored) != 0 {
		t.Fatalf("got %d unhonored rows, want 0: %+v", len(unhonored), unhonored)
	}
	for _, key := range []string{
		"desktop.input.sensitivity", "desktop.input.clickfinger",
		"desktop.cursor.inactiveTimeout",
	} {
		if !mapped[key] {
			t.Fatalf("store key %q was not consumed", key)
		}
	}
}

func TestTouchpadNaturalScrollOwnsKey(t *testing.T) {
	store := map[string]any{"desktop": map[string]any{"input": map[string]any{
		"naturalScroll": true,
		"swipeInvert":   true,
	}}}
	sections, _, mapped := inputSettings(store)
	toml := renderInputToml(sections)
	if !strings.Contains(toml, "natural_scroll = true") {
		t.Fatalf("touchpad natural scroll was overwritten:\n%s", toml)
	}
	if !mapped["desktop.input.naturalScroll"] || mapped["desktop.input.swipeInvert"] {
		t.Fatalf("unexpected consumed keys: %+v", mapped)
	}
}
