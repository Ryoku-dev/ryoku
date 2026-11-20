package main

import (
	"strings"
	"testing"
)

func TestWmSettingsMapping(t *testing.T) {
	store := map[string]any{
		"wm": map[string]any{"umbriel": map[string]any{
			"warpMouseToFocus":              true,
			"focusFollowsMouseScroll":       0.5,
			"workspaceAutoBackAndForth":     true,
			"modKey":                        "Alt",
			"dndEdgeViewScrollTriggerWidth": 40.0,
			"dndEdgeViewScrollDelayMs":      150.0,
			"dndEdgeViewScrollMaxSpeed":     1500.0,
		}},
		"desktop": map[string]any{"input": map[string]any{"followMouse": 1.0}},
	}
	sections, unhonored, mapped := wmSettings(store)
	toml := renderInputToml(sections)
	for _, want := range []string{
		"[input.cursor]", "follows_focus = true",
		"[input.focus]", "follows_mouse = true", "follows_mouse_max_scroll = 0.5",
		"[input.drag_edge_scroll]", "trigger_zone = 40", "delay_ms = 150", "max_speed = 1500",
		"[general]", `mod_key = "Alt"`,
		"[workspaces]", "back_and_forth = true",
	} {
		if !strings.Contains(toml, want) {
			t.Fatalf("rendered config missing %q:\n%s", want, toml)
		}
	}
	if len(unhonored) != 0 {
		t.Fatalf("got %d unhonored rows, want 0: %+v", len(unhonored), unhonored)
	}
	if !mapped["wm.umbriel.modKey"] || !mapped["desktop.input.followMouse"] {
		t.Fatalf("store keys were not consumed: %+v", mapped)
	}
}
