package migration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/session"
)

func migrationFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	for key, dir := range map[string]string{"XDG_CONFIG_HOME": "config", "XDG_STATE_HOME": "state", "XDG_DATA_HOME": "data", "XDG_CACHE_HOME": "cache", "XDG_RUNTIME_DIR": "runtime"} {
		t.Setenv(key, filepath.Join(root, dir))
	}
	return root
}

func migrationPut(t *testing.T, path string, content []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}

func migrationBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected bytes at %s", path)
	}
}

func migrationAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected absent %s, got %v", path, err)
	}
}

func migrationBackup(t *testing.T) string {
	t.Helper()
	backup, err := loadPointer("complete.json")
	if err != nil {
		t.Fatal(err)
	}
	return backup
}

func TestEnsureFreshAndSharedVaultOnlyCreateNothing(t *testing.T) {
	migrationFixture(t)
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.StateHome(), paths.RuntimeDir(), paths.DataDir(), paths.MigrationBackups()} {
		migrationAbsent(t, path)
	}
	vault := filepath.Join(paths.LegacyRashinData(), "memory", "notes.md")
	migrationPut(t, vault, []byte("shared knowledge"), 0o600)
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	migrationBytes(t, vault, []byte("shared knowledge"))
	for _, path := range []string{paths.StateHome(), paths.RuntimeDir(), paths.DataDir(), paths.MigrationBackups()} {
		migrationAbsent(t, path)
	}
}

func TestEnsurePreservesSessionsKeysModelsConfigAndSharedData(t *testing.T) {
	migrationFixture(t)
	model := append([]byte("GGUF\x00\xff\x00"), bytes.Repeat([]byte{0, 1, 2, 255, 128}, 8192)...)
	key := bytes.Repeat([]byte{0x91, 0x02, 0xff, 0x00}, 8)
	legacyModel := filepath.Join(paths.LegacyRashinData(), "models", "publisher", "model.gguf")
	migrationPut(t, legacyModel, model, 0o640)
	migrationPut(t, filepath.Join(paths.LegacyRashinData(), "llama.cpp", "llama-b42", "bin", "llama-server"), []byte("executable"), 0o750)
	migrationPut(t, filepath.Join(paths.LegacyRashinState(), "master.key"), key, 0o600)
	migrationPut(t, filepath.Join(paths.LegacyRashinState(), "gateway.db"), []byte("opaque encrypted DB\x00\xff"), 0o600)
	migrationPut(t, filepath.Join(paths.LegacyRashinState(), "hindsight", "postgres", "base", "1234"), []byte("opaque PostgreSQL data\x00\xff"), 0o600)
	migrationPut(t, filepath.Join(paths.LegacyRashinData(), "hindsight", "runtime.bin"), []byte("managed runtime artifact\x00\xff"), 0o750)
	meta, _ := json.Marshal(map[string]any{"id": "ask-preserved", "kind": "ask", "title": "Preserved conversation", "cwd": paths.Home()})
	migrationPut(t, filepath.Join(paths.LegacyRashinState(), "sessions", "ask-preserved", "meta.json"), meta, 0o600)
	migrationPut(t, filepath.Join(paths.LegacyRashinState(), "sessions", "ask-preserved", "events.jsonl"), []byte(""), 0o600)
	migrationPut(t, paths.LegacyRashinConfig(), []byte("{\"active\":false,\"unknown_authored_setting\":\"keep-me\"}\n"), 0o640)
	migrationPut(t, paths.LegacyRashinAppState(), []byte("{\"width\":1050}"), 0o600)
	migrationPut(t, filepath.Join(paths.LegacyRashinCache(), "artifact.bin"), []byte("cached\x00bytes"), 0o600)
	untouched := []string{
		filepath.Join(paths.LegacyRashinData(), "AGENTS.md"), filepath.Join(paths.LegacyRashinData(), "desktop.md"),
		filepath.Join(paths.LegacyRashinData(), "wiki", "private.md"), filepath.Join(paths.LegacyRashinData(), "memory", "private.md"),
		filepath.Join(paths.LegacyRashinData(), "journal", "private.md"), filepath.Join(paths.LegacyRashinData(), "unrelated.bin"),
		filepath.Join(paths.DataHome(), "ryoku", "worktrees", "project", ".git"), filepath.Join(paths.Home(), "external.gguf"),
	}
	for _, path := range untouched {
		migrationPut(t, path, []byte("unrelated original"), 0o600)
	}
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	migrationBytes(t, filepath.Join(paths.ModelsDir(), "publisher", "model.gguf"), model)
	migrationBytes(t, paths.MasterKey(), key)
	migrationBytes(t, paths.GatewayDB(), []byte("opaque encrypted DB\x00\xff"))
	for _, path := range untouched {
		migrationBytes(t, path, []byte("unrelated original"))
	}
	migrationBytes(t, filepath.Join(paths.StateDir(), "hindsight", "postgres", "base", "1234"), []byte("opaque PostgreSQL data\x00\xff"))
	migrationBytes(t, filepath.Join(paths.DataDir(), "hindsight", "runtime.bin"), []byte("managed runtime artifact\x00\xff"))
	info, err := os.Stat(filepath.Join(paths.LlamaRuntimeDir(), "llama-b42", "bin", "llama-server"))
	if err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("executable permissions lost: %v %v", info, err)
	}
	info, err = os.Stat(filepath.Join(paths.ModelsDir(), "publisher", "model.gguf"))
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("model permissions lost: %v %v", info, err)
	}
	stored, err := session.List("")
	if err != nil || len(stored) != 1 || stored[0].ID != "ask-preserved" || stored[0].Title != "Preserved conversation" {
		t.Fatalf("session consumer lost migrated session: %#v %v", stored, err)
	}
	var config map[string]any
	b, err := os.ReadFile(paths.ConfigFile())
	if err != nil || json.Unmarshal(b, &config) != nil || config["unknown_authored_setting"] != "keep-me" {
		t.Fatalf("authored config lost: %v", err)
	}
	backup := migrationBackup(t)
	m, err := readBackup(backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(backup, m); err != nil {
		t.Fatal(err)
	}
	if m.Status != "committed" {
		t.Fatal("missing durable commit")
	}
	migrationAbsent(t, legacyModel)
	if err := Ensure(); err != nil {
		t.Fatalf("one-shot retry: %v", err)
	}
}

