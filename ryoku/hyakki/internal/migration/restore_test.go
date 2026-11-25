package migration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"ryoku-rashin/internal/paths"
)

func TestRestoreOriginalBytesPreservesAllNewHyakkiWork(t *testing.T) {
	migrationFixture(t)
	oldModel := filepath.Join(paths.LegacyRashinData(), "models", "original.gguf")
	model := append([]byte("GGUF"), bytes.Repeat([]byte{0, 255, 7}, 4096)...)
	migrationPut(t, oldModel, model, 0o640)
	oldRuntime := filepath.Join(paths.LegacyRashinData(), "llama.cpp", "llama-b42", "bin", "llama-server")
	migrationPut(t, oldRuntime, []byte("binary bytes\x00\xff"), 0o750)
	config := []byte("{ \"providers\": {\"models\": {\"binary\": " + migrationJSONString(oldRuntime) + ", \"folders\": [" + migrationJSONString(filepath.Dir(oldModel)) + "]}}, \"opaque\": 9007199254740993 }\n")
	migrationPut(t, paths.LegacyRashinConfig(), config, 0o640)
	startup := filepath.Join(paths.LegacyRashinData(), "screen-home", ".mozilla", "firefox", "profile", "addonStartup.json.lz4")
	startupBytes := []byte("mozLz40\x00\x00opaque-compressed-startup\xff")
	migrationPut(t, startup, startupBytes, 0o600)
	installed := filepath.Join(paths.LegacyRashinData(), "hindsight", "installed")
	migrationPut(t, installed, []byte("pinned-runtime-lock-sum\n"), 0o600)
	key := bytes.Repeat([]byte{0xff, 0, 0x42, 1}, 8)
	migrationPut(t, filepath.Join(paths.LegacyRashinState(), "master.key"), key, 0o600)
	migrationPut(t, filepath.Join(paths.LegacyRashinState(), "sessions", "original", "meta.json"), []byte("{\"id\":\"original\",\"kind\":\"ask\",\"title\":\"original title\"}"), 0o600)
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	newConfig, err := os.ReadFile(paths.ConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(newConfig, []byte(paths.LlamaRuntimeDir())) || bytes.Contains(newConfig, []byte(oldRuntime)) {
		t.Fatal("config's owned executable was not rebased")
	}
	migrationAbsent(t, filepath.Join(paths.DataDir(), "screen-home", ".mozilla", "firefox", "profile", "addonStartup.json.lz4"))
	migrationAbsent(t, filepath.Join(paths.DataDir(), "hindsight", "installed"))
	backup := migrationBackup(t)
	// Both new files and edits of migrated content survive restore unchanged.
	migrationPut(t, filepath.Join(paths.ModelsDir(), "new.gguf"), []byte("new download"), 0o600)
	migrationPut(t, filepath.Join(paths.ModelsDir(), "original.gguf"), []byte("newer model revision"), 0o600)
	migrationPut(t, filepath.Join(paths.SessionsDir(), "new", "events.jsonl"), []byte("new conversation"), 0o600)
	migrationPut(t, paths.MasterKey(), []byte("new active encryption key"), 0o600)
	if err := Restore(backup); err != nil {
		t.Fatal(err)
	}
	migrationBytes(t, oldModel, model)
	migrationBytes(t, oldRuntime, []byte("binary bytes\x00\xff"))
	migrationBytes(t, paths.LegacyRashinConfig(), config)
	migrationBytes(t, startup, startupBytes)
	migrationBytes(t, installed, []byte("pinned-runtime-lock-sum\n"))
	migrationBytes(t, filepath.Join(paths.LegacyRashinState(), "master.key"), key)
	migrationBytes(t, filepath.Join(paths.ModelsDir(), "new.gguf"), []byte("new download"))
	migrationBytes(t, filepath.Join(paths.ModelsDir(), "original.gguf"), []byte("newer model revision"))
	migrationBytes(t, filepath.Join(paths.SessionsDir(), "new", "events.jsonl"), []byte("new conversation"))
	migrationBytes(t, paths.MasterKey(), []byte("new active encryption key"))
	migrationBytes(t, paths.ConfigFile(), newConfig)
	info, err := os.Stat(oldRuntime)
	if err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("restored executable mode: %v %v", info, err)
	}
	if err := Restore(backup); err != nil {
		t.Fatalf("idempotent restore: %v", err)
	}
	if err := Ensure(); err != nil {
		t.Fatalf("restored legacy content caused destructive re-migration: %v", err)
	}
}

