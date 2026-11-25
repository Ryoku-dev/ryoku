package main

import (
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
	got := genGestureLive(o)
	if !strings.Contains(got, `pcall(function() hl.gesture({ fingers = 3, direction = "pinch", action = "unset" }) end)`) {
		t.Errorf("preview did not unset the previous pinch:\n%s", got)
	}
	if !strings.Contains(got, `action = "cursor_zoom", zoom_level = "1.5", mode = "toggle"`) {
		t.Errorf("preview did not add the replacement:\n%s", got)
	}
}
