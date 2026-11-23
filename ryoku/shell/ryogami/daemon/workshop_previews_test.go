package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newWorkshopScanDaemon(t *testing.T) (*daemon, string, string) {
	t.Helper()
	old := weScanHook
	t.Cleanup(func() { weScanHook = old })
	d := newSettingsDaemon(t)
	cache := t.TempDir()
	d.cfg.Paths.Cache = cache
	d.store = openStore(cache)
	d.workshop = newWorkshopLib(d)
	root := t.TempDir()
	if err := d.persistSetting("paths.steamWorkshop", root); err != nil {
		t.Fatal(err)
	}
	return d, root, cache
}

func writeWorkshopItem(t *testing.T, root, id, project string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.json"), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func writeTinyClip(t *testing.T, path string) {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-y", "-v", "error", "-f", "lavfi",
		"-i", "testsrc=size=320x180:rate=24:duration=1", "-pix_fmt", "yuv420p", path).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, out)
	}
}

func TestWorkshopVideoItemsGetPreviewClip(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	d, root, cache := newWorkshopScanDaemon(t)
	const videoID, sceneID = "3100000001", "3100000002"
	videoDir := writeWorkshopItem(t, root, videoID, `{"type":"video","title":"Clip","file":"clip.mp4"}`, nil)
	writeTinyClip(t, filepath.Join(videoDir, "clip.mp4"))
	writeWorkshopItem(t, root, sceneID, `{"type":"scene","title":"Harbour"}`, map[string]string{"scene.pkg": "x"})

	fired := map[string]int{}
	got := d.workshop.scanEntries(cache, nil, func(e Entry) { fired[e.Key]++ })
	video, scene := got["we:"+videoID], got["we:"+sceneID]
	if video.WeType != "video" || scene.WeType != "scene" {
		t.Fatalf("rows: video=%+v scene=%+v", video, scene)
	}
	if video.VideoPrev == "" || !fileExists(video.VideoPrev) {
		t.Fatalf("video item has no preview clip: %q", video.VideoPrev)
	}
	if filepath.Dir(video.VideoPrev) != filepath.Join(cache, "wallpaper", "video-thumbs") {
		t.Fatalf("clip outside the video thumb cache: %s", video.VideoPrev)
	}
	if scene.VideoPrev != "" {
		t.Fatalf("scene got a preview clip: %s", scene.VideoPrev)
	}

	// a library indexed before workshop clips existed, thumbs warm
	clip := video.VideoPrev
	if err := os.Remove(clip); err != nil {
		t.Fatal(err)
	}
	thumb, _ := weThumbPaths(cache, videoID)
	if err := os.MkdirAll(filepath.Dir(thumb), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(thumb, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	warm := video
	warm.Thumb = thumb
	warm.VideoPrev = ""
	fired = map[string]int{}
	got = d.workshop.scanEntries(cache, map[string]Entry{warm.Key: warm}, func(e Entry) { fired[e.Key]++ })
	if p := got[warm.Key].VideoPrev; p != clip || !fileExists(p) {
		t.Fatalf("warm row not backfilled: %q", p)
	}
	if fired[warm.Key] != 1 {
		t.Fatalf("backfilled row announced %d times, want 1", fired[warm.Key])
	}
}

func TestWorkshopPreviewClipsFollowTheToggle(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	d, root, cache := newWorkshopScanDaemon(t)
	if err := d.persistSetting("videoPreview.enabled", false); err != nil {
		t.Fatal(err)
	}
	const id = "3100000003"
	dir := writeWorkshopItem(t, root, id, `{"type":"video","title":"Clip","file":"clip.mp4"}`, nil)
	writeTinyClip(t, filepath.Join(dir, "clip.mp4"))

	got := d.workshop.scanEntries(cache, nil, func(Entry) {})
	if p := got["we:"+id].VideoPrev; p != "" {
		t.Fatalf("clip built with previews off: %s", p)
	}
	if clips, _ := filepath.Glob(filepath.Join(cache, "wallpaper", "video-thumbs", "*-prev.mp4")); len(clips) != 0 {
		t.Fatalf("transcoded with previews off: %v", clips)
	}
}

// Steam revalidation moves workshop mtimes past a freshly rendered clip at
// every login: the size record beside the clip, not the mtime, marks it
// reusable. A real update swaps the file and its size, and only that
// re-renders.
func TestWorkshopPreviewReuseIgnoresSteamMtime(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	d, root, cache := newWorkshopScanDaemon(t)
	const id = "3100000004"
	dir := writeWorkshopItem(t, root, id, `{"type":"video","title":"Clip","file":"clip.mp4"}`, nil)
	clip := filepath.Join(dir, "clip.mp4")
	writeTinyClip(t, clip)

	first := d.workshop.scanEntries(cache, nil, func(Entry) {})
	row := first["we:"+id]
	if row.VideoPrev == "" || !fileExists(row.VideoPrev) {
		t.Fatalf("first scan rendered no clip: %q", row.VideoPrev)
	}

	// The revalidation stand-in: mtime moves into the future, the bytes do
	// not. The clip becomes a sentinel so a re-render is a visible
	// overwrite.
	if err := os.WriteFile(row.VideoPrev, []byte("sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(clip, future, future); err != nil {
		t.Fatal(err)
	}
	again := d.workshop.scanEntries(cache, map[string]Entry{row.Key: row}, func(Entry) {})
	b, err := os.ReadFile(again[row.Key].VideoPrev)
	if err != nil || string(b) != "sentinel" {
		t.Fatalf("unchanged source re-rendered the clip: err=%v len=%d", err, len(b))
	}

	out, err := exec.Command("ffmpeg", "-y", "-v", "error", "-f", "lavfi",
		"-i", "testsrc=size=320x180:rate=24:duration=2", "-pix_fmt", "yuv420p", clip).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg second clip: %v: %s", err, out)
	}
	// The updated source carries an old mtime, as a Steam redownload can:
	// older than the sentinel clip, so only the size record can see the
	// change.
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(clip, past, past); err != nil {
		t.Fatal(err)
	}
	third := d.workshop.scanEntries(cache, map[string]Entry{row.Key: row}, func(Entry) {})
	b, err = os.ReadFile(third[row.Key].VideoPrev)
	if err != nil || string(b) == "sentinel" {
		t.Fatalf("a changed source kept the stale clip: err=%v len=%d", err, len(b))
	}

	// A same-size update must not wear the previous clip forever: flip one
	// metadata byte in the source's head (Lavf -> Lavq, still a valid file,
	// byte count unchanged) and the identity must move with it. The scan
	// re-renders, and the recorded identity under the new clip matches the
	// flipped source.
	prevPath := third[row.Key].VideoPrev
	before, err := os.Stat(prevPath)
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(clip)
	if err != nil {
		t.Fatal(err)
	}
	flipped := strings.Replace(string(src), "Lavf", "Lavq", 1)
	if len(flipped) != len(src) {
		t.Fatalf("flip changed the byte count: %d -> %d", len(src), len(flipped))
	}
	if err := os.WriteFile(clip, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // let the mtime second tick over
	fourth := d.workshop.scanEntries(cache, map[string]Entry{row.Key: row}, func(Entry) {})
	after, err := os.Stat(fourth[row.Key].VideoPrev)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().After(before.ModTime()) {
		t.Fatalf("a same-size update kept the previous clip (prev mtime %v)", after.ModTime())
	}
	if id := prevSourceIdentity(fourth[row.Key].VideoPrev); id == "" || id != previewSourceIdentity(clip, int64(len(flipped))) {
		t.Fatalf("recorded identity %q does not match the flipped source", id)
	}
}

// A source whose identity cannot be computed, and a clip with no identity
// record, are not a match: two empty strings are not evidence the clip is
// current. This is the tail of the size-before-mtime fix.
func TestWorkshopPreviewRequiresAReadableIdentity(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	d, root, cache := newWorkshopScanDaemon(t)
	const id = "3100000010"
	dir := writeWorkshopItem(t, root, id, `{"type":"video","title":"Clip","file":"clip.mp4"}`, nil)
	writeTinyClip(t, filepath.Join(dir, "clip.mp4"))

	first := d.workshop.scanEntries(cache, nil, func(Entry) {})
	prev := first["we:"+id].VideoPrev
	if prev == "" {
		t.Fatal("first scan rendered no clip")
	}
	if err := os.Remove(prev + ".meta"); err != nil {
		t.Fatal(err)
	}

	// os.Open succeeds on a directory but the head read fails, so the
	// identity is "" exactly as it is for an unreadable source file.
	got := ensureVideoPreview(scanned{
		we:        true,
		src:       t.TempDir(),
		filesize:  1,
		thumbPath: strings.TrimSuffix(prev, "-prev.mp4") + ".webp",
	})
	if got != "" {
		t.Fatalf("unidentifiable source reused clip %q", got)
	}
}

// If the previous identity record cannot be removed, publishing a new clip
// under it leaves a stale record on a fresh clip: a later source matching the
// old identity then reuses the wrong preview.
func TestWorkshopPreviewKeepsOldClipWhenRecordCannotBeRemoved(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	d, root, cache := newWorkshopScanDaemon(t)
	const id = "3100000011"
	dir := writeWorkshopItem(t, root, id, `{"type":"video","title":"Clip","file":"clip.mp4"}`, nil)
	clip := filepath.Join(dir, "clip.mp4")
	writeTinyClip(t, clip)

	first := d.workshop.scanEntries(cache, nil, func(Entry) {})
	prev := first["we:"+id].VideoPrev
	if prev == "" {
		t.Fatal("first scan rendered no clip")
	}
	if err := os.WriteFile(prev, []byte("sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := prev + ".meta"
	if err := os.Remove(meta); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(meta, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meta, "record"), []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("ffmpeg", "-y", "-v", "error", "-f", "lavfi",
		"-i", "testsrc=size=320x180:rate=24:duration=2", "-pix_fmt", "yuv420p", clip).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg: %v: %s", err, out)
	}
	it, ok := weVideoSource(cache, first["we:"+id])
	if !ok {
		t.Fatal("workshop source missing")
	}
	if got := ensureVideoPreview(it); got != "" {
		t.Fatalf("published a clip under an unremovable record: %q", got)
	}
	if b, err := os.ReadFile(prev); err != nil || string(b) != "sentinel" {
		t.Fatalf("old clip was replaced: err=%v content=%q", err, string(b))
	}
}

// A same-size update that changes bytes beyond the first 64 KiB must still
// move the identity; hashing only a fixed prefix let such an update wear its
// old preview forever.
func TestPreviewSourceIdentityHashesBeyondFirst64KiB(t *testing.T) {
	dir := t.TempDir()
	base := bytes.Repeat([]byte{'a'}, 2*64*1024+1)
	mutated := append([]byte(nil), base...)
	mutated[len(mutated)-1] = 'b'

	beforePath := filepath.Join(dir, "before.bin")
	afterPath := filepath.Join(dir, "after.bin")
	if err := os.WriteFile(beforePath, base, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(afterPath, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	before := previewSourceIdentity(beforePath, int64(len(base)))
	after := previewSourceIdentity(afterPath, int64(len(mutated)))
	if before == "" || after == "" {
		t.Fatalf("empty identity: before=%q after=%q", before, after)
	}
	if before == after {
		t.Fatalf("same-size sources differing after 64 KiB share identity %q", before)
	}
}

// The preview reuse decision must use the whole-source identity: a same-size
// source whose tail changed must not reuse the clip recorded for the old
// bytes, even when its mtime is older than the clip.
func TestWorkshopPreviewRegeneratesOnLateSameSizeChange(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "clip.bin")
	base := bytes.Repeat([]byte{'a'}, 2*64*1024+1)
	if err := os.WriteFile(src, base, 0o644); err != nil {
		t.Fatal(err)
	}
	thumb := filepath.Join(dir, "video.webp")
	prev := strings.TrimSuffix(thumb, ".webp") + "-prev.mp4"
	if err := os.WriteFile(prev, []byte("sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePrevSourceIdentity(prev, previewSourceIdentity(src, int64(len(base)))); err != nil {
		t.Fatal(err)
	}

	mutated := append([]byte(nil), base...)
	mutated[len(mutated)-1] = 'b'
	if err := os.WriteFile(src, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	got := ensureVideoPreview(scanned{
		we:        true,
		src:       src,
		filesize:  int64(len(mutated)),
		thumbPath: thumb,
	})
	if got == prev {
		t.Fatalf("a same-size tail change reused the old preview: %q", got)
	}
}

// The per-preview lock registry must drop idle entries: a library that
// churns cannot grow it without bound.
func TestPreviewRenderLocksReleaseIdleEntries(t *testing.T) {
	const prev = "we--lock-registry-prev.mp4"
	before := len(previewLocks)
	unlock := lockPreviewRender(prev)
	if len(previewLocks) != before+1 {
		t.Fatalf("registry grew by %d, want 1", len(previewLocks)-before)
	}
	unlock()
	if len(previewLocks) != before {
		t.Fatalf("idle entry retained: registry has %d entries, want %d", len(previewLocks), before)
	}
}
