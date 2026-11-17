package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func desktopStore(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".config/ryoku/desktop.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	return root
}

func desktopInput(t *testing.T, root map[string]any) map[string]any {
	t.Helper()
	desktop, ok := root["desktop"].(map[string]any)
	if !ok {
		t.Fatalf("desktop settings = %#v", root["desktop"])
	}
	input, ok := desktop["input"].(map[string]any)
	if !ok {
		t.Fatalf("desktop input settings = %#v", desktop["input"])
	}
	return input
}

func TestSeedDesktopKeymapMergesNeutralStore(t *testing.T) {
	t.Run("absent store", func(t *testing.T) {
		home := t.TempDir()
		e := &engine{f: &facts{
			homeDir: home, kbLayout: "fr", kbVariant: "azerty", kbOptions: "ctrl:nocaps", kbSource: "GNOME",
		}, p: &plan{}}
		seeded, err := seedDesktopKeymap(e)
		if err != nil {
			t.Fatal(err)
		}
		if !seeded {
			t.Fatal("a fresh store was not seeded")
		}
		input := desktopInput(t, desktopStore(t, home))
		if input["kbLayout"] != "fr" || input["kbVariant"] != "azerty" || input["kbOptions"] != "ctrl:nocaps" {
			t.Fatalf("seeded input = %#v", input)
		}
	})

	t.Run("unrelated settings survive", func(t *testing.T) {
		home := t.TempDir()
		store := filepath.Join(home, ".config/ryoku/desktop.json")
		if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
			t.Fatal(err)
		}
		before := `{"appearance":{"theme":"sumi"},"desktop":{"apps":{"browser":"firefox"},"input":{"repeatRate":31}}}`
		if err := os.WriteFile(store, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		e := &engine{f: &facts{homeDir: home, kbLayout: "de", kbVariant: "nodeadkeys", kbSource: "KDE"}, p: &plan{}}
		if _, err := seedDesktopKeymap(e); err != nil {
			t.Fatal(err)
		}
		root := desktopStore(t, home)
		appearance, ok := root["appearance"].(map[string]any)
		if !ok || appearance["theme"] != "sumi" {
			t.Fatalf("appearance settings were lost: %#v", root)
		}
		desktop := root["desktop"].(map[string]any)
		apps, ok := desktop["apps"].(map[string]any)
		if !ok || apps["browser"] != "firefox" {
			t.Fatalf("app settings were lost: %#v", desktop)
		}
		input := desktopInput(t, root)
		if input["repeatRate"] != float64(31) || input["kbLayout"] != "de" {
			t.Fatalf("input settings were not merged: %#v", input)
		}
	})

	t.Run("existing non-us layout wins", func(t *testing.T) {
		home := t.TempDir()
		store := filepath.Join(home, ".config/ryoku/desktop.json")
		if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
			t.Fatal(err)
		}
		before := `{"desktop":{"input":{"kbLayout":"de","kbVariant":"nodeadkeys","kbOptions":"ctrl:nocaps"}}}`
		if err := os.WriteFile(store, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		e := &engine{f: &facts{homeDir: home, kbLayout: "fr", kbSource: "GNOME"}, p: &plan{}}
		seeded, err := seedDesktopKeymap(e)
		if err != nil {
			t.Fatal(err)
		}
		if seeded {
			t.Fatal("a deliberate layout was reported as seeded, so the system keymap would be pushed over it")
		}
		input := desktopInput(t, desktopStore(t, home))
		if input["kbLayout"] != "de" || input["kbVariant"] != "nodeadkeys" || input["kbOptions"] != "ctrl:nocaps" {
			t.Fatalf("existing input was overwritten: %#v", input)
		}
	})

	t.Run("explicit AZERTY overrides", func(t *testing.T) {
		home := t.TempDir()
		store := filepath.Join(home, ".config/ryoku/desktop.json")
		if err := os.MkdirAll(filepath.Dir(store), 0o755); err != nil {
			t.Fatal(err)
		}
		before := `{"desktop":{"input":{"kbLayout":"de","kbVariant":"nodeadkeys","kbOptions":"ctrl:nocaps"}}}`
		if err := os.WriteFile(store, []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		e := &engine{f: &facts{homeDir: home, kbLayout: "de", kbSource: "KDE"}, p: &plan{azertyBE: true}}
		if _, err := seedDesktopKeymap(e); err != nil {
			t.Fatal(err)
		}
		input := desktopInput(t, desktopStore(t, home))
		if input["kbLayout"] != "be" || input["kbVariant"] != "" || input["kbOptions"] != "" {
			t.Fatalf("explicit AZERTY did not override the store: %#v", input)
		}
	})
}

// A plain us keyboard carries nothing: the store default already says us, and
// pushing it onto the console would only touch files for no change.
func TestSeedDesktopKeymapSkipsPlainUS(t *testing.T) {
	home := t.TempDir()
	e := &engine{f: &facts{homeDir: home, kbLayout: "us", kbSource: "localectl"}, p: &plan{}}
	seeded, err := seedDesktopKeymap(e)
	if err != nil {
		t.Fatal(err)
	}
	if seeded {
		t.Fatal("a plain us layout was seeded")
	}
	if _, err := os.Stat(filepath.Join(home, ".config/ryoku/desktop.json")); err == nil {
		t.Fatal("a store was written for a plain us layout")
	}
}

// A KDE or GNOME pair carries no xkb switch key (their own shortcut does not
// come along), so the seeded layouts would be unreachable at the login screen.
func TestSeedDesktopKeymapGivesACarriedPairASwitchKey(t *testing.T) {
	for _, tc := range []struct{ name, layout, variant, options, wantLayout, wantVariant, wantOptions string }{
		{"no options", "ru,us", ",", "", "ru,us", ",", "grp:alt_shift_toggle"},
		{"other options kept", "ru,us", "", "ctrl:nocaps", "ru,us", "", "ctrl:nocaps,grp:alt_shift_toggle"},
		{"own switch key kept", "ru,us", "", "grp:win_space_toggle", "ru,us", "", "grp:win_space_toggle"},
		{"single layout untouched", "ru", "", "", "ru", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			e := &engine{f: &facts{
				homeDir: home, kbLayout: tc.layout, kbVariant: tc.variant, kbOptions: tc.options, kbSource: "KDE",
			}, p: &plan{}}
			if _, err := seedDesktopKeymap(e); err != nil {
				t.Fatal(err)
			}
			input := desktopInput(t, desktopStore(t, home))
			if input["kbLayout"] != tc.wantLayout || input["kbVariant"] != tc.wantVariant || input["kbOptions"] != tc.wantOptions {
				t.Fatalf("seeded input = %#v", input)
			}
		})
	}
}

