package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	wm "ryoku-wm"
)

// niri's event stream carries the whole picture: a connect replays full
// workspace, window and keyboard lists, and every later event is either a new
// full list or a delta this file folds into the one it holds. So unlike the
// Hyprland provider there is no query pass and nothing to debounce, and a focus
// change is emitted straight from the event line.
//
// Outputs are the exception: niri has no output event. Workspaces live on
// outputs and niri moves them when one appears or disappears, so an output read
// rides the workspace event.

const reconnectBackoff = 500 * time.Millisecond

type niriTimestamp struct {
	Secs  int64 `json:"secs"`
	Nanos int64 `json:"nanos"`
}

func (t niriTimestamp) after(other niriTimestamp) bool {
	if t.Secs != other.Secs {
		return t.Secs > other.Secs
	}
	return t.Nanos > other.Nanos
}

type niriWindow struct {
	ID             uint64           `json:"id"`
	Title          string           `json:"title"`
	AppID          string           `json:"app_id"`
	Pid            int              `json:"pid"`
	WorkspaceID    uint64           `json:"workspace_id"`
	IsFocused      bool             `json:"is_focused"`
	IsFloating     bool             `json:"is_floating"`
	FocusTimestamp niriTimestamp    `json:"focus_timestamp"`
	Layout         niriWindowLayout `json:"layout"`
}

type niriWindowLayout struct {
	TileSize               [2]float64  `json:"tile_size"`
	WindowSize             [2]int      `json:"window_size"`
	TilePosInWorkspaceView *[2]float64 `json:"tile_pos_in_workspace_view"`
	WindowOffsetInTile     [2]float64  `json:"window_offset_in_tile"`
}

type niriWorkspace struct {
	ID        uint64 `json:"id"`
	Idx       int    `json:"idx"`
	Name      string `json:"name"`
	Output    string `json:"output"`
	IsActive  bool   `json:"is_active"`
	IsFocused bool   `json:"is_focused"`
}

type niriOutput struct {
	Name         string     `json:"name"`
	Make         string     `json:"make"`
	Model        string     `json:"model"`
	PhysicalSize *[2]int    `json:"physical_size"`
	Modes        []niriMode `json:"modes"`
	CurrentMode  *int       `json:"current_mode"`
	VRREnabled   bool       `json:"vrr_enabled"`
	Logical      *struct {
		X         int     `json:"x"`
		Y         int     `json:"y"`
		Width     int     `json:"width"`
		Height    int     `json:"height"`
		Scale     float64 `json:"scale"`
		Transform string  `json:"transform"`
	} `json:"logical"`
}

// niri reports refresh in millihertz; the editor speaks "WxH@Hz".
type niriMode struct {
	Width       int  `json:"width"`
	Height      int  `json:"height"`
	RefreshRate int  `json:"refresh_rate"`
	IsPreferred bool `json:"is_preferred"`
}

type niriKeyboard struct {
	Names      []string `json:"names"`
	CurrentIdx int      `json:"current_idx"`
}

// session holds what the stream has told us so far, so a delta event can be
// turned into a full frame without asking the compositor anything.
type session struct {
	workspaces []niriWorkspace
	windows    []niriWindow
	keyboard   niriKeyboard
	// outputs is kept only so the maximise correction can size a window against
	// its own output; a plain watch never reads it.
	outputs []wm.Output
	// overview is the compositor's native overview state, replayed on connect
	// and folded from every OverviewOpenedOrClosed event.
	overview bool
	// ready gates the correction: everything replayed before it is a window that
	// was already up, never an open this watch owns.
	ready bool
	// tamer is nil unless the open-maximise correction is on for this stream.
	tamer *tamer
}

// runWatch streams every frame kind, or only the ones named in args. A narrowed
// watch skips the frames it does not need, which keeps a burst cheap for a
// consumer that only wants one kind.
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
		conn, _, err := open("EventStream")
		if err != nil {
			time.Sleep(reconnectBackoff)
			continue
		}
		consume(conn, emit, wants)
		_ = conn.Close()
		time.Sleep(reconnectBackoff)
	}
}