func TestEnsureOldOwnerRefusedBeforeMutation(t *testing.T) {
	for _, name := range []string{"app.lock", "rashind.lock", "config.lock"} {
		t.Run(name, func(t *testing.T) {
			migrationFixture(t)
			migrationPut(t, paths.LegacyRashinConfig(), []byte("{}"), 0o600)
			lockPath := filepath.Join(paths.LegacyRashinRuntime(), name)
			migrationPut(t, lockPath, nil, 0o600)
			f, err := os.OpenFile(lockPath, os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				t.Fatal(err)
			}
			if err := Ensure(); err == nil || !strings.Contains(err.Error(), "using its data") {
				t.Fatalf("missing actionable old-owner refusal: %v", err)
			}
			migrationBytes(t, paths.LegacyRashinConfig(), []byte("{}"))
			for _, path := range []string{paths.ConfigFile(), paths.StateDir(), paths.MigrationBackups(), paths.AppLock(), paths.DaemonLock()} {
				migrationAbsent(t, path)
			}
		})
	}
}

func TestEnsureDestinationConflictPreservesBoth(t *testing.T) {
	migrationFixture(t)
	old := filepath.Join(paths.LegacyRashinData(), "models", "original.gguf")
	current := filepath.Join(paths.ModelsDir(), "current.gguf")
	migrationPut(t, old, []byte("original"), 0o600)
	migrationPut(t, current, []byte("current newer work"), 0o600)
	if err := Ensure(); err == nil || !strings.Contains(err.Error(), "destination already exists") {
		t.Fatalf("expected explicit collision: %v", err)
	}
	migrationBytes(t, old, []byte("original"))
	migrationBytes(t, current, []byte("current newer work"))
	migrationAbsent(t, paths.MigrationBackups())
}

