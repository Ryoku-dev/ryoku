package daemon

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestNativeEngineMigrationRemovesLegacyStateOnceAndRecordsTeams(t *testing.T) {
	isolateDaemon(t)
	legacy := paths.LegacyHarnessState()
	for _, dir := range []string{
		filepath.Join(legacy[0], "codex", "team-one"),
		filepath.Join(legacy[0], "claude", "team-one"),
		filepath.Join(legacy[0], "claude", "team-two"),
		legacy[1], legacy[3],
		filepath.Join(paths.LegacySkillsDir(), "old-hash"),
		filepath.Join(paths.LegacySkillsDir(), "builtin"),
		filepath.Join(paths.LegacySkillsDir(), "managed"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(legacy[2], []byte("old capabilities"), 0o600); err != nil {
		t.Fatal(err)
	}
	mod := filepath.Join(legacy[0], "codex", "team-one", "go", "pkg", "mod", "x@v1")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	modFile := filepath.Join(mod, "a.go")
	if err := os.WriteFile(modFile, []byte("package x"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(mod, 0o555); err != nil {
		t.Fatal(err)
	}
	teams, err := MigrateNativeEngine()
	if err != nil || !slices.Equal(teams, []string{"team-one", "team-two"}) {
		t.Fatalf("removed login teams = %v, error = %v", teams, err)
	}
	for _, path := range append(legacy, filepath.Join(paths.LegacySkillsDir(), "old-hash")) {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("legacy state still at %s: %v", path, err)
		}
	}
	for _, name := range []string{"builtin", "managed"} {
		if _, err := os.Stat(filepath.Join(paths.LegacySkillsDir(), name)); err != nil {
			t.Fatalf("%s was removed: %v", name, err)
		}
	}
	ledger := filepath.Join(paths.TeamsDir(), "ledger.jsonl")
	file, err := os.Open(ledger)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	seen := make(map[string]int)
	scan := bufio.NewScanner(file)
	for scan.Scan() {
		var row protocol.Activity
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		if row.Action == "team.logins" {
			seen[row.Team]++
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if seen["team-one"] != 1 || seen["team-two"] != 1 || len(seen) != 2 {
		t.Fatalf("removed login ledger lines = %v", seen)
	}
	before, err := os.Stat(ledger)
	if err != nil {
		t.Fatal(err)
	}
	teams, err = MigrateNativeEngine()
	if err != nil || len(teams) != 0 {
		t.Fatalf("second migration = %v, %v", teams, err)
	}
	after, err := os.Stat(ledger)
	if err != nil || after.Size() != before.Size() {
		t.Fatalf("second migration changed the ledger: %v", err)
	}
}
