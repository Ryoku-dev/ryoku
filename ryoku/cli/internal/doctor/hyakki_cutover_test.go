package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"ryoku-cli/internal/host"
)

func TestHyakkiReconcileUsesMigratedPreference(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(strconv.FormatBool(enabled), func(t *testing.T) {
			config, bin := t.TempDir(), t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", config)
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			t.Setenv("PATH", bin)
			if err := os.WriteFile(filepath.Join(bin, "hyakki"), []byte("#!/bin/sh\nexit 99\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			oldInit, oldService, oldCommand := hyakkiInit, hyakkiService, hyakkiCommand
			oldRunning, oldVersion, oldMigration := oniRunning, reportPackageVersion, hyakkiMigration
			t.Cleanup(func() {
				hyakkiInit, hyakkiService, hyakkiCommand = oldInit, oldService, oldCommand
				oniRunning, reportPackageVersion, hyakkiMigration = oldRunning, oldVersion, oldMigration
			})
			hyakkiInit = func() (host.InitSystem, error) { return host.Systemd, nil }
			oniRunning = func() bool { return false }
			reportPackageVersion = func(string) (string, bool) { return "cutover", true }
			armed := map[string]bool{}
			hyakkiService = func(args []string) int {
				unit := args[len(args)-1]
				switch args[1] {
				case "is-enabled", "is-active":
					if armed[unit] {
						return host.ExitOK
					}
					return host.ExitAbsent
				case "enable":
					armed[unit] = true
				case "disable", "stop":
					armed[unit] = false
				case "daemon-reload":
				default:
					t.Fatalf("unexpected supervisor operation: %v", args)
				}
				return host.ExitOK
			}
			migrated, off := false, false
			hyakkiMigration = func() error {
				migrated = true
				path := filepath.Join(config, "hyakki", "hyakki.json")
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					return err
				}
				return os.WriteFile(path, []byte(`{"enabled":`+strconv.FormatBool(enabled)+`}`), 0o600)
			}
			hyakkiCommand = func(args ...string) error {
				switch args[0] {
				case "off":
					off = true
				default:
					t.Fatalf("doctor changed the preference explicitly: %v", args)
				}
				return nil
			}
			if result := reconcileHyakkiDaemon(false); result.status != recFixed {
				t.Fatalf("cutover result = %+v", result)
			}
			if !migrated || readHyakkiEnabled() != enabled || off == enabled {
				t.Fatalf("migrated preference was not respected: enabled=%v migrated=%v off=%v", readHyakkiEnabled(), migrated, off)
			}
			if armed[oniSocket] != enabled || armed[aiUsageTimer] != enabled || armed[routinesTimer] {
				t.Fatalf("wrong services armed after migration: %v", armed)
			}
		})
	}
}

func TestRetiredVisionSettingsPreserveClipboardAndUnrelatedData(t *testing.T) {
	for _, existing := range []string{"", `,"ocrLanguages":["fra"]`} {
		raw := []byte(`{"sight":{"on":true,"features":{"inplace":true},"ocrLanguages":["eng","jpn"]},"clipboard":{"readPictures":false` + existing + `},"unrelated":9007199254740993123}`)
		top, changed, err := retireVisionSettings(raw)
		if err != nil || !changed {
			t.Fatalf("migration = %v, %v", changed, err)
		}
		if _, exists := top["sight"]; exists || string(top["unrelated"]) != "9007199254740993123" || top["oni"] != nil {
			t.Fatalf("retired permissions were kept or unrelated data changed: %s", top)
		}
		var clipboard struct {
			ReadPictures bool     `json:"readPictures"`
			Languages    []string `json:"ocrLanguages"`
		}
		if err := json.Unmarshal(top["clipboard"], &clipboard); err != nil {
			t.Fatal(err)
		}
		want := `["eng","jpn"]`
		if existing != "" {
			want = `["fra"]`
		}
		got, _ := json.Marshal(clipboard.Languages)
		if clipboard.ReadPictures || string(got) != want {
			t.Fatalf("clipboard preference changed: %+v", clipboard)
		}
		out, err := json.Marshal(top)
		if err != nil {
			t.Fatal(err)
		}
		if _, again, err := retireVisionSettings(out); err != nil || again {
			t.Fatalf("migration is not idempotent: %v, %v", again, err)
		}
	}
}

func TestRetiredVisionReconcilerChecksWithoutWritingAndBacksUpBeforeReplacing(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	path := filepath.Join(config, "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("{\n  \"sight\": {\"enabled\": true, \"ocrLanguages\": [\"jpn\"]},\n  \"clipboard\": {\"readPictures\": false}\n}\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if result := reconcileRetiredVisionSettings(true); result.status != recWouldFix {
		t.Fatalf("check result = %+v", result)
	}
	before, _ := os.ReadFile(path)
	backups, _ := filepath.Glob(path + ".before-oni-vision-*")
	if !bytes.Equal(before, raw) || len(backups) != 0 {
		t.Fatal("check-only changed preferences or wrote a backup")
	}
	if result := reconcileRetiredVisionSettings(false); result.status != recFixed {
		t.Fatalf("migration result = %+v", result)
	}
	backups, _ = filepath.Glob(path + ".before-oni-vision-*")
	if len(backups) != 1 {
		t.Fatalf("expected one original backup, got %v", backups)
	}
	saved, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(saved, raw) {
		t.Fatalf("backup differs from original: %q, %v", saved, err)
	}
	info, err := os.Stat(backups[0])
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup is not private: %v, %v", info, err)
	}
	info, err = os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("migration widened shell preferences' permissions: %v, %v", info, err)
	}
	if result := reconcileRetiredVisionSettings(false); result.status != recOK {
		t.Fatalf("repeat migration = %+v", result)
	}
}
