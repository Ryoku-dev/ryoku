package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The wall preview, pinned down: a preview is a paint, never an apply. It
// publishes the previewed still flagged as a preview, and it leaves the
// catalog, the stored choice, and the applied state exactly as the last
// commit left them. Clearing repaints the committed wallpaper.

func previewTestDaemon(t *testing.T) *daemon {
	t.Helper()
	root := t.TempDir()
	var cfg config
	cfg.Paths.Wallpaper = filepath.Join(root, "wall")
	cfg.Paths.Cache = filepath.Join(root, "cache")
	if err := os.MkdirAll(cfg.Paths.Wallpaper, 0o755); err != nil {
		t.Fatal(err)
	}
	return &daemon{
		cfg:            cfg,
		surface:        newWallSurface(),
		store:          openStore(cfg.Paths.Cache),
		events:         newEventHub(),
		video:          newVideoPlayer(),
		lastTransition: -1,
	}
}

func previewTestStill(t *testing.T, d *daemon, name string) string {
	t.Helper()
	p := filepath.Join(d.config().wallpaperDir(), name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	d.store.entries[name] = Entry{Key: name, Name: name, Type: "static", Path: p}
	return p
}

func TestPreviewPaintsFlaggedAndRecordsNothing(t *testing.T) {
	d := previewTestDaemon(t)
	committed := previewTestStill(t, d, "committed.png")
	preview := previewTestStill(t, d, "preview.png")
	events := d.events.subscribe()
	if err := d.applyWallpaper("static", committed, "set", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	default:
		t.Fatal("committed apply did not publish its event")
	}
	_, _, frames := d.surface.topic.subscribe()
	savedBefore, err := os.ReadFile(filepath.Join(d.config().cacheDir(), "outputs.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := d.previewWallpaper(preview, nil); err != nil {
		t.Fatal(err)
	}
	var published wallFrame
	if err := json.Unmarshal([]byte(<-frames), &published); err != nil {
		t.Fatal(err)
	}
	if !published.Default.Preview {
		t.Fatal("the first published preview frame was not flagged")
	}
	select {
	case frame := <-frames:
		t.Fatalf("preview published a second frame: %s", frame)
	default:
	}
	for {
		select {
		case event := <-events:
			if strings.Contains(event, `"event":"ryogami.wall.applied"`) {
				t.Fatalf("preview published an applied event: %s", event)
			}
		default:
			goto eventsDrained
		}
	}
eventsDrained:
	frame := d.surface.snapshot()
	if frame.Default.Path != preview {
		t.Fatalf("frame path=%q, want the previewed still %q", frame.Default.Path, preview)
	}
	if !frame.Default.Preview {
		t.Fatal("frame entry is not flagged as a preview")
	}
	savedAfter, err := os.ReadFile(filepath.Join(d.config().cacheDir(), "outputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(savedAfter) != string(savedBefore) {
		t.Fatalf("preview rewrote the stored choice:\nbefore %s\nafter  %s", savedBefore, savedAfter)
	}
	if e, _ := d.store.get("preview.png"); e.ApplyCount != 0 {
		t.Fatalf("preview bumped its apply count to %d", e.ApplyCount)
	}
	if e, _ := d.store.get("committed.png"); e.ApplyCount != 1 {
		t.Fatalf("committed apply count=%d, want 1", e.ApplyCount)
	}
}

func TestPreviewReplacesPreview(t *testing.T) {
	d := previewTestDaemon(t)
	first := previewTestStill(t, d, "first.png")
	second := previewTestStill(t, d, "second.png")
	if err := d.previewWallpaper(first, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.previewWallpaper(second, nil); err != nil {
		t.Fatal(err)
	}
	frame := d.surface.snapshot()
	if frame.Default.Path != second || !frame.Default.Preview {
		t.Fatalf("frame=(%q, preview=%v), want the second still flagged", frame.Default.Path, frame.Default.Preview)
	}
	st := d.preview
	if st == nil || st.path != second {
		t.Fatalf("preview state=%+v, want it tracking the second still", st)
	}
}

func TestClearPreviewRepaintsCommitted(t *testing.T) {
	d := previewTestDaemon(t)
	committed := previewTestStill(t, d, "committed.png")
	preview := previewTestStill(t, d, "preview.png")
	if err := d.applyWallpaper("static", committed, "set", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.previewWallpaper(preview, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.clearPreview(); err != nil {
		t.Fatal(err)
	}
	if d.preview != nil {
		t.Fatal("preview state survived the clear")
	}
	frame := d.surface.snapshot()
	if frame.Default.Path != committed {
		t.Fatalf("frame path=%q, want the committed still %q back", frame.Default.Path, committed)
	}
	if frame.Default.Preview {
		t.Fatal("repainted committed entry is still flagged as a preview")
	}
	if got := d.currentName(); got != "committed.png" {
		t.Fatalf("current name=%q, want the pre-preview name restored", got)
	}
}

func TestClearPreviewRestoresEveryTouchedOutput(t *testing.T) {
	d := previewTestDaemon(t)
	base := previewTestStill(t, d, "base.png")
	left := previewTestStill(t, d, "left.png")
	right := previewTestStill(t, d, "right.png")
	first := previewTestStill(t, d, "first.png")
	second := previewTestStill(t, d, "second.png")
	for _, apply := range []struct {
		path    string
		outputs []string
	}{
		{base, nil},
		{left, []string{"DP-1"}},
		{right, []string{"DP-2"}},
	} {
		if err := d.applyWallpaper("static", apply.path, "set", apply.outputs, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.previewWallpaper(first, []string{"DP-1"}); err != nil {
		t.Fatal(err)
	}
	if err := d.previewWallpaper(second, []string{"DP-2"}); err != nil {
		t.Fatal(err)
	}
	frame := d.surface.snapshot()
	if frame.Outputs["DP-1"].Path != first || frame.Outputs["DP-2"].Path != second {
		t.Fatalf("preview frame = %+v", frame.Outputs)
	}
	if err := d.clearPreview(); err != nil {
		t.Fatal(err)
	}
	frame = d.surface.snapshot()
	if frame.Outputs["DP-1"].Path != left || frame.Outputs["DP-2"].Path != right {
		t.Fatalf("restored frame = %+v", frame.Outputs)
	}
	if frame.Outputs["DP-1"].Preview || frame.Outputs["DP-2"].Preview {
		t.Fatal("a restored output is still marked as a preview")
	}
}

func TestApplySupersedesPreview(t *testing.T) {
	d := previewTestDaemon(t)
	preview := previewTestStill(t, d, "preview.png")
	other := previewTestStill(t, d, "other.png")
	if err := d.previewWallpaper(preview, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.applyWallpaper("static", other, "set", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if d.preview != nil {
		t.Fatal("a committed apply left preview state behind")
	}
	frame := d.surface.snapshot()
	if frame.Default.Path != other || frame.Default.Preview {
		t.Fatalf("frame=(%q, preview=%v), want the applied still unflagged", frame.Default.Path, frame.Default.Preview)
	}
}

func TestApplyMatchingPreviewDoesNotRestartReveal(t *testing.T) {
	d := previewTestDaemon(t)
	preview := previewTestStill(t, d, "preview.png")
	if err := d.previewWallpaper(preview, nil); err != nil {
		t.Fatal(err)
	}
	before := d.surface.snapshot().Default.Revision
	if err := d.applyWallpaper("static", preview, "set", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	frame := d.surface.snapshot().Default
	if frame.Revision != before {
		t.Fatalf("matching apply revision=%d, want preview revision %d", frame.Revision, before)
	}
	if frame.Preview {
		t.Fatal("matching apply left the frame flagged as a preview")
	}
}

func TestClearPreviewWithoutPreviewIsNoOp(t *testing.T) {
	d := previewTestDaemon(t)
	if err := d.clearPreview(); err != nil {
		t.Fatalf("clear with no live preview: %v", err)
	}
}

func TestPreviewOverLiveKeepsRenderer(t *testing.T) {
	d := previewTestDaemon(t)
	poster := previewTestStill(t, d, "poster.png")
	preview := previewTestStill(t, d, "preview.png")
	player := &liveProc{cmd: &exec.Cmd{}, output: ""}
	d.video.procs[""] = player
	d.video.slotGen[""] = 1
	d.video.path = "clip.mp4"
	d.video.outputs = []string{"*"}
	d.surface.show(poster, "Cover", nil, true, true, videoClip{})

	if err := d.previewWallpaper(preview, nil); err != nil {
		t.Fatal(err)
	}
	if d.video.procs[""] != player {
		t.Fatal("preview stopped the live renderer")
	}
	if err := d.clearPreview(); err != nil {
		t.Fatal(err)
	}
	if d.video.procs[""] != player {
		t.Fatal("clear restarted the live renderer")
	}
	frame := d.surface.snapshot().Default
	if frame.Path != poster || !frame.Live || frame.Preview {
		t.Fatalf("restored frame=(%q, live=%v, preview=%v)", frame.Path, frame.Live, frame.Preview)
	}
}

func TestPreviewRejectsNonStaticAndMissing(t *testing.T) {
	d := previewTestDaemon(t)
	clip := filepath.Join(d.config().wallpaperDir(), "clip.mp4")
	if err := os.WriteFile(clip, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.previewWallpaper(clip, nil); err == nil {
		t.Fatal("preview accepted a video")
	}
	if err := d.previewWallpaper(filepath.Join(d.config().wallpaperDir(), "gone.png"), nil); err == nil {
		t.Fatal("preview accepted a missing file")
	}
	if err := d.previewWallpaper("", nil); err == nil {
		t.Fatal("preview accepted an empty path")
	}
	if d.preview != nil {
		t.Fatal("a rejected preview left state behind")
	}
}

func TestPreviewByKeyResolvesEntry(t *testing.T) {
	d := previewTestDaemon(t)
	still := previewTestStill(t, d, "still.png")
	clip := filepath.Join(d.config().wallpaperDir(), "clip.mp4")
	if err := os.WriteFile(clip, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	d.store.entries["clip.mp4"] = Entry{Key: "clip.mp4", Name: "clip.mp4", Type: "video", Path: clip}
	d.store.entries["scene"] = Entry{Key: "scene", Name: "scene", Type: "we", Path: filepath.Join(d.config().wallpaperDir(), "scene")}

	if err := d.previewWallpaperKey("still.png", nil); err != nil {
		t.Fatal(err)
	}
	frame := d.surface.snapshot()
	if frame.Default.Path != still || !frame.Default.Preview {
		t.Fatalf("frame=(%q, preview=%v), want the keyed still %q flagged", frame.Default.Path, frame.Default.Preview, still)
	}
	if err := d.previewWallpaperKey("never-scanned", nil); err == nil {
		t.Fatal("preview accepted an unknown key")
	}
	if err := d.previewWallpaperKey("clip.mp4", nil); err == nil {
		t.Fatal("preview accepted a video entry by key")
	}
	if err := d.previewWallpaperKey("scene", nil); err == nil {
		t.Fatal("preview accepted a Wallpaper Engine entry by key")
	}
	if st := d.preview; st == nil || st.path != still {
		t.Fatalf("preview state=%+v, want it still tracking the keyed still after the rejections", st)
	}
}

func TestPreviewPaletteIsIsolatedCachedAndRestored(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	d := previewTestDaemon(t)
	previewTestStill(t, d, "old.png")
	wall := previewTestStill(t, d, "new.png")
	d.store.mutate("new.png", func(e *Entry) { e.Thumb = wall; e.Mtime = 42 })
	committed := []byte(`{"primary":"#123456"}`)
	if err := os.MkdirAll(filepath.Dir(themeColorsPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(themeColorsPath(), committed, 0o644); err != nil {
		t.Fatal(err)
	}
	previewMu.Lock()
	oldCache := previewCache
	previewCache = map[string]cachedPreview{}
	previewMu.Unlock()
	oldGenerator := previewColorGenerator
	t.Cleanup(func() {
		previewColorGenerator = oldGenerator
		previewMu.Lock()
		previewCache = oldCache
		previewMu.Unlock()
	})
	generated := make(chan struct{}, 4)
	previewColorGenerator = func(string) (map[string]string, error) {
		generated <- struct{}{}
		return map[string]string{"primary": "#abcdef", "background": "#222222"}, nil
	}
	previewPath := filepath.Join(runtimeDir, "ryoku-wall-preview.json")
	for range 2 {
		if err := d.previewWallpaperKey("new.png", nil); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(time.Second)
		for {
			b, err := os.ReadFile(previewPath)
			if err == nil && strings.Contains(string(b), "#abcdef") {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("preview palette did not reach the shell-only runtime file")
			}
			time.Sleep(5 * time.Millisecond)
		}
		if err := d.clearPreview(); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(previewPath)
		if err != nil || string(b) != "{}" {
			t.Fatalf("cancel palette = %s, %v", b, err)
		}
		b, err = os.ReadFile(themeColorsPath())
		if err != nil || string(b) != string(committed) {
			t.Fatalf("committed palette changed during preview: %s, %v", b, err)
		}
	}
	select {
	case <-generated:
	default:
		t.Fatal("preview did not generate a palette")
	}
	select {
	case <-generated:
		t.Fatal("repeating a preview generated its palette twice")
	default:
	}
}

func TestClearedPreviewDropsLatePalette(t *testing.T) {
	runtimeDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	d := previewTestDaemon(t)
	wall := previewTestStill(t, d, "late.png")
	if err := d.previewWallpaper(wall, nil); err != nil {
		t.Fatal(err)
	}
	state := d.preview
	epoch := state.epoch
	if err := d.clearPreview(); err != nil {
		t.Fatal(err)
	}
	d.publishPreviewPalette(state, epoch, wall, map[string]string{"primary": "#abcdef"})
	b, err := os.ReadFile(filepath.Join(runtimeDir, "ryoku-wall-preview.json"))
	if err != nil || string(b) != "{}" {
		t.Fatalf("late palette repainted after cancel: %s, %v", b, err)
	}
}
