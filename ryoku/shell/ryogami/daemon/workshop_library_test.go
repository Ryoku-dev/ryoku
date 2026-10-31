package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func writeWEProject(t *testing.T, dir string, doc map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWorkshopWatchCoalescesWriteBurst(t *testing.T) {
	root := t.TempDir()
	item := filepath.Join(root, "123")
	writeWEProject(t, item, map[string]interface{}{"type": "scene"})

	watcher, err := openWorkshopWatch(root)
	if err != nil {
		t.Fatal(err)
	}
	const debounce = 50 * time.Millisecond
	rescans := make(chan struct{}, 8)
	type watchResult struct {
		rebuild bool
		err     error
	}
	done := make(chan watchResult, 1)
	go func() {
		rebuild, err := runWorkshopWatch(watcher, debounce, false, nil, func() {
			rescans <- struct{}{}
		})
		done <- watchResult{rebuild: rebuild, err: err}
	}()

	for i := range 8 {
		tmp := filepath.Join(item, "download.tmp")
		if err := os.WriteFile(tmp, []byte{byte(i)}, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, filepath.Join(item, "project.json")); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-rescans:
	case <-time.After(time.Second):
		t.Fatal("workshop writes did not trigger a rescan")
	}
	select {
	case <-rescans:
		t.Fatal("workshop burst triggered more than one rescan")
	case <-time.After(3 * debounce):
	}

	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		watcher.close()
		if result.err != nil {
			t.Fatal(result.err)
		}
		if !result.rebuild {
			t.Fatal("removed workshop root did not request a watch rebuild")
		}
	case <-time.After(time.Second):
		watcher.close()
		t.Fatal("workshop watcher did not stop after its root was removed")
	}
}

func TestWorkshopWatchRebuildSignal(t *testing.T) {
	watcher, err := openWorkshopWatch(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.close()
	rewatch := make(chan struct{}, 1)
	rewatch <- struct{}{}
	rebuild, err := runWorkshopWatch(watcher, time.Second, false, rewatch, func() {})
	if err != nil {
		t.Fatal(err)
	}
	if !rebuild {
		t.Fatal("path change did not request a watch rebuild")
	}
}

func TestDetectSteamRoot(t *testing.T) {
	homeDir := "/home/tester"
	flatpak := filepath.Join(homeDir, ".var/app/com.valvesoftware.Steam/.local/share/Steam")

	got := detectSteamRoot(homeDir, func(p string) bool {
		return p == filepath.Join(flatpak, "steamapps")
	})
	if got != flatpak {
		t.Fatalf("flatpak root: got %q want %q", got, flatpak)
	}

	native := filepath.Join(homeDir, ".local/share/Steam")
	if got := detectSteamRoot(homeDir, func(string) bool { return false }); got != native {
		t.Fatalf("fallback root: got %q want %q", got, native)
	}

	if got := detectSteamRoot(homeDir, func(string) bool { return true }); got != native {
		t.Fatalf("native precedence: got %q want %q", got, native)
	}
}

func TestProjectDependency(t *testing.T) {
	cases := []struct {
		name    string
		doc     map[string]interface{}
		wantID  string
		wantHas bool
		wantErr bool
	}{
		{"plain", map[string]interface{}{"type": "scene"}, "", false, false},
		{"string dep", map[string]interface{}{"dependency": "100", "preset": map[string]interface{}{"a": 1}}, "100", true, false},
		{"number dep", map[string]interface{}{"dependency": float64(100), "preset": map[string]interface{}{"a": 1}}, "100", true, false},
		{"dep no preset", map[string]interface{}{"dependency": "100"}, "", false, true},
		{"preset no dep", map[string]interface{}{"preset": map[string]interface{}{"a": 1}}, "", false, true},
		{"zero dep", map[string]interface{}{"dependency": "0", "preset": map[string]interface{}{"a": 1}}, "", false, true},
		{"non-digit dep", map[string]interface{}{"dependency": "12a", "preset": map[string]interface{}{"a": 1}}, "", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, has, err := projectDependency(tc.doc)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if id != tc.wantID || has != tc.wantHas {
				t.Fatalf("got (%q,%v) want (%q,%v)", id, has, tc.wantID, tc.wantHas)
			}
		})
	}
}

