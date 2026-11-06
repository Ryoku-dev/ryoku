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

// A dock is worn from the desktop editor, so its "open in settings" reaches the
// Stage Editor's Dock catalogue in the running shell and never the Hub.
func TestDockSettingsOpenTheDesktopEditor(t *testing.T) {
	if _, hub := settingsSection("docks"); hub {
		t.Fatal("docks route to a Hub page")
	}
	if !hasSettingsSection("docks") {
		t.Fatal("docks offer no settings route")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	qs := filepath.Join(dir, "qs")
	if err := os.WriteFile(qs, []byte("#!/bin/sh\necho \"$@\" >> \""+log+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := dispatch([]string{"settings", "docks", "engawa"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != "-c shell ipc call desktop editSection dock" {
		t.Fatalf("qs argv = %q", got)
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
	dir := t.TempDir()
	qs := filepath.Join(dir, "qs")
	if err := os.WriteFile(qs, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
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
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	for name, body := range map[string]string{
		"flock": "#!/bin/sh\nexit 0\n",
		"qs":    "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + log + "\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := dispatch([]string{"settings", "profiles", "hacker"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := []string{
		"-c hub ipc call nav open users",
		"-c hub ipc call users fromProfile hacker",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("qs calls = %q, want %q", got, want)
	}
	if err := dispatch([]string{"settings", "profiles"}); err == nil {
		t.Fatal("profile settings accepted no profile id")
	}
}

// Bar-style routes stay complete while the dedicated Subshells route remains.
func TestBarStyleSettingsHandoff(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "navigation.log")
	t.Setenv("RYOKU_TEST_QS_LOG", log)
	for filename, body := range map[string]string{
		"flock": "#!/bin/sh\nexit 0\n",
		"qs":    "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$RYOKU_TEST_QS_LOG\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
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
		raw, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		want := "-c hub ipc call nav open " + tc.expected
		got := strings.TrimSpace(string(raw))
		if !strings.HasSuffix(got, want) {
			t.Fatalf("settings %s %s routed to %q, want suffix %q", tc.category, tc.id, got, want)
		}
		if err := os.Remove(log); err != nil {
			t.Fatal(err)
		}
	}
}
