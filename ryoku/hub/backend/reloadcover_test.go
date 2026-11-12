package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func reloadCoverTestHome(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	cfg := filepath.Join(root, "config")
	data := filepath.Join(root, "data")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_DATA_HOME", data)
	return cfg, data
}

func writeReloadAsset(t *testing.T, dir, name string, body []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func replaceReloadCoverDirWithSymlink(t *testing.T, data string) (string, map[string][]byte) {
	t.Helper()
	managed := filepath.Join(data, "ryoku", "reload-cover")
	external := t.TempDir()
	files := map[string][]byte{
		strings.Repeat("a", 64) + ".png": []byte("external image"),
		strings.Repeat("b", 64) + ".gif": []byte("external animation"),
	}
	for name, body := range files {
		writeReloadAsset(t, external, name, body)
	}
	if err := os.MkdirAll(filepath.Dir(managed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, managed); err != nil {
		t.Fatal(err)
	}
	return external, files
}

func assertReloadCoverFilesUnchanged(t *testing.T, dir string, want map[string][]byte) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(want) {
		t.Fatalf("external entry count = %d, want %d", len(entries), len(want))
	}
	for name, body := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != string(body) {
			t.Fatalf("external %q = %q, %v; want %q", name, got, err, body)
		}
	}
}