func TestEnsureBackupFailureNeverRelocatesOrMarksSuccess(t *testing.T) {
	migrationFixture(t)
	migrationPut(t, paths.LegacyRashinConfig(), []byte("{}"), 0o600)
	// A real filesystem failure, not an injected successful fake snapshot.
	migrationPut(t, paths.MigrationBackups(), []byte("not a directory"), 0o600)
	if err := Ensure(); err == nil {
		t.Fatal("backup failure ignored")
	}
	migrationBytes(t, paths.LegacyRashinConfig(), []byte("{}"))
	migrationAbsent(t, paths.ConfigFile())
	migrationAbsent(t, paths.StateDir())
}

func TestEnsureRejectsNestedGitBeforeBackup(t *testing.T) {
	for _, marker := range []string{"repository/.git/config", "worktree/.git", "bare/HEAD"} {
		t.Run(marker, func(t *testing.T) {
			migrationFixture(t)
			root := filepath.Join(paths.LegacyRashinData(), "skills")
			migrationPut(t, filepath.Join(root, marker), []byte("original git data"), 0o600)
			if marker == "bare/HEAD" {
				for _, dir := range []string{"objects", "refs"} {
					if err := os.MkdirAll(filepath.Join(root, "bare", dir), 0o700); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := Ensure(); err == nil || !strings.Contains(err.Error(), "Git repository") {
				t.Fatalf("expected repository ownership refusal: %v", err)
			}
			migrationBytes(t, filepath.Join(root, marker), []byte("original git data"))
			migrationAbsent(t, paths.MigrationBackups())
			migrationAbsent(t, paths.DataDir())
		})
	}
}

func TestEnsureInterruptedRetryRetainsAttemptAndCompletes(t *testing.T) {
	migrationFixture(t)
	original := filepath.Join(paths.LegacyRashinData(), "models", "original.gguf")
	migrationPut(t, original, []byte("original model bytes"), 0o600)
	plan, err := relocationPlan()
	if err != nil {
		t.Fatal(err)
	}
	backup, m, err := createBackup(plan)
	if err != nil {
		t.Fatal(err)
	}
	m.Entries[0].Phase = "installing"
	if err := writeManifest(backup, m); err != nil {
		t.Fatal(err)
	}
	if err := writePointer("active.json", backup); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(m.Entries[0].Destination), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := copyObject(snapshotPath(backup, 0), m.Entries[0].Destination); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(m.Entries[0].Source); err != nil {
		t.Fatal(err)
	}
	migrationPut(t, filepath.Join(m.Entries[0].Destination, "newer.gguf"), []byte("new work after interruption"), 0o600)
	if err := Ensure(); err != nil {
		t.Fatal(err)
	}
	migrationBytes(t, filepath.Join(paths.ModelsDir(), "original.gguf"), []byte("original model bytes"))
	found := false
	if err := filepath.WalkDir(filepath.Join(backup, "recovered"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == "newer.gguf" {
			migrationBytes(t, path, []byte("new work after interruption"))
			found = true
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("interrupted destination's newer work was discarded")
	}
	migrationAbsent(t, filepath.Join(paths.MigrationBackups(), "active.json"))
}

func TestPendingRecoveryRefusesPostgreSQLInEitherTree(t *testing.T) {
	for _, side := range []string{"source", "destination"} {
		t.Run(side, func(t *testing.T) {
			migrationFixture(t)
			relative := filepath.Join("hindsight", "home", ".pg0", "instances", "rashin", "data")
			migrationPut(t, filepath.Join(paths.LegacyRashinState(), relative, "page"), []byte("snapshot database"), 0o600)
			plan, err := relocationPlan()
			if err != nil {
				t.Fatal(err)
			}
			backup, manifest, err := createBackup(plan)
			if err != nil {
				t.Fatal(err)
			}
			manifest.Entries[0].Phase = "installing"
			if err := writeManifest(backup, manifest); err != nil {
				t.Fatal(err)
			}
			if err := writePointer("active.json", backup); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(paths.StateDir()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := copyObject(snapshotPath(backup, 0), paths.StateDir()); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(paths.LegacyRashinState()); err != nil {
				t.Fatal(err)
			}
			live := paths.LegacyRashinState()
			if side == "destination" {
				live = paths.StateDir()
			}
			page := filepath.Join(live, relative, "page")
			marker := filepath.Join(live, relative, "postmaster.pid")
			migrationPut(t, page, []byte("new database work"), 0o600)
			migrationPut(t, marker, []byte("12345\n"), 0o600)
			if err := Ensure(); err == nil {
				t.Fatal("recovery accepted a possibly live database")
			}
			migrationBytes(t, page, []byte("new database work"))
			migrationBytes(t, marker, []byte("12345\n"))
			migrationAbsent(t, filepath.Join(backup, "recovered"))
			if pending, err := pendingBackup(); err != nil || pending != backup {
				t.Fatalf("ownership refusal discarded the recoverable transaction: %q, %v", pending, err)
			}
		})
	}
}

func TestEnsureMetadataFailureRestoresOriginalBytes(t *testing.T) {
	migrationFixture(t)
	original := []byte("{invalid-json-private-token-do-not-log}")
	migrationPut(t, paths.LegacyRashinConfig(), original, 0o600)
	migrationPut(t, filepath.Join(paths.LegacyRashinData(), "models", "model.gguf"), []byte("unchanged model"), 0o600)
	err := Ensure()
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("expected metadata rollback: %v", err)
	}
	if strings.Contains(err.Error(), "private-token-do-not-log") {
		t.Fatal("error leaked original config contents")
	}
	migrationBytes(t, paths.LegacyRashinConfig(), original)
	migrationBytes(t, filepath.Join(paths.LegacyRashinData(), "models", "model.gguf"), []byte("unchanged model"))
	migrationAbsent(t, paths.ConfigFile())
	migrationAbsent(t, paths.ModelsDir())
	migrationAbsent(t, filepath.Join(paths.MigrationBackups(), "complete.json"))
}

func TestEnsureSerializesConcurrentStartup(t *testing.T) {
	migrationFixture(t)
	migrationPut(t, filepath.Join(paths.LegacyRashinData(), "models", "model.gguf"), bytes.Repeat([]byte("GGUF"), 8192), 0o600)
	for _, name := range []string{"app.lock", "rashind.lock", "config.lock"} {
		migrationPut(t, filepath.Join(paths.LegacyRashinRuntime(), name), nil, 0o600)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	start := make(chan struct{})
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; errs <- Ensure() }()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	_ = migrationBackup(t)
	entries, err := os.ReadDir(paths.MigrationBackups())
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "rashin-to-hyakki-") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("concurrent starts created %d transactions", count)
	}
}

func TestEnsureRefusesXDGStateAliasingSharedVault(t *testing.T) {
	migrationFixture(t)
	t.Setenv("XDG_STATE_HOME", paths.DataHome())
	vault := filepath.Join(paths.LegacyRashinData(), "memory", "private.md")
	migrationPut(t, vault, []byte("shared vault must stay"), 0o600)
	if err := Ensure(); err == nil || !strings.Contains(err.Error(), "shared Ryoku vault") {
		t.Fatalf("unsafe ownership accepted: %v", err)
	}
	migrationBytes(t, vault, []byte("shared vault must stay"))
	migrationAbsent(t, paths.StateDir())
	migrationAbsent(t, paths.MigrationBackups())
}

func TestEnsureRefusesPossiblyLivePostgreSQLBeforeBackup(t *testing.T) {
	migrationFixture(t)
	marker := filepath.Join(paths.LegacyRashinState(), "hindsight", "home", ".pg0", "instances", "local", "data", "postmaster.pid")
	migrationPut(t, marker, []byte("12345\n"), 0o600)
	if err := Ensure(); err == nil || !strings.Contains(err.Error(), "PostgreSQL ownership marker") {
		t.Fatalf("possibly live database accepted: %v", err)
	}
	migrationBytes(t, marker, []byte("12345\n"))
	migrationAbsent(t, paths.StateDir())
	migrationAbsent(t, paths.MigrationBackups())
}