func TestResolveProjectPresetChain(t *testing.T) {
	lib := t.TempDir()
	parent := filepath.Join(lib, "100")
	writeWEProject(t, parent, map[string]interface{}{
		"type":  "scene",
		"title": "Parent",
		"general": map[string]interface{}{
			"properties": map[string]interface{}{
				"tint": map[string]interface{}{"type": "color", "value": "1 1 1", "text": "Tint"},
			},
		},
	})
	touch(t, filepath.Join(parent, "scene.pkg"))

	child := filepath.Join(lib, "200")
	writeWEProject(t, child, map[string]interface{}{
		"dependency": "100",
		"preset":     map[string]interface{}{"tint": "0 0 0"},
	})

	project, err := resolveProject(child)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	tint, ok := project.declarations()["tint"].(map[string]interface{})
	if !ok {
		t.Fatalf("tint declaration missing: %#v", project.declarations())
	}
	if tint["value"] != "0 0 0" {
		t.Fatalf("preset override not applied: value=%v", tint["value"])
	}

	weType, err := validateProject(child)
	if err != nil || weType != "scene" {
		t.Fatalf("validate child: type=%q err=%v", weType, err)
	}
}

func TestResolveProjectMissingDependency(t *testing.T) {
	lib := t.TempDir()
	child := filepath.Join(lib, "200")
	writeWEProject(t, child, map[string]interface{}{
		"dependency": "999",
		"preset":     map[string]interface{}{"a": 1},
	})
	if _, err := resolveProject(child); err == nil {
		t.Fatal("expected error for missing dependency item")
	}
}

func TestValidateProjectTypes(t *testing.T) {
	root := t.TempDir()

	scene := filepath.Join(root, "1")
	writeWEProject(t, scene, map[string]interface{}{"type": "scene"})
	if _, err := validateProject(scene); err == nil {
		t.Fatal("scene without package should fail")
	}
	touch(t, filepath.Join(scene, "gifscene.pkg"))
	if ty, err := validateProject(scene); err != nil || ty != "scene" {
		t.Fatalf("scene: type=%q err=%v", ty, err)
	}

	video := filepath.Join(root, "2")
	writeWEProject(t, video, map[string]interface{}{"type": "video", "file": "clip.mp4"})
	if _, err := validateProject(video); err == nil {
		t.Fatal("video without file should fail")
	}
	touch(t, filepath.Join(video, "clip.mp4"))
	if ty, err := validateProject(video); err != nil || ty != "video" {
		t.Fatalf("video: type=%q err=%v", ty, err)
	}

	for _, bad := range []string{"web", "application"} {
		dir := filepath.Join(root, bad)
		writeWEProject(t, dir, map[string]interface{}{"type": bad})
		if _, err := validateProject(dir); err == nil {
			t.Fatalf("%s project should be rejected", bad)
		}
	}
}

func TestSafeItemJoin(t *testing.T) {
	root := t.TempDir()
	touch(t, filepath.Join(root, "clip.mp4"))
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	touch(t, filepath.Join(root, "sub", "deep.mp4"))

	if p, ok := safeItemJoin(root, "clip.mp4"); !ok || p == "" {
		t.Fatalf("in-root join rejected: %q %v", p, ok)
	}
	if p, ok := safeItemJoin(root, "sub/deep.mp4"); !ok || p == "" {
		t.Fatalf("nested join rejected: %q %v", p, ok)
	}
	if _, ok := safeItemJoin(root, "../escape"); ok {
		t.Fatal("parent traversal accepted")
	}
	if _, ok := safeItemJoin(root, "/etc/passwd"); ok {
		t.Fatal("absolute path accepted")
	}
}

func TestFindPreviewUsesDecodableFallback(t *testing.T) {
	item := t.TempDir()
	writeWEProject(t, item, map[string]interface{}{
		"type":    "scene",
		"title":   "Fallback scene",
		"preview": "preview.tex",
	})
	touch(t, filepath.Join(item, "preview.tex"))
	if err := os.MkdirAll(filepath.Join(item, "materials"), 0o755); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(item, "materials", "cover.jpg")
	touch(t, fallback)

	project, err := resolveProject(item)
	if err != nil {
		t.Fatal(err)
	}
	if got := findPreview(item, project); got != fallback {
		t.Fatalf("preview = %q, want fallback %q", got, fallback)
	}
}

