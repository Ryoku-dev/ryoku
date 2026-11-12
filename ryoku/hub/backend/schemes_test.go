package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// currentScheme must report "follow" whenever the wallpaper is being followed,
// even when theme.json omits the scheme key (the exact state the Appearance
// control persists when you pick Follow). Regression guard for the bug where the
// "mono" default filled the omitted key and the scheme check ran before the
// follow check, so Follow silently reverted to Mono on the next read.
func TestCurrentSchemeReportsFollow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	tp := filepath.Join(dir, "ryoku", "theme.json")
	if err := os.MkdirAll(filepath.Dir(tp), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, file, want string }{
		{"follow, scheme key omitted", `{"followWallpaper":true}`, "follow"},
		{"follow, empty scheme", `{"followWallpaper":true,"scheme":""}`, "follow"},
		{"locked mono", `{"followWallpaper":false,"scheme":"mono"}`, "mono"},
		{"locked light", `{"followWallpaper":false,"scheme":"light"}`, "light"},
		{"locked dark", `{"followWallpaper":false,"scheme":"dark"}`, "dark"},
	}
	for _, c := range cases {
		if err := os.WriteFile(tp, []byte(c.file), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := currentScheme(); got != c.want {
			t.Errorf("%s: currentScheme() = %q, want %q", c.name, got, c.want)
		}
	}

	// a missing file is a fresh box: the shipped default now follows the wallpaper.
	os.Remove(tp)
	if got := currentScheme(); got != "follow" {
		t.Errorf("missing file: currentScheme() = %q, want follow", got)
	}
}

// The follow state must survive a save/load round-trip: writing FollowWallpaper
// with an empty scheme and reading it back must still resolve to "follow", not
// the default mono.
func TestSchemeFollowRoundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	saveThemeState(themeState{FollowWallpaper: true, Scheme: ""})
	if got := currentScheme(); got != "follow" {
		t.Errorf("follow round-trip: currentScheme() = %q, want follow", got)
	}
}

func TestApplyRyokuThemeUsesRyokuFrameBarLook(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("PATH", "")

	shellPath := filepath.Join(dir, "config", "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(shellPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shellPath, []byte(`{"barStyle":"stele","frameBars":{"style":"slate-frame","rails":{"top":{"size":44}}},"sidebarWidth":420}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := applyRyokuTheme(); err != nil {
		t.Fatal(err)
	}
	got := readJSONMap(shellPath)
	frameBars := got["frameBars"].(map[string]any)
	if frameBars["style"] != "ryoku-frame" {
		t.Fatalf("frameBars.style = %v, want ryoku-frame", frameBars["style"])
	}
	if frameBars["rails"].(map[string]any)["top"].(map[string]any)["size"] != float64(44) {
		t.Fatalf("frameBars was not preserved: %v", frameBars)
	}
	if got["sidebarWidth"] != float64(420) {
		t.Fatalf("sidebarWidth was not preserved: %v", got["sidebarWidth"])
	}
}

// ryogami refusing the batch leaves theme.json as it was, not with the hub's half applied
func TestRefusedThemeBatchLeavesThemeJSON(t *testing.T) {
	matugenHome(t)
	origRun := riceRun
	riceRun = func(string, ...string) error { return errors.New("ryogami: the daemon is not running") }
	t.Cleanup(func() { riceRun = origRun })
	const saved = `{"followWallpaper":true,"scheme":"mono"}`
	writeTestFile(t, themeStatePath(), saved)

	on := true
	st := loadThemeState()
	st.Scheme, st.FollowWallpaper, st.ThemeApps = "dark", false, &on
	if err := saveThemeState(st); err == nil {
		t.Fatal("a refused theme batch reported success")
	}
	if b, _ := os.ReadFile(themeStatePath()); string(b) != saved {
		t.Fatalf("theme.json after a refused batch = %s, want %s", b, saved)
	}
}
