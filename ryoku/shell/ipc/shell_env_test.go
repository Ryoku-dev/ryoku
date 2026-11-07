package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExpandItemEnvValue(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_STATE_HOME", "/state")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	vars := itemEnvVars("/items/alpha")
	cases := [][2]string{
		{"$XDG_DATA_HOME/ryoku/alpha", "/home/u/.local/share/ryoku/alpha"},
		{"${XDG_STATE_HOME}/x", "/state/x"},
		{"$XDG_CONFIG_HOME:$XDG_CACHE_HOME", "/home/u/.config:/home/u/.cache"},
		{"$HOME/bin", "/home/u/bin"},
		{"$RYOKU_ITEM_DIR/data", "/items/alpha/data"},
		{"plain", "plain"},
	}
	for _, c := range cases {
		if got, ok := expandItemEnvValue(c[0], vars); !ok || got != c[1] {
			t.Errorf("expand(%q) = (%q,%v), want %q", c[0], got, ok, c[1])
		}
	}
	for _, raw := range []string{"$PATH", "$USER/x", "cost $5", "trailing $", "${HOME", "$home"} {
		if got, ok := expandItemEnvValue(raw, vars); ok {
			t.Errorf("expand(%q) = %q, want rejection", raw, got)
		}
	}
}

func TestItemEnvDropsProtectedAndInvalidKeys(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_DATA_HOME", "/data")
	env := map[string]string{
		"OMARCHY_PATH": "$XDG_DATA_HOME/ryoku/nomarchy",
		"ALPHA_THEME":  "$RYOKU_ITEM_DIR/data/theme",
		"BAD_VALUE":    "$SECRET",
	}
	for _, key := range []string{
		"PATH", "HOME", "USER", "SHELL", "LANG", "LC_ALL", "LD_PRELOAD", "XDG_DATA_HOME",
		"QT_QPA_PLATFORM", "QML_IMPORT_PATH", "QML2_IMPORT_PATH", "WAYLAND_DISPLAY",
		"DBUS_SESSION_BUS_ADDRESS", "DISPLAY", "RYOKU_ITEM_ID", "MALLOC_CONF", "lowercase", "1LEAD",
	} {
		env[key] = "/evil"
	}
	item := &shellItem{ID: "alpha", Category: "barstyles", Source: "store", Dir: "/items/alpha", Env: env}
	want := []string{"ALPHA_THEME=/items/alpha/data/theme", "OMARCHY_PATH=/data/ryoku/nomarchy"}
	if got := itemEnv(item); !reflect.DeepEqual(got, want) {
		t.Fatalf("itemEnv = %v, want %v", got, want)
	}
	merged := withItemEnv([]string{"OMARCHY_PATH=/old", "PATH=/usr/bin"}, item)
	if want := []string{"PATH=/usr/bin", "ALPHA_THEME=/items/alpha/data/theme", "OMARCHY_PATH=/data/ryoku/nomarchy"}; !reflect.DeepEqual(merged, want) {
		t.Fatalf("withItemEnv = %v, want %v", merged, want)
	}
}

func TestShippedItemEnvExpandsShippedDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RYOSTORE_SHIPPED_ROOT", root)
	item := &shellItem{ID: "alpha", Category: "subshells", Source: "shipped", Dir: "subshells/alpha",
		Env: map[string]string{"ALPHA_ROOT": "$RYOKU_ITEM_DIR"}}
	want := "ALPHA_ROOT=" + filepath.Join(root, "subshells", "alpha")
	if got := itemEnv(item); len(got) != 1 || got[0] != want {
		t.Fatalf("itemEnv = %v, want [%s]", got, want)
	}
}

func TestItemEnvReachesShellAndHooks(t *testing.T) {
	item := shellItem{ID: "alpha", Category: "barstyles", Dir: "/items/alpha", Prepared: true,
		Env: map[string]string{"ALPHA_MODE": "on"}}
	d := &daemon{shellIndex: &shellIndex{Active: "alpha", Items: []shellItem{item}}}
	if env := strings.Join(d.shellQsEnv(), "\n"); !strings.Contains(env, "ALPHA_MODE=on") {
		t.Fatal("shell environment lacks the active item's env")
	}
	hook := strings.Join(shellItemEnv(&item), "\n")
	if !strings.Contains(hook, "ALPHA_MODE=on") || !strings.Contains(hook, "RYOKU_ITEM_DIR=/items/alpha") {
		t.Fatal("hook environment lacks the item's env")
	}
	d.shellIndex.Items[0].Prepared = false
	if env := strings.Join(d.shellQsEnv(), "\n"); strings.Contains(env, "ALPHA_MODE=on") {
		t.Fatal("packed item's env reached the shell")
	}
}

func TestActiveItemEnvChangeRequestsRestart(t *testing.T) {
	index := &shellIndex{Schema: 1, Active: "alpha", Items: []shellItem{{
		ID: "alpha", Category: "barstyles", Dir: "/items/alpha", InUse: true, Prepared: true,
		Env: map[string]string{"ALPHA_MODE": "on"},
	}}}
	d := &daemon{
		settings:         &settingsStore{raw: map[string]any{"barStyle": "alpha"}},
		shellIndex:       index,
		shellSnapshot:    snapshotFor("alpha", index),
		shellSnapshotSet: true,
	}
	if d.shellCommitNeeded() {
		t.Fatal("unchanged env requested a restart")
	}
	index.Items[0].Env = map[string]string{"ALPHA_MODE": "off"}
	if !d.shellCommitNeeded() {
		t.Fatal("changed env did not request a restart")
	}
}
