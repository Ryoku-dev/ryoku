package main

import (
	"encoding/json"
	"testing"

	wm "ryoku-wm"
)

// watch_test pins the fold: the merge of mango's per-target push streams into
// neutral frames. The live proof (a nested session, real pushes) ran once by
// hand; these are the deterministic twins the CI gate runs.

func newTestSession(wanted ...wm.FrameKind) (*session, *[]wm.Frame) {
	set := map[wm.FrameKind]bool{}
	for _, k := range wanted {
		set[k] = true
	}
	var frames []wm.Frame
	s := &session{
		firstSeen: map[wm.FrameKind]bool{},
		known:     map[int]bool{},
		wants:     func(k wm.FrameKind) bool { return len(set) == 0 || set[k] },
		emit:      func(f wm.Frame) { frames = append(frames, f) },
	}
	return s, &frames
}

func monitorsLine(t *testing.T, monitors ...string) []byte {
	t.Helper()
	env := monitorsEnvelope{}
	for _, m := range monitors {
		env.Monitors = append(env.Monitors, mangoMonitor{
			Name: m, Active: m == "HEADLESS-1", Width: 1280, Height: 720, Scale: 1,
			Tags: []mangoTag{
				{Index: 1, IsActive: true, Layout: "T", ClientCount: 2},
				{Index: 2, Layout: "S", ClientCount: 0},
			},
		})
	}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func clientsLine(t *testing.T, clients ...mangoClient) []byte {
	t.Helper()
	b, err := json.Marshal(clientsEnvelope{Clients: clients})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func client(id int, focused bool, tags ...int) mangoClient {
	return mangoClient{ID: id, Title: "w", AppID: "kitty", Monitor: "HEADLESS-1",
		Tags: tags, IsFocused: focused, IsVisible: true, X: 16, Y: 16, Width: 600, Height: 400}
}

// One monitors push carries outputs, workspaces and focus: the tag rows embed
// the active state, so one stream refreshes all three frames.
func TestFoldMonitorsEmitsThreeFrames(t *testing.T) {
	s, frames := newTestSession(wm.FrameOutputs, wm.FrameWorkspaces, wm.FrameFocus)
	s.fold(rawLine{kind: wm.FrameOutputs, line: monitorsLine(t, "HEADLESS-1")})
	kinds := []wm.FrameKind{}
	for _, f := range *frames {
		kinds = append(kinds, f.Kind)
	}
	// The monitors stream is the only one wanted, so ready follows its first
	// push in the same fold.
	if len(kinds) != 4 || kinds[0] != wm.FrameOutputs || kinds[1] != wm.FrameWorkspaces ||
		kinds[2] != wm.FrameFocus || kinds[3] != wm.FrameReady {
		t.Fatalf("frames = %v, want outputs, workspaces, focus, ready", kinds)
	}
	out := (*frames)[0].Outputs
	if len(out) != 1 || out[0].Name != "HEADLESS-1" || !out[0].Focused {
		t.Errorf("outputs wrong: %+v", out)
	}
	ws := (*frames)[1].Workspaces
	// Two numbered tags plus the per-monitor special entry (tag 0 is a real
	// workspace the bar can show, and no client sits on it in this fixture).
	if len(ws) != 3 {
		t.Fatalf("want both tags plus the special entry, got %d", len(ws))
	}
	if ws[2].ID != "0" || !ws[2].Special || ws[2].Active || ws[2].Windows != 0 {
		t.Errorf("special entry wrong: %+v", ws[2])
	}
	if ws[0].ID != "1" || !ws[0].Active || ws[0].Windows != 2 {
		t.Errorf("tag 1 wrong: %+v", ws[0])
	}
	// The layout symbol resolves through the cached layout table.
	s.layouts = map[string]string{"T": "tile", "S": "scroller"}
	s.fold(rawLine{kind: wm.FrameOutputs, line: monitorsLine(t, "HEADLESS-1")})
	ws = (*frames)[5].Workspaces
	if ws[0].Layout != "tile" || ws[1].Layout != "scroller" {
		t.Errorf("layouts not resolved: %+v", ws)
	}
	if (*frames)[2].FocusedOutput != "HEADLESS-1" {
		t.Errorf("focus frame wrong: %+v", (*frames)[2])
	}
}

// Readiness is the last of the firsts across the wanted streams, never a
// single server marker.
func TestFoldReadyAfterLastFirst(t *testing.T) {
	s, frames := newTestSession(wm.FrameOutputs, wm.FrameWindows, wm.FrameKeyboard)
	s.fold(rawLine{kind: wm.FrameOutputs, line: monitorsLine(t, "HEADLESS-1")})
	s.fold(rawLine{kind: wm.FrameWindows, line: clientsLine(t, client(1, true, 1))})
	for _, f := range *frames {
		if f.Kind == wm.FrameReady {
			t.Fatal("ready fired before the keyboard stream delivered")
		}
	}
	s.fold(rawLine{kind: wm.FrameKeyboard, line: []byte(`{"layout":"English (US)"}`)})
	ready := false
	for _, f := range *frames {
		if f.Kind == wm.FrameReady {
			ready = true
		}
	}
	if !ready {
		t.Fatal("ready never fired after every wanted stream delivered")
	}
	// Exactly once.
	n := 0
	s.fold(rawLine{kind: wm.FrameKeyboard, line: []byte(`{"layout":"German"}`)})
	for _, f := range *frames {
		if f.Kind == wm.FrameReady {
			n++
		}
	}
	if n != 1 {
		t.Errorf("ready fired %d times, want 1", n)
	}
}

// Focus history is folded locally (mango has no order query): most recently
// focused first, and a closed window leaves so a recycled id never sits in
// history.
func TestFoldFocusHistory(t *testing.T) {
	s, _ := newTestSession(wm.FrameWindows)
	s.fold(rawLine{kind: wm.FrameWindows, line: clientsLine(t, client(1, true, 1), client(2, false, 1))})
	if len(s.history) != 1 || s.history[0] != 1 {
		t.Fatalf("history = %v, want [1]", s.history)
	}
	s.fold(rawLine{kind: wm.FrameWindows, line: clientsLine(t, client(1, false, 1), client(2, true, 1))})
	if len(s.history) != 2 || s.history[0] != 2 || s.history[1] != 1 {
		t.Fatalf("history = %v, want [2 1]", s.history)
	}
	// Window 2 closes: it leaves the history even though 1 is focused again.
	s.fold(rawLine{kind: wm.FrameWindows, line: clientsLine(t, client(1, true, 1))})
	if len(s.history) != 1 || s.history[0] != 1 {
		t.Fatalf("history = %v after close, want [1]", s.history)
	}
	// The windows frame carries the focusOrder the bar reads.
	s2, frames := newTestSession(wm.FrameWindows)
	s2.fold(rawLine{kind: wm.FrameWindows, line: clientsLine(t, client(7, true, 1), client(8, false, 1))})
	s2.fold(rawLine{kind: wm.FrameWindows, line: clientsLine(t, client(7, false, 1), client(8, true, 1))})
	last := (*frames)[len(*frames)-1]
	if len(last.Windows) != 2 || last.Windows[1].FocusOrder != 0 || last.Windows[0].FocusOrder != 1 {
		t.Errorf("focusOrder wrong: %+v", last.Windows)
	}
	// Geometry rides the frame: CapWindowGeometry is claimed.
	if last.Windows[0].X == 0 && last.Windows[0].Width == 0 {
		t.Error("window geometry missing")
	}
}

// A watch that wants only one stream still reaches ready on that stream's
// first push alone.
func TestFoldReadySingleStream(t *testing.T) {
	s, frames := newTestSession(wm.FrameWindows)
	s.fold(rawLine{kind: wm.FrameWindows, line: clientsLine(t, client(1, true, 1))})
	found := false
	for _, f := range *frames {
		if f.Kind == wm.FrameReady {
			found = true
		}
	}
	if !found {
		t.Error("single-stream watch never reached ready")
	}
}

// streamsFor names the minimum set of connections a want list needs: the
// monitors stream carries outputs, workspaces and focus together.
func TestStreamsFor(t *testing.T) {
	any := func(wm.FrameKind) bool { return true }
	if got := len(streamsFor(any)); got != 3 {
		t.Errorf("full want opens %d streams, want 3 (monitors, clients, keyboard)", got)
	}
	onlyFocus := func(k wm.FrameKind) bool { return k == wm.FrameFocus }
	if got := streamsFor(onlyFocus); len(got) != 1 || got[0].kind != wm.FrameOutputs {
		t.Errorf("focus alone should ride the monitors stream, got %+v", got)
	}
	onlyWindows := func(k wm.FrameKind) bool { return k == wm.FrameWindows }
	if got := streamsFor(onlyWindows); len(got) != 1 || got[0].cmd != "watch all-clients" {
		t.Errorf("windows alone should open the clients stream, got %+v", got)
	}
}
