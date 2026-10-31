package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// The readers behind state and watch, and the frames they render. The dialect
// structs below pin wayfire's field names and types, so a protocol change
// reads as itself in a test rather than as a silently empty frame.
//
// A view's position is relative to the current workspace: the compositor
// translates every view by the workspace delta on navigation, so a window on
// cell (0,0) reads x=173 at home and x=-1107 after switching to (1,0) on a
// 1280-wide output. There is no field for the cell itself (views carry only
// their workspace set), so it is derived rather than read: current + floor
// ((view - output origin) / output size). A view may straddle cells, so the
// cell under its top-left corner is its handle.

type wayfireGeometry struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type wayfireWorkspace struct {
	X          int `json:"x"`
	Y          int `json:"y"`
	GridWidth  int `json:"grid_width"`
	GridHeight int `json:"grid_height"`
}

type wayfireOutput struct {
	ID        int              `json:"id"`
	Name      string           `json:"name"`
	Geometry  wayfireGeometry  `json:"geometry"`
	Workarea  wayfireGeometry  `json:"workarea"`
	WsetIndex int64            `json:"wset-index"`
	Workspace wayfireWorkspace `json:"workspace"`
}

type wayfireWset struct {
	Index      int64            `json:"index"`
	Name       string           `json:"name"`
	OutputID   int              `json:"output-id"`
	OutputName string           `json:"output-name"`
	Workspace  wayfireWorkspace `json:"workspace"`
}

type wayfireView struct {
	ID                 int64           `json:"id"`
	Title              string          `json:"title"`
	AppID              string          `json:"app-id"`
	Geometry           wayfireGeometry `json:"geometry"`
	BBox               wayfireGeometry `json:"bbox"`
	OutputID           int             `json:"output-id"`
	OutputName         string          `json:"output-name"`
	LastFocusTimestamp int64           `json:"last-focus-timestamp"`
	Mapped             bool            `json:"mapped"`
	Role               string          `json:"role"`
	TiledEdges         int             `json:"tiled-edges"`
	Fullscreen         bool            `json:"fullscreen"`
	Minimized          bool            `json:"minimized"`
	Activated          bool            `json:"activated"`
	Sticky             bool            `json:"sticky"`
	WsetIndex          int64           `json:"wset-index"`
	Pid                int             `json:"pid"`
}

// isWindow keeps the seam's window list to application windows. list-views
// and the view events also carry every layer-shell surface (role
// "desktop-environment": the shell's own bar, dock and backdrop) and unmanaged
// X11 popups. Reporting those as windows made the shell count its own dock as
// a window on the workspace, react by rebuilding the dock, and see that as yet
// another window change: a loop that recreated the dock several times a second.
func (v wayfireView) isWindow() bool {
	return v.Mapped && v.Role == "toplevel"
}

type wayfireKeyboard struct {
	PossibleLayouts []string `json:"possible-layouts"`
	Layout          string   `json:"layout"`
}

// configOption keeps value and default as raw JSON: most options are strings,
// but bindings and rules are arrays, and one non-string key must not fail the
// whole tree.
type configOption struct {
	Value   json.RawMessage `json:"value"`
	Default json.RawMessage `json:"default"`
}

// valueString reads a string option; a non-string value reads as absent,
// which is correct for every key this provider asks.
func (o configOption) valueString() (string, bool) {
	var s string
	if err := json.Unmarshal(o.Value, &s); err != nil {
		return "", false
	}
	return s, true
}

// stage is one consistent read of the compositor, enough to render any frame
// without a second query. watch holds one and folds events into it.
type stage struct {
	outputs  []wayfireOutput
	wsets    []wayfireWset
	views    []wayfireView
	keyboard wayfireKeyboard
	focused  string
	config   map[string]map[string]configOption
}

func readStage() (*stage, error) {
	outputs, err := readOutputs()
	if err != nil {
		return nil, err
	}
	wsets, err := readWsets()
	if err != nil {
		return nil, err
	}
	views, err := readViews()
	if err != nil {
		return nil, err
	}
	focused, err := readFocusedOutput()
	if err != nil {
		return nil, err
	}
	keyboard, err := readKeyboard()
	if err != nil {
		return nil, err
	}
	config, err := readConfig()
	if err != nil {
		return nil, err
	}
	return &stage{
		outputs:  outputs,
		wsets:    wsets,
		views:    views,
		keyboard: keyboard,
		focused:  focused,
		config:   config,
	}, nil
}

