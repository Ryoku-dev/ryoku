//go:build linux

package palette

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/design"
)

// watching starts WatchDesktop on h and returns its readings and a stop function that
// waits for it to return.
func watching(t *testing.T, h home) (<-chan Desktop, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	readings := make(chan Desktop, 16)
	ready := make(chan struct{})
	done := make(chan error, 1)
	options := append(h.options(false), func(c *readConfig) { c.armed = func() { close(ready) } })
	go func() { done <- WatchDesktop(ctx, func(d Desktop) { readings <- d }, options...) }()
	select {
	case <-ready:
	case err := <-done:
		t.Fatalf("WatchDesktop returned before arming: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("WatchDesktop never armed")
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("WatchDesktop = %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Error("WatchDesktop did not return after cancel")
			}
		})
	}
	t.Cleanup(stop)
	return readings, stop
}

// replace writes value to path the way the shell daemon does: a temporary file renamed
// over the old one.
func replace(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temporary, path); err != nil {
		t.Fatal(err)
	}
}

func next(t *testing.T, readings <-chan Desktop) Desktop {
	t.Helper()
	select {
	case d := <-readings:
		return d
	case <-time.After(5 * time.Second):
		t.Fatal("no reading after a relevant change")
		return Desktop{}
	}
}

func quiet(t *testing.T, readings <-chan Desktop, wait time.Duration) {
	t.Helper()
	select {
	case d := <-readings:
		t.Fatalf("unexpected reading %+v", d)
	case <-time.After(wait):
	}
}

func TestWatchDesktopAtomicReplaceCallsOnce(t *testing.T) {
	h := newHome(t)
	shell := filepath.Join(h.config, "ryoku", "shell.json")
	replace(t, shell, map[string]any{"theme": map[string]any{"design": "lithic"}})
	readings, _ := watching(t, h)

	replace(t, shell, map[string]any{"theme": map[string]any{"design": "material"}})
	if d := next(t, readings); d.Design != design.Material {
		t.Fatalf("design = %v", d.Design)
	}
	quiet(t, readings, 500*time.Millisecond)
}

func TestWatchDesktopIgnoresIrrelevantShellEdits(t *testing.T) {
	h := newHome(t)
	shell := filepath.Join(h.config, "ryoku", "shell.json")
	replace(t, shell, map[string]any{"bar": map[string]any{"height": 30}, "theme": map[string]any{"design": "fluent"}})
	readings, _ := watching(t, h)

	replace(t, shell, map[string]any{"bar": map[string]any{"height": 34}, "theme": map[string]any{"design": "fluent"}})
	quiet(t, readings, 500*time.Millisecond)

	// the watch is still alive: a relevant edit after the irrelevant one arrives alone
	replace(t, shell, map[string]any{"bar": map[string]any{"height": 34}, "theme": map[string]any{"design": "fluent", "motion": map[string]any{"reduce": true}}})
	if d := next(t, readings); !d.ReduceMotion || d.Design != design.Fluent {
		t.Fatalf("reading = %+v", d)
	}
	quiet(t, readings, 400*time.Millisecond)
}

func TestWatchDesktopFollowsTheSurfaceOnRyokuColours(t *testing.T) {
	h := newHome(t)
	h.matugen(t, "smart")
	readings, _ := watching(t, h)

	// the cache tree does not exist yet: the watch re-arms as the directories appear
	replace(t, filepath.Join(h.cache, "ryoku", "colors.json"), materialRoles(false))
	if d := next(t, readings); d.Mode != Light || d.Roles != Signature(false) {
		t.Fatalf("light surface = %+v", d)
	}

	replace(t, filepath.Join(h.config, "ryoku", "matugen.json"), map[string]any{"mode": "dark"})
	if d := next(t, readings); d.Mode != Dark {
		t.Fatalf("matugen dark = %+v", d)
	}

	replace(t, filepath.Join(h.config, "ryoku", "theme.json"), map[string]any{"followWallpaper": false})
	quiet(t, readings, 400*time.Millisecond)
}

func TestWatchDesktopStopsOnCancel(t *testing.T) {
	_, stop := watching(t, newHome(t))
	stop()
}
