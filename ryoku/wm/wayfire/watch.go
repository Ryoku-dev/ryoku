package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"time"

	wm "ryoku-wm"
)

// wayfire's event stream: subscribe once on a dedicated socket, then fold
// events into the stage state and watch render. Calling
// window-rules/events/watch with an empty object subscribes to every builtin
// event, and the compositor pushes each as framed JSON ({"event": name, ...})
// on that same socket. There is no replay inside the protocol, so a connect
// reads the full state first; a reconnect starts over, because events missed
// while disconnected have no way back.

const reconnectBackoff = 500 * time.Millisecond

// keyboardPoll bounds how long the stream may sit silent. Wayfire has no
// keyboard-layout event: the layout only changes through an act or a
// compositor binding, so the stream asks after it whenever it wakes up and
// emits only on a change.
const keyboardPoll = 2 * time.Second

// runWatch streams every frame kind, or only the ones named in args. A
// narrowed watch skips the frames it does not need, which keeps a burst cheap
// for a consumer that only wants one kind.
func runWatch(args []string) error {
	want := map[wm.FrameKind]bool{}
	for _, a := range args {
		want[wm.FrameKind(a)] = true
	}
	wants := func(k wm.FrameKind) bool { return len(want) == 0 || want[k] }
	return streamWatch(wants)
}

func streamWatch(wants func(wm.FrameKind) bool) error {
	enc := json.NewEncoder(stdout)
	emit := func(f wm.Frame) {
		// Flush per frame: the consumer is a live shell, and an unflushed focus
		// frame is a bar that never updates.
		if enc.Encode(f) != nil || stdout.Flush() != nil {
			os.Exit(0)
		}
	}

	for {
		conn, err := dialWatch()
		if err != nil {
			time.Sleep(reconnectBackoff)
			continue
		}
		consume(conn, emit, wants)
		_ = conn.Close()
		time.Sleep(reconnectBackoff)
	}
}

