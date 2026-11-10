package harness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

func TestOutsideReadRequiresPermissionEvenUnderAuto(t *testing.T) {
	r, sink := helperRunner(t, "file-read-outside", protocol.ApprovalsAuto, nil)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RASHIN_ACP_TEST_FILE", outside)
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "read"}, sink)
		done <- err
	}()
	permission := sink.waitKind(t, protocol.EvPermission, 1)[0].data.(protocol.PermissionData)
	if permission.Kind != protocol.ToolRead {
		t.Fatalf("outside read requested wrong permission: %+v", permission)
	}
	select {
	case err := <-done:
		t.Fatalf("outside read completed before permission: %v", err)
	default:
	}
	if err := r.Permission(permission.Request, "allow"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	texts := sink.waitKind(t, protocol.EvText, 1)
	if texts[0].data.(protocol.TextData).Delta != "secret" {
		t.Fatalf("approved read returned the wrong content: %+v", texts)
	}
}

func TestAskWriteWaitsForPermission(t *testing.T) {
	r, sink := helperRunner(t, "file-write-ask", protocol.ApprovalsAsk, nil)
	path := filepath.Join(r.opts.Cwd, "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RASHIN_ACP_TEST_FILE", path)
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "write"}, sink)
		done <- err
	}()
	permission := sink.waitKind(t, protocol.EvPermission, 1)[0].data.(protocol.PermissionData)
	if permission.Kind != protocol.ToolEdit {
		t.Fatalf("write requested wrong permission: %+v", permission)
	}
	before, err := os.ReadFile(path)
	if err != nil || string(before) != "old" {
		t.Fatalf("write happened before approval: %q, %v", before, err)
	}
	if err := r.Permission(permission.Request, "allow"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != "changed" {
		t.Fatalf("approved write failed: %q, %v", after, err)
	}
}

func TestReadOnlyWriteIsRefused(t *testing.T) {
	r, sink := helperRunner(t, "file-write-readonly", protocol.ApprovalsReadOnly, nil)
	path := filepath.Join(r.opts.Cwd, "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RASHIN_ACP_TEST_FILE", path)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "write"}, sink); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "old" {
		t.Fatalf("read-only write changed the file: %q, %v", content, err)
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for _, event := range sink.events {
		if event.kind == protocol.EvPermission {
			t.Fatal("read-only write incorrectly requested approval")
		}
	}
}

func TestRelativeFilePathIsRefused(t *testing.T) {
	r, sink := helperRunner(t, "file-relative", protocol.ApprovalsAuto, nil)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "read"}, sink); err != nil {
		t.Fatal(err)
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for _, event := range sink.events {
		if event.kind == protocol.EvPermission {
			t.Fatal("relative path reached the permission path")
		}
	}
}

func TestWriteThroughFileSymlinkPreservesSymlink(t *testing.T) {
	r, sink := helperRunner(t, "file-write-auto", protocol.ApprovalsAuto, nil)
	target := filepath.Join(r.opts.Cwd, "target.txt")
	link := filepath.Join(r.opts.Cwd, "link.txt")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RASHIN_ACP_TEST_FILE", link)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "write"}, sink); err != nil {
		t.Fatal(err)
	}
	st, err := os.Lstat(link)
	if err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("write replaced the symlink: mode=%v, err=%v", st.Mode(), err)
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "changed" {
		t.Fatalf("write did not update the symlink target: %q, %v", content, err)
	}
}

func TestWriteThroughOutsideDirectorySymlinkIsRefused(t *testing.T) {
	r, sink := helperRunner(t, "file-write-outside", protocol.ApprovalsAuto, nil)
	outside := t.TempDir()
	link := filepath.Join(r.opts.Cwd, "outside")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(link, "escape.txt")
	t.Setenv("RASHIN_ACP_TEST_FILE", target)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "write"}, sink); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside write was not refused: %v", err)
	}
}
