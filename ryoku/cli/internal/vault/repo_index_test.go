package vault

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPackagedRepoIndexOmitsCheckoutIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ryoku", "cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ryoku", "cli", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	view := filepath.Join(root, "view")
	if err := os.MkdirAll(view, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "ryoku"), filepath.Join(view, "ryoku")); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeGit := `#!/bin/sh
case "$*" in
  *--is-inside-work-tree*) echo true ;;
  *"rev-parse --short HEAD"*) echo abc1234 ;;
  *"rev-parse --abbrev-ref HEAD"*) echo hyperion ;;
  *"log -1 --format=%cs"*) echo 2026-11-10 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(fakeGit), 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate repository index test")
	}
	helper := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "vault", "repo-index"))
	run := func(name string, packaged bool, source string) string {
		t.Helper()
		output := filepath.Join(root, name+".md")
		args := []string{source, output}
		if packaged {
			args = append([]string{"--packaged"}, args...)
		}
		command := exec.Command(helper, args...)
		command.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
		if result, err := command.CombinedOutput(); err != nil {
			t.Fatalf("repo-index failed: %v: %s", err, result)
		}
		data, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	dev := run("dev", false, root)
	packaged := run("packaged", true, root)
	linkedPackaged := run("linked-packaged", true, view)
	if !strings.Contains(dev, "## Checkout") {
		t.Fatalf("development map has no checkout identity: %q", dev)
	}
	if strings.Contains(packaged, "## Checkout") {
		t.Fatalf("packaged map contains checkout identity: %q", packaged)
	}
	if !strings.Contains(packaged, "| `ryoku/` | - |") || !strings.Contains(packaged, "| `installation/` | - |") {
		t.Fatalf("packaged map does not carry the stable source layout: %q", packaged)
	}
	if linkedPackaged != packaged {
		t.Fatalf("packaged map changed when source directories were symlinked\nregular: %q\nsymlinked: %q", packaged, linkedPackaged)
	}
}
