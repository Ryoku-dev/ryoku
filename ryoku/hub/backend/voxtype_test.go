package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVoxtypePresetsAreCuratedWhisperCatalogue(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	want := []struct {
		key, model, size, language, speed, group, configLanguage string
		sizeBytes                                                int64
	}{
		{"whisper-tiny-en", "tiny.en", "39 MB", "English", "fastest", "english", "en", 39_000_000},
		{"whisper-fast", "base.en", "142 MB", "English", "fast", "english", "en", 142_000_000},
		{"whisper-small-en", "small.en", "466 MB", "English", "balanced", "english", "en", 466_000_000},
		{"whisper-base", "base", "142 MB", "99+ languages", "fast", "multilingual", "auto", 142_000_000},
		{"whisper-medium", "medium", "1.5 GB", "99+ languages", "quality", "multilingual", "auto", 1_500_000_000},
		{"whisper-accurate", "large-v3-turbo", "1.6 GB", "99+ languages", "fast + accurate", "multilingual", "auto", 1_600_000_000},
		{"openai", "whisper-1", "cloud", "99+ languages", "cloud", "cloud", "en", 0},
	}

	got := voxtypePresets()
	if len(got) != len(want) {
		t.Fatalf("preset count = %d, want %d", len(got), len(want))
	}
	seen := make(map[string]bool, len(got))
	for i, p := range got {
		w := want[i]
		if seen[p.Key] {
			t.Fatalf("duplicate preset key %q", p.Key)
		}
		seen[p.Key] = true
		if p.Key != w.key || p.model != w.model || p.Size != w.size || p.SizeBytes != w.sizeBytes ||
			p.Language != w.language || p.Speed != w.speed || p.Group != w.group || p.lang != w.configLanguage {
			t.Errorf("preset %d = %#v, want key=%q model=%q size=%q bytes=%d language=%q speed=%q group=%q config language=%q",
				i, p, w.key, w.model, w.size, w.sizeBytes, w.language, w.speed, w.group, w.configLanguage)
		}
	}
}

func TestVoxtypeLocalModelsUseWhisperFilenames(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	want := map[string]string{
		"whisper-tiny-en":  "ggml-tiny.en.bin",
		"whisper-fast":     "ggml-base.en.bin",
		"whisper-small-en": "ggml-small.en.bin",
		"whisper-base":     "ggml-base.bin",
		"whisper-medium":   "ggml-medium.bin",
		"whisper-accurate": "ggml-large-v3-turbo.bin",
	}
	for _, p := range voxtypePresets() {
		if p.Cloud || p.Key == reduxPresetKey {
			continue
		}
		expected := filepath.Join(root, "voxtype", "models", want[p.Key])
		if got := modelFilePath(p); got != expected {
			t.Errorf("%s model path = %q, want %q", p.Key, got, expected)
		}
	}
}

func TestVoxtypeMultilingualConfigsUseLanguageAuto(t *testing.T) {
	for _, key := range []string{"whisper-base", "whisper-medium", "whisper-accurate"} {
		p, ok := presetByKey(key)
		if !ok {
			t.Fatalf("missing preset %q", key)
		}
		want := "language = \"auto\""
		if config := buildVoxtypeConfig(p, ""); !strings.Contains(config, "\n"+want+"\n") {
			t.Errorf("%s config does not contain %q:\n%s", key, want, config)
		}
	}
}

func TestVoxtypeConfigsLoadWhisperOnDemand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p, ok := presetByKey("whisper-fast")
	if !ok {
		t.Fatal("missing whisper-fast preset")
	}
	if config := buildVoxtypeConfig(p, ""); !strings.Contains(config, "\n[whisper]\non_demand_loading = true\n") {
		t.Fatalf("config does not enable on-demand loading:\n%s", config)
	}
}

