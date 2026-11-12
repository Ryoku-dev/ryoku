package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// The cheatsheet and the Hub ask for the legend with no store path (the seam
// client runs a bare "binds"), so the bare verb must read the saved store: a
// rebind or custom shortcut made in Settings has to show on Super+K.
func TestBindsVerbReadsSavedStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	store := `{"desktop":{"keybindRebinds":{"SUPER + L":"SUPER + SHIFT + P"},` +
		`"keybinds":[{"keys":"SUPER + T","action":"exec","value":"alacritty"}]}}`
	if err := os.WriteFile(filepath.Join(home, "ryoku", "desktop.json"), []byte(store), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	prev := stdout
	stdout = bufio.NewWriter(&buf)
	defer func() { stdout = prev }()
	if err := runBinds(nil); err != nil {
		t.Fatal(err)
	}
	stdout.Flush()
	var rows []wm.BindRow
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}

	var lock, custom wm.BindRow
	for _, r := range rows {
		switch {
		case r.ID == "shell.lock":
			lock = r
		case strings.HasPrefix(r.ID, "custom."):
			custom = r
		}
	}
	if lock.Chord != "SUPER + SHIFT + P" {
		t.Errorf("shell.lock chord = %q, want the saved rebind SUPER + SHIFT + P", lock.Chord)
	}
	if custom.Label != "Run: alacritty" {
		t.Errorf("saved custom shortcut missing from the legend: %+v", custom)
	}
}
