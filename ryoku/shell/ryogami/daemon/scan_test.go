package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func writePNG(t *testing.T, path string, c color.Color) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func haveMagick() bool {
	_, err := exec.LookPath("magick")
	return err == nil
}

// TestScanNaming pins the key/name/'--' conventions and subdir handling. Thumb
// generation is irrelevant here (it may fail without magick); only the derived
// identity fields are asserted.
func TestScanNaming(t *testing.T) {
	root := t.TempDir()
	wall := filepath.Join(root, "wall")
	videos := filepath.Join(root, "videos")
	cache := filepath.Join(root, "cache")

	writePNG(t, filepath.Join(wall, "a.png"), color.RGBA{200, 20, 20, 255})
	writePNG(t, filepath.Join(wall, "sub", "b.PNG"), color.RGBA{20, 200, 20, 255})
	if err := os.WriteFile(filepath.Join(wall, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(videos, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(videos, "clip.mp4"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ScanDirs(wall, videos, cache, nil, func(Entry) {})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d: %v", len(got), keys(got))
	}

	a, ok := got["a.png"]
	if !ok {
		t.Fatalf("missing key a.png in %v", keys(got))
	}
	if a.Name != "a.png" || a.Type != "static" || a.VideoFile != "" {
		t.Errorf("a.png entry wrong: %+v", a)
	}

	b, ok := got["sub--b.PNG"]
	if !ok {
		t.Fatalf("missing key sub--b.PNG in %v", keys(got))
	}
	if b.Name != "sub/b.PNG" || b.Type != "static" {
		t.Errorf("subdir entry wrong: %+v", b)
	}

	v, ok := got["clip.mp4"]
	if !ok {
		t.Fatalf("missing key clip.mp4 in %v", keys(got))
	}
	if v.Type != "video" || v.VideoFile == "" {
		t.Errorf("video entry wrong: %+v", v)
	}
	if !filepath.IsAbs(v.VideoFile) {
		t.Errorf("video_file must be absolute: %q", v.VideoFile)
	}
}

// TestScanMergeFavouriteSurvives: an unchanged mtime returns the prior row
// verbatim, so user state (favourite) persists and no work is done.
func TestScanMergeFavouriteSurvives(t *testing.T) {
	root := t.TempDir()
	wall := filepath.Join(root, "wall")
	cache := filepath.Join(root, "cache")
	src := filepath.Join(wall, "a.png")
	writePNG(t, src, color.RGBA{200, 20, 20, 255})

	fi, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	mt := fi.ModTime().Unix()

	prior := map[string]Entry{
		"a.png": {Key: "a.png", Name: "a.png", Type: "static", Favourite: 1, ApplyCount: 7, Mtime: mt},
	}

	calls := 0
	got, err := ScanDirs(wall, wall, cache, prior, func(Entry) { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("onItem must not fire for unchanged entry, fired %d", calls)
	}
	e := got["a.png"]
	if e.Favourite != 1 || e.ApplyCount != 7 {
		t.Fatalf("prior state not preserved: %+v", e)
	}
}

// TestScanMergeFollowsMovedFile: an unchanged file found at a new location (a
// moved library or home keeps mtimes) is applied from where it is now, and an
// animated still keeps its cached clip.
func TestScanMergeFollowsMovedFile(t *testing.T) {
	root := t.TempDir()
	wall := filepath.Join(root, "moved")
	cache := filepath.Join(root, "cache")
	src := filepath.Join(wall, "a.png")
	writePNG(t, src, color.RGBA{20, 200, 20, 255})
	fi, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	prior := map[string]Entry{
		"a.png": {Key: "a.png", Name: "a.png", Type: "static", Favourite: 1,
			Path: "/old/home/a.png", VideoFile: "/cache/a.mp4", Mtime: fi.ModTime().Unix()},
	}
	got, err := ScanDirs(wall, wall, cache, prior, func(Entry) {})
	if err != nil {
		t.Fatal(err)
	}
	e := got["a.png"]
	if e.Path != src {
		t.Fatalf("Path = %q, want the file's current location %q", e.Path, src)
	}
	if e.VideoFile != "/cache/a.mp4" || e.Favourite != 1 {
		t.Fatalf("cached clip or favourite lost on a warm rescan: %+v", e)
	}
}

// TestScanMergeVanishedDropped: a prior key with no source file on disk is not
// carried into the fresh scan.
func TestScanMergeVanishedDropped(t *testing.T) {
	root := t.TempDir()
	wall := filepath.Join(root, "wall")
	cache := filepath.Join(root, "cache")
	writePNG(t, filepath.Join(wall, "a.png"), color.RGBA{200, 20, 20, 255})

	prior := map[string]Entry{
		"gone.png": {Key: "gone.png", Name: "gone.png", Type: "static", Favourite: 1, Mtime: 123},
	}
	got, err := ScanDirs(wall, wall, cache, prior, func(Entry) {})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["gone.png"]; ok {
		t.Fatalf("vanished entry should be dropped, got %v", keys(got))
	}
}

// TestScanMergeMtimeChangeReprocesses: a changed mtime keeps favourites and
// counts but rebuilds derived analysis fields (Width/Height) from the new file.
func TestScanMergeMtimeChangeReprocesses(t *testing.T) {
	root := t.TempDir()
	wall := filepath.Join(root, "wall")
	cache := filepath.Join(root, "cache")
	src := filepath.Join(wall, "a.png")
	writePNG(t, src, color.RGBA{200, 20, 20, 255})

	fi, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	newMt := fi.ModTime().Unix()

	prior := map[string]Entry{
		"a.png": {
			Key: "a.png", Name: "a.png", Type: "static",
			Favourite: 1, ApplyCount: 3,
			Width: 1920, Height: 1080, Mtime: newMt - 500,
		},
	}
	got, err := ScanDirs(wall, wall, cache, prior, func(Entry) {})
	if err != nil {
		t.Fatal(err)
	}
	e := got["a.png"]
	if e.Favourite != 1 || e.ApplyCount != 3 {
		t.Fatalf("favourite/count must survive mtime change: %+v", e)
	}
	if e.Width != 8 || e.Height != 8 {
		t.Fatalf("dims must be rebuilt from the changed file, not carried over: %+v", e)
	}
	if e.Mtime != newMt {
		t.Fatalf("mtime not refreshed: got %d want %d", e.Mtime, newMt)
	}
}

// TestScanGeneratesThumb exercises the full magick path: a new file yields an
// onItem callback and real thumbnail files under the cache dir.
func TestScanGeneratesThumb(t *testing.T) {
	if !haveMagick() {
		t.Skip("magick not on PATH")
	}
	root := t.TempDir()
	wall := filepath.Join(root, "wall")
	cache := filepath.Join(root, "cache")
	writePNG(t, filepath.Join(wall, "sub", "b.png"), color.RGBA{200, 20, 20, 255})

	var fired []Entry
	got, err := ScanDirs(wall, wall, cache, nil, func(e Entry) { fired = append(fired, e) })
	if err != nil {
		t.Fatal(err)
	}
	if len(fired) != 1 {
		t.Fatalf("want 1 onItem call, got %d", len(fired))
	}

	e := got["sub--b.png"]
	wantThumb := filepath.Join(cache, "wallpaper", "thumbs", "sub--b.png.webp")
	wantSm := filepath.Join(cache, "wallpaper", "thumbs-sm", "sub--b.png.webp")
	if e.Thumb != wantThumb {
		t.Errorf("thumb path: got %q want %q", e.Thumb, wantThumb)
	}
	if e.ThumbSm != wantSm {
		t.Errorf("thumb_sm path: got %q want %q", e.ThumbSm, wantSm)
	}
	if _, err := os.Stat(wantThumb); err != nil {
		t.Errorf("full thumb not written: %v", err)
	}
	if _, err := os.Stat(wantSm); err != nil {
		t.Errorf("small thumb not written: %v", err)
	}
}

// TestScanSkipsFreshThumb: when a thumbnail already exists newer than its source
// and there is no prior row, the file is reused (no onItem) rather than rebuilt.
func TestScanSkipsFreshThumb(t *testing.T) {
	if !haveMagick() {
		t.Skip("magick not on PATH")
	}
	root := t.TempDir()
	wall := filepath.Join(root, "wall")
	cache := filepath.Join(root, "cache")
	writePNG(t, filepath.Join(wall, "a.png"), color.RGBA{200, 20, 20, 255})

	if _, err := ScanDirs(wall, wall, cache, nil, func(Entry) {}); err != nil {
		t.Fatal(err)
	}
	thumb := filepath.Join(cache, "wallpaper", "thumbs", "a.png.webp")
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(thumb, future, future); err != nil {
		t.Fatal(err)
	}

	calls := 0
	if _, err := ScanDirs(wall, wall, cache, nil, func(Entry) { calls++ }); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("fresh on-disk thumb must be reused without onItem, fired %d", calls)
	}
}

// fakeFFprobeFrames puts a stub ffprobe on PATH that reports a frame count by
// filename (anything but a name containing "still" reads as multi-frame), so
// the webp classifier is tested without a real animated webp on disk.
func fakeFFprobeFrames(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	body := "#!/bin/sh\nlast=\"\"\nfor a in \"$@\"; do last=\"$a\"; done\n" +
		"case \"$last\" in *still*) printf '1\\n';; *) printf '12\\n';; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

// TestIsAnimatedImageWebp: a single-frame webp is a still and must not be sent
// down the mp4 transcode path -- that forced loop was what made a static webp
// cost video-level GPU power. A multi-frame webp still animates, and a format
// that cannot animate short-circuits before ffprobe is ever consulted.
func TestIsAnimatedImageWebp(t *testing.T) {
	fakeFFprobeFrames(t)
	dir := t.TempDir()
	mk := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if isAnimatedImage(mk("still.webp")) {
		t.Fatal("a single-frame webp must be classified as static")
	}
	if !isAnimatedImage(mk("motion.webp")) {
		t.Fatal("a multi-frame webp must be classified as animated")
	}
	if isAnimatedImage(mk("photo.jpg")) {
		t.Fatal("a jpg must never be classified as animated")
	}
}

func keys(m map[string]Entry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Startup sweeps only the temp siblings tmpPath creates, never finished
// artifacts whose stem happens to contain ".tmp." (photo.tmp.png.webp).
func TestSweepTmpFilesRemovesOnlyTempSuffixes(t *testing.T) {
	dir := t.TempDir()
	vids := filepath.Join(dir, "wallpaper", "video-thumbs")
	thumbs := filepath.Join(dir, "wallpaper", "thumbs")
	for _, d := range []string{vids, thumbs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	keep := []string{
		filepath.Join(vids, "we--2215418014-prev.mp4"),
		filepath.Join(thumbs, "photo.tmp.webp"),
		filepath.Join(thumbs, "photo.tmp.png.webp"),
		filepath.Join(thumbs, "photo.tmp.1234.webp"),
		filepath.Join(thumbs, "photo.tmp.1234-abc.webp"),
	}
	remove := []string{
		filepath.Join(vids, "we--2215418014-prev.tmp.4938-1795126831842862467.mp4"),
		filepath.Join(vids, "we--2215418014-prev.meta.tmp.4938-1795126831842862467.meta"),
		filepath.Join(thumbs, "photo.tmp.4938-1795126831842862467.webp"),
		filepath.Join(thumbs, "photo.tmp.png.tmp.4938-1795126831842862467.webp"),
	}
	for _, p := range append(append([]string{}, keep...), remove...) {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sweepTmpFiles(dir)
	for _, p := range keep {
		if !fileExists(p) {
			t.Fatalf("sweep removed a finished artifact: %s", p)
		}
	}
	for _, p := range remove {
		if fileExists(p) {
			t.Fatalf("sweep left an orphan temp behind: %s", p)
		}
	}
}

// Warm rescans reuse the identity hashed on an earlier scan while the source's
// cheap stat signature is unchanged, and rehash as soon as it moves. A fresh
// hash of a removed source is empty, so the memo is observable without timing.
func TestCachedPreviewSourceIdentitySkipsUnchangedSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "clip.bin")
	if err := os.WriteFile(src, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	it := scanned{src: src, filesize: st.Size(), mtime: st.ModTime().Unix()}

	first := cachedPreviewSourceIdentity(it)
	if first == "" {
		t.Fatal("first identity is empty")
	}
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if second := cachedPreviewSourceIdentity(it); second != first {
		t.Fatalf("unchanged source was rehashed: %q -> %q", first, second)
	}

	// A moved timestamp drops the memo and forces a fresh full-content hash.
	it.mtime++
	if third := cachedPreviewSourceIdentity(it); third != "" {
		t.Fatalf("stale identity survived a stat change: %q", third)
	}
	if err := os.WriteFile(src, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A different recorded size also misses the memo (and the identity itself
	// changes because the size is part of it).
	it.filesize++
	if fourth := cachedPreviewSourceIdentity(it); fourth == "" || fourth == first {
		t.Fatalf("size change did not produce a fresh identity: %q", fourth)
	}
}

// The startup temp sweep must run before any watcher, restore or rescan can
// start a generator; running it inside the restore goroutine let watchers
// create live temps that the sweep then deleted.
func TestStartupSweepPrecedesGenerators(t *testing.T) {
	src, err := os.ReadFile("daemon.go")
	if err != nil {
		t.Fatal(err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), "daemon.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var run *ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "runDaemon" {
			run = fn
			break
		}
	}
	if run == nil {
		t.Fatal("runDaemon not found")
	}
	tracked := map[string]bool{
		"sweepTmpFiles":           true,
		"rescan":                  true,
		"restoreActiveWorkspaces": true,
		"startLibraryWatch":       true,
		"watch":                   true,
	}
	positions := map[string]token.Pos{}
	ast.Inspect(run, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var name string
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			name = fun.Name
		case *ast.SelectorExpr:
			name = fun.Sel.Name
		}
		if tracked[name] {
			if _, seen := positions[name]; !seen {
				positions[name] = call.Pos()
			}
		}
		return true
	})
	sweep, ok := positions["sweepTmpFiles"]
	if !ok {
		t.Fatal("runDaemon never sweeps temp files")
	}
	for _, generator := range []string{"rescan", "restoreActiveWorkspaces", "startLibraryWatch", "watch"} {
		p, ok := positions[generator]
		if ok && p < sweep {
			t.Fatalf("%s starts before the startup temp sweep", generator)
		}
	}
}