func readViews() ([]wayfireView, error) {
	raw, err := request("window-rules/list-views", nil)
	if err != nil {
		return nil, err
	}
	var all []wayfireView
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, fmt.Errorf("wayfire list-views: bad reply %q", string(raw))
	}
	out := all[:0]
	for _, v := range all {
		if v.isWindow() {
			out = append(out, v)
		}
	}
	return out, nil
}

func readOutputs() ([]wayfireOutput, error) {
	raw, err := request("window-rules/list-outputs", nil)
	if err != nil {
		return nil, err
	}
	var out []wayfireOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("wayfire list-outputs: bad reply %q", string(raw))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func readWsets() ([]wayfireWset, error) {
	raw, err := request("window-rules/list-wsets", nil)
	if err != nil {
		return nil, err
	}
	var out []wayfireWset
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("wayfire list-wsets: bad reply %q", string(raw))
	}
	return out, nil
}

// readFocusedOutput answers the output holding the seat. It is the one query
// wrapped in an "info" envelope rather than raw; a null info is a compositor
// with no output at all, which reads as "none focused".
func readFocusedOutput() (string, error) {
	raw, err := request("window-rules/get-focused-output", nil)
	if err != nil {
		return "", err
	}
	var reply struct {
		Info json.RawMessage `json:"info"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil {
		return "", fmt.Errorf("wayfire get-focused-output: bad reply %q", string(raw))
	}
	if len(reply.Info) == 0 || string(reply.Info) == "null" {
		return "", nil
	}
	var out wayfireOutput
	if err := json.Unmarshal(reply.Info, &out); err != nil {
		return "", fmt.Errorf("wayfire get-focused-output: bad info %q", string(reply.Info))
	}
	return out.Name, nil
}

// readFocusedView answers the window the seat is on, in the same "info"
// envelope. No window focused reads as absent rather than as a failure: an
// empty desktop is a state, and the act decides whether it can proceed. A
// shell surface holding keyboard focus (the launcher, a panel) is not a
// window either, so window acts never land on the shell itself.
func readFocusedView() (wayfireView, bool, error) {
	raw, err := request("window-rules/get-focused-view", nil)
	if err != nil {
		return wayfireView{}, false, err
	}
	var reply struct {
		Info json.RawMessage `json:"info"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil {
		return wayfireView{}, false, fmt.Errorf("wayfire get-focused-view: bad reply %q", string(raw))
	}
	if len(reply.Info) == 0 || string(reply.Info) == "null" {
		return wayfireView{}, false, nil
	}
	var v wayfireView
	if err := json.Unmarshal(reply.Info, &v); err != nil {
		return wayfireView{}, false, fmt.Errorf("wayfire get-focused-view: bad info %q", string(reply.Info))
	}
	return v, v.isWindow(), nil
}

func readKeyboard() (wayfireKeyboard, error) {
	raw, err := request("wayfire/get-keyboard-state", nil)
	if err != nil {
		return wayfireKeyboard{}, err
	}
	var k wayfireKeyboard
	if err := json.Unmarshal(raw, &k); err != nil {
		return wayfireKeyboard{}, fmt.Errorf("wayfire get-keyboard-state: bad reply %q", string(raw))
	}
	return k, nil
}

// readConfig is the whole option tree at once: the per-output scale, mode and
// transform ride it, and get-config-option only answers names that exist.
func readConfig() (map[string]map[string]configOption, error) {
	raw, err := request("wayfire/list-config-options", nil)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Options map[string]map[string]configOption `json:"options"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil || reply.Options == nil {
		return nil, fmt.Errorf("wayfire list-config-options: bad reply %q", string(raw))
	}
	return reply.Options, nil
}

// workspaceHandle is the opaque id a workspace frame carries and an act
// accepts back: the workspace set, then the grid cell. Round-tripping it
// needs no table of ids the compositor never handed out.
func workspaceHandle(wset int64, x, y int) string {
	return strconv.FormatInt(wset, 10) + ":" + strconv.Itoa(x) + ":" + strconv.Itoa(y)
}

// workspaceName is the row-major display label for a cell. Identity stays in
// workspaceHandle so equal labels in different sets never collide.
func workspaceName(x, y, gridW int) string {
	return strconv.Itoa(y*gridW + x + 1)
}

// rect is the view's logical geometry; bbox also spans the decor's border and
// shadow. A fullscreen XWayland view can leave geometry stale, so it uses bbox.
func (v wayfireView) rect() wayfireGeometry {
	hasBBox := v.BBox.Width > 0 && v.BBox.Height > 0
	if hasBBox && (v.Fullscreen || v.Geometry.Width <= 0 || v.Geometry.Height <= 0) {
		return v.BBox
	}
	return v.Geometry
}

func parseWorkspaceHandle(id string) (wset int64, x, y int, ok bool) {
	parts := strings.Split(id, ":")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var err error
	if wset, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
		return 0, 0, 0, false
	}
	if x, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, 0, false
	}
	if y, err = strconv.Atoi(parts[2]); err != nil {
		return 0, 0, 0, false
	}
	return wset, x, y, true
}

// cellOf derives the grid cell under a view's top-left corner. The current
// workspace of the view's own set anchors the arithmetic, because that is the
// translation the compositor applied when it last navigated. The result is
// clamped to the grid: a hidden or unmapped view can report a corner outside
// it, and a cell the pills never list would make its window vanish from the
// occupancy counts.
func cellOf(v wayfireView, out wayfireOutput, cur wayfireWorkspace, gridW, gridH int) (int, int) {
	if out.Geometry.Width <= 0 || out.Geometry.Height <= 0 {
		return cur.X, cur.Y
	}
	r := v.rect()
	x := cur.X + int(math.Floor((r.X-out.Geometry.X)/out.Geometry.Width))
	y := cur.Y + int(math.Floor((r.Y-out.Geometry.Y)/out.Geometry.Height))
	return clamp(x, 0, gridW-1), clamp(y, 0, gridH-1)
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return v
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (s *stage) outputByName(name string) (wayfireOutput, bool) {
	for _, o := range s.outputs {
		if o.Name == name {
			return o, true
		}
	}
	return wayfireOutput{}, false
}

func (s *stage) wsetByIndex(index int64) (wayfireWset, bool) {
	for _, w := range s.wsets {
		if w.Index == index {
			return w, true
		}
	}
	return wayfireWset{}, false
}

// cellOfView resolves the output and the set's current workspace the
// derivation needs, plus its grid width. A view whose set is unknown falls back
// to its output's current cell, which is where an ordinary view lives anyway;
// a view with no output reports grid width 0.
func (s *stage) cellOfView(v wayfireView) (int, int, int) {
	if isScratchWset(v.WsetIndex) {
		return 0, 0, 1
	}
	out, ok := s.outputByName(v.OutputName)
	if !ok {
		return 0, 0, 0
	}
	ws, ok := s.wsetByIndex(v.WsetIndex)
	if !ok {
		x, y := cellOf(v, out, out.Workspace, out.Workspace.GridWidth, out.Workspace.GridHeight)
		return x, y, out.Workspace.GridWidth
	}
	x, y := cellOf(v, out, ws.Workspace, ws.Workspace.GridWidth, ws.Workspace.GridHeight)
	return x, y, ws.Workspace.GridWidth
}

// workspaceFrame renders one cell per grid slot of every ordinary set.
// Scratch sets are one special workspace so their backing grid never leaks
// into workspace pills. A set without an output is listed only while it holds
// windows: an output re-created on hotplug leaves its old empty set behind.
func (s *stage) workspaceFrame() []wm.Workspace {
	counts := map[string]int{}
	setWindows := map[int64]int{}
	fullscreen := map[string]bool{}
	for _, v := range s.views {
		x, y, _ := s.cellOfView(v)
		handle := workspaceHandle(v.WsetIndex, x, y)
		counts[handle]++
		setWindows[v.WsetIndex]++
		if v.Fullscreen {
			fullscreen[handle] = true
		}
	}
	out := make([]wm.Workspace, 0, len(s.wsets)*3*3)
	for _, ws := range s.wsets {
		if isScratchWset(ws.Index) {
			attached, _ := s.attachedCurrent(ws)
			handle := workspaceHandle(ws.Index, 0, 0)
			out = append(out, wm.Workspace{
				ID: handle, Name: "scratchpad", Output: ws.OutputName,
				Active: attached, Windows: counts[handle], Fullscreen: fullscreen[handle],
				Special: true,
			})
			continue
		}
		gridW, gridH := ws.Workspace.GridWidth, ws.Workspace.GridHeight
		if gridW <= 0 || gridH <= 0 {
			continue
		}
		attached, onCurrent := s.attachedCurrent(ws)
		if !attached && setWindows[ws.Index] == 0 {
			continue
		}
		for y := range gridH {
			for x := range gridW {
				handle := workspaceHandle(ws.Index, x, y)
				out = append(out, wm.Workspace{
					ID:         handle,
					Name:       workspaceName(x, y, gridW),
					Output:     ws.OutputName,
					Index:      y*gridW + x + 1,
					Active:     attached && onCurrent.X == x && onCurrent.Y == y,
					Windows:    counts[handle],
					Fullscreen: fullscreen[handle],
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Output != out[j].Output {
			return out[i].Output < out[j].Output
		}
		_, ax, ay, _ := parseWorkspaceHandle(out[i].ID)
		_, bx, by, _ := parseWorkspaceHandle(out[j].ID)
		if ay != by {
			return ay < by
		}
		if ax != bx {
			return ax < bx
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// attachedCurrent reports whether a set is the one an output is showing, and
// that output's current cell. A detached set has no active cell: its windows
// are still counted, they just are not where anyone is looking.
func (s *stage) attachedCurrent(ws wayfireWset) (attached bool, cur wayfireWorkspace) {
	for _, o := range s.outputs {
		if o.WsetIndex == ws.Index {
			return true, o.Workspace
		}
	}
	return false, ws.Workspace
}

// windowFrame renders the held views ordered by focus: the activated view
// first, then the freshest focus timestamp. Both are carried because wayfire
// deactivates the old view without an event naming it, so the timestamp alone
// keeps the order honest when a stale activated flag survives.
func (s *stage) windowFrame() []wm.Window {
	ordered := make([]wayfireView, len(s.views))
	copy(ordered, s.views)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Activated != ordered[j].Activated {
			return ordered[i].Activated
		}
		return ordered[i].LastFocusTimestamp > ordered[j].LastFocusTimestamp
	})
	out := make([]wm.Window, 0, len(ordered))
	for i, v := range ordered {
		x, y, _ := s.cellOfView(v)
		workspace := workspaceHandle(v.WsetIndex, x, y)
		rect := v.rect()
		frame := wm.Window{
			ID:         strconv.FormatInt(v.ID, 10),
			AppID:      v.AppID,
			Title:      v.Title,
			Workspace:  workspace,
			Output:     v.OutputName,
			Pid:        v.Pid,
			FocusOrder: i,
			Floating:   v.TiledEdges == 0 && !v.Fullscreen,
			X:          int(math.Round(rect.X)),
			Y:          int(math.Round(rect.Y)),
			Width:      int(math.Round(rect.Width)),
			Height:     int(math.Round(rect.Height)),
		}
		out = append(out, frame)
	}
	return out
}

func (s *stage) outputFrame(o wayfireOutput) wm.Output {
	frame := wm.Output{
		Name:            o.Name,
		Width:           int(math.Round(o.Geometry.Width)),
		Height:          int(math.Round(o.Geometry.Height)),
		Scale:           1,
		X:               int(math.Round(o.Geometry.X)),
		Y:               int(math.Round(o.Geometry.Y)),
		Focused:         o.Name == s.focused,
		ActiveWorkspace: activeWorkspaceHandle(o),
	}
	section, ok := s.config["output:"+o.Name]
	if !ok {
		return frame
	}
	if v, ok := section["scale"]; ok {
		if s, ok := v.valueString(); ok {
			if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 {
				frame.Scale = f
			}
		}
	}
	if v, ok := section["mode"]; ok {
		if s, ok := v.valueString(); ok && s != "" && s != "auto" {
			if mirror, found := strings.CutPrefix(s, "mirror "); found {
				frame.Mirror = strings.TrimSpace(mirror)
			} else {
				frame.Mode = s
			}
		}
	}
	if v, ok := section["hdr"]; ok {
		if s, ok := v.valueString(); ok {
			if s == "true" {
				frame.ColorMode = "hdr"
			} else {
				frame.ColorMode = "srgb"
			}
		}
	}
	if v, ok := section["transform"]; ok {
		if s, ok := v.valueString(); ok {
			frame.Transform = transformFromWayfire(s)
		}
	}
	if v, ok := section["vrr"]; ok {
		if s, ok := v.valueString(); ok {
			frame.VRR = s == "true"
		}
	}
	return frame
}

func (s *stage) outputsFrame() []wm.Output {
	out := make([]wm.Output, 0, len(s.outputs))
	for _, o := range s.outputs {
		out = append(out, s.outputFrame(o))
	}
	return out
}

// activeWorkspaceHandle is the provider-owned key for the cell an output
// shows, matching Workspace.ID and Window.Workspace.
func activeWorkspaceHandle(o wayfireOutput) string {
	if isScratchWset(o.WsetIndex) {
		return workspaceHandle(o.WsetIndex, 0, 0)
	}
	return workspaceHandle(o.WsetIndex, o.Workspace.X, o.Workspace.Y)
}

// keyboardPair folds wayfire's "unknown" layout (no keyboard ever named one)
// to the empty current layout the bar renders as absent.
func (k wayfireKeyboard) keyboardPair() (string, []string) {
	all := k.PossibleLayouts
	if all == nil {
		all = []string{}
	}
	if k.Layout == "" || k.Layout == "unknown" {
		return "", all
	}
	return k.Layout, all
}
