package main

import (
	"encoding/json"
	"testing"

	wm "ryoku-wm"
)

// stubRequest pins the dialect: the readers must accept the replies wayfire
// actually sends, frame for frame, without a compositor.
func stubRequest(t *testing.T, replies map[string]string) {
	t.Helper()
	original := request
	request = func(method string, data any) (json.RawMessage, error) {
		body, ok := replies[method]
		if !ok {
			t.Errorf("unexpected query %q", method)
			return nil, nil
		}
		return json.RawMessage(body), nil
	}
	t.Cleanup(func() { request = original })
}

func stageReplies() map[string]string {
	return map[string]string{
		"window-rules/list-outputs": `[{"id":1,"name":"HEADLESS-1",
			"geometry":{"x":0,"y":0,"width":1280,"height":720},
			"workarea":{"x":0,"y":0,"width":1280,"height":720},
			"wset-index":1,
			"workspace":{"x":1,"y":0,"grid_width":3,"grid_height":3}}]`,
		"window-rules/list-wsets": `[{"index":1,"name":"2","output-id":1,"output-name":"HEADLESS-1",
			"workspace":{"x":1,"y":0,"grid_width":3,"grid_height":3}}]`,
		"window-rules/list-views": `[{"id":7,"pid":100,"title":"term","app-id":"kitty",
			"base-geometry":{"x":177,"y":34,"width":926,"height":682},
			"geometry":{"x":-1107,"y":0,"width":934,"height":720},
			"bbox":{"x":-1107,"y":0,"width":934,"height":720},
			"output-id":1,"output-name":"HEADLESS-1",
			"last-focus-timestamp":1241198347800739,"mapped":true,
			"tiled-edges":0,"fullscreen":false,"minimized":false,
			"activated":false,"sticky":false,"wset-index":1}]`,
		"window-rules/get-focused-output": `{"result":"ok","info":{"id":1,"name":"HEADLESS-1"}}`,
		"window-rules/get-focused-view": `{"result":"ok","info":{"id":7,"pid":100,"title":"term","app-id":"kitty",
			"base-geometry":{"x":177,"y":34,"width":926,"height":682},
			"geometry":{"x":-1107,"y":0,"width":934,"height":720},
			"bbox":{"x":-1107,"y":0,"width":934,"height":720},
			"output-id":1,"output-name":"HEADLESS-1",
			"last-focus-timestamp":1241198347800739,"mapped":true,
			"tiled-edges":0,"fullscreen":false,"minimized":false,
			"activated":false,"sticky":false,"wset-index":1}}`,
		"wayfire/get-keyboard-state":  `{"possible-layouts":["us","br"],"layout":"us"}`,
		"wayfire/list-config-options": `{"result":"ok","options":{"output:HEADLESS-1":{"scale":{"value":"2.000000","default":"1.000000"},"mode":{"value":"auto","default":"auto"},"transform":{"value":"0","default":"0"}}}}`,
	}
}

func TestReplayReadsFixture(t *testing.T) {
	stubRequest(t, stageReplies())
	s, err := readStage()
	if err != nil {
		t.Fatalf("readStage: %v", err)
	}

	outs := s.outputsFrame()
	if len(outs) != 1 || !outs[0].Focused || outs[0].ActiveWorkspace != "2" {
		t.Fatalf("output frame wrong: %+v", outs)
	}
	if outs[0].Scale != 2 {
		t.Errorf("scale from config = %v, want 2", outs[0].Scale)
	}
	if outs[0].Mode != "" {
		t.Errorf("auto mode reported as %q, want empty", outs[0].Mode)
	}

	ws := s.workspaceFrame()
	if len(ws) != 9 {
		t.Fatalf("got %d cells, want 9", len(ws))
	}

	wins := s.windowFrame()
	if len(wins) != 1 {
		t.Fatalf("got %d windows, want 1", len(wins))
	}
	// The view is translated to x=-1107 while its set shows cell (1,0), so it
	// belongs to the cell it started on, flat name 1.
	if wins[0].Workspace != "1" {
		t.Errorf("workspace = %q, want 1", wins[0].Workspace)
	}
	if wins[0].FocusOrder != 0 {
		t.Errorf("focus order = %d, want 0", wins[0].FocusOrder)
	}
}

