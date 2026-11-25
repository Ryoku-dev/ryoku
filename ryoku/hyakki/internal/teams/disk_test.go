package teams

import (
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestDiskTaskCleanupRetainsValidatedDirectoryAfterSymlinkSwap(t *testing.T) {
	base := t.TempDir()
	tasks := filepath.Join(base, "tasks")
	if err := os.Mkdir(tasks, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasks, "finished.json"), []byte("approved task record"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(tasks)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	retained := filepath.Join(base, "retained")
	if err := os.Rename(tasks, retained); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	victim := filepath.Join(outside, "finished.json")
	if err := os.WriteFile(victim, []byte("unlisted data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, tasks); err != nil {
		t.Fatal(err)
	}
	st := &state{dir: base, taskSet: map[string]protocol.TeamTask{"finished": {ID: "finished"}}}
	removed, err := st.removeTaskConfined("finished", root, func(protocol.TeamTask) bool { return true })
	if err != nil || !removed {
		t.Fatalf("confined removal failed: %v", err)
	}
	value, err := os.ReadFile(victim)
	if err != nil || string(value) != "unlisted data" {
		t.Fatalf("unlisted file was touched: %v", err)
	}
	if _, err := os.Stat(filepath.Join(retained, "finished.json")); !os.IsNotExist(err) {
		t.Fatal("approved file was not removed through the retained directory")
	}
	if _, ok := st.taskSet["finished"]; ok {
		t.Fatal("removed task stayed in memory")
	}
}

func TestDiskTaskCleanupKeepsStateWhenConfinedRemovalFails(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	st := &state{taskSet: map[string]protocol.TeamTask{"finished": {ID: "finished"}}}
	if removed, err := st.removeTaskConfined("finished", root, func(protocol.TeamTask) bool { return true }); err == nil || removed {
		t.Fatal("missing file counted as removed")
	}
	if _, ok := st.taskSet["finished"]; !ok {
		t.Fatal("task state was removed before its file")
	}
	if removed, err := st.removeTaskConfined("finished", root, func(protocol.TeamTask) bool { return false }); err == nil || removed {
		t.Fatal("ineligible task was removed")
	}
}
