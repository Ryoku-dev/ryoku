package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveWritableTreeDoesNotFollowLinks(t *testing.T) {
	parent := t.TempDir()
	outside := filepath.Join(parent, "keep")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "scratch")
	locked := filepath.Join(root, "go", "pkg", "mod")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "module.go"), []byte("package mod"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(locked, "external")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := RemoveWritableTree(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("readonly scratch tree remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "secret")); err != nil {
		t.Fatalf("cleanup followed external link: %v", err)
	}
}
