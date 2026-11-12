package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
