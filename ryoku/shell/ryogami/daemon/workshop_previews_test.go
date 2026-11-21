package main

import (
	"os"
	"os/exec"
	"path/filepath"
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
}