// The console and login screen follow the carried layout through localectl
// under the installer's own sudo, and the restore script puts the previous
// X11 keymap back.
func TestApplySystemKeymapUsesSudoAndRecordsRestore(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "sudo.log")
	sudo := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$SUDO_LOG\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "sudo"), []byte(sudo), 0o755); err != nil {
		t.Fatal(err)
	}
	localectl := "#!/bin/sh\nprintf '   System Locale: LANG=en_US.UTF-8\\n       VC Keymap: us\\n      X11 Layout: us\\n     X11 Variant: intl\\n'\n"
	if err := os.WriteFile(filepath.Join(binDir, "localectl"), []byte(localectl), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SUDO_LOG", logPath)
	restorePath := filepath.Join(root, "restore.sh")
	if err := os.WriteFile(restorePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := &engine{f: &facts{homeDir: root}, p: &plan{}, restorePath: restorePath}
	if err := applySystemKeymap(e, "de", "nodeadkeys", "caps:escape"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := "-n localectl set-x11-keymap de  nodeadkeys caps:escape"; strings.TrimSpace(string(got)) != want {
		t.Fatalf("sudo call = %q, want %q", strings.TrimSpace(string(got)), want)
	}
	restore, err := os.ReadFile(restorePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(restore), `sudo localectl set-x11-keymap "us" '' "intl" ""`) {
		t.Fatalf("restore script does not put the previous keymap back: %q", restore)
	}
}
