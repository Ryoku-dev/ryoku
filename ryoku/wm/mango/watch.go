package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	wm "ryoku-wm"
)

// watch merges mango's per-target push streams into one neutral frame stream.
//
// Unlike niri (one event channel carrying everything) mango answers one stream
// per watched target, each on its own connection: all-monitors pushes the whole
// monitor list (tags and client counts included), all-clients pushes the window
// list, keyboardlayout pushes the layout name. Each connection's FIRST line is
// its initial state, so readiness is the last of the firsts, and a reconnect
// re-pushes that stream's truth without a query.
//
// One reader goroutine per stream; every emit runs under one mutex so stdout
// gets exactly one frame per line from a single writer.

const reconnectBackoff = 500 * time.Millisecond

// streamSpec names one watched target and the frame kind its pushes fold into.
// Outputs are the exception that carries two kinds: the monitor reply embeds
// every tag's state, so one push refreshes outputs, workspaces and focus.
type streamSpec struct {
	kind wm.FrameKind
	cmd  string
}

func streamsFor(wants func(wm.FrameKind) bool) []streamSpec {
	var out []streamSpec
	if wants(wm.FrameOutputs) || wants(wm.FrameWorkspaces) || wants(wm.FrameFocus) {
		out = append(out, streamSpec{kind: wm.FrameOutputs, cmd: "watch all-monitors"})
	}
	if wants(wm.FrameWindows) {
		out = append(out, streamSpec{kind: wm.FrameWindows, cmd: "watch all-clients"})
	}
	if wants(wm.FrameKeyboard) {
		out = append(out, streamSpec{kind: wm.FrameKeyboard, cmd: "watch keyboardlayout"})
	}
	return out
}

type rawLine struct {
	kind wm.FrameKind
	line []byte
}

// session holds what the streams have told us so far, so each push becomes a
// full neutral frame without asking the compositor anything.
type session struct {
	mu        sync.Mutex
	monitors  []mangoMonitor
	clients   []mangoClient
	history   []int // window ids, most recently focused first
	layouts   map[string]string
	kbLayout  string
	kbSet     []string
	ready     bool
	firstSeen map[wm.FrameKind]bool
	wants     func(wm.FrameKind) bool
	emit      func(wm.Frame)
	// tamer resets a window an app opened maximised back to an ordinary tile.
	// It rides the windows stream, so a watch that skips windows skips it;
	// whether it acts is read from the store on each open.
	tamer *tamer
	// known holds the ids seen since the last push, so the next one can tell
	// a fresh open from an update of a window it already holds.
	known map[int]bool
}

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
	s := &session{
		firstSeen: map[wm.FrameKind]bool{},
		known:     map[int]bool{},
		wants:     wants,
		emit: func(f wm.Frame) {
			// Flush per frame: the consumer is a live shell, and an unflushed
			// focus frame is a bar that never updates.
			if enc.Encode(f) != nil || stdout.Flush() != nil {
				os.Exit(0)
			}
		},
	}
	if wants(wm.FrameWindows) {
		s.tamer = newTamer()
	}

	lines := make(chan rawLine)
	for _, spec := range streamsFor(wants) {
		go func(spec streamSpec) {
			for {
				conn, _, err := open(spec.cmd)
				if err != nil {
					time.Sleep(reconnectBackoff)
					continue
				}
				readStream(conn, spec, lines)
				_ = conn.Close()
				time.Sleep(reconnectBackoff)
			}
		}(spec)
	}

	for raw := range lines {
		s.fold(raw)
	}
	return nil
}

// readStream pumps one connection's pushes into the merge channel until the
// socket drops. It never closes the shared channel: a stream reconnecting is
// normal, and the merge must live as long as the session.
func readStream(conn net.Conn, spec streamSpec, out chan<- rawLine) {
	scan := bufio.NewScanner(conn)
	scan.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scan.Scan() {
		out <- rawLine{kind: spec.kind, line: append([]byte(nil), scan.Bytes()...)}
	}
}

// fold applies one stream push to the session and emits the frames it changed.
func (s *session) fold(raw rawLine) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch raw.kind {
	case wm.FrameOutputs:
		var env monitorsEnvelope
		if json.Unmarshal(raw.line, &env) != nil {
			return
		}
		s.monitors = env.Monitors
		s.firstSeen[wm.FrameOutputs] = true
		if wants(s, wm.FrameOutputs) {
			s.emit(wm.Frame{Kind: wm.FrameOutputs, Outputs: s.outputFrames(false)})
		}
		if wants(s, wm.FrameWorkspaces) {
			s.emit(wm.Frame{Kind: wm.FrameWorkspaces, Workspaces: s.workspaceFrames()})
		}
		if wants(s, wm.FrameFocus) {
			s.emit(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: s.focusedOutput()})
		}
	case wm.FrameWindows:
		var env clientsEnvelope
		if json.Unmarshal(raw.line, &env) != nil {
			return
		}
		s.clients = env.Clients
		s.stampFocus()
		s.trackOpen(env.Clients)
		s.firstSeen[wm.FrameWindows] = true
		if wants(s, wm.FrameWindows) {
			s.emit(wm.Frame{Kind: wm.FrameWindows, Windows: s.windowFrames()})
		}
	case wm.FrameKeyboard:
		var k struct {
			Layout string `json:"layout"`
		}
		if json.Unmarshal(raw.line, &k) != nil {
			return
		}
		s.kbLayout = k.Layout
		s.firstSeen[wm.FrameKeyboard] = true
		if wants(s, wm.FrameKeyboard) {
			s.emit(wm.Frame{Kind: wm.FrameKeyboard, KeyboardLayout: s.kbLayout, KeyboardLayouts: s.layoutSet()})
		}
	}

	if !s.ready && s.initialDone() {
		s.ready = true
		s.emit(wm.Frame{Kind: wm.FrameReady})
	}
}