func allFrames(wm.FrameKind) bool { return true }

func collect() (func(wm.Frame), *[]wm.Frame) {
	var frames []wm.Frame
	return func(f wm.Frame) { frames = append(frames, f) }, &frames
}

// wayfire names only the newly focused view, so the fold retires the previous
// activation itself: two activated views would rank both at order 0.
func TestViewFocusedClearsOthers(t *testing.T) {
	s := &stage{views: []wayfireView{
		{ID: 1, Activated: true, LastFocusTimestamp: 500},
		{ID: 2, LastFocusTimestamp: 100},
	}}
	frame := json.RawMessage(`{"event":"view-focused","view":{"id":2,"activated":true,"last-focus-timestamp":900}}`)
	emit, got := collect()
	s.apply("view-focused", frame, emit, allFrames)

	wins := s.windowFrame()
	if wins[0].ID != "2" || wins[0].FocusOrder != 0 {
		t.Errorf("focused window is %s at %d, want 2 at 0", wins[0].ID, wins[0].FocusOrder)
	}
	if s.views[0].Activated {
		t.Error("previous view still activated")
	}
	if len(*got) != 1 || (*got)[0].Kind != wm.FrameWindows {
		t.Errorf("emitted %v, want one windows frame", *got)
	}
}

func TestUnmappedRemovesAndCountsFollow(t *testing.T) {
	s := &stage{
		outputs: []wayfireOutput{{
			ID: 1, Name: "HEADLESS-1",
			Geometry:  wayfireGeometry{Width: 1280, Height: 720},
			WsetIndex: 1,
			Workspace: wayfireWorkspace{X: 0, Y: 0, GridWidth: 3, GridHeight: 3},
		}},
		wsets: []wayfireWset{{
			Index: 1, OutputName: "HEADLESS-1",
			Workspace: wayfireWorkspace{X: 0, Y: 0, GridWidth: 3, GridHeight: 3},
		}},
		views: []wayfireView{
			{ID: 1, OutputName: "HEADLESS-1", WsetIndex: 1,
				Geometry: wayfireGeometry{X: 10, Y: 10, Width: 100, Height: 100}},
			{ID: 2, OutputName: "HEADLESS-1", WsetIndex: 1,
				Geometry: wayfireGeometry{X: 20, Y: 20, Width: 100, Height: 100}},
		},
	}
	frame := json.RawMessage(`{"event":"view-unmapped","view":{"id":1}}`)
	emit, _ := collect()
	s.apply("view-unmapped", frame, emit, allFrames)

	if len(s.views) != 1 || s.views[0].ID != 2 {
		t.Fatalf("views after unmapped: %+v", s.views)
	}
	ws := s.workspaceFrame()
	if ws[0].Windows != 1 {
		t.Errorf("occupancy = %d, want 1 after the close", ws[0].Windows)
	}
}

// A layout or navigation event invalidates every held position, so the fold
// reads the whole picture again; a stub proves the refresh swaps the stage in
// place instead of leaving two disagreeing copies.
func TestRefreshReplacesStage(t *testing.T) {
	stubRequest(t, stageReplies())
	s := &stage{views: []wayfireView{{ID: 999}}}
	emit, got := collect()
	s.apply("wset-workspace-changed", json.RawMessage(`{"event":"wset-workspace-changed"}`), emit, allFrames)
	if len(s.views) != 1 || s.views[0].ID != 7 {
		t.Errorf("stage not replaced: %+v", s.views)
	}
	if len(*got) != 4 {
		t.Errorf("refresh emitted %d frames, want outputs+workspaces+windows+focus", len(*got))
	}
}
