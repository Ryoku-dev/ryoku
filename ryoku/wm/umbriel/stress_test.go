package main

import (
	"encoding/json"
	"strings"
	"testing"

	wm "ryoku-wm"
)

func TestInputSettingsBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		input map[string]any
		want  []string
	}{
		{"low", map[string]any{
			"sensitivity": -1.0, "accelProfile": "flat", "leftHanded": false,
			"mouseNaturalScroll": false, "middleClickPaste": false,
			"tapToClick": false, "tapAndDrag": false, "disableWhileTyping": false,
			"clickfinger": false, "touchScrollFactor": 0.2, "naturalScroll": false,
			"followMouse": 0.0,
		}, []string{"sensitivity = -1", `accel_profile = "flat"`, `click_method = "button_areas"`, "scroll_factor = 0.2"}},
		{"high", map[string]any{
			"sensitivity": 1.0, "accelProfile": "adaptive", "leftHanded": true,
			"mouseNaturalScroll": true, "middleClickPaste": true,
			"tapToClick": true, "tapAndDrag": true, "disableWhileTyping": true,
			"clickfinger": true, "touchScrollFactor": 3.0, "naturalScroll": true,
			"followMouse": 3.0,
		}, []string{"sensitivity = 1", `accel_profile = "adaptive"`, `click_method = "clickfinger"`, "scroll_factor = 3"}},
		{"unknown profile", map[string]any{"accelProfile": "bogus"}, nil},
		{"wrong types", map[string]any{"sensitivity": "fast", "clickfinger": "yes"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := map[string]any{"desktop": map[string]any{"input": tc.input}}
			sections, unhonored, mapped := inputSettings(store)
			toml := renderInputToml(sections)
			if strings.Contains(toml, "accel_profile = \"bogus\"") || strings.Contains(toml, "sensitivity = fast") {
				t.Fatalf("invalid value rendered:\n%s", toml)
			}
			if strings.Contains(toml, "[input") && !strings.HasPrefix(toml, "[input") {
				t.Fatalf("malformed TOML:\n%s", toml)
			}
			for _, want := range tc.want {
				if !strings.Contains(toml, want) {
					t.Fatalf("missing %q:\n%s", want, toml)
				}
			}
			_ = unhonored
			_ = mapped
		})
	}
}

func TestWmSettingsBoundaryValues(t *testing.T) {
	store := map[string]any{"wm": map[string]any{"umbriel": map[string]any{
		"warpMouseToFocus":              true,
		"focusFollowsMouseScroll":       10.0,
		"workspaceAutoBackAndForth":     true,
		"modKey":                        "Shift",
		"dndEdgeViewScrollTriggerWidth": 1000.0,
		"dndEdgeViewScrollDelayMs":      10000.0,
		"dndEdgeViewScrollMaxSpeed":     100000.0,
	}}}
	sections, unhonored, mapped := wmSettings(store)
	toml := renderInputToml(sections)
	for _, want := range []string{
		"follows_focus = true", "follows_mouse_max_scroll = 10", "back_and_forth = true",
		`mod_key = "Shift"`, "trigger_zone = 1000", "delay_ms = 10000", "max_speed = 100000",
	} {
		if !strings.Contains(toml, want) {
			t.Fatalf("missing %q:\n%s", want, toml)
		}
	}
	if len(unhonored) != 0 {
		t.Fatalf("unexpected unhonoured rows: %+v", unhonored)
	}
	for key := range store["wm"].(map[string]any)["umbriel"].(map[string]any) {
		if !mapped["wm.umbriel."+key] {
			t.Fatalf("row %q was not consumed", key)
		}
	}
}