func TestVoxtypeEnsureMigratesManagedConfigToOnDemand(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "voxtype"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, ok := presetByKey("whisper-fast")
	if !ok {
		t.Fatal("missing whisper-fast preset")
	}
	config := strings.Replace(buildVoxtypeConfig(p, ""), "on_demand_loading = true\n", "", 1)
	if err := os.MkdirAll(filepath.Dir(voxtypeConfigPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(voxtypeConfigPath(), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := voxtypeEnsure(); err != nil {
		t.Fatalf("voxtypeEnsure() error = %v", err)
	}
	got, err := os.ReadFile(voxtypeConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(got), "on_demand_loading = true") != 1 {
		t.Fatalf("migrated config does not contain one on-demand setting:\n%s", got)
	}
}

func TestVoxtypeMigrationPreservesUserConfig(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p, ok := presetByKey("whisper-fast")
	if !ok {
		t.Fatal("missing whisper-fast preset")
	}
	managedFalse := strings.Replace(buildVoxtypeConfig(p, ""), "on_demand_loading = true", "on_demand_loading = false", 1)
	tests := []struct {
		name   string
		config string
	}{
		{name: "explicit managed value", config: managedFalse},
		{name: "unmanaged config", config: "[whisper]\nmodel = \"base.en\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			if err := os.MkdirAll(filepath.Dir(voxtypeConfigPath()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(voxtypeConfigPath(), []byte(tt.config), 0o600); err != nil {
				t.Fatal(err)
			}
			changed, err := migrateVoxtypeOnDemandLoading()
			if err != nil {
				t.Fatalf("migrateVoxtypeOnDemandLoading() error = %v", err)
			}
			if changed {
				t.Fatal("user config was migrated")
			}
			got, err := os.ReadFile(voxtypeConfigPath())
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.config {
				t.Fatalf("config changed:\n%s", got)
			}
		})
	}
}

func TestVoxtypeEnsureDoesNothingWithoutVoxtype(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("PATH", t.TempDir())

	if err := voxtypeEnsure(); err != nil {
		t.Fatalf("voxtypeEnsure() error = %v", err)
	}
	if _, err := os.Stat(voxtypeConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("config created without voxtype installed: stat error = %v", err)
	}
}

func TestVoxtypePackageAvailabilityUsesVoidReason(t *testing.T) {
	bin := t.TempDir()
	host := filepath.Join(bin, "ryoku-host")
	script := `#!/bin/sh
if [ "$1" = pkgmgr ]; then
	printf 'xbps\n'
	exit 0
fi
if [ "$1" = pkg ] && [ "$2" = available ] && [ "$3" = --aur ] && [ "$4" = voxtype-bin ]; then
	exit 3
fi
if [ "$1" = pkg ] && [ "$2" = why ] && [ "$3" = voxtype-bin ]; then
	printf '%s\n' "Voxtype needs packages this system does not provide."
	exit 0
fi
exit 1
`
	if err := os.WriteFile(host, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	available, reason := voxtypePackageAvailability()
	if available {
		t.Fatal("voxtype package reported available")
	}
	if reason != "Voxtype needs packages this system does not provide." {
		t.Fatalf("reason = %q", reason)
	}
}

func TestVoxtypePackageAvailabilityUsesAURGateOnArch(t *testing.T) {
	bin := t.TempDir()
	host := filepath.Join(bin, "ryoku-host")
	script := `#!/bin/sh
if [ "$1" = pkgmgr ]; then
	printf 'pacman\n'
	exit 0
fi
if [ "$1" = pkg ] && [ "$2" = available ] && [ "$3" = --aur ] && [ "$4" = voxtype-bin ]; then
	exit 0
fi
if [ "$1" = pkg ] && [ "$2" = available ]; then
	exit 3
fi
exit 1
`
	if err := os.WriteFile(host, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	available, reason := voxtypePackageAvailability()
	if !available || reason != "" {
		t.Fatalf("availability = %t, reason = %q", available, reason)
	}
}

func TestEnsureVoxtypeUnitSkipsSystemdSetupOnRunit(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(bin, "voxtype-called")
	host := "#!/bin/sh\nif [ \"$1\" = init ]; then printf 'runit\\n'; exit 0; fi\nexit 1\n"
	voxtype := "#!/bin/sh\nprintf called > " + marker + "\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(host), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "voxtype"), []byte(voxtype), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	ensureVoxtypeUnit()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("voxtype systemd setup ran on runit (stat error = %v)", err)
	}
}

func TestEnsureVoxtypeUnitRunsSetupOnSystemd(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(bin, "voxtype-called")
	host := "#!/bin/sh\nif [ \"$1\" = init ]; then printf 'systemd\\n'; fi\nexit 0\n"
	voxtype := "#!/bin/sh\nif [ \"$1\" = setup ] && [ \"$2\" = systemd ]; then printf called > " + marker + "; fi\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(host), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "voxtype"), []byte(voxtype), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	ensureVoxtypeUnit()
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("voxtype systemd setup did not run: %v", err)
	}
}

// a refused service call puts config.toml and the Redux drop-in back as they were
// and re-runs the service step for them, so the Hub can show the change as not made
func TestVoxtypeSetRollsBackOnServiceFailure(t *testing.T) {
	for _, tc := range []struct {
		fail    string
		present bool
	}{{"daemon-reload", true}, {"restart", true}, {"restart", false}} {
		name := tc.fail
		if !tc.present {
			name += "-absent"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", root)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
			bin := filepath.Join(root, "bin")
			if err := os.MkdirAll(bin, 0o700); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(root, "calls")
			host := "#!/bin/sh\nif [ \"$1\" = init ]; then echo systemd; exit 0; fi\nprintf '%s\\n' \"$*\" >> '" + log + "'\n" +
				"case \"$*\" in *' " + tc.fail + " '*|*' " + tc.fail + "') exit 1;; esac\n"
			if err := os.WriteFile(filepath.Join(bin, "ryoku-host"), []byte(host), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			config, dropin := voxtypeConfigPath(), reduxDropinPath()
			want := map[string]string{config: "# mine\nremote_api_key = \"sk-old\"\n", dropin: "[Unit]\nRequires=voxtype-redux.service\n# hand\n"}
			modes := map[string]os.FileMode{config: 0o600, dropin: 0o644}
			if tc.present {
				for path, body := range want {
					if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(body), modes[path]); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := voxtypeSet(`{"preset":"whisper-fast","enabled":true}`); err == nil {
				t.Fatal("voxtypeSet succeeded with a failing service call")
			}
			for path, body := range want {
				st, err := os.Stat(path)
				if !tc.present {
					if err == nil {
						t.Fatalf("%s left behind by a failed set", path)
					}
					continue
				}
				if err != nil {
					t.Fatalf("%s gone after a failed set: %v", path, err)
				}
				if got := readFileString(path); got != body || st.Mode().Perm() != modes[path] {
					t.Fatalf("%s = %q %v, want %q %v", path, got, st.Mode().Perm(), body, modes[path])
				}
			}
			if n := strings.Count(readFileString(log), "daemon-reload"); n != 2 {
				t.Fatalf("daemon-reload ran %d times, want the set and the rollback:\n%s", n, readFileString(log))
			}
		})
	}
}
