package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// dockTestDaemon wires a daemon with a settings store over a temp shell.json,
// so the dock verbs run end-to-end without a live shell.
func dockTestDaemon(t *testing.T) *daemon {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	d := &daemon{}
	d.settings = newSettingsStore(filepath.Join(root, "shell.json"))
	return d
}

// TestDockVerbsThroughStore drives the dock verbs and checks the top-level dock
// object round-trips, keeping the shipped fields the CLI does not touch.
func TestDockVerbsThroughStore(t *testing.T) {
	d := dockTestDaemon(t)

	if got := d.dispatch("dock show"); got != "ok" {
		t.Fatalf("dock show: %s", got)
	}
	if got := d.dispatch("dock edge left"); got != "ok" {
		t.Fatalf("dock edge: %s", got)
	}
	if got := d.dispatch("dock autohide off"); got != "ok" {
		t.Fatalf("dock autohide: %s", got)
	}
	if got := d.dispatch("dock pin org.kitty"); got != "ok" {
		t.Fatalf("dock pin: %s", got)
	}
	if got := d.dispatch("dock pin org.kitty"); got != "ok" {
		t.Fatalf("re-pin should be idempotent: %s", got)
	}

	out := d.dispatch("dock list --json")
	var view map[string]any
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("dock list --json did not parse: %v (%s)", err, out)
	}
	if view["enabled"] != true || view["edge"] != "left" || view["autohide"] != false {
		t.Fatalf("dock list = %+v", view)
	}
	pinned := toStringSlice(view["pinned"])
	if len(pinned) != 1 || pinned[0] != "org.kitty" {
		t.Fatalf("pinned = %v, want [org.kitty]", pinned)
	}

	// The shipped fields the CLI never sets survive the write.
	dock, _ := d.settingsFrame()["dock"].(map[string]any)
	if dock["magnify"] != true || dock["frost"] != true || dock["revealOnEmpty"] != true {
		t.Fatalf("dock write dropped untouched fields: %+v", dock)
	}

	if got := d.dispatch("dock unpin org.kitty"); got != "ok" {
		t.Fatalf("dock unpin: %s", got)
	}
	if got := d.dispatch("dock unpin org.kitty"); !strings.HasPrefix(got, "err dock") {
		t.Fatalf("unpinning a missing app should error, got %s", got)
	}
	if got := d.dispatch("dock edge sideways"); !strings.HasPrefix(got, "err dock") {
		t.Fatalf("bad edge should error, got %s", got)
	}
	for _, command := range []string{"dock", "dock autohide maybe", "dock pin", "dock bogus"} {
		if got := d.dispatch(command); !strings.HasPrefix(got, "err dock") {
			t.Fatalf("dispatch(%q) = %q, want err dock", command, got)
		}
	}
}
