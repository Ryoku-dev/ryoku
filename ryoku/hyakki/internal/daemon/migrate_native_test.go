package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/paths"
)

func TestStartupPreservesImportedHarnessLoginsAndAuthoredSkills(t *testing.T) {
	isolateDaemon(t)
	files := map[string]string{
		filepath.Join(paths.DataDir(), "harness-homes", "codex", "team-one", "auth.json"):    "sealed external login fixture",
		filepath.Join(paths.StateDir(), "harness-homes", "claude", "team-two", "guard.json"): "guard state fixture",
		filepath.Join(paths.StateDir(), "harness-capabilities.json"):                         "capability state fixture",
		filepath.Join(paths.CacheDir(), "probe", "result.json"):                              "probe state fixture",
		filepath.Join(paths.DataDir(), "skills", "authored-entry", "SKILL.md"):               "# A skill the user kept\n",
		filepath.Join(paths.ManagedSkillsDir(), "local", "SKILL.md"):                         "# A managed local skill\n",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	daemon := New(Deps{})
	if err := daemon.runMigrations(context.Background()); err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Fatalf("startup discarded imported user data at %s: %q, %v", path, got, err)
		}
	}
}
