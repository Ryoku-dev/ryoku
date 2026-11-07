package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsSectionRoutesStoreCategories(t *testing.T) {
	cases := map[string]string{
		"rices":     "appearance",
		"plugins":   "addons",
		"bundles":   "addons",
		"barstyles": "bar-studio",
		"subshells": "subshells",
		"launchers": "launcher",
		// one Hub page wears all three overlays
		"osds":          "overlays",
		"notifications": "overlays",
		"sidebars":      "overlays",
		"fastfetch":     "fastfetch",
		"profiles":      "users",
	}
	for category, want := range cases {
		got, ok := settingsSection(category)
		if !ok || got != want {
			t.Fatalf("settingsSection(%q) = %q, %v; want %q, true", category, got, ok, want)
		}
	}
	// Today has no Settings page, the lockscreen page is edit-only, and the
	// flat-image categories have no page, so none routes to Settings.
	for _, edit := range []string{"today", "lockscreens", "decors", "launcher-images", "ryotunes-skins"} {
		if hasSettingsSection(edit) {
			t.Fatalf("%q should have no Settings route", edit)
		}
	}
}

// fakeRyokuShell puts a ryoku-shell on PATH that logs each call's argv, one
// bracketed entry per argument so an empty argument stays visible.
func fakeRyokuShell(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	script := "#!/bin/sh\n{ printf '[%s]' \"$@\"; printf '\\n'; } >> \"" + log + "\"\n" + body
	if err := os.WriteFile(filepath.Join(dir, "ryoku-shell"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func shellCalls(t *testing.T, log string) []string {
	t.Helper()
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// A dock is worn from the desktop editor, so its "open in settings" reaches the
// Stage Editor's Dock catalogue in the running shell and never the Hub.
func TestDockSettingsOpenTheDesktopEditor(t *testing.T) {
	if _, hub := settingsSection("docks"); hub {
		t.Fatal("docks route to a Hub page")
	}
	if !hasSettingsSection("docks") {
		t.Fatal("docks offer no settings route")
	}
	log := fakeRyokuShell(t, "")
	if err := dispatch([]string{"settings", "docks", "engawa"}); err != nil {
		t.Fatal(err)
	}
	want := "[ipc][call][desktop][editSection][dock][]"
	if got := shellCalls(t, log); len(got) != 1 || got[0] != want {
		t.Fatalf("ryoku-shell calls = %q, want [%q]", got, want)
	}
}

// A daemon refusal (no shell, unknown section) reaches the caller with its reason.
func TestSettingsReportsTheDaemonRefusal(t *testing.T) {
	fakeRyokuShell(t, "echo 'err ryoku-shell is not running' >&2\nexit 1\n")
	err := dispatch([]string{"settings", "rices"})
	if err == nil || !strings.Contains(err.Error(), "ryoku-shell is not running") {
		t.Fatalf("settings error = %v, want the daemon's reason", err)
	}
}

func TestStoreSectionIncludesShowroomRoutesAndEveryCategory(t *testing.T) {
	for _, section := range []string{"discover", "library", "rices", "lockscreens", "subshells", "barstyles", "docks", "launchers", "fastfetch", "plugins", "bundles", "ryotunes-skins", "profiles"} {
		if !storeSection(section) {
			t.Fatalf("storeSection(%q) = false", section)
		}
	}
	for _, section := range []string{"today", "installed", "unknown"} {
		if storeSection(section) {
			t.Fatalf("storeSection(%q) = true", section)
		}
	}
}

func TestDispatchRoutesHandoffCommands(t *testing.T) {
	fakeRyokuShell(t, "")
	dir := t.TempDir()
	for _, name := range []string{"qs", "flock"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	for name, args := range map[string][]string{
		"open":     {"open", "library"},
		"settings": {"settings", "rices"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := dispatch(args); err != nil {
				t.Fatalf("dispatch(%q): %v", args, err)
			}
		})
	}
}

func TestProfileSettingsPassesIDToUsersPage(t *testing.T) {
	log := fakeRyokuShell(t, "")
	if err := dispatch([]string{"settings", "profiles", "hacker"}); err != nil {
		t.Fatal(err)
	}
	want := "[hub][open][users][hacker]"
	if got := shellCalls(t, log); len(got) != 1 || got[0] != want {
		t.Fatalf("ryoku-shell calls = %q, want [%q]", got, want)
	}
	if err := dispatch([]string{"settings", "profiles"}); err == nil {
		t.Fatal("profile settings accepted no profile id")
	}
}

// Bar-style routes stay complete while the dedicated Subshells route remains.
func TestBarStyleSettingsHandoff(t *testing.T) {
	log := fakeRyokuShell(t, "")
	for _, tc := range []struct{ category, id, expected string }{
		{"barstyles", "python", "bar-studio"},
		{"barstyles", "gokko", "bar-studio"},
		{"barstyles", "tansu", "bar-studio"},
		{"barstyles", "caelestia", "bar-studio"},
		{"barstyles", "iris", "bar-studio"},
		{"subshells", "python", "subshells"},
	} {
		if err := runSettings([]string{tc.category, tc.id}); err != nil {
			t.Fatalf("settings %s %s: %v", tc.category, tc.id, err)
		}
		want := "[hub][open][" + tc.expected + "]"
		if got := shellCalls(t, log); len(got) != 1 || got[0] != want {
			t.Fatalf("settings %s %s routed to %q, want [%q]", tc.category, tc.id, got, want)
		}
		if err := os.Remove(log); err != nil {
			t.Fatal(err)
		}
	}
}
