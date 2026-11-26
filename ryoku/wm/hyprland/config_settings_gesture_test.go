package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenGestureOffByDefault(t *testing.T) {
	if got := genGesture(defaultOverrides()); got != "" {
		t.Fatalf("an untouched store must not emit a gesture:\n%s", got)
	}
}

func TestLegacyWorkspaceSwipeStillEmits(t *testing.T) {
	o := defaultOverrides()
	o.Input.WorkspaceSwipe = true
	o.Input.SwipeFingers = 4
	want := "hl.gesture({ fingers = 4, direction = \"horizontal\", action = \"workspace\" })\n"
	if got := genGesture(o); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestGestureListReplacesLegacySwipe(t *testing.T) {
	o := defaultOverrides()
	o.Input.WorkspaceSwipe = true
	o.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 3, Direction: "down", Action: "close"},
	}
	got := genGesture(o)
	if strings.Contains(got, `action = "workspace"`) {
		t.Fatalf("the old toggle must not be added beside the list:\n%s", got)
	}
	if !strings.Contains(got, `direction = "down", action = "close"`) {
		t.Fatalf("missing the list row:\n%s", got)
	}
}

func TestGenGestureEmitsEachEnabledAction(t *testing.T) {
	o := defaultOverrides()
	o.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 3, Direction: "right", Action: "float", Mode: "tile"},
		{Enabled: true, Fingers: 3, Direction: "horizontal", Action: "workspace"},
		{Enabled: true, Fingers: 4, Direction: "up", Action: "special", WorkspaceName: "scratch"},
		{Enabled: true, Fingers: 3, Direction: "pinch", Action: "cursor_zoom", ZoomLevel: "1.5", Mode: "mult"},
		{Enabled: true, Fingers: 4, Direction: "left", Action: "close"},
		{Enabled: true, Fingers: 4, Direction: "down", Action: "fullscreen", Mode: "maximize"},
		{Enabled: false, Fingers: 3, Direction: "up", Action: "resize"},
		{Enabled: true, Fingers: 3, Direction: "", Action: "move"},
	}
	want := []string{
		`hl.gesture({ fingers = 3, direction = "right", action = "float", mode = "tile" })`,
		`hl.gesture({ fingers = 3, direction = "horizontal", action = "workspace" })`,
		`hl.gesture({ fingers = 4, direction = "up", action = "special", workspace_name = "scratch" })`,
		`hl.gesture({ fingers = 3, direction = "pinch", action = "cursor_zoom", zoom_level = "1.5", mode = "mult" })`,
		`hl.gesture({ fingers = 4, direction = "left", action = "close" })`,
		`hl.gesture({ fingers = 4, direction = "down", action = "fullscreen", mode = "maximize" })`,
	}
	got := strings.Split(strings.TrimRight(genGesture(o), "\n"), "\n")
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), genGesture(o))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got: %s\nwant: %s", i, got[i], want[i])
		}
	}
}

func TestThreeFingerAxisDoesNotDropFourFinger(t *testing.T) {
	o := defaultOverrides()
	o.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 3, Direction: "vertical", Action: "workspace"},
		{Enabled: true, Fingers: 3, Direction: "up", Action: "float", Mode: "toggle"},
		{Enabled: true, Fingers: 4, Direction: "up", Action: "fullscreen", Mode: "maximize"},
		{Enabled: true, Fingers: 4, Direction: "left", Action: "special", WorkspaceName: "scratch"},
		{Enabled: true, Fingers: 3, Direction: "horizontal", Action: "workspace"},
	}
	got := genGesture(o)
	if strings.Contains(got, `direction = "up", action = "float"`) {
		t.Errorf("3-finger up should be dropped behind vertical:\n%s", got)
	}
	for _, want := range []string{
		`fingers = 4, direction = "up", action = "fullscreen"`,
		`fingers = 4, direction = "left", action = "special"`,
		`fingers = 3, direction = "horizontal", action = "workspace"`,
		`fingers = 3, direction = "vertical", action = "workspace"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s\n%s", want, got)
		}
	}
}

func TestGenGestureLiveOmitsZoomLevel(t *testing.T) {
	o := defaultOverrides()
	o.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 3, Direction: "pinch", Action: "cursor_zoom", ZoomLevel: "1.5", Mode: "live"},
	}
	want := "hl.gesture({ fingers = 3, direction = \"pinch\", action = \"cursor_zoom\", mode = \"live\" })\n"
	if got := genGesture(o); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestGenGestureLiveReplacesRegisteredGesture(t *testing.T) {
	o := defaultOverrides()
	o.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 3, Direction: "pinch", Action: "cursor_zoom", Mode: "toggle", ZoomLevel: "1.5"},
	}
	got := previewGestures(o, nil)
	cleared := strings.Index(got, `pcall(function() hl.gesture({ fingers = 3, direction = "pinch", action = "unset" }) end)`)
	added := strings.Index(got, `action = "cursor_zoom", zoom_level = "1.5", mode = "toggle"`)
	if cleared < 0 || added < 0 || cleared > added {
		t.Fatalf("preview must clear the pinch before adding the replacement:\n%s", got)
	}
	if strings.Contains(got, `direction = "left"`) {
		t.Fatalf("preview cleared a trigger it is not adding:\n%s", got)
	}
}

func TestGenGestureLiveClearsDisabledAndRetargetedRows(t *testing.T) {
	prev := defaultOverrides()
	prev.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 3, Direction: "down", Action: "close"},
		{Enabled: true, Fingers: 3, Direction: "up", Action: "float", Mode: "toggle"},
	}
	o := defaultOverrides()
	o.Input.Gestures = []Gesture{
		{Enabled: false, Fingers: 3, Direction: "down", Action: "close"},
		{Enabled: true, Fingers: 4, Direction: "up", Action: "fullscreen", Mode: "maximize"},
	}
	got := previewGestures(o, hubGestureKeys(prev))
	for _, want := range []string{
		`fingers = 3, direction = "down", action = "unset"`,
		`fingers = 3, direction = "up", action = "unset"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("preview left a hub trigger registered, missing %s\n%s", want, got)
		}
	}
	if strings.Contains(got, `direction = "left"`) || strings.Contains(got, `fingers = 4, direction = "down"`) {
		t.Errorf("preview cleared a trigger the hub never registered:\n%s", got)
	}
	if !strings.Contains(got, `fingers = 4, direction = "up", action = "fullscreen", mode = "maximize"`) {
		t.Errorf("preview did not add the remaining gesture:\n%s", got)
	}
	saved := genGesture(o)
	if strings.Contains(saved, "unset") || strings.Contains(saved, `action = "close"`) {
		t.Errorf("saved config must only add the enabled row:\n%s", saved)
	}
}

