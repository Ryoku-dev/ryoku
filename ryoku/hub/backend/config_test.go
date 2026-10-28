package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// no file -> default.
	if c := loadConfig(); c.UI.Section != "displays" {
		t.Fatalf("default section = %q, want displays", c.UI.Section)
	}

	if err := configSet("section", "appearance"); err != nil {
		t.Fatal(err)
	}
	if v, ok := configGet("section"); !ok || v != "appearance" {
		t.Fatalf("after set: got %q ok=%v, want appearance", v, ok)
	}

	// landed on disk as real TOML.
	b, err := os.ReadFile(filepath.Join(dir, "ryoku", "hub.toml"))
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if !contains(string(b), "section = \"appearance\"") {
		t.Errorf("toml missing section line:\n%s", b)
	}
}

func TestUpdateInterval(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// no file -> default.
	if c := loadConfig(); c.UI.UpdateInterval != "daily" {
		t.Fatalf("default update_interval = %q, want daily", c.UI.UpdateInterval)
	}

	if err := configSet("update_interval", "off"); err != nil {
		t.Fatal(err)
	}
	if v, ok := configGet("update_interval"); !ok || v != "off" {
		t.Fatalf("after set: got %q ok=%v, want off", v, ok)
	}
}

func TestUpdateSystem(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// the distribution upgrade stays opt-in: no file reads off.
	if v, ok := configGet("update_system"); !ok || v != "off" {
		t.Fatalf("default update_system = %q ok=%v, want off", v, ok)
	}

	if err := configSet("update_system", "on"); err != nil {
		t.Fatal(err)
	}
	if v, ok := configGet("update_system"); !ok || v != "on" {
		t.Fatalf("after set: got %q ok=%v, want on", v, ok)
	}

	if err := configSet("update_system", "maybe"); err == nil {
		t.Error("update_system should reject anything but on or off")
	}
	if v, _ := configGet("update_system"); v != "on" {
		t.Errorf("a rejected value must not change the setting, got %q", v)
	}
}

func TestAdvanced(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// the Advanced reveal is off until set, so an unset value reads empty.
	if v, ok := configGet("advanced"); !ok || v != "" {
		t.Fatalf("default advanced = %q ok=%v, want empty", v, ok)
	}

	if err := configSet("advanced", "1"); err != nil {
		t.Fatal(err)
	}
	if v, ok := configGet("advanced"); !ok || v != "1" {
		t.Fatalf("after set: got %q ok=%v, want 1", v, ok)
	}
}

func TestConfigUnknownKey(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := configSet("nope", "x"); err == nil {
		t.Error("setting an unknown key should error")
	}
	if _, ok := configGet("nope"); ok {
		t.Error("getting an unknown key should report not-ok")
	}
}

func TestLegacyBarAndFrameSectionsMigrateToBarStudio(t *testing.T) {
	for _, legacy := range []string{"bar", "frame"} {
		t.Run(legacy, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			path := filepath.Join(dir, "ryoku", "hub.toml")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("[ui]\nsection = \""+legacy+"\"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := loadConfig().UI.Section; got != "bar-studio" {
				t.Fatalf("loaded legacy %q section = %q, want bar-studio", legacy, got)
			}
			bytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !contains(string(bytes), "section = \"bar-studio\"") {
				t.Fatalf("legacy section was not persisted as bar-studio:\n%s", bytes)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
