package main

import (
	"os"
	"path/filepath"
	"testing"
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
