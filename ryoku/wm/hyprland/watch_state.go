package main

import (
	"reflect"
	"strings"

	wm "ryoku-wm"
)

type refreshMask uint8

const (
	refreshOutputs refreshMask = 1 << iota
	refreshWorkspaces
	refreshWindows
	refreshKeyboard
	refreshAll = refreshOutputs | refreshWorkspaces | refreshWindows | refreshKeyboard
)

type watchState struct {
	monitors []hyprMonitor
	windows  []wm.Window
	last     map[wm.FrameKind]wm.Frame
	emit     func(wm.Frame)
	wants    func(wm.FrameKind) bool
}

func newWatchState(emit func(wm.Frame), wants func(wm.FrameKind) bool) *watchState {
	return &watchState{emit: emit, wants: wants, last: make(map[wm.FrameKind]wm.Frame)}
}

func (s *watchState) publish(f wm.Frame) {
	if !s.wants(f.Kind) {
		return
	}
	if old, ok := s.last[f.Kind]; ok && reflect.DeepEqual(old, f) {
		return
	}
	s.last[f.Kind] = f
	s.emit(f)
}

func (s *watchState) refresh(dirty refreshMask) {
	if dirty&(refreshOutputs|refreshWorkspaces|refreshWindows) != 0 &&
		(s.monitors == nil || dirty&refreshOutputs != 0) {
		mons, err := readMonitors()
		if err != nil {
			return
		}
		s.monitors = mons
	}
	if dirty&refreshOutputs != 0 {
		s.publish(wm.Frame{Kind: wm.FrameOutputs, Outputs: monitorOutputs(s.monitors, false)})
		s.publish(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: focusedName(s.monitors)})
	}
	if dirty&refreshWorkspaces != 0 && s.wants(wm.FrameWorkspaces) {
		if ws := readWorkspaces(s.monitors); ws != nil {
			s.publish(wm.Frame{Kind: wm.FrameWorkspaces, Workspaces: ws})
		}
	}
	if dirty&refreshWindows != 0 && s.wants(wm.FrameWindows) {
		if wins := readWindows(s.monitors); wins != nil {
			s.windows = wins
			s.publish(wm.Frame{Kind: wm.FrameWindows, Windows: wins})
		}
	}
	if dirty&refreshKeyboard != 0 && s.wants(wm.FrameKeyboard) {
		if active, all := readKeyboard(); active != "" {
			s.publish(wm.Frame{Kind: wm.FrameKeyboard, KeyboardLayout: active, KeyboardLayouts: all})
		}
	}
}

func (s *watchState) event(line string) refreshMask {
	name, data, ok := strings.Cut(line, ">>")
	if !ok {
		return 0
	}
	switch name {
	case "activewindow":
		// v1 has no address; v2 supplies the unambiguous window identity.
		return 0
	case "activewindowv2":
		if !s.wants(wm.FrameWindows) || s.focusWindow(data) {
			return 0
		}
		// A newly opened window may arrive before the debounced client snapshot.
		return refreshWindows
	case "windowtitle", "windowtitlev2", "changefloatingmode", "urgent":
		return refreshWindows
	case "openwindow", "closewindow", "movewindow", "movewindowv2", "fullscreen":
		return refreshWindows | refreshWorkspaces
	case "workspace", "workspacev2", "focusedmon", "focusedmonv2",
		"createworkspace", "createworkspacev2", "destroyworkspace", "destroyworkspacev2",
		"moveworkspace", "moveworkspacev2", "renameworkspace", "activespecial", "activespecialv2":
		return refreshOutputs | refreshWorkspaces | refreshWindows
	case "monitoradded", "monitoraddedv2", "monitorremoved", "configreloaded":
		return refreshAll
	case "activelayout":
		return refreshKeyboard
	}
	return 0
}

// Promote the known address without re-querying the compositor. Copy the slice:
// frames already handed to consumers must remain immutable.
func (s *watchState) focusWindow(address string) bool {
	address = strings.TrimSpace(address)
	if address != "" && !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}
	at := -1
	for i, w := range s.windows {
		if w.ID == address {
			at = i
			break
		}
	}
	if address != "" && at < 0 {
		return false
	}
	wins := append([]wm.Window(nil), s.windows...)
	if at < 0 {
		focused := false
		for _, w := range wins {
			focused = focused || w.FocusOrder == 0
		}
		if !focused {
			return true
		}
		for i := range wins {
			if wins[i].FocusOrder >= 0 {
				wins[i].FocusOrder++
			}
		}
	} else {
		old := wins[at].FocusOrder
		if old == 0 {
			return true
		}
		for i := range wins {
			if i != at && wins[i].FocusOrder >= 0 && (old < 0 || wins[i].FocusOrder < old) {
				wins[i].FocusOrder++
			}
		}
		wins[at].FocusOrder = 0
	}
	s.windows = wins
	s.publish(wm.Frame{Kind: wm.FrameWindows, Windows: wins})
	return true
}
