package chats

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivacyCountsOnlyChatsAndRemovesOnlyImportRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "chats")
	if err := os.MkdirAll(filepath.Join(root, "claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "original-export.json")
	for _, file := range []string{outside, filepath.Join(root, "claude", "one.md"), filepath.Join(root, "claude", "two.md"), filepath.Join(root, "claude", ".import-partial")} {
		if err := os.WriteFile(file, []byte("local fixture words"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(parent, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	if got, err := Count(context.Background(), root); err != nil || got != 2 {
		t.Fatalf("count %d: %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Count(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled count: %v", err)
	}
	if err := ForgetAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("original export was touched", err)
	}
	if got, err := Count(context.Background(), root); err != nil || got != 0 {
		t.Fatalf("after forgetting %d: %v", got, err)
	}
	if err := os.Symlink(parent, root); err != nil {
		t.Fatal(err)
	}
	if err := ForgetAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("a symlinked import root escaped its owner", err)
	}
}
