package main

import (
	"strings"
	"testing"
)

func TestBuiltinWindowRulesCoverRyokuFloats(t *testing.T) {
	wantKeys := []string{
		"system_pavucontrol", "system_network", "system_bluetooth",
		"system_polkit_kde", "system_portal_gtk", "system_polkit",
		"picker_open_file", "picker_save_file", "picker_save_as",
		"picker_choose_files", "picker_open_folder", "picture_in_picture",
		"picture_in_picture_chromium", "ghosttype", "spotify_lower",
		"spotify_upper", "nautilus", "hub", "rashin", "phone_tools",
		"update", "ryostore_title", "ryoport", "ryoport_ssh", "ryoport_spicy",
		"ryoport_console", "ryoport_vnc_lower", "ryoport_vnc_upper",
		"ryostore_app", "looking_glass", "qemu_lower",
		"qemu_upper", "welcome", "ryotunes", "ryotunes_qml",
		"chromium_sharebar",
	}

	byKey := make(map[string]wayfireBuiltinRule, len(wayfireBuiltinRules))
	for _, r := range wayfireBuiltinRules {
		if _, exists := byKey[r.key]; exists {
			t.Errorf("duplicate built-in rule key %q", r.key)
		}
		byKey[r.key] = r
		if !r.floating {
			t.Errorf("built-in %q is not excluded from simple-tile", r.key)
		}
	}
	if len(byKey) != len(wantKeys) {
		t.Fatalf("got %d built-in float matches, want %d", len(byKey), len(wantKeys))
	}
	for _, key := range wantKeys {
		if _, ok := byKey[key]; !ok {
			t.Errorf("missing built-in float match %q", key)
		}
	}

	checks := map[string]struct {
		condition string
		action    string
	}{
		"hub":                         {`title is "Ryoku Settings"`, pageFit},
		"nautilus":                    {`app_id is "org.gnome.Nautilus"`, largeGeometry},
		"rashin":                      {`app_id is "dev.ryoku.rashin"`, largeGeometry},
		"phone_tools":                 {`title contains "ryoku-phone-"`, phoneGeometry},
		"update":                      {`app_id is "dev.ryoku.update"`, largeGeometry},
		"ryoport":                     {`app_id is "dev.ryoku.ryoport"`, largeGeometry},
		"ryoport_ssh":                 {`app_id is "dev.ryoku.ryoport_ssh"`, compactGeometry},
		"ryostore_app":                {`app_id is "ryostore"`, mediumGeometry},
		"picture_in_picture":          {`title contains "Picture-in-Picture"`, ""},
		"picture_in_picture_chromium": {`title contains "Picture in picture"`, ""},
		"welcome":                     {`title is "Welcome to Ryoku"`, largeGeometry},
		"ryotunes_qml":                {`app_id is "org.quickshell" & title is "Ryotunes"`, ""},
		"chromium_sharebar":           {`app_id is "" & title contains "is sharing"`, "assign_workspace 4 1"},
	}
	for key, want := range checks {
		got := byKey[key]
		if got.condition != want.condition || got.action != want.action {
			t.Errorf("built-in %s = condition %q action %q, want %q and %q", key, got.condition, got.action, want.condition, want.action)
		}
	}
}

func TestBuiltinFloatsStayOutOfSimpleTile(t *testing.T) {
	matcher := tileByDefaultMatcher()
	if !strings.HasPrefix(matcher, "!(") || !strings.HasSuffix(matcher, ")") {
		t.Fatalf("simple-tile matcher %q is not one negated group", matcher)
	}
	for _, r := range wayfireBuiltinRules {
		if !strings.Contains(matcher, "("+r.condition+")") {
			t.Errorf("simple-tile matcher does not exclude %s: %q", r.key, matcher)
		}
	}

	storeDoc := renderStoreIni(defaultStore())
	if got, _ := storeDoc.get("simple-tile", "tile_by_default"); got != matcher {
		t.Errorf("rendered tile_by_default = %q, want %q", got, matcher)
	}
}

func TestBuiltinGeometryAndUserRulesCompose(t *testing.T) {
	s := defaultStore()
	s.WindowRules = []WindowRule{{Class: "kitty", Action: "maximize"}}
	doc := genWindowRules(s)

	for _, r := range wayfireBuiltinRules {
		if r.action == "" {
			continue
		}
		want := "on created if " + r.condition + " then " + r.action
		if got, ok := doc.get("window-rules", "ryoku_builtin_"+r.key); !ok || got != want {
			t.Errorf("built-in rule %s = %q, %v; want %q", r.key, got, ok, want)
		}
	}
	if got, _ := doc.get("window-rules", "ryoku_rule_0"); got != `on created if app_id is "kitty" then maximize` {
		t.Errorf("store window rule was lost behind built-ins: %q", got)
	}
	if got, _ := doc.get("window-rules", "ryoku_builtin_hub"); got != `on created if title is "Ryoku Settings" then maximize` {
		t.Errorf("Hub rule = %q", got)
	}
}
