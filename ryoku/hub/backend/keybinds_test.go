package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	wm "ryoku-wm"
)

// stubProvider installs a fake ryoku-wm-<name> on PATH that prints out on every
// verb, and forces detection to it, so keybinds() drives a known binds answer
// with no live compositor. wm.Open() reads RYOKU_WM first, so the name here is
// the provider keybinds() will run.
func stubProvider(t *testing.T, name, out string) {
	t.Helper()
	bin := t.TempDir()
	// printf is a shell builtin, so the script needs no tool from PATH; that lets
	// the test hold PATH to just this dir and still emit the stubbed answer.
	script := "#!/bin/sh\nprintf '%s' '" + out + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-wm-"+name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("RYOKU_WM", name)
}

func catByName(l legend, name string) *category {
	for i := range l.Categories {
		if l.Categories[i].Name == name {
			return &l.Categories[i]
		}
	}
	return nil
}

// The provider's rows are grouped into sections in arrival order: a category
// opens the first time one of its rows appears and later rows join it, so two
// non-adjacent Windows rows are one section that keeps its place ahead of Apps
// and the exclusive niri group. combo mirrors the row's Default (the shipped
// chord a rebind records over), not the effective Chord, and desc mirrors Label.
func TestKeybindsGroupsRowsInArrivalOrder(t *testing.T) {
	stubProvider(t, "stub", `[
  {"id":"window.close","category":"Windows","label":"Close window","keys":["Super","Q"],"default":"SUPER + Q","chord":"SUPER + Q","kind":"wm","rebindable":true},
  {"id":"app.terminal","category":"Apps","label":"Terminal","hint":"Open a terminal","keys":["Super","Enter"],"default":"SUPER + Return","chord":"SUPER + X","kind":"app","rebindable":true},
  {"id":"window.fullscreen","category":"Windows","label":"Fullscreen","keys":["Super","F"],"default":"SUPER + F","chord":"SUPER + F","kind":"wm","rebindable":true,"unhonored":"niri has no fullscreen toggle"},
  {"id":"niri.center","category":"niri","label":"Center column","keys":["Super","C"],"default":"SUPER + C","chord":"SUPER + C","kind":"wm"}
]`)

	l := keybinds()

	wantOrder := []string{"Windows", "Apps", "niri"}
	if len(l.Categories) != len(wantOrder) {
		t.Fatalf("got %d categories, want %d: %+v", len(l.Categories), len(wantOrder), l.Categories)
	}
	for i, n := range wantOrder {
		if l.Categories[i].Name != n {
			t.Errorf("category %d = %q, want %q", i, l.Categories[i].Name, n)
		}
	}

	win := catByName(l, "Windows")
	if win == nil || len(win.Binds) != 2 {
		t.Fatalf("Windows category malformed: %+v", win)
	}
	if win.Binds[0].ID != "window.close" || win.Binds[1].ID != "window.fullscreen" {
		t.Errorf("Windows binds out of order: %q, %q", win.Binds[0].ID, win.Binds[1].ID)
	}
	// desc mirrors Label, combo mirrors Default.
	if win.Binds[0].Desc != "Close window" {
		t.Errorf("desc = %q, want the row label", win.Binds[0].Desc)
	}
	if win.Binds[0].Combo != "SUPER + Q" {
		t.Errorf("combo = %q, want the row default", win.Binds[0].Combo)
	}
	// The struck row is still listed, carrying the provider's reason.
	if win.Binds[1].Unhonored != "niri has no fullscreen toggle" {
		t.Errorf("unhonored = %q, want the provider reason", win.Binds[1].Unhonored)
	}

	apps := catByName(l, "Apps")
	if apps == nil || len(apps.Binds) != 1 {
		t.Fatalf("Apps category malformed: %+v", apps)
	}
	// combo is the default chord, not the rebound effective chord.
	if apps.Binds[0].Combo != "SUPER + Return" {
		t.Errorf("combo = %q, want the default, not the rebound chord", apps.Binds[0].Combo)
	}
	if apps.Binds[0].Chord != "SUPER + X" {
		t.Errorf("chord = %q, want the effective rebound chord passed through", apps.Binds[0].Chord)
	}
	if apps.Binds[0].Hint != "Open a terminal" {
		t.Errorf("hint = %q, want the row hint passed through", apps.Binds[0].Hint)
	}

	if n := len(catByName(l, "niri").Binds); n != 1 {
		t.Fatalf("niri exclusive group has %d binds, want 1", n)
	}
}