func wants(s *session, k wm.FrameKind) bool { return s.wants(k) }

// initialDone reports whether every wanted stream has delivered its first push.
// The streams are independent connections, so readiness is the last of the
// firsts rather than one server-side marker.
func (s *session) initialDone() bool {
	for _, spec := range streamsFor(s.wants) {
		if !s.firstSeen[spec.kind] {
			return false
		}
	}
	return true
}

// stampFocus keeps a most-recently-focused-first list from the is_focused flag
// on each push: mango has no focus-order query. Closed windows leave the list
// so a recycled id never sits in history.
func (s *session) stampFocus() {
	for _, c := range s.clients {
		if !c.IsFocused {
			continue
		}
		out := make([]int, 0, len(s.history)+1)
		out = append(out, c.ID)
		for _, id := range s.history {
			if id != c.ID {
				out = append(out, id)
			}
		}
		s.history = out
	}
	live := map[int]bool{}
	for _, c := range s.clients {
		live[c.ID] = true
	}
	kept := s.history[:0]
	for _, id := range s.history {
		if live[id] {
			kept = append(kept, id)
		}
	}
	s.history = kept
}

// trackOpen runs the open-maximise correction for one windows push. Before the
// session is ready every window is one that was already up, so it is only
// recorded; after, a genuine open is stamped and the one an app maximised for
// itself is reset to an ordinary tile. Closed windows leave the tamer so a
// recycled id starts clean.
func (s *session) trackOpen(clients []mangoClient) {
	if s.tamer == nil {
		return
	}
	live := make(map[int]bool, len(clients))
	for _, c := range clients {
		live[c.ID] = true
		if !s.ready {
			s.tamer.seen(c.ID)
			continue
		}
		if !s.known[c.ID] {
			s.tamer.open(c.ID)
		}
		if s.tamer.wantsClear(c) {
			s.tamer.cleared(c.ID)
			clearMaximized(c.ID)
		}
	}
	for id := range s.known {
		if !live[id] {
			s.tamer.forget(id)
		}
	}
	s.known = live
}

func (s *session) focusedOutput() string {
	for _, m := range s.monitors {
		if m.Active {
			return m.Name
		}
	}
	return ""
}

func (s *session) outputFrames(full bool) []wm.Output {
	out := make([]wm.Output, 0, len(s.monitors))
	for _, m := range s.monitors {
		out = append(out, outputFrame(m, full))
	}
	return out
}

func (s *session) workspaceFrames() []wm.Workspace {
	size := len(s.monitors)
	for _, m := range s.monitors {
		size += len(m.Tags)
	}
	out := make([]wm.Workspace, 0, size)
	for _, m := range s.monitors {
		for _, t := range m.Tags {
			out = append(out, wm.Workspace{
				ID:      strconv.Itoa(t.Index),
				Name:    strconv.Itoa(t.Index),
				Output:  m.Name,
				Active:  t.IsActive,
				Windows: t.ClientCount,
				Layout:  s.layoutName(t.Layout),
			})
		}
		out = append(out, specialWorkspaceFrame(m, s.clients, s.layoutName(m.LayoutSymbol)))
	}
	return out
}

// layoutName maps a tag's layout symbol to its config name. The table is read
// once from `get layouts` and cached: layouts do not change while running.
func (s *session) layoutName(symbol string) string {
	if s.layouts == nil {
		var env layoutEnvelope
		if err := get("layouts", &env); err != nil {
			return symbol
		}
		s.layouts = layoutNames(env)
	}
	if n, ok := s.layouts[symbol]; ok {
		return n
	}
	return symbol
}

// layoutSet is the loaded layout list for the bar indicator: the active name
// from the stream, the set from the machine seed (mango's IPC answers only the
// active layout).
func (s *session) layoutSet() []string {
	if len(s.kbSet) == 0 {
		s.kbSet = seededLayouts()
	}
	return s.kbSet
}

func (s *session) windowFrames() []wm.Window {
	current := currentTags(s.monitors)
	out := make([]wm.Window, 0, len(s.clients))
	for _, c := range s.clients {
		order := -1
		for i, id := range s.history {
			if id == c.ID {
				order = i
				break
			}
		}
		if c.IsFocused {
			order = 0
		}
		out = append(out, clientFrame(c, order, current))
	}
	return out
}
