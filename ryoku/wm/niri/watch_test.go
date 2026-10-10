package main

import (
	"encoding/json"
	"testing"

	wm "ryoku-wm"
)

// foldEvents runs one or more niri events through the session fold and returns
// every frame the stream would emit for them, so the wiring between an event
// and the frame a consumer sees is pinned without a compositor or a socket.
func foldEvents(t *testing.T, s *session, wants func(wm.FrameKind) bool, events ...map[string]any) []wm.Frame {
	t.Helper()
	var out []wm.Frame
	emit := func(f wm.Frame) { out = append(out, f) }
	for _, ev := range events {
		for name, body := range ev {
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			s.apply(name, raw, emit, wants)
		}
	}
	return out
}

func allKinds(wm.FrameKind) bool { return true }

// The overview state is what gates the backdrop's blur pass, so the fold must
// carry is_open straight onto a FrameOverview and keep it on the session for a
// later connect replay.
func TestFoldOverviewEmitsFrame(t *testing.T) {
	s := &session{}
	frames := foldEvents(t, s, allKinds,
		map[string]any{"OverviewOpenedOrClosed": map[string]any{"is_open": true}},
		map[string]any{"OverviewOpenedOrClosed": map[string]any{"is_open": false}},
	)
	if len(frames) != 2 {
		t.Fatalf("got %d frames, want 2", len(frames))
	}
	if frames[0].Kind != wm.FrameOverview || !frames[0].OverviewOpen {
		t.Errorf("open frame = %+v, want overview/open", frames[0])
	}
	if frames[1].Kind != wm.FrameOverview || frames[1].OverviewOpen {
		t.Errorf("close frame = %+v, want overview/closed", frames[1])
	}
	if s.overview {
		t.Error("session must hold the latest state (closed)")
	}
}

// A narrowed watch that does not ask for the overview kind must not be sent it,
// the same skip every other kind honours.
func TestFoldOverviewSkippedWhenNotWanted(t *testing.T) {
	s := &session{}
	wants := func(k wm.FrameKind) bool { return k == wm.FrameWindows }
	frames := foldEvents(t, s, wants,
		map[string]any{"OverviewOpenedOrClosed": map[string]any{"is_open": true}},
	)
	if len(frames) != 0 {
		t.Fatalf("got %d frames, want none for a watch that skips overview", len(frames))
	}
	if !s.overview {
		t.Error("the session still tracks overview state even when the frame is skipped")
	}
}

func TestWindowEventCarriesProcessID(t *testing.T) {
	s := &session{}
	frames := foldEvents(t, s, allKinds, map[string]any{
		"WindowOpenedOrChanged": map[string]any{
			"window": map[string]any{
				"id":    7,
				"pid":   4242,
				"title": "Terminal",
			},
		},
	})
	if len(frames) == 0 || len(frames[0].Windows) != 1 || frames[0].Windows[0].Pid != 4242 {
		t.Fatalf("window frames = %+v, want pid 4242", frames)
	}
}

// Floating geometry is the persistence source after a native move/resize. niri
// reports it output-local, while wm.Window is globally logical, so the output
// origin is added and a layout delta must publish a fresh window frame.
func TestFloatingLayoutChangePublishesGlobalGeometry(t *testing.T) {
	pos := [2]float64{10, 20}
	s := &session{
		outputs:    []wm.Output{{Name: "DP-2", X: 1600, Y: -40}},
		workspaces: []niriWorkspace{{ID: 3, Output: "DP-2"}},
		windows: []niriWindow{{
			ID:          7,
			WorkspaceID: 3,
			IsFloating:  true,
			Layout: niriWindowLayout{
				TileSize:               [2]float64{400, 300},
				WindowSize:             [2]int{392, 292},
				TilePosInWorkspaceView: &pos,
			},
		}},
	}

	frames := foldEvents(t, s, allKinds, map[string]any{
		"WindowLayoutsChanged": map[string]any{
			"changes": []any{[]any{7, map[string]any{
				"tile_size":                  []float64{701.6, 521.6},
				"window_size":                []int{694, 514},
				"tile_pos_in_workspace_view": []float64{100.4, 180.6},
				"window_offset_in_tile":      []float64{4, 4},
			}}},
		},
	})
	if len(frames) != 1 || frames[0].Kind != wm.FrameWindows || len(frames[0].Windows) != 1 {
		t.Fatalf("layout delta frames = %+v, want one window frame", frames)
	}
	got := frames[0].Windows[0]
	if got.X != 1704 || got.Y != 145 || got.Width != 694 || got.Height != 514 {
		t.Errorf("floating client geometry = (%d,%d %dx%d), want (1704,145 694x514)", got.X, got.Y, got.Width, got.Height)
	}
}