func migrationJSONString(value string) string { b, _ := json.Marshal(value); return string(b) }

func TestRestoreConflictingOriginalRefusesBeforeAnyRestore(t *testing.T) {
	migrationFixture(t)
	migrationPut(t, paths.LegacyRashinConfig(), []byte("{}\n"), 0o600)
	migrationPut(t, filepath.Join(paths.LegacyRashinState(), "master.key"), []byte("original key"), 0o600)
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	backup := migrationBackup(t)
	migrationPut(t, paths.LegacyRashinConfig(), []byte("new legacy settings"), 0o600)
	if err := Restore(backup); err == nil || !strings.Contains(err.Error(), "newer legacy content") {
		t.Fatalf("expected overwrite refusal: %v", err)
	}
	migrationBytes(t, paths.LegacyRashinConfig(), []byte("new legacy settings"))
	migrationAbsent(t, paths.LegacyRashinState())
	migrationBytes(t, paths.MasterKey(), []byte("original key"))
}

func TestRestoreRejectsActiveNewAndOldOwners(t *testing.T) {
	for _, current := range []bool{false, true} {
		t.Run(map[bool]string{false: "old", true: "new"}[current], func(t *testing.T) {
			migrationFixture(t)
			migrationPut(t, paths.LegacyRashinConfig(), []byte("{}"), 0o600)
			if err := Ensure(); err != nil {
				t.Fatal(err)
			}
			backup := migrationBackup(t)
			lock := filepath.Join(paths.LegacyRashinRuntime(), "rashind.lock")
			if current {
				lock = paths.AppLock()
			}
			f, err := os.OpenFile(lock, os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				t.Fatal(err)
			}
			if err := Restore(backup); err == nil || !strings.Contains(err.Error(), "using its data") {
				t.Fatalf("restore ignored active owner: %v", err)
			}
			migrationAbsent(t, paths.LegacyRashinConfig())
			migrationBytes(t, paths.ConfigFile(), []byte("{}"))
		})
	}
}

func TestRestoreRejectsForeignForgedAndDamagedSnapshots(t *testing.T) {
	for _, damage := range []string{"outside", "public", "manifest-path", "snapshot-bytes", "backup-symlink"} {
		t.Run(damage, func(t *testing.T) {
			migrationFixture(t)
			migrationPut(t, paths.LegacyRashinConfig(), []byte("{}"), 0o600)
			if err := Ensure(); err != nil {
				t.Fatal(err)
			}
			backup := migrationBackup(t)
			target := backup
			switch damage {
			case "outside":
				target = t.TempDir()
			case "public":
				if err := os.Chmod(backup, 0o755); err != nil {
					t.Fatal(err)
				}
			case "manifest-path":
				m, err := readBackup(backup)
				if err != nil {
					t.Fatal(err)
				}
				m.Entries[0].Source = filepath.Join(paths.Home(), "unrelated-original")
				if err := writeManifest(backup, m); err != nil {
					t.Fatal(err)
				}
			case "snapshot-bytes":
				migrationPut(t, snapshotPath(backup, 0), []byte("corrupted"), 0o600)
			case "backup-symlink":
				target = filepath.Join(paths.MigrationBackups(), "rashin-to-hyakki-symlink")
				if err := os.Symlink(backup, target); err != nil {
					t.Fatal(err)
				}
			}
			if err := Restore(target); err == nil {
				t.Fatal("unsafe backup accepted")
			}
			migrationAbsent(t, paths.LegacyRashinConfig())
			migrationBytes(t, paths.ConfigFile(), []byte("{}"))
		})
	}
}

