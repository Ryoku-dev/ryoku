package main

import (
	"os"
	"path/filepath"
	"testing"

	wm "ryoku-wm"
)

func twoOutputDesk(t *testing.T, general string) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	cfgDir := filepath.Join(t.TempDir(), "config")
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	if err := os.MkdirAll(filepath.Join(cfgDir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"general":{"notifyOnWallpaperChange":false` + general + `}}`
	if err := os.WriteFile(filepath.Join(cfgDir, "ryoku", "ryogami.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	previousOutputs := outputs.list()
	wmCache.mu.Lock()
	previousFocus := wmCache.focusedOutput
	wmCache.focusedOutput = "DP-2"
	wmCache.mu.Unlock()
	t.Cleanup(func() {
		outputs.set(previousOutputs)
		wmCache.mu.Lock()
		wmCache.focusedOutput = previousFocus
		wmCache.mu.Unlock()
	})
	outputs.set([]wm.Output{{Name: "DP-1"}, {Name: "DP-2", Focused: true}})
}

func storedOutputs(t *testing.T, d *daemon) map[string]string {
	t.Helper()
	state := map[string]map[string]interface{}{}
	loadJSON(filepath.Join(d.config().cacheDir(), "outputs.json"), &state)
	paths := map[string]string{}
	for k, v := range state {
		paths[k], _ = v["path"].(string)
	}
	return paths
}

// #751: rotation folded per-output picks into one "*" entry
func TestRandomRotationPicksPerOutput(t *testing.T) {
	twoOutputDesk(t, "")
	d := previewTestDaemon(t)
	left := previewTestStill(t, d, "left.png")
	right := previewTestStill(t, d, "right.png")
	previewTestStill(t, d, "third.png")
	previewTestStill(t, d, "fourth.png")
	if err := d.applyWallpaper("static", left, "set", []string{"DP-1"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := d.applyWallpaper("static", right, "set", []string{"DP-2"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	before := storedOutputs(t, d)
	for range 20 {
		d.randomPick([]string{"static"}, false)
		after := storedOutputs(t, d)
		if _, broadcast := after["*"]; broadcast || len(after) != 2 {
			t.Fatalf("rotation folded the per-output state: %v", after)
		}
		for _, output := range []string{"DP-1", "DP-2"} {
			if after[output] == before[output] {
				t.Fatalf("%s kept %s across a rotation tick", output, after[output])
			}
		}
		if after["DP-1"] == after["DP-2"] {
			t.Fatalf("both outputs got %s with spare wallpapers in the pool", after["DP-1"])
		}
		before = after
	}
}

func TestRandomRotationStaysSharedWithoutPerOutputState(t *testing.T) {
	twoOutputDesk(t, "")
	d := previewTestDaemon(t)
	shared := previewTestStill(t, d, "shared.png")
	previewTestStill(t, d, "other.png")
	if err := d.applyWallpaper("static", shared, "set", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	d.randomPick([]string{"static"}, false)
	after := storedOutputs(t, d)
	if len(after) != 1 || after["*"] == "" {
		t.Fatalf("a shared desk must keep rotating as one: %v", after)
	}
}