func TestGenGestureLiveClearsWhenNothingIsConfigured(t *testing.T) {
	prev := defaultOverrides()
	prev.Input.WorkspaceSwipe = true
	got := previewGestures(defaultOverrides(), hubGestureKeys(prev))
	if !strings.Contains(got, `fingers = 3, direction = "horizontal", action = "unset"`) {
		t.Fatalf("an empty draft must clear a swipe the hub had registered:\n%s", got)
	}
	if strings.Contains(got, `direction = "left"`) || strings.Contains(got, `action = "workspace"`) || strings.Contains(got, `action = "close"`) {
		t.Fatalf("an empty draft must not add a gesture or clear one the hub never owned:\n%s", got)
	}
}

func TestGenGestureLiveLeavesCustomGesture(t *testing.T) {
	prev := defaultOverrides()
	prev.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 3, Direction: "horizontal", Action: "workspace"},
	}
	got := previewGestures(prev, hubGestureKeys(prev))
	if strings.Contains(got, `direction = "left"`) || strings.Contains(got, `direction = "pinch"`) {
		t.Fatalf("a preview of the same list cleared a trigger user.lua may own:\n%s", got)
	}
	if !strings.Contains(got, `direction = "horizontal", action = "unset"`) {
		t.Fatalf("preview must refresh the hub's own swipe:\n%s", got)
	}
	if !strings.Contains(got, `direction = "horizontal", action = "workspace"`) {
		t.Fatalf("preview dropped the hub swipe:\n%s", got)
	}
}

func TestGenGestureSkipsInvalidZoom(t *testing.T) {
	o := defaultOverrides()
	o.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 3, Direction: "pinch", Action: "cursor_zoom", Mode: "mult", ZoomLevel: "0"},
		{Enabled: true, Fingers: 3, Direction: "pinchin", Action: "cursor_zoom", Mode: "toggle", ZoomLevel: "nope"},
		{Enabled: true, Fingers: 3, Direction: "pinchout", Action: "cursor_zoom", Mode: "toggle", ZoomLevel: "1"},
		{Enabled: true, Fingers: 4, Direction: "pinch", Action: "cursor_zoom", Mode: "mult", ZoomLevel: "0.5"},
	}
	got := genGesture(o)
	if strings.Contains(got, `fingers = 3`) {
		t.Fatalf("invalid zoom must not be written:\n%s", got)
	}
	if !strings.Contains(got, `fingers = 4, direction = "pinch", action = "cursor_zoom", zoom_level = "0.5", mode = "mult"`) {
		t.Fatalf("a usable zoom level must still be written:\n%s", got)
	}
}

func TestGenGestureLiveReadsHubHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	ryoku := filepath.Join(dir, "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	saved := `{"desktop":{"input":{"gestures":[{"enabled":true,"fingers":3,"direction":"down","action":"close"}]}}}`
	if err := os.WriteFile(filepath.Join(ryoku, "desktop.json"), []byte(saved), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := `{"pid":1,"draft":{"desktop":{"input":{"gestures":[{"enabled":true,"fingers":4,"direction":"up","action":"fullscreen","mode":"maximize"}]}}}}`
	if err := os.WriteFile(filepath.Join(ryoku, ".desktop-preview.json"), []byte(marker), 0o600); err != nil {
		t.Fatal(err)
	}
	o := defaultOverrides()
	o.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 4, Direction: "up", Action: "fullscreen", Mode: "maximize"},
	}
	got := genGestureLive(o)
	for _, want := range []string{
		`fingers = 4, direction = "up", action = "unset"`,
		`fingers = 3, direction = "down", action = "unset"`,
		`fingers = 4, direction = "up", action = "fullscreen", mode = "maximize"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s\n%s", want, got)
		}
	}
	if strings.Contains(got, `direction = "left"`) {
		t.Fatalf("history cleared a trigger neither the saved store nor the preview owned:\n%s", got)
	}
}

func TestGenGestureQuotesWorkspaceNameForLua(t *testing.T) {
	o := defaultOverrides()
	o.Input.Gestures = []Gesture{
		{Enabled: true, Fingers: 4, Direction: "left", Action: "special", WorkspaceName: "a\"b\\c\n\x01"},
	}
	want := "hl.gesture({ fingers = 4, direction = \"left\", action = \"special\", workspace_name = \"a\\\"b\\\\c\\n\\001\" })\n"
	if got := genGesture(o); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
