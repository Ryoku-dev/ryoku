package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderActiveTemplatesFollowsXDGDirs(t *testing.T) {
	home := t.TempDir()
	cfgHome := filepath.Join(home, "profile", ".config")
	cacheHome := filepath.Join(home, "profile", ".cache")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "profile", "data"))

	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	log := filepath.Join(home, "seen.toml")
	shim := "#!/bin/sh\ncat \"$2\" >> \"" + log + "\"\n"
	if err := os.WriteFile(filepath.Join(bin, "matugen"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}

	mg := filepath.Join(cfgHome, "matugen")
	if err := os.MkdirAll(mg, 0o755); err != nil {
		t.Fatal(err)
	}
	core := "[config]\n\n[templates.kitty]\ninput_path = \"~/.config/matugen/templates/kitty-colors.conf\"\noutput_path = \"~/.config/kitty/current-theme.conf\"\n"
	apps := "[config]\n\n[templates.gtk3]\ninput_path = \"~/.config/matugen/templates/gtk3-colors.css\"\noutput_path = \"~/.config/gtk-3.0/gtk.css\"\n"
	for name, body := range map[string]string{"config.toml": core, "apps.toml": apps} {
		if err := os.WriteFile(filepath.Join(mg, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	renderActiveTemplates(defaultMatugenConfig(), map[string]string{"primary": "#112233"})

	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`input_path = "` + cfgHome + `/matugen/templates/kitty-colors.conf"`,
		`output_path = "` + cfgHome + `/kitty/current-theme.conf"`,
		`output_path = "` + cfgHome + `/gtk-3.0/gtk.css"`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("matugen was handed a config without %s:\n%s", want, got)
		}
	}
	if strings.Contains(string(got), "~/") {
		t.Errorf("matugen config still has a home-relative path:\n%s", got)
	}
}
