package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHealAnimatedWebp: a catalog poisoned by the old classifier (every webp
// forced onto the video path) is reclassified once -- a single-frame webp goes
// back to a still in both the store and the stored wallpaper choice, while a
// genuinely animated webp keeps its video type. The revision flag makes a second
// run a no-op so ffprobe is never re-consulted on every boot.
func TestHealAnimatedWebp(t *testing.T) {
	fakeFFprobeFrames(t)
	root := t.TempDir()
	wall := filepath.Join(root, "wall")
	cache := filepath.Join(root, "cache")
	if err := os.MkdirAll(wall, 0o755); err != nil {
		t.Fatal(err)
	}
	mk := func(name string) string {
		p := filepath.Join(wall, name)
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	stillSrc := mk("still.webp")
	mk("motion.webp")

	var cfg config
	cfg.Paths.Wallpaper = wall
	cfg.Paths.Cache = cache
	d := &daemon{cfg: cfg, store: openStore(cache)}
	// Seed the poisoned state: both webps recorded as video with a transcode.
	d.store.entries["still.webp"] = Entry{Key: "still.webp", Name: "still.webp", Type: "video", VideoFile: "anim/still.mp4"}
	d.store.entries["motion.webp"] = Entry{Key: "motion.webp", Name: "motion.webp", Type: "video", VideoFile: "anim/motion.mp4"}
	// The stored choice paints a webp as a looping video on restore.
	saveJSON(filepath.Join(cache, "outputs.json"), map[string]map[string]interface{}{
		"*": {"type": "video", "path": stillSrc},
	})

	d.healAnimatedWebp()

	if e, _ := d.store.get("still.webp"); e.Type != "static" || e.VideoFile != "" {
		t.Fatalf("static webp must heal to a still with no clip, got type=%q clip=%q", e.Type, e.VideoFile)
	}
	if e, _ := d.store.get("motion.webp"); e.Type != "video" {
		t.Fatalf("animated webp must stay a video, got %q", e.Type)
	}
	var out map[string]map[string]interface{}
	loadJSON(filepath.Join(cache, "outputs.json"), &out)
	if got := out["*"]["type"]; got != "static" {
		t.Fatalf("stored choice must drop the video type, got %v", got)
	}

	// A second call is a no-op: clearing the store and re-running must not
	// reclassify, proving the revision flag short-circuits before any probing.
	d.store.entries["still.webp"] = Entry{Key: "still.webp", Name: "still.webp", Type: "video", VideoFile: "anim/still.mp4"}
	d.healAnimatedWebp()
	if e, _ := d.store.get("still.webp"); e.Type != "video" {
		t.Fatalf("second run must be a no-op, but reclassified to %q", e.Type)
	}
}
