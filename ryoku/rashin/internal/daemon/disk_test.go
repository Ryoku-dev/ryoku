package daemon

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func diskTestDaemon(t *testing.T) (*Daemon, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	root := filepath.Join(paths.SessionsDir(), "finished")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	d := New(Deps{})
	d.settings.Features = map[string]bool{"diskcare": true}
	return d, root
}

func diskTestPlan(t *testing.T, d *Daemon, root string) {
	t.Helper()
	_, files, partial := diskWalk(context.Background(), root, "logs", func(path string, _ fs.FileInfo) bool { return filepath.Base(path) == "events.jsonl" })
	if partial || len(files) != 1 {
		t.Fatalf("unexpected scan: %d files, partial %v", len(files), partial)
	}
	d.diskPlans = map[string][]protocol.DiskFile{"confirmed": files}
	d.diskCached = protocol.DiskUsage{Items: []protocol.DiskItem{{ID: "logs", Plan: "confirmed", Files: files, Cleanable: true}}}
}

func TestDiskCleanRemovesOnlyConfirmedFilesOnce(t *testing.T) {
	d, root := diskTestDaemon(t)
	listed, unlisted := filepath.Join(root, "events.jsonl"), filepath.Join(root, "keep.txt")
	if err := os.WriteFile(listed, []byte("old log"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unlisted, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	diskTestPlan(t, d, root)
	if _, err := d.diskClean(context.Background(), nil, protocol.DiskCleanParams{Plan: "confirmed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(listed); !os.IsNotExist(err) {
		t.Fatalf("listed log remains: %v", err)
	}
	value, err := os.ReadFile(unlisted)
	if err != nil || string(value) != "keep me" {
		t.Fatalf("unlisted file changed: %v", err)
	}
	if _, err := d.diskClean(context.Background(), nil, protocol.DiskCleanParams{Plan: "confirmed"}); err == nil {
		t.Fatal("cleanup plan was reusable")
	}
}

func TestDiskCleanRejectsChangedFileBeforeDeletingAnything(t *testing.T) {
	d, root := diskTestDaemon(t)
	listed := filepath.Join(root, "events.jsonl")
	if err := os.WriteFile(listed, []byte("old log"), 0o600); err != nil {
		t.Fatal(err)
	}
	diskTestPlan(t, d, root)
	if err := os.WriteFile(listed, []byte("new log with more content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.diskClean(context.Background(), nil, protocol.DiskCleanParams{Plan: "confirmed"}); err == nil {
		t.Fatal("changed file was accepted")
	}
	value, err := os.ReadFile(listed)
	if err != nil || string(value) != "new log with more content" {
		t.Fatalf("changed log was removed: %v", err)
	}
}

func TestDiskCleanDoesNotFollowReplacedFileOrParentSymlinks(t *testing.T) {
	for _, parent := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "parent"}[parent], func(t *testing.T) {
			d, root := diskTestDaemon(t)
			listed := filepath.Join(root, "events.jsonl")
			if err := os.WriteFile(listed, []byte("log"), 0o600); err != nil {
				t.Fatal(err)
			}
			diskTestPlan(t, d, root)
			outside := t.TempDir()
			victim := filepath.Join(outside, "events.jsonl")
			if err := os.WriteFile(victim, []byte("private data"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(listed); err != nil {
				t.Fatal(err)
			}
			if parent {
				if err := os.Remove(root); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, root); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Symlink(victim, listed); err != nil {
				t.Fatal(err)
			}
			if _, err := d.diskClean(context.Background(), nil, protocol.DiskCleanParams{Plan: "confirmed"}); err == nil {
				t.Fatal("symlink replacement was accepted")
			}
			value, err := os.ReadFile(victim)
			if err != nil || string(value) != "private data" {
				t.Fatalf("outside data changed: %v", err)
			}
		})
	}
}

func TestDiskCareOffCannotClean(t *testing.T) {
	d, root := diskTestDaemon(t)
	listed := filepath.Join(root, "events.jsonl")
	if err := os.WriteFile(listed, []byte("log"), 0o600); err != nil {
		t.Fatal(err)
	}
	diskTestPlan(t, d, root)
	d.settings.Features["diskcare"] = false
	if _, err := d.diskClean(context.Background(), nil, protocol.DiskCleanParams{Plan: "confirmed"}); err == nil {
		t.Fatal("disabled feature cleaned files")
	}
	if _, err := os.Stat(listed); err != nil {
		t.Fatal(err)
	}
}