// The wire shape the cheatsheet and Hub page read: every BindRow field plus desc
// and combo, so a rename of one drops a field one of those surfaces expects.
func TestKeybindsJSONShape(t *testing.T) {
	stubProvider(t, "stub", `[
  {"id":"window.close","category":"Windows","label":"Close window","keys":["Super","Q"],"default":"SUPER + Q","chord":"SUPER + Q","kind":"wm","rebindable":true,"locked":false}
]`)

	raw, err := json.Marshal(keybinds())
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Categories []struct {
			Name  string                       `json:"name"`
			Binds []map[string]json.RawMessage `json:"binds"`
		} `json:"categories"`
		GestureKeys []wm.GestureKey `json:"gestureKeys"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Categories) != 1 || len(got.Categories[0].Binds) != 1 {
		t.Fatalf("unexpected shape: %s", raw)
	}
	if !reflect.DeepEqual(got.GestureKeys, wm.GestureKeys) {
		t.Errorf("gesture key catalogue missing from JSON: %s", raw)
	}
	b := got.Categories[0].Binds[0]
	for _, field := range []string{"keys", "combo", "desc", "rebindable", "id", "kind", "locked", "default", "chord", "label", "category"} {
		if _, ok := b[field]; !ok {
			t.Errorf("emitted bind is missing the %q field: %s", field, raw)
		}
	}
}

// No honest legend to draw when the provider errors (its binary is absent), so
// the Hub gets an empty, non-nil set and shows its empty state rather than a
// partial or stale sheet.
func TestKeybindsEmptyOnProviderError(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // ryoku-wm-missing is not here, so binds fails
	t.Setenv("RYOKU_WM", "missing")

	l := keybinds()
	if l.Categories == nil {
		t.Fatal("categories must be an empty slice, not nil, so the JSON is [] not null")
	}
	if len(l.Categories) != 0 {
		t.Errorf("got %d categories on provider error, want 0", len(l.Categories))
	}
}

func TestSightEffectiveGestureKeys(t *testing.T) {
	stubProvider(t, "stub", `[{"id":"shell.sight","category":"Shell","label":"Theia","keys":["Right Alt"],"default":"Alt_R","chord":"Alt_R","kind":"daemon","taps":3,"rebindable":true},{"id":"shell.sightHighlight","category":"Shell","label":"Theia on highlighted text","keys":["Left Alt"],"default":"Alt_L","chord":"Alt_L","kind":"daemon","whileHighlighting":true,"rebindable":true},{"id":"shell.sightInPlace","category":"Shell","label":"Theia in place","keys":["Right Ctrl"],"default":"Control_R","chord":"Control_R","kind":"daemon","taps":3,"rebindable":true}]`)
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	if err := os.MkdirAll(filepath.Join(config, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, settings, tap, highlight, inPlace string
	}{
		{"default", `{}`, "Alt_R", "Alt_L", "Control_R"},
		{"chosen", `{"sight":{"tapKey":"Control_R","highlightKey":"Shift_L","inplace":{"key":"Shift_R"}}}`, "Control_R", "Shift_L", "Shift_R"},
		{"in place only", `{"sight":{"inplace":{"key":"Control_L"}}}`, "Alt_R", "Alt_L", "Control_L"},
		{"unknown", `{"sight":{"tapKey":"Super_L","highlightKey":"Q","inplace":{"key":"Super_L"}}}`, "Alt_R", "Alt_L", "Control_R"},
		{"invalid", `{`, "Alt_R", "Alt_L", "Control_R"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(config, "ryoku", "shell.json"), []byte(tc.settings), 0o644); err != nil {
				t.Fatal(err)
			}
			l := keybinds()
			if len(l.Categories) != 1 || len(l.Categories[0].Binds) != 3 {
				t.Fatalf("missing Sight rows: %+v", l)
			}
			if len(l.GestureKeys) != len(wm.GestureKeys) {
				t.Fatalf("gesture keys missing: %+v", l.GestureKeys)
			}
			for i, want := range []string{tc.tap, tc.highlight, tc.inPlace} {
				row := l.Categories[0].Binds[i]
				def := []string{"Alt_R", "Alt_L", "Control_R"}[i]
				if row.Chord != want || row.Combo != def || row.Unhonored != "" || row.WhileHighlighting != (i == 1) || (i != 1 && row.Taps != 3) {
					t.Errorf("Sight row = %+v, want key %q and stable default %q", row, want, def)
				}
				if !reflect.DeepEqual(row.Keys, wm.DisplayKeys(want)) {
					t.Errorf("display keys = %v, want %v", row.Keys, wm.DisplayKeys(want))
				}
				raw, err := json.Marshal(row)
				if err != nil {
					t.Fatal(err)
				}
				var wire map[string]any
				if err := json.Unmarshal(raw, &wire); err != nil {
					t.Fatal(err)
				}
				if i != 1 && wire["taps"] != float64(3) || i == 1 && wire["whileHighlighting"] != true {
					t.Errorf("gesture lost on QML boundary: %s", raw)
				}
			}
		})
	}
}