func TestEnsureAndRestoreNeverMoveConfigSymlinkTarget(t *testing.T) {
	migrationFixture(t)
	dotfile := filepath.Join(paths.Home(), "dotfiles", "rashin.json")
	original := []byte("{\"authored\":\"settings from dotfiles\"}\n")
	migrationPut(t, dotfile, original, 0o640)
	migrationPut(t, filepath.Join(paths.Home(), "dotfiles", ".git", "config"), []byte("dotfiles git"), 0o600)
	if err := os.MkdirAll(filepath.Dir(paths.LegacyRashinConfig()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dotfile, paths.LegacyRashinConfig()); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	migrationBytes(t, dotfile, original)
	migrationBytes(t, paths.ConfigFile(), original)
	target, err := os.Readlink(paths.LegacyRashinConfig())
	if err != nil || target != dotfile {
		t.Fatalf("user symlink moved or changed: %s %v", target, err)
	}
	// The owner may edit their dotfiles after cutover; restore must not modify
	// that target merely because the retained original is a symlink to it.
	migrationPut(t, dotfile, []byte("new authored settings"), 0o640)
	if err := Restore(migrationBackup(t)); err != nil {
		t.Fatal(err)
	}
	migrationBytes(t, dotfile, []byte("new authored settings"))
	migrationBytes(t, paths.ConfigFile(), original)
}

func TestEnsureInteriorSymlinksPreserveTargetsWithoutCopyingExternalData(t *testing.T) {
	migrationFixture(t)
	root := filepath.Join(paths.LegacyRashinData(), "attachments")
	original := filepath.Join(root, "image.bin")
	migrationPut(t, original, []byte("image payload"), 0o600)
	external := filepath.Join(paths.Home(), "external", "private.bin")
	migrationPut(t, external, []byte("external private content"), 0o600)
	links := map[string]string{"absolute-internal": original, "relative-internal": "image.bin", "absolute-external": external}
	relExternal, err := filepath.Rel(root, external)
	if err != nil {
		t.Fatal(err)
	}
	links["relative-external"] = relExternal
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"absolute-internal", "relative-internal"} {
		migrationBytes(t, filepath.Join(paths.AttachmentsDir(), name), []byte("image payload"))
	}
	for _, name := range []string{"absolute-external", "relative-external"} {
		migrationBytes(t, filepath.Join(paths.AttachmentsDir(), name), []byte("external private content"))
	}
	migrationBytes(t, external, []byte("external private content"))
	backup := migrationBackup(t)
	if err := Restore(backup); err != nil {
		t.Fatal(err)
	}
	for name, want := range links {
		got, err := os.Readlink(filepath.Join(root, name))
		if err != nil || got != want {
			t.Fatalf("restore changed original symlink %s: %q %v", name, got, err)
		}
	}
}

func TestEnsureRootAndDestinationAncestorSymlinksRefused(t *testing.T) {
	for _, scenario := range []string{"source-root", "destination-root", "destination-ancestor"} {
		t.Run(scenario, func(t *testing.T) {
			migrationFixture(t)
			external := filepath.Join(paths.Home(), "external")
			migrationPut(t, filepath.Join(external, "private.gguf"), []byte("do not move"), 0o600)
			switch scenario {
			case "source-root":
				if err := os.MkdirAll(paths.LegacyRashinData(), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, filepath.Join(paths.LegacyRashinData(), "models")); err != nil {
					t.Fatal(err)
				}
			case "destination-root", "destination-ancestor":
				migrationPut(t, filepath.Join(paths.LegacyRashinData(), "models", "original.gguf"), []byte("original"), 0o600)
				link := paths.DataDir()
				if scenario == "destination-root" {
					if err := os.MkdirAll(paths.DataDir(), 0o700); err != nil {
						t.Fatal(err)
					}
					link = paths.ModelsDir()
				}
				if err := os.Symlink(external, link); err != nil {
					t.Fatal(err)
				}
			}
			if err := Ensure(); err == nil {
				t.Fatal("unsafe symlink accepted")
			}
			migrationBytes(t, filepath.Join(external, "private.gguf"), []byte("do not move"))
			migrationAbsent(t, paths.MigrationBackups())
		})
	}
}

func TestIncompleteBackupHasNoManifestOrSuccessPointer(t *testing.T) {
	migrationFixture(t)
	migrationPut(t, paths.LegacyRashinConfig(), []byte("{}"), 0o600)
	plan, err := relocationPlan()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(paths.LegacyRashinConfig()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := createBackup(plan); err == nil {
		t.Fatal("source IO failure falsely produced a backup")
	}
	migrationAbsent(t, filepath.Join(paths.MigrationBackups(), "complete.json"))
	entries, err := os.ReadDir(paths.MigrationBackups())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("incomplete backup left usable-looking transaction data")
	}
}
