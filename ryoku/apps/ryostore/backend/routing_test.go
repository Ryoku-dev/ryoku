package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsSectionRoutesStoreCategories(t *testing.T) {
	cases := map[string]string{
		"plugins":       "add-ons",
		"bundles":       "add-ons",
		"barstyles":     "bar",
		"subshells":     "bar",
		"launchers":     "launcher",
		"osds":          "notifications",
		"notifications": "notifications",
		"sidebars":      "quick-settings/panel-design",
		"fastfetch":     "terminal",
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

// rices are worn from the wallpaper picker, so their route asks ryogami and
// never names a Settings page
func TestRiceSettingsOpenTheWallpaperPicker(t *testing.T) {
	if _, hub := settingsSection("rices"); hub {
		t.Fatal("rices route to a Settings page")
	}
	if !hasSettingsSection("rices") {
		t.Fatal("rices offer no settings route")
	}
	shell := fakeRyokuShell(t, "")
	log := fakeCommand(t, "ryogami", "")
	if err := dispatch([]string{"settings", "rices", "kanso"}); err != nil {
		t.Fatal(err)
	}
	want := "[wallpaper][collection][rices]"
	if got := shellCalls(t, log); len(got) != 1 || got[0] != want {
		t.Fatalf("ryogami calls = %q, want [%q]", got, want)
	}
	if _, err := os.Stat(shell); err == nil {
		t.Fatal("rices also called ryoku-shell")
	}
}

func TestRiceSettingsReportsTheDaemonRefusal(t *testing.T) {
	fakeCommand(t, "ryogami", "echo 'ryogami: daemon unreachable' >&2\nexit 1\n")
	err := dispatch([]string{"settings", "rices"})
	if err == nil || !strings.Contains(err.Error(), "daemon unreachable") {
		t.Fatalf("settings error = %v, want ryogami's reason", err)
	}
}

func fakeRyokuShell(t *testing.T, body string) string {
	t.Helper()
	return fakeCommand(t, "ryoku-shell", body)
}

// one bracketed entry per argument so an empty one stays visible
func fakeCommand(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	script := "#!/bin/sh\n{ printf '[%s]' \"$@\"; printf '\\n'; } >> \"" + log + "\"\n" + body
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
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

func TestSettingsReportsTheDaemonRefusal(t *testing.T) {
	fakeRyokuShell(t, "echo 'err ryoku-shell is not running' >&2\nexit 1\n")
	err := dispatch([]string{"settings", "plugins"})
	if err == nil || !strings.Contains(err.Error(), "ryoku-shell is not running") {
		t.Fatalf("settings error = %v, want the daemon's reason", err)
	}
}

func TestStoreSectionIncludesShowroomRoutesAndEveryCategory(t *testing.T) {
	sections := []string{"discover", "library"}
	for _, p := range providersWithProfileGate(func() bool { return true }) {
		sections = append(sections, p.Category().ID)
	}
	for _, section := range sections {
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

// an add-on's or bundle's "Open in Settings" lands on that item, not the list
func TestAddOnSettingsOpenTheItem(t *testing.T) {
	log := fakeRyokuShell(t, "")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"plugins", "vpn"}, "[hub][open][add-ons/add-on:vpn]"},
		{[]string{"bundles", "ryoku-discord"}, "[hub][open][add-ons/bundle:ryoku-discord]"},
		{[]string{"plugins"}, "[hub][open][add-ons]"},
		{[]string{"sidebars", "glass"}, "[hub][open][quick-settings/panel-design]"},
	} {
		if err := runSettings(tc.args); err != nil {
			t.Fatalf("settings %q: %v", tc.args, err)
		}
		if got := shellCalls(t, log); len(got) != 1 || got[0] != tc.want {
			t.Fatalf("settings %q routed to %q, want [%q]", tc.args, got, tc.want)
		}
		if err := os.Remove(log); err != nil {
			t.Fatal(err)
		}
	}
	if err := runSettings([]string{"plugins", "../vpn"}); err == nil {
		t.Fatal("settings accepted an add-on id that is not an id")
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
		"settings": {"settings", "plugins"},
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

// every bar style and subshell is worn from the one Bar page
func TestBarStyleSettingsHandoff(t *testing.T) {
	log := fakeRyokuShell(t, "")
	for _, tc := range []struct{ category, id, expected string }{
		{"barstyles", "python", "bar"},
		{"barstyles", "gokko", "bar"},
		{"barstyles", "tansu", "bar"},
		{"barstyles", "caelestia", "bar"},
		{"barstyles", "iris", "bar"},
		{"subshells", "python", "bar"},
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