func TestFindPreviewKeepsAnimatedAndVideoSources(t *testing.T) {
	for _, name := range []string{"preview.gif", "preview.webm"} {
		t.Run(name, func(t *testing.T) {
			item := t.TempDir()
			writeWEProject(t, item, map[string]interface{}{"type": "scene", "preview": name})
			source := filepath.Join(item, name)
			touch(t, source)
			project, err := resolveProject(item)
			if err != nil {
				t.Fatal(err)
			}
			if got := findPreview(item, project); got != source {
				t.Fatalf("preview = %q, want %q", got, source)
			}
		})
	}
}

func TestProjectSchemeColorProvidesFallbackTint(t *testing.T) {
	doc := map[string]interface{}{
		"general": map[string]interface{}{
			"properties": map[string]interface{}{
				"schemecolor": map[string]interface{}{"value": "0.1 0.8 0.2"},
			},
		},
	}
	hue, sat, ok := projectSchemeColor(doc)
	if !ok || hue < 3 || hue > 5 || sat < 70 {
		t.Fatalf("scheme tint = (%d, %d, %v), want saturated green", hue, sat, ok)
	}
}

func TestGenerateWeThumbReadsGIFFrame(t *testing.T) {
	if !haveMagick() {
		t.Skip("magick not on PATH")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	root := t.TempDir()
	source := filepath.Join(root, "preview.gif")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	palette := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 32, 18), palette)
	second := image.NewPaletted(image.Rect(0, 0, 32, 18), palette)
	for i := range second.Pix {
		second.Pix[i] = 1
	}
	if err := gif.EncodeAll(file, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{2, 2}}); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	thumb := filepath.Join(root, "cache", "thumb.webp")
	small := filepath.Join(root, "cache-sm", "thumb.webp")
	if err := generateWeThumb(source, thumb, small); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{thumb, small} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("thumbnail %q missing or empty: %v", path, err)
		}
	}
}

func TestValidWeID(t *testing.T) {
	for _, id := range []string{"123", "3808523371"} {
		if !validWeID(id) {
			t.Fatalf("%q should be valid", id)
		}
	}
	for _, id := range []string{"", ".", "..", "-1", "a/b", "a\\b", "x..y"} {
		if validWeID(id) {
			t.Fatalf("%q should be invalid", id)
		}
	}
}

// With nothing able to fetch an item, a machine without Steam gets the item's web page
// rather than a steam:// link nothing on it can open.
func TestWorkshopDownloadWithoutAFetcher(t *testing.T) {
	d := newSettingsDaemon(t)
	w := &workshopLib{d: d}
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	home := t.TempDir()
	t.Setenv("HOME", home)
	saved := flatpakSystemApps
	flatpakSystemApps = filepath.Join(t.TempDir(), "app")
	t.Cleanup(func() { flatpakSystemApps = saved })

	status, url := w.download("3100000002")
	if status != "no_steam" || url != "https://steamcommunity.com/sharedfiles/filedetails/?id=3100000002" {
		t.Fatalf("no Steam: got %q %q", status, url)
	}

	if err := os.MkdirAll(filepath.Join(home, ".local", "share", "flatpak", "app", "com.valvesoftware.Steam"), 0o755); err != nil {
		t.Fatal(err)
	}
	if status, url := w.download("3100000002"); status != "open_in_steam" || url != "steam://url/CommunityFilePage/3100000002" {
		t.Fatalf("Flatpak Steam: got %q %q", status, url)
	}

	if err := os.RemoveAll(filepath.Join(home, ".local", "share", "flatpak")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "steam"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if status, _ := w.download("3100000002"); status != "open_in_steam" {
		t.Fatalf("native Steam: got %q", status)
	}
}

func TestReconcileWeItemReplacesBrokenHomeSymlink(t *testing.T) {
	root := t.TempDir()
	workshop := filepath.Join(root, "workshop")
	actual := filepath.Join(root, "new-home", "scene")
	if err := os.MkdirAll(actual, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workshop, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(workshop, "123")
	if err := os.Symlink("/home/old-user/.steam/scene", target); err != nil {
		t.Fatal(err)
	}

	reconcileWeItem(workshop, "123", actual)
	got, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != actual {
		t.Fatalf("scene link = %q, want %q", got, actual)
	}
}
