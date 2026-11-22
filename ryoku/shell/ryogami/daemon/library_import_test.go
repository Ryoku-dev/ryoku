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

func TestImportCopiesWhatTheScannerWouldNotList(t *testing.T) {
	d, wall, vid := daemonWithDirs(t)
	inVideoDir := filepath.Join(vid, "still.jpg")
	writeMedia(t, inVideoDir, "still in the video dir")
	hidden := filepath.Join(wall, ".stash", "kept.png")
	writeMedia(t, hidden, "hidden png")

	for src, want := range map[string]string{
		inVideoDir: filepath.Join(wall, "still.jpg"),
		hidden:     filepath.Join(wall, "kept.png"),
	} {
		got, err := d.importToLibrary(src)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("import of %q went to %q, want %q where the picker lists it", src, got, want)
		}
	}
}

func TestImportReplacesASymlinkWithARealCopy(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	target := filepath.Join(t.TempDir(), "photo.jpg")
	writeMedia(t, target, "linked bytes")
	link := filepath.Join(wall, "photo.jpg")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	got, err := d.importToLibrary(link)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wall, "photo-1.jpg"); got != want {
		t.Fatalf("imported to %q, want a real file at %q", got, want)
	}
	if info, err := os.Lstat(got); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("import is not a regular file the scanner lists: %v", err)
	}
}

func TestDolphinMenuOffersOnlyTypesTheLibraryShows(t *testing.T) {
	body := readBody(t, filepath.Join("..", "..", "..", "apps", "dolphin", "ryoku-set-wallpaper.desktop"))
	byMime := map[string][]string{
		"image/jpeg": {"jpg", "jpeg"}, "image/png": {"png"}, "image/webp": {"webp"},
		"image/gif": {"gif"}, "image/bmp": {"bmp"}, "image/avif": {"avif"}, "image/tiff": {"tiff", "tif"},
	}
	var mimes []string
	for _, line := range strings.Split(body, "\n") {
		if v, ok := strings.CutPrefix(line, "MimeType="); ok {
			mimes = strings.FieldsFunc(v, func(r rune) bool { return r == ';' })
		}
	}
	if len(mimes) == 0 {
		t.Fatal("the menu declares no MimeType")
	}
	for _, m := range mimes {
		exts, known := byMime[m]
		if !known {
			t.Errorf("the menu offers %s, which import has no extension mapping for", m)
			continue
		}
		for _, e := range exts {
			if !imageExts[e] {
				t.Errorf("the menu offers %s but import refuses .%s", m, e)
			}
		}
	}
}

func TestImportGivesAHiddenFileAListedName(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	src := filepath.Join(t.TempDir(), ".photo.jpg")
	writeMedia(t, src, "hidden name")

	got, err := d.importToLibrary(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wall, "photo.jpg"); got != want {
		t.Fatalf("imported to %q, want %q where the picker lists it", got, want)
	}
}

func TestImportCopiesOutOfASymlinkedFolder(t *testing.T) {
	d, wall, _ := daemonWithDirs(t)
	elsewhere := t.TempDir()
	writeMedia(t, filepath.Join(elsewhere, "linked.png"), "png via a linked folder")
	if err := os.Symlink(elsewhere, filepath.Join(wall, "linked")); err != nil {
		t.Fatal(err)
	}

	got, err := d.importToLibrary(filepath.Join(wall, "linked", "linked.png"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(wall, "linked.png"); got != want {
		t.Fatalf("imported to %q, want %q where the picker lists it", got, want)
	}
}
