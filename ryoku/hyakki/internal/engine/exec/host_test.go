package exec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHostFilesAndVersionedEdits(t *testing.T) {
	ctx := context.Background()
	e := Host(HostOptions{})
	defer e.Close()
	root := t.TempDir()
	p := filepath.Join(root, "nested", "note.txt")
	created, err := e.Write(ctx, WriteRequest{Path: p, Content: []byte("one\n"), MakeDirs: true})
	if err != nil || created.Existed || created.Info.Mode != 0o600 {
		t.Fatalf("create: %+v, %v", created, err)
	}
	read, err := e.Read(ctx, ReadRequest{Path: p})
	if err != nil || string(read.Data) != "one\n" || read.Binary {
		t.Fatalf("read: %+v, %v", read, err)
	}
	changed, err := e.Edit(ctx, EditRequest{Path: p, IfVersion: read.Info.Version, Replacements: []Replacement{{Old: "one", New: "two"}}})
	if err != nil || string(changed.Old) != "one\n" || string(changed.New) != "two\n" {
		t.Fatalf("edit: %+v, %v", changed, err)
	}
	if _, err := e.Write(ctx, WriteRequest{Path: p, Content: []byte("stale"), IfVersion: read.Info.Version}); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale write: %v", err)
	}
	if _, err := e.Write(ctx, WriteRequest{Path: p, Content: []byte("stale"), IfAbsent: true}); !errors.Is(err, ErrExists) {
		t.Fatalf("overwrite: %v", err)
	}
	if err := e.Remove(ctx, RemoveRequest{Path: p, IfVersion: changed.Info.Version}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Stat(ctx, p); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed path: %v", err)
	}
	if err := e.Remove(ctx, RemoveRequest{Path: root}); !errors.Is(err, ErrIsDir) {
		t.Fatalf("remove directory: %v", err)
	}
}

func TestHostGlobGrepAndSymlink(t *testing.T) {
	ctx := context.Background()
	e := Host(HostOptions{})
	defer e.Close()
	root := t.TempDir()
	for path, text := range map[string]string{
		"src/main.go": "alpha\nbeta\n", "src/sub/other.go": "beta\n", "src/ignore.log": "beta\n",
		".secret.go": "beta\n", ".git/config": "beta\n",
	} {
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := e.Glob(ctx, GlobRequest{Root: root, Pattern: "*.go"})
	if err != nil || paths.Total != 2 {
		t.Fatalf("glob: %+v %v", paths, err)
	}
	found, err := e.Grep(ctx, GrepRequest{Root: root, Pattern: "beta", Include: "*.go", Context: 1})
	if err != nil || found.Files != 2 || len(found.Matches) != 2 || len(found.Matches[0].Before) != 1 {
		t.Fatalf("grep: %+v %v", found, err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "src", "main.go"), link); err != nil {
		t.Fatal(err)
	}
	info, err := e.Stat(ctx, link)
	if err != nil || info.Kind != KindSymlink {
		t.Fatalf("link: %+v %v", info, err)
	}
	if _, err := e.Write(ctx, WriteRequest{Path: link, Content: []byte("edited\n")}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "src", "main.go")); err != nil || string(b) != "edited\n" {
		t.Fatalf("link target: %q %v", b, err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("link was replaced: %v", err)
	}
	throughLink, err := e.Read(ctx, ReadRequest{Path: link})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Remove(ctx, RemoveRequest{Path: link, IfVersion: throughLink.Info.Version}); err != nil {
		t.Fatalf("remove observed symlink: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "src", "main.go")); err != nil || string(b) != "edited\n" {
		t.Fatalf("removed link target: %q %v", b, err)
	}
}

func TestHostProcessStreamingAndTimeout(t *testing.T) {
	ctx := context.Background()
	e := Host(HostOptions{MaxProcesses: 1})
	defer e.Close()
	var received strings.Builder
	id, err := e.Start(ctx, StartRequest{Argv: []string{"sh", "-c", "printf 'first\\n'; sleep 0.1; printf 'second\\n'"}, Dir: t.TempDir()}, func(o Output) { received.WriteString(o.Delta) })
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Wait(ctx, id)
	if err != nil || res.Running || res.Code != 0 || res.Output != "first\nsecond\n" || received.String() != res.Output {
		t.Fatalf("wait: %+v %v output=%q", res, err, received.String())
	}
	if err := e.Release(ctx, id); err != nil {
		t.Fatal(err)
	}
	id, err = e.Start(ctx, StartRequest{Argv: []string{"sh", "-c", "sleep 10"}, Dir: t.TempDir(), Timeout: 30 * time.Millisecond}, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = e.Wait(ctx, id)
	if err != nil || !res.TimedOut || res.Running {
		t.Fatalf("timed job: %+v %v", res, err)
	}
}