// consume reads the stream until it drops. The first line is the request
// acknowledgement; the replay that follows is the full state, so readiness is
// the end of that burst rather than a separate query.
func consume(conn net.Conn, emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	scan := bufio.NewScanner(conn)
	scan.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	if !scan.Scan() {
		return
	}
	if _, err := unwrap("EventStream", scan.Bytes()); err != nil {
		return
	}

	s := &session{}
	// The correction rides this stream, set up once per connect. It needs the
	// window events it hinges on, so a watch that skips those skips it; whether
	// it acts is read from the store on each open.
	if wants(wm.FrameWindows) {
		s.tamer = newTamer()
	}
	for scan.Scan() {
		var event map[string]json.RawMessage
		if json.Unmarshal(scan.Bytes(), &event) != nil {
			continue
		}
		for name, body := range event {
			s.apply(name, body, emit, wants)
		}
		// The replay ends with the events niri sends once per connect, so the
		// first of those marks a complete picture.
		if !s.ready && (event["ConfigLoaded"] != nil || event["OverviewOpenedOrClosed"] != nil) {
			s.ready = true
			emit(wm.Frame{Kind: wm.FrameReady})
		}
	}
}

// apply folds one event into the session and emits what changed.
func (s *session) apply(name string, body json.RawMessage, emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	switch name {
	case "WorkspacesChanged":
		var e struct {
			Workspaces []niriWorkspace `json:"workspaces"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		s.workspaces = e.Workspaces
		s.emitWorkspaces(emit, wants)
		s.emitFocus(emit, wants)
		// No output event exists, so this is where a hotplug surfaces. The
		// correction needs the output sizes too, so read them when it is on even
		// if this watch would not otherwise emit them.
		if wants(wm.FrameOutputs) || s.tamer != nil {
			if outs, err := readOutputs(s.workspaces, false); err == nil {
				s.outputs = outs
				if wants(wm.FrameOutputs) {
					emit(wm.Frame{Kind: wm.FrameOutputs, Outputs: outs})
				}
			}
		}

	case "WorkspaceActivated":
		var e struct {
			ID      uint64 `json:"id"`
			Focused bool   `json:"focused"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		s.activate(e.ID, e.Focused)
		s.emitWorkspaces(emit, wants)
		if e.Focused {
			s.emitFocus(emit, wants)
		}

	case "WindowsChanged":
		var e struct {
			Windows []niriWindow `json:"windows"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		s.windows = e.Windows
		s.emitWindows(emit, wants)
		s.emitWorkspaces(emit, wants)
		for _, w := range e.Windows {
			s.tameWindow(w.ID, w.Layout.WindowSize[0], w.Layout.WindowSize[1])
		}

	case "WindowOpenedOrChanged":
		var e struct {
			Window niriWindow `json:"window"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		s.upsert(e.Window)
		s.emitWindows(emit, wants)
		s.emitWorkspaces(emit, wants)
		s.tameWindow(e.Window.ID, e.Window.Layout.WindowSize[0], e.Window.Layout.WindowSize[1])

	case "WindowClosed":
		var e struct {
			ID uint64 `json:"id"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		s.remove(e.ID)
		s.emitWindows(emit, wants)
		s.emitWorkspaces(emit, wants)
		if s.tamer != nil {
			s.tamer.forget(e.ID)
		}

	case "WindowFocusChanged":
		var e struct {
			ID *uint64 `json:"id"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		s.focusWindow(e.ID)
		s.emitWindows(emit, wants)

	case "WindowLayoutsChanged":
		// niri reports native moves, resizes and maximise changes here rather
		// than through WindowOpenedOrChanged. Fold the whole layout: size-only
		// updates would leave a dragged floating window's persisted position
		// stale until some unrelated full window event.
		var e struct {
			Changes [][]json.RawMessage `json:"changes"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		for _, ch := range e.Changes {
			if len(ch) != 2 {
				continue
			}
			var id uint64
			if json.Unmarshal(ch[0], &id) != nil {
				continue
			}
			var layout niriWindowLayout
			if json.Unmarshal(ch[1], &layout) != nil {
				continue
			}
			s.setWindowLayout(id, layout)
			s.tameWindow(id, layout.WindowSize[0], layout.WindowSize[1])
		}
		s.emitWindows(emit, wants)

	case "KeyboardLayoutsChanged":
		var e struct {
			KeyboardLayouts niriKeyboard `json:"keyboard_layouts"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		s.keyboard = e.KeyboardLayouts
		s.emitKeyboard(emit, wants)

	case "KeyboardLayoutSwitched":
		var e struct {
			Idx int `json:"idx"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		s.keyboard.CurrentIdx = e.Idx
		s.emitKeyboard(emit, wants)

	case "OverviewOpenedOrClosed":
		var e struct {
			IsOpen bool `json:"is_open"`
		}
		if json.Unmarshal(body, &e) != nil {
			return
		}
		s.overview = e.IsOpen
		if wants(wm.FrameOverview) {
			emit(wm.Frame{Kind: wm.FrameOverview, OverviewOpen: e.IsOpen})
		}
	}
}

// tameWindow runs the open-maximise correction for one window update. Before the
// session is ready every window is one that was already up, so it is only
// recorded; after, a genuine open is stamped and the one an app maximised for
// itself is reset to an ordinary column.
func (s *session) tameWindow(id uint64, w, h int) {
	if s.tamer == nil {
		return
	}
	if !s.ready {
		s.tamer.seen(id)
		return
	}
	s.tamer.open(id)
	out, ok := s.outputForWindow(id)
	if !ok {
		return
	}
	if s.tamer.wantsClear(out, id, w, h) {
		s.tamer.cleared(id)
		clearClientMaximized(id)
	}
}

// outputForWindow resolves the output a held window sits on, so its size can be
// measured against the right screen. Empty when the window or its output is not
// held yet, which reads as "cannot judge, leave it".
func (s *session) outputForWindow(id uint64) (wm.Output, bool) {
	ws := uint64(0)
	found := false
	for _, win := range s.windows {
		if win.ID == id {
			ws, found = win.WorkspaceID, true
			break
		}
	}
	if !found {
		return wm.Output{}, false
	}
	name := ""
	for _, w := range s.workspaces {
		if w.ID == ws {
			name = w.Output
			break
		}
	}
	for _, o := range s.outputs {
		if o.Name == name {
			return o, true
		}
	}
	return wm.Output{}, false
}

// setWindowLayout folds WindowLayoutsChanged into the held window, so the
// geometry a later frame reports stays current after a native move or resize.
func (s *session) setWindowLayout(id uint64, layout niriWindowLayout) {
	for i := range s.windows {
		if s.windows[i].ID == id {
			s.windows[i].Layout = layout
			return
		}
	}
}

// activate folds a WorkspaceActivated event in. niri keeps one active
// workspace per output and one focused overall, so the sibling it replaces is
// the one sharing its output, not every other workspace.
func (s *session) activate(id uint64, focused bool) {
	output := ""
	for _, ws := range s.workspaces {
		if ws.ID == id {
			output = ws.Output
			break
		}
	}
	for i := range s.workspaces {
		ws := &s.workspaces[i]
		if ws.ID == id {
			ws.IsActive = true
			ws.IsFocused = focused
			continue
		}
		if ws.Output == output {
			ws.IsActive = false
		}
		if focused {
			ws.IsFocused = false
		}
	}
}

func (s *session) upsert(w niriWindow) {
	for i := range s.windows {
		if s.windows[i].ID == w.ID {
			s.windows[i] = w
			return
		}
	}
	s.windows = append(s.windows, w)
}

func (s *session) remove(id uint64) {
	for i := range s.windows {
		if s.windows[i].ID == id {
			s.windows = append(s.windows[:i], s.windows[i+1:]...)
			return
		}
	}
}

// focusWindow updates the focus flag and stamps the new focus, so focus order
// stays correct without waiting for niri to resend the list.
func (s *session) focusWindow(id *uint64) {
	now := time.Now()
	for i := range s.windows {
		w := &s.windows[i]
		w.IsFocused = id != nil && w.ID == *id
		if w.IsFocused {
			w.FocusTimestamp = niriTimestamp{Secs: now.Unix(), Nanos: int64(now.Nanosecond())}
		}
	}
}

func (s *session) emitWorkspaces(emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	if !wants(wm.FrameWorkspaces) {
		return
	}
	emit(wm.Frame{Kind: wm.FrameWorkspaces, Workspaces: s.workspaceFrame()})
}

func (s *session) emitWindows(emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	if !wants(wm.FrameWindows) {
		return
	}
	emit(wm.Frame{Kind: wm.FrameWindows, Windows: s.windowFrame()})
}

func (s *session) emitKeyboard(emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	if !wants(wm.FrameKeyboard) {
		return
	}
	current, all := s.keyboard.current()
	emit(wm.Frame{Kind: wm.FrameKeyboard, KeyboardLayout: current, KeyboardLayouts: all})
}

func (s *session) emitFocus(emit func(wm.Frame), wants func(wm.FrameKind) bool) {
	if !wants(wm.FrameFocus) {
		return
	}
	for _, ws := range s.workspaces {
		if ws.IsFocused {
			emit(wm.Frame{Kind: wm.FrameFocus, FocusedOutput: ws.Output})
			return
		}
	}
}

func (k niriKeyboard) current() (string, []string) {
	all := k.Names
	if all == nil {
		all = []string{}
	}
	if k.CurrentIdx < 0 || k.CurrentIdx >= len(all) {
		return "", all
	}
	return all[k.CurrentIdx], all
}

// workspaceFrame renders the held workspaces, counting windows from the held
// window list rather than a separate query.
func (s *session) workspaceFrame() []wm.Workspace {
	counts := map[uint64]int{}
	for _, w := range s.windows {
		counts[w.WorkspaceID]++
	}
	out := make([]wm.Workspace, 0, len(s.workspaces))
	for _, ws := range s.workspaces {
		out = append(out, wm.Workspace{
			ID:      formatUint(ws.ID),
			Name:    workspaceName(ws),
			Output:  ws.Output,
			Active:  ws.IsActive,
			Windows: counts[ws.ID],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// windowFrame renders the held windows. niri exposes an absolute position only
// for floating windows. That is enough for a native drag/resize to round-trip:
// tile_pos_in_workspace_view is output-local logical geometry, then the output
// origin turns it into the global logical coordinate convention wm.Window uses.
func (s *session) windowFrame() []wm.Window {
	byWorkspace := map[uint64]niriWorkspace{}
	for _, ws := range s.workspaces {
		byWorkspace[ws.ID] = ws
	}
	byOutput := map[string]wm.Output{}
	for _, output := range s.outputs {
		byOutput[output.Name] = output
	}
	ordered := make([]niriWindow, len(s.windows))
	copy(ordered, s.windows)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].IsFocused != ordered[j].IsFocused {
			return ordered[i].IsFocused
		}
		return ordered[i].FocusTimestamp.after(ordered[j].FocusTimestamp)
	})
	out := make([]wm.Window, 0, len(ordered))
	for i, w := range ordered {
		outputName := byWorkspace[w.WorkspaceID].Output
		frame := wm.Window{
			ID:         formatUint(w.ID),
			AppID:      w.AppID,
			Title:      w.Title,
			Pid:        w.Pid,
			Workspace:  formatUint(w.WorkspaceID),
			Output:     outputName,
			FocusOrder: i,
			Floating:   w.IsFloating,
			Width:      w.Layout.WindowSize[0],
			Height:     w.Layout.WindowSize[1],
		}
		if w.IsFloating && w.Layout.TilePosInWorkspaceView != nil {
			origin := byOutput[outputName]
			frame.X = origin.X + int(math.Round(w.Layout.TilePosInWorkspaceView[0]+w.Layout.WindowOffsetInTile[0]))
			frame.Y = origin.Y + int(math.Round(w.Layout.TilePosInWorkspaceView[1]+w.Layout.WindowOffsetInTile[1]))
		}
		out = append(out, frame)
	}
	return out
}

// The configured name is what a user recognises; the index is the fallback,
// because a niri workspace is unnamed until someone names it.
func workspaceName(ws niriWorkspace) string {
	if ws.Name != "" {
		return ws.Name
	}
	return formatInt(ws.Idx)
}

// niri hands out numeric ids; the frames carry them as text so a consumer
// round-trips a handle without caring what it is.
func formatUint(n uint64) string { return strconv.FormatUint(n, 10) }

func formatInt(n int) string { return strconv.Itoa(n) }

// readOutputs reads the output list and marks the focused one from the
// workspaces already held, so no second query is needed to resolve focus.
func readOutputs(workspaces []niriWorkspace, full bool) ([]wm.Output, error) {
	raw, err := request("Outputs")
	if err != nil {
		return nil, err
	}
	var byName map[string]niriOutput
	if err := decode(raw, "Outputs", &byName); err != nil {
		return nil, err
	}
	focused, active := "", map[string]string{}
	for _, ws := range workspaces {
		if ws.IsFocused {
			focused = ws.Output
		}
		if ws.IsActive {
			active[ws.Output] = formatUint(ws.ID)
		}
	}
	out := make([]wm.Output, 0, len(byName))
	for _, o := range byName {
		out = append(out, outputFrame(o, focused, active, full))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// A disabled output has no logical rectangle, which is also how its size stops
// being meaningful, so the mode is only read for an enabled one. Position is
// always carried because window frames use the global logical coordinate
// convention; full adds rotation, VRR and physical modes for the display editor.
func outputFrame(o niriOutput, focused string, active map[string]string, full bool) wm.Output {
	out := wm.Output{
		Name:            o.Name,
		Make:            o.Make,
		Model:           o.Model,
		Focused:         o.Name == focused,
		ActiveWorkspace: active[o.Name],
		Disabled:        o.Logical == nil,
	}
	if o.PhysicalSize != nil {
		out.PhysicalWidth = o.PhysicalSize[0]
	}
	if o.Logical != nil {
		out.Width = o.Logical.Width
		out.Height = o.Logical.Height
		out.Scale = o.Logical.Scale
		out.X = o.Logical.X
		out.Y = o.Logical.Y
	}
	if !full {
		return out
	}
	if o.Logical != nil {
		out.Transform = niriTransformToWayland(o.Logical.Transform)
	}
	out.VRR = o.VRREnabled
	for _, m := range o.Modes {
		out.Modes = append(out.Modes, niriModeString(m))
	}
	if o.CurrentMode != nil && *o.CurrentMode >= 0 && *o.CurrentMode < len(o.Modes) {
		out.Mode = niriModeString(o.Modes[*o.CurrentMode])
	}
	return out
}

// niriModeString renders a niri mode as the "WxH@Hz" the editor uses, keeping the
// fractional refresh niri reports so a picked mode round-trips to the exact one.
func niriModeString(m niriMode) string {
	return fmt.Sprintf("%dx%d@%g", m.Width, m.Height, float64(m.RefreshRate)/1000)
}

// niriTransformToWayland maps niri's logical transform name onto the wayland
// transform integer the neutral output shape carries. An unknown name reads as
// normal, the safe default for a rotation the editor cannot show.
func niriTransformToWayland(s string) int {
	switch normTransform(s) {
	case "90":
		return 1
	case "180":
		return 2
	case "270":
		return 3
	case "flipped":
		return 4
	case "flipped90":
		return 5
	case "flipped180":
		return 6
	case "flipped270":
		return 7
	}
	return 0
}

// normTransform folds niri's spellings ("Normal", "_90", "Flipped-90") to a bare
// token, so the mapping does not depend on which form this niri version emits.
func normTransform(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

func readWindows() ([]niriWindow, error) {
	raw, err := request("Windows")
	if err != nil {
		return nil, err
	}
	var wins []niriWindow
	if err := decode(raw, "Windows", &wins); err != nil {
		return nil, err
	}
	return wins, nil
}

func readWorkspaces() ([]niriWorkspace, error) {
	raw, err := request("Workspaces")
	if err != nil {
		return nil, err
	}
	var out []niriWorkspace
	if err := decode(raw, "Workspaces", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// keyboardLayouts is the current layout and the loaded set, empty when nothing
// answers, so a caller with no session still gets a usable pair.
func keyboardLayouts() (string, []string) {
	if !live() {
		return "", nil
	}
	raw, err := request("KeyboardLayouts")
	if err != nil {
		return "", nil
	}
	var k niriKeyboard
	if decode(raw, "KeyboardLayouts", &k) != nil {
		return "", nil
	}
	return k.current()
}

// runState is one snapshot for callers that ask once and exit. It queries
// rather than opening the stream, so a one-shot caller pays one round trip per
// list instead of waiting for a replay to finish.
func runState() error {
	s := &session{}
	workspaces, err := readWorkspaces()
	if err != nil {
		return err
	}
	s.workspaces = workspaces
	if s.windows, err = readWindows(); err != nil {
		return err
	}
	outs, err := readOutputs(s.workspaces, true)
	if err != nil {
		return err
	}
	s.outputs = outs
	current, all := keyboardLayouts()
	snap := wm.Snapshot{
		Outputs:         outs,
		Workspaces:      s.workspaceFrame(),
		Windows:         s.windowFrame(),
		KeyboardLayout:  current,
		KeyboardLayouts: all,
	}
	for _, o := range outs {
		if o.Focused {
			snap.FocusedOutput = o.Name
		}
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}
