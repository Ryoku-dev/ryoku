package teams

import (
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestDeletingTeamRemovesScratchHomesAndCaches(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Work", nil)
	task := h.newTask(t, team, "Try a compiler.")
	home := filepath.Join(paths.ToolHome(team.ID, task.ID), "go", "pkg", "mod")
	cache := paths.ToolCache(team.ID)
	for _, path := range []string{home, cache} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "file"), []byte("data"), 0o444); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o555); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.call(protocol.MTeamsDelete, protocol.TeamRef{ID: team.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.ToolHomes(), team.ID)); !os.IsNotExist(err) {
		t.Fatalf("team scratch directories survived deletion: %v", err)
	}
}
