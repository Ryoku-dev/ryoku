package main

import (
	"os"
	"path/filepath"
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