func TestMonitorsTomlBoundaries(t *testing.T) {
	layout := []wm.OutputLayout{
		{Name: "eDP-1", Enabled: false},
		{Name: "Virtual-1", Enabled: true, Mode: "1024x768@59940", Scale: 1.25,
			X: -100, Y: 50, Transform: 3, VRR: true,
			Mirror: "eDP-1", ColorMode: "hdr", SdrBrightness: 200},
		{Name: "weird\"name", Enabled: true, Mode: "bad-mode", Scale: 2},
	}
	body := string(monitorsToml(layout))
	for _, want := range []string{
		`[output."eDP-1"]`, "enabled = false",
		`[output."Virtual-1"]`, `mode = "1024x768@59.94"`, "scale = 1.25",
		"position = [-100, 50]", `transform = "270"`,
		`[output."weird\"name"]`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q:\n%s", want, body)
		}
	}
	if len(outputsUnhonored(layout)) != 5 {
		t.Fatalf("expected five unhonoured fields, got %+v", outputsUnhonored(layout))
	}
}

func TestInvalidModeSkipped(t *testing.T) {
	layout := []wm.OutputLayout{{Name: "Virtual-1", Enabled: true, Mode: "bad-mode", Scale: 1}}
	body := string(monitorsToml(layout))
	if strings.Contains(body, "mode =") {
		t.Fatalf("invalid mode was rendered:\n%s", body)
	}
	rows := outputsUnhonored(layout)
	if len(rows) != 1 || rows[0].Key != "output.Virtual-1.mode" {
		t.Fatalf("invalid mode was not reported: %+v", rows)
	}
}

func TestDuplicateOutputNamesWrittenOnce(t *testing.T) {
	layout := []wm.OutputLayout{
		{Name: "Virtual-1", Enabled: true, Scale: 1},
		{Name: "Virtual-1", Enabled: false},
	}
	body := string(monitorsToml(layout))
	if strings.Count(body, `[output."Virtual-1"]`) != 1 {
		t.Fatalf("duplicate table rendered:\n%s", body)
	}
	if len(outputsUnhonored(layout)) != 1 {
		t.Fatalf("duplicate was not reported: %+v", outputsUnhonored(layout))
	}
}

func TestReadStoreErrors(t *testing.T) {
	if _, err := readStore("/definitely/missing/store.json"); err == nil {
		t.Fatal("missing store did not error")
	}
}

func TestSchemaRowsAreConsumed(t *testing.T) {
	body, err := schemaFS.ReadFile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatal(err)
	}
	store := map[string]any{"wm": map[string]any{"umbriel": map[string]any{}}}
	for _, row := range rows {
		key, _ := row["key"].(string)
		leaf := strings.TrimPrefix(key, "wm.umbriel.")
		store["wm"].(map[string]any)["umbriel"].(map[string]any)[leaf] = "test"
	}
	_, _, mapped := wmSettings(store)
	for _, row := range rows {
		key, _ := row["key"].(string)
		if !mapped[key] {
			t.Fatalf("schema row %q has no writer", key)
		}
	}
}

// Wrong-typed store values must be dropped, not rendered as bare TOML words
// that break the whole generated config.
func TestWrongTypesAreDropped(t *testing.T) {
	store := map[string]any{"desktop": map[string]any{"input": map[string]any{
		"leftHanded": "yes", "naturalScroll": "maybe", "tapToClick": 5,
		"middleClickPaste": "true", "numlockByDefault": "on",
		"repeatRate": "fast", "kbLayout": 123,
	}, "cursor": map[string]any{"inactiveTimeout": "soon"}},
		"wm": map[string]any{"umbriel": map[string]any{
			"warpMouseToFocus": "yes", "focusFollowsMouseScroll": "x",
			"dndEdgeViewScrollTriggerWidth": "wide", "modKey": 9,
		}}}
	ins, _, _ := inputSettings(store)
	ws, _, _ := wmSettings(store)
	body := renderInputToml(ins) + renderInputToml(ws)
	t.Logf("rendered:\n%s", body)
	// Values that cannot be expressed must not reach the TOML as bare words.
	for _, bad := range []string{"= yes", "= maybe", "= true\n", "= fast", "= soon", "= wide", "= x\n"} {
		if strings.Contains(body, bad) {
			t.Errorf("invalid TOML token rendered: %q in\n%s", bad, body)
		}
	}
}
