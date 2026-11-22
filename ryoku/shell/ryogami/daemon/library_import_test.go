package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func readBody(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestImportCopiesAPictureIntoTheWallpaperDir(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	src := filepath.Join(t.TempDir(), "Downloads", "photo.jpg")
	writeMedia(t, src, "jpeg bytes")

	got, err := d.importToLibrary(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wall, "photo.jpg"); got != want {
		t.Fatalf("imported to %q, want %q", got, want)
	}
	if readBody(t, got) != "jpeg bytes" {
		t.Fatal("imported copy does not match the source")
	}
	if readBody(t, src) != "jpeg bytes" {
		t.Fatal("the source picture was changed")
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("imported copy has mode %v, want 0644 like the rest of the library", info.Mode().Perm())
	}
}

func TestImportSendsAVideoToTheVideoDir(t *testing.T) {
	d, _, vid := daemonWithDirs(t)
	src := filepath.Join(t.TempDir(), "clip.mp4")
	writeMedia(t, src, "mp4 bytes")

	got, err := d.importToLibrary(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(vid, "clip.mp4"); got != want {
		t.Fatalf("imported to %q, want %q", got, want)
	}
}

func TestImportLeavesALibraryFileInPlace(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	src := filepath.Join(wall, "nested", "already.png")
	writeMedia(t, src, "png bytes")

	got, err := d.importToLibrary(src)
	if err != nil {
		t.Fatal(err)
	}
	if got != src {
		t.Fatalf("a picture already in the library moved to %q", got)
	}
	entries, _ := os.ReadDir(wall)
	if len(entries) != 1 {
		t.Fatalf("library holds %d entries, want only the nested dir", len(entries))
	}
}

func TestImportReusesAnIdenticalCopy(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	writeMedia(t, filepath.Join(wall, "photo.jpg"), "same bytes")
	src := filepath.Join(t.TempDir(), "photo.jpg")
	writeMedia(t, src, "same bytes")

	got, err := d.importToLibrary(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wall, "photo.jpg"); got != want {
		t.Fatalf("imported to %q, want the identical copy %q", got, want)
	}
	entries, _ := os.ReadDir(wall)
	if len(entries) != 1 {
		t.Fatalf("library holds %d files, want 1", len(entries))
	}
}

func TestImportNeverOverwritesADifferentPicture(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	writeMedia(t, filepath.Join(wall, "photo.jpg"), "theirs")
	writeMedia(t, filepath.Join(wall, "photo-1.jpg"), "also theirs")
	src := filepath.Join(t.TempDir(), "photo.jpg")
	writeMedia(t, src, "mine")

	got, err := d.importToLibrary(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wall, "photo-2.jpg"); got != want {
		t.Fatalf("imported to %q, want %q", got, want)
	}
	if readBody(t, filepath.Join(wall, "photo.jpg")) != "theirs" ||
		readBody(t, filepath.Join(wall, "photo-1.jpg")) != "also theirs" {
		t.Fatal("an existing library picture was overwritten")
	}
	if readBody(t, got) != "mine" {
		t.Fatal("imported copy does not match the source")
	}
}

func TestImportRefusesWhatTheLibraryCannotShow(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	notes := filepath.Join(t.TempDir(), "notes.txt")
	writeMedia(t, notes, "text")

	for _, src := range []string{notes, filepath.Join(t.TempDir(), "missing.jpg"), t.TempDir()} {
		if _, err := d.importToLibrary(src); err == nil {
			t.Errorf("import of %q succeeded", src)
		}
	}
	if entries, _ := os.ReadDir(wall); len(entries) != 0 {
		t.Fatalf("a refused import left %d files in the library", len(entries))
	}
}

// swapLink replaces the publish step for one test, so a race with another
// import can be staged at the exact moment the name is claimed.
func swapLink(t *testing.T, fn func(tmp, dest string) error) {
	t.Helper()
	orig := linkFile
	linkFile = fn
	t.Cleanup(func() { linkFile = orig })
}

func TestImportReusesACopyThatLandsMidImport(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	src := filepath.Join(t.TempDir(), "photo.jpg")
	writeMedia(t, src, "same bytes")
	swapLink(t, func(tmp, dest string) error {
		writeMedia(t, dest, "same bytes")
		return os.ErrExist
	})

	got, err := d.importToLibrary(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wall, "photo.jpg"); got != want {
		t.Fatalf("imported to %q, want the copy that landed %q", got, want)
	}
}

func TestImportTakesTheNextNameWhenOneIsClaimedMidImport(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	src := filepath.Join(t.TempDir(), "photo.jpg")
	writeMedia(t, src, "mine")
	claimed := false
	swapLink(t, func(tmp, dest string) error {
		if !claimed {
			claimed = true
			writeMedia(t, dest, "theirs")
			return os.ErrExist
		}
		return os.Link(tmp, dest)
	})

	got, err := d.importToLibrary(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wall, "photo-1.jpg"); got != want {
		t.Fatalf("imported to %q, want %q", got, want)
	}
	if readBody(t, filepath.Join(wall, "photo.jpg")) != "theirs" || readBody(t, got) != "mine" {
		t.Fatal("the racing import's picture was replaced")
	}
}

func TestImportWithoutHardLinksNeverOverwrites(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	src := filepath.Join(t.TempDir(), "photo.jpg")
	writeMedia(t, src, "mine")
	raced := false
	swapLink(t, func(tmp, dest string) error {
		if !raced {
			raced = true
			writeMedia(t, dest, "theirs")
		}
		return &os.LinkError{Op: "link", Old: tmp, New: dest, Err: syscall.EPERM}
	})

	got, err := d.importToLibrary(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wall, "photo-1.jpg"); got != want {
		t.Fatalf("imported to %q, want %q", got, want)
	}
	if readBody(t, filepath.Join(wall, "photo.jpg")) != "theirs" {
		t.Fatal("the fallback overwrote a picture that appeared mid-import")
	}
	if readBody(t, got) != "mine" {
		t.Fatal("the fallback copy does not match the source")
	}
	if info, err := os.Stat(got); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("fallback copy has the wrong mode: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(wall, ".import-*")); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestSameContentComparesPastTheFirstChunk(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("x", 3*compareChunk+17)
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	writeMedia(t, a, big)
	writeMedia(t, b, big)
	writeMedia(t, c, big[:len(big)-1]+"y")

	if same, err := sameContent(a, b); err != nil || !same {
		t.Fatalf("identical files compared as different (err %v)", err)
	}
	if same, err := sameContent(a, c); err != nil || same {
		t.Fatalf("files differing in the last byte compared as the same (err %v)", err)
	}
}
