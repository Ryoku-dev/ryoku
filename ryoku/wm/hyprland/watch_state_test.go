package main

import (
	"reflect"
	"testing"

	wm "ryoku-wm"
)

func TestFocusEventsDoNotQueryAndPreserveHistory(t *testing.T) {
	restore := stubCtl(t, func(...string) ([]byte, error) { t.Fatal("focus queried compositor"); return nil, nil })
	defer restore()
	var frames []wm.Frame
	s := newWatchState(func(f wm.Frame) { frames = append(frames, f) }, func(wm.FrameKind) bool { return true })
	s.windows = []wm.Window{{ID: "0xa", FocusOrder: 0}, {ID: "0xb", FocusOrder: 1}, {ID: "0xc", FocusOrder: 2}}
	s.publish(wm.Frame{Kind: wm.FrameWindows, Windows: s.windows})
	if s.event("activewindow>>kitty,a title, with commas") != 0 {
		t.Fatal("v1 refreshed")
	}
	if s.event("activewindowv2>>b") != 0 {
		t.Fatal("known focus refreshed")
	}
	if got := []int{s.windows[0].FocusOrder, s.windows[1].FocusOrder, s.windows[2].FocusOrder}; !reflect.DeepEqual(got, []int{1, 0, 2}) {
		t.Fatal(got)
	}
	if frames[0].Windows[0].FocusOrder != 0 {
		t.Fatal("mutated prior frame")
	}
	s.event("activewindowv2>>0xb")
	if len(frames) != 2 {
		t.Fatal("duplicate focus published")
	}
	s.event("activewindowv2>>")
	for _, w := range s.windows {
		if w.FocusOrder == 0 {
			t.Fatal("empty focus retained focused window")
		}
	}
	s.event("activewindowv2>>")
	if len(frames) != 3 {
		t.Fatal("duplicate empty focus published")
	}
	if s.event("activewindowv2>>dead") != refreshWindows {
		t.Fatal("unknown window needs refresh")
	}
}

func TestWindowOpenQueriesOnlyClientsAndWorkspaces(t *testing.T) {
	var calls []string
	restore := stubCtl(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args[0])
		switch args[0] {
		case "workspaces":
			return []byte(`[{"id":1,"name":"1","monitor":"DP-1","windows":1}]`), nil
		case "clients":
			return []byte(`[{"address":"0xa","class":"kitty","monitor":0,"workspace":{"id":1,"name":"1"},"focusHistoryID":0}]`), nil
		default:
			t.Fatalf("unexpected query %v", args)
			return nil, nil
		}
	})
	defer restore()
	var frames []wm.Frame
	s := newWatchState(func(f wm.Frame) { frames = append(frames, f) }, func(wm.FrameKind) bool { return true })
	s.monitors = []hyprMonitor{{ID: 0, Name: "DP-1"}}
	s.refresh(s.event("openwindow>>a,1,kitty,title") | s.event("activewindowv2>>a"))
	if !reflect.DeepEqual(calls, []string{"workspaces", "clients"}) {
		t.Fatal(calls)
	}
	if len(frames) != 2 || frames[1].Windows[0].Output != "DP-1" {
		t.Fatal(frames)
	}
	s.refresh(refreshWindows | refreshWorkspaces)
	if len(frames) != 2 {
		t.Fatal("unchanged state republished")
	}
}

func TestKeyboardAndTopologyRefresh(t *testing.T) {
	var calls []string
	restore := stubCtl(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args[0])
		if args[0] == "devices" {
			return []byte(`{"keyboards":[{"main":true,"active_keymap":"English (UK)","layout":"gb,us"}]}`), nil
		}
		return []byte(`[]`), nil
	})
	defer restore()
	var frames []wm.Frame
	s := newWatchState(func(f wm.Frame) { frames = append(frames, f) }, func(wm.FrameKind) bool { return true })
	s.refresh(s.event("activelayout>>keyboard,English (UK)"))
	if !reflect.DeepEqual(calls, []string{"devices"}) || len(frames) != 1 || frames[0].KeyboardLayout != "English (UK)" {
		t.Fatal(calls, frames)
	}
	for _, event := range []string{"monitoraddedv2>>1,DP-1,test", "monitorremoved>>DP-1", "configreloaded>>"} {
		if s.event(event) != refreshAll {
			t.Fatal(event)
		}
	}
	if s.event("activewindowgarbage>>abc") != 0 {
		t.Fatal("prefix collision")
	}
}

// Hyprland reports the transient "ERROR" active_keymap while a keyboard's xkb
// state is mid-transition (a virtual device flapping, as dictation does), and no
// further activelayout event follows once it settles. Publishing it would latch
// the bar on "ERROR" forever, so a refresh that resolves only to the sentinel
// must publish nothing and leave the last-good layout standing.
func TestKeyboardErrorSentinelIsNotPublished(t *testing.T) {
	restore := stubCtl(t, func(args ...string) ([]byte, error) {
		return []byte(`{"keyboards":[{"main":true,"active_keymap":"ERROR","layout":"us"}]}`), nil
	})
	defer restore()
	var frames []wm.Frame
	s := newWatchState(func(f wm.Frame) { frames = append(frames, f) }, func(wm.FrameKind) bool { return true })
	s.refresh(s.event("activelayout>>keyboard,ERROR"))
	if len(frames) != 0 {
		t.Fatalf("ERROR sentinel published: %+v", frames)
	}
}