func TestReloadCoverKind(t *testing.T) {
	cases := map[string]string{
		"mark.bmp":  "image",
		"mark.jpg":  "image",
		"mark.png":  "image",
		"mark.JPEG": "image",
		"mark.webp": "image",
		"mark.svg":  "image",
		"loop.gif":  "animated",
		"loop.MP4":  "video",
		"loop.webm": "video",
		"loop.mkv":  "video",
		"loop.mov":  "video",
	}
	for name, want := range cases {
		got, err := reloadCoverKind(name)
		if err != nil || got != want {
			t.Errorf("reloadCoverKind(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if _, err := reloadCoverKind("notes.txt"); err == nil {
		t.Fatal("unsupported extension was accepted")
	}
}

func TestResolveReloadCoverSourceDecodesLocalFileURL(t *testing.T) {
	root, _ := reloadCoverTestHome(t)
	source := writeReloadAsset(t, root, "ink loop.png", []byte("ink"))
	got, err := resolveReloadCoverSource("file://" + strings.ReplaceAll(source, " ", "%20"))
	if err != nil {
		t.Fatal(err)
	}
	if got != source {
		t.Fatalf("resolved path = %q, want %q", got, source)
	}
	if _, err := resolveReloadCoverSource("file://remotehost/tmp/ink.png"); err == nil {
		t.Fatal("non-local file URL was accepted")
	}
	if _, err := resolveReloadCoverSource("https://example.com/ink.png"); err == nil {
		t.Fatal("remote URL was accepted")
	}
}

func TestImportReloadCoverCopiesContentAddressedAsset(t *testing.T) {
	_, data := reloadCoverTestHome(t)
	body := []byte("small valid fixture bytes")
	source := writeReloadAsset(t, t.TempDir(), "Ink Loop.PNG", body)

	got, err := importReloadCover(source)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	wantPath := filepath.Join(data, "ryoku", "reload-cover", hex.EncodeToString(digest[:])+".png")
	if got.Path != wantPath || got.Name != "Ink Loop.PNG" || got.Kind != "image" || got.Bytes != int64(len(body)) {
		t.Fatalf("descriptor = %+v, want path=%q name=%q kind=image bytes=%d", got, wantPath, "Ink Loop.PNG", len(body))
	}
	copied, err := os.ReadFile(got.Path)
	if err != nil || string(copied) != string(body) {
		t.Fatalf("copied bytes = %q, %v", copied, err)
	}
	historical := time.Unix(1_234_567_890, 0)
	if err := os.Chtimes(got.Path, historical, historical); err != nil {
		t.Fatal(err)
	}
	again, err := importReloadCover(source)
	if err != nil || again != got {
		t.Fatalf("deduplicated import = %+v, %v; want %+v", again, err, got)
	}
	info, err := os.Stat(got.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(historical) {
		t.Fatalf("reused asset modification time = %v, want %v", info.ModTime(), historical)
	}
}

func TestImportReloadCoverRejectsSymlinkedManagedDirectory(t *testing.T) {
	_, data := reloadCoverTestHome(t)
	external, files := replaceReloadCoverDirWithSymlink(t, data)
	source := writeReloadAsset(t, t.TempDir(), "source.png", []byte("source"))

	if _, err := importReloadCover(source); err == nil {
		t.Fatal("import through a symlinked managed directory was accepted")
	}
	assertReloadCoverFilesUnchanged(t, external, files)
}

func TestImportReloadCoverRejectsInvalidDestinationCollision(t *testing.T) {
	body := []byte("collision fixture")
	digest := sha256.Sum256(body)
	cases := []struct {
		name   string
		create func(t *testing.T, destination string)
		verify func(t *testing.T, destination string)
	}{
		{
			name: "directory",
			create: func(t *testing.T, destination string) {
				t.Helper()
				if err := os.Mkdir(destination, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			verify: func(t *testing.T, destination string) {
				t.Helper()
				info, err := os.Lstat(destination)
				if err != nil || !info.IsDir() {
					t.Fatalf("directory collision changed: %v, %v", info, err)
				}
			},
		},
		{
			name: "symlink",
			create: func(t *testing.T, destination string) {
				t.Helper()
				target := writeReloadAsset(t, t.TempDir(), "target.png", []byte("target"))
				if err := os.Symlink(target, destination); err != nil {
					t.Fatal(err)
				}
			},
			verify: func(t *testing.T, destination string) {
				t.Helper()
				info, err := os.Lstat(destination)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("symlink collision changed: %v, %v", info, err)
				}
			},
		},
		{
			name: "wrong-content",
			create: func(t *testing.T, destination string) {
				t.Helper()
				if err := os.WriteFile(destination, []byte("wrong"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			verify: func(t *testing.T, destination string) {
				t.Helper()
				got, err := os.ReadFile(destination)
				if err != nil || string(got) != "wrong" {
					t.Fatalf("wrong-content collision changed: %q, %v", got, err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, data := reloadCoverTestHome(t)
			source := writeReloadAsset(t, t.TempDir(), "collision.png", body)
			dir := filepath.Join(data, "ryoku", "reload-cover")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(dir, hex.EncodeToString(digest[:])+".png")
			tc.create(t, destination)
			if _, err := importReloadCover(source); err == nil {
				t.Fatal("invalid destination collision was accepted")
			}
			tc.verify(t, destination)
		})
	}
}

func TestImportReloadCoverRejectsFIFODestinationCollision(t *testing.T) {
	_, data := reloadCoverTestHome(t)
	body := []byte("FIFO collision fixture")
	source := writeReloadAsset(t, t.TempDir(), "collision.png", body)
	digest := sha256.Sum256(body)
	dir := filepath.Join(data, "ryoku", "reload-cover")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, hex.EncodeToString(digest[:])+".png")
	if err := syscall.Mkfifo(destination, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(destination) })

	done := make(chan error, 1)
	go func() {
		_, err := importReloadCover(source)
		done <- err
	}()

	// the deadline only guards against a true hang (a blocking open on the
	// reader-less FIFO): import rejects it via a non-blocking open, but first
	// fsyncs its temp file, which on a slow CI disk can take well over 100ms, so
	// keep the window generous rather than timing that fsync.
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO collision was accepted")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("import blocked on FIFO collision")
	}
	info, err := os.Lstat(destination)
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("FIFO collision changed: %v, %v", info, err)
	}
}

func TestImportReloadCoverEnforcesLimit(t *testing.T) {
	reloadCoverTestHome(t)
	source := filepath.Join(t.TempDir(), "large.mp4")
	if err := os.WriteFile(source, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(source, reloadCoverMaxBytes+1); err != nil {
		t.Fatal(err)
	}
	_, err := importReloadCover(source)
	if err == nil || !strings.Contains(err.Error(), "64 MiB") || !strings.Contains(err.Error(), "20 MB") {
		t.Fatalf("over-limit error = %v", err)
	}
}

func TestImportReloadCoverAcceptsExactLimit(t *testing.T) {
	reloadCoverTestHome(t)
	source := filepath.Join(t.TempDir(), "limit.webm")
	if err := os.WriteFile(source, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(source, reloadCoverMaxBytes); err != nil {
		t.Fatal(err)
	}
	got, err := importReloadCover(source)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bytes != reloadCoverMaxBytes || got.Kind != "video" {
		t.Fatalf("descriptor = %+v, want %d-byte video", got, reloadCoverMaxBytes)
	}
}

func TestImportReloadCoverRejectsInvalidSources(t *testing.T) {
	reloadCoverTestHome(t)
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe.png")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(fifo) })
	cases := []struct {
		name string
		path string
	}{
		{name: "missing", path: filepath.Join(dir, "missing.png")},
		{name: "directory", path: dir},
		{name: "FIFO", path: fifo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := importReloadCover(tc.path); err == nil {
				t.Fatal("invalid source was accepted")
			}
		})
	}
}

func TestImportReloadCoverRemovesTemporaryFileWhenSourceGrows(t *testing.T) {
	_, data := reloadCoverTestHome(t)
	source := filepath.Join(t.TempDir(), "growing.mp4")
	if err := os.WriteFile(source, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(source, reloadCoverMaxBytes); err != nil {
		t.Fatal(err)
	}
	var appendErr error
	appended := false
	oldHook := reloadCoverAfterFirstCopyWriteHook
	reloadCoverAfterFirstCopyWriteHook = func() {
		file, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			appendErr = err
			return
		}
		_, appendErr = file.Write([]byte{0})
		if err := file.Close(); appendErr == nil {
			appendErr = err
		}
		appended = appendErr == nil
	}
	t.Cleanup(func() { reloadCoverAfterFirstCopyWriteHook = oldHook })
	_, err := importReloadCover(source)
	if appendErr != nil {
		t.Fatal(appendErr)
	}
	if !appended {
		t.Fatal("source was not appended after copying began")
	}
	if err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatalf("growing source error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(data, "ryoku", "reload-cover"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if reloadCoverManagedName.MatchString(entry.Name()) || strings.HasPrefix(entry.Name(), ".import-") {
			t.Fatalf("failed import left %q behind", entry.Name())
		}
	}
}

func TestPruneReloadCoverKeepsOnlyManagedSelection(t *testing.T) {
	_, data := reloadCoverTestHome(t)
	dir := filepath.Join(data, "ryoku", "reload-cover")
	keep := writeReloadAsset(t, dir, strings.Repeat("a", 64)+".png", []byte("keep"))
	drop := writeReloadAsset(t, dir, strings.Repeat("b", 64)+".gif", []byte("drop"))
	stray := writeReloadAsset(t, dir, "notes.txt", []byte("user file"))

	if err := pruneReloadCover(keep); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("kept asset missing: %v", err)
	}
	if _, err := os.Stat(drop); !os.IsNotExist(err) {
		t.Fatalf("stale managed asset remains: %v", err)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Fatalf("unowned file was removed: %v", err)
	}

	external := writeReloadAsset(t, t.TempDir(), "outside.png", []byte("outside"))
	if err := pruneReloadCover(external); err == nil {
		t.Fatal("external keep path was accepted")
	}
	if _, err := os.Stat(external); err != nil {
		t.Fatalf("external path was touched: %v", err)
	}
}

func TestPruneReloadCoverRejectsSymlinkedManagedDirectory(t *testing.T) {
	_, data := reloadCoverTestHome(t)
	external, files := replaceReloadCoverDirWithSymlink(t, data)

	if err := pruneReloadCover(""); err == nil {
		t.Fatal("prune through a symlinked managed directory was accepted")
	}
	assertReloadCoverFilesUnchanged(t, external, files)
}
func TestImportReloadCoverKeepsCommittedAssetWhenPruning(t *testing.T) {
	cfg, data := reloadCoverTestHome(t)
	dir := filepath.Join(data, "ryoku", "reload-cover")
	committed := writeReloadAsset(t, dir, strings.Repeat("c", 64)+".webm", []byte("committed"))
	stale := writeReloadAsset(t, dir, strings.Repeat("d", 64)+".png", []byte("stale"))
	brand := filepath.Join(cfg, "ryoku", "brand.json")
	if err := os.MkdirAll(filepath.Dir(brand), 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"reloadCover": map[string]any{"path": committed}})
	if err := os.WriteFile(brand, body, 0o644); err != nil {
		t.Fatal(err)
	}
	source := writeReloadAsset(t, t.TempDir(), "new.svg", []byte("<svg/>"))
	got, err := importReloadCover(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(committed); err != nil {
		t.Fatalf("committed asset was pruned: %v", err)
	}
	if _, err := os.Stat(got.Path); err != nil {
		t.Fatalf("new asset was pruned: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale asset remains: %v", err)
	}
}

func TestCommittedReloadCoverPath(t *testing.T) {
	cfg, _ := reloadCoverTestHome(t)
	managed := filepath.Join(reloadCoverDir(), strings.Repeat("c", 64)+".webm")
	path := filepath.Join(cfg, "ryoku", "brand.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"reloadCover": map[string]any{"path": managed}})
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := committedReloadCoverPath(); got != managed {
		t.Fatalf("committed path = %q, want %q", got, managed)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := committedReloadCoverPath(); got != "" {
		t.Fatalf("malformed brand path = %q, want empty", got)
	}
}

func TestRunReloadCoverImportWritesDescriptor(t *testing.T) {
	reloadCoverTestHome(t)
	source := writeReloadAsset(t, t.TempDir(), "cli.gif", []byte("gif bytes"))
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = writer
	err = runReloadCover([]string{"import", source})
	writer.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	var got reloadCoverAsset
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("CLI JSON = %q, %v", body, err)
	}
	if got.Name != "cli.gif" || got.Kind != "animated" || got.Bytes != int64(len("gif bytes")) || !managedReloadCoverPath(got.Path) {
		t.Fatalf("CLI descriptor = %+v", got)
	}
}