// dialWatch subscribes before any query, so an event racing the replay is
// queued on the socket instead of falling between query and subscription. The
// acknowledgement is synchronous in the request handler, so it is always the
// first frame read here.
func dialWatch() (net.Conn, error) {
	path := socketPath()
	if path == "" {
		return nil, fmt.Errorf("watch: no socket")
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(ipcRequest{Method: "window-rules/events/watch", Data: map[string]any{}})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := writeFrame(conn, body); err != nil {
		_ = conn.Close()
		return nil, err
	}
	ack, err := readFrame(conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	var reply map[string]json.RawMessage
	if json.Unmarshal(ack, &reply) == nil {
		if raw, ok := reply["error"]; ok {
			var msg string
			if json.Unmarshal(raw, &msg) == nil && msg != "" {
				_ = conn.Close()
				return nil, fmt.Errorf("watch: %s", msg)
			}
		}
	}
	return conn, nil
}

// consume replays the full state, marks readiness, then folds events until
// the socket drops. Reads carry a deadline so the keyboard can be polled
// without a second goroutine or a channel the fold would race.
func consume(conn net.Conn, emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	s, err := readStage()
	if err != nil {
		return
	}
	s.emitReplay(emit, wants)
	emit(wm.Frame{Kind: wm.FrameReady})

	for {
		_ = conn.SetReadDeadline(time.Now().Add(keyboardPoll))
		frame, err := readFrame(conn)
		if err != nil {
			if isTimeout(err) {
				s.pollKeyboard(emit, wants)
				continue
			}
			return
		}
		var event struct {
			Event string `json:"event"`
		}
		if json.Unmarshal(frame, &event) != nil || event.Event == "" {
			continue
		}
		s.apply(event.Event, frame, emit, wants)
	}
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// emitReplay is the connect burst: outputs first (the rest name them), then
// workspaces, windows, keyboard and focus, so a consumer has every cross
// reference by the time it reads it.
func (s *stage) emitReplay(emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	if wants(wm.FrameOutputs) {
		emit(wm.Frame{Kind: wm.FrameOutputs, Outputs: s.outputsFrame()})
	}
	if wants(wm.FrameWorkspaces) {
		emit(wm.Frame{Kind: wm.FrameWorkspaces, Workspaces: s.workspaceFrame()})
	}
	if wants(wm.FrameWindows) {
		emit(wm.Frame{Kind: wm.FrameWindows, Windows: s.windowFrame()})
	}
	s.emitKeyboard(emit, wants)
	if wants(wm.FrameFocus) {
		emit(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: s.focused})
	}
}

func (s *stage) emitKeyboard(emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	if !wants(wm.FrameKeyboard) {
		return
	}
	current, all := s.keyboard.keyboardPair()
	emit(wm.Frame{Kind: wm.FrameKeyboard, KeyboardLayout: current, KeyboardLayouts: all})
}

// pollKeyboard re-reads the layout and emits only on a change, so the stream
// costs one small query per tick while idle and a frame only when the bar
// indicator would actually move.
func (s *stage) pollKeyboard(emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	if !wants(wm.FrameKeyboard) {
		return
	}
	k, err := readKeyboard()
	if err != nil || (k.Layout == s.keyboard.Layout && slices.Equal(k.PossibleLayouts, s.keyboard.PossibleLayouts)) {
		return
	}
	s.keyboard = k
	s.emitKeyboard(emit, wants)
}

// apply folds one event into the stage and emits what changed. Events are
// idempotent upserts keyed by view id, so an event queued behind the replay
// re-states what the query already knew instead of corrupting it.
func (s *stage) apply(name string, frame []byte, emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	switch name {
	case "view-unmapped":
		var e struct {
			View wayfireView `json:"view"`
		}
		if json.Unmarshal(frame, &e) == nil {
			s.removeView(e.View.ID)
			s.emitWindows(emit, wants)
			s.emitWorkspaces(emit, wants)
		}

	case "view-focused":
		// wayfire deactivates the previous view without naming it, so focus
		// moved here means everyone else lost it.
		var e struct {
			View wayfireView `json:"view"`
		}
		if json.Unmarshal(frame, &e) == nil {
			for i := range s.views {
				s.views[i].Activated = false
			}
			s.upsert(e.View)
			s.emitWindows(emit, wants)
		}

	case "view-mapped",
		"view-geometry-changed",
		"view-title-changed",
		"view-app-id-changed",
		"view-minimized",
		"view-fullscreen",
		"view-sticky",
		"view-always-on-top",
		"view-tiled",
		"view-set-output",
		"view-wset-changed",
		"view-workspace-changed":
		var e struct {
			View wayfireView `json:"view"`
		}
		if json.Unmarshal(frame, &e) == nil {
			s.upsert(e.View)
			s.emitWindows(emit, wants)
			if name == "view-mapped" || name == "view-set-output" ||
				name == "view-wset-changed" || name == "view-workspace-changed" {
				s.emitWorkspaces(emit, wants)
			}
		}

	case "output-gain-focus":
		if focused, err := readFocusedOutput(); err == nil && focused != s.focused {
			s.focused = focused
			if wants(wm.FrameFocus) {
				emit(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: focused})
			}
		}

	case "output-added",
		"output-removed",
		"output-layout-changed",
		"output-wset-changed",
		"wset-workspace-changed":
		// Navigation translates every view, and layout changes move outputs
		// under them: both invalidate positions held anywhere in the stage, so
		// the whole picture is read again rather than patched twice and left
		// disagreeing.
		if fresh, err := readStage(); err == nil {
			*s = *fresh
			if wants(wm.FrameOutputs) {
				emit(wm.Frame{Kind: wm.FrameOutputs, Outputs: s.outputsFrame()})
			}
			if wants(wm.FrameWorkspaces) {
				emit(wm.Frame{Kind: wm.FrameWorkspaces, Workspaces: s.workspaceFrame()})
			}
			if wants(wm.FrameWindows) {
				emit(wm.Frame{Kind: wm.FrameWindows, Windows: s.windowFrame()})
			}
			if wants(wm.FrameFocus) {
				emit(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: s.focused})
			}
		}
	}
}

func (s *stage) upsert(v wayfireView) {
	for i := range s.views {
		if s.views[i].ID == v.ID {
			s.views[i] = v
			return
		}
	}
	s.views = append(s.views, v)
}

func (s *stage) removeView(id int64) {
	for i := range s.views {
		if s.views[i].ID == id {
			s.views = append(s.views[:i], s.views[i+1:]...)
			return
		}
	}
}

func (s *stage) emitWindows(emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	if !wants(wm.FrameWindows) {
		return
	}
	emit(wm.Frame{Kind: wm.FrameWindows, Windows: s.windowFrame()})
}

func (s *stage) emitWorkspaces(emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	if !wants(wm.FrameWorkspaces) {
		return
	}
	emit(wm.Frame{Kind: wm.FrameWorkspaces, Workspaces: s.workspaceFrame()})
}
