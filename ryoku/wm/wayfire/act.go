package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	wm "ryoku-wm"
)

// One neutral action id in, one wayfire ipc request out. The only file
// allowed to spell a wayfire method name for actions, the sibling of the
// Hyprland and niri act files.
//
// Nearly everything rides the same socket the state verbs query. Three jobs
// have no socket path and own their helper below: ending the session is a
// signal to the process that holds the socket, the night light is a detached
// wlsunset over wlr-gamma-control, and the arrangement cycle records its
// position because wayfire keeps no arrangement state of its own.

func runAct(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("act: missing action id")
	}
	act := wm.Action(args[0])
	rest := args[1:]
	if !live() {
		return fmt.Errorf("act %s: no live wayfire session", act)
	}

	switch act {
	case wm.ActionWindowFocus:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		return perform("window-rules/focus-view", map[string]any{"id": id})

	case wm.ActionWindowClose:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		return perform("window-rules/close-view", map[string]any{"id": id})

	case wm.ActionWindowFullscreen:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		view, err := viewByID(id)
		if err != nil {
			return err
		}
		return perform("wm-actions/set-fullscreen", map[string]any{
			"id": id, "state": !view.Fullscreen,
		})

	case wm.ActionWindowFloat:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		view, err := viewByID(id)
		if err != nil {
			return err
		}
		// configure-view takes one edge set and has no split to pick, so the
		// toggle swaps free floating against the full-workspace tile.
		edges := 0
		if view.TiledEdges == 0 {
			edges = 15
		}
		return perform("window-rules/configure-view", map[string]any{
			"id": id, "tiled-edges": edges,
		})

	case wm.ActionWindowPlace:
		return placeWindow(rest)

	case wm.ActionWindowMoveToWorkspace:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		ws, err := arg(rest, 1, "workspace id")
		if err != nil {
			return err
		}
		return moveViewToCell(id, ws, true)

	case wm.ActionWindowSummon:
		title, err := arg(rest, 0, "window title")
		if err != nil {
			return err
		}
		view, ok, err := newestView(func(v wayfireView) bool { return v.Title == title })
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("act %s: no window titled %q", act, title)
		}
		out, x, y, err := focusedCell()
		if err != nil {
			return err
		}
		return moveViewToCell(view.ID, workspaceHandle(out.WsetIndex, x, y), true)

	// A direction is the whole argument of the acts a keybind points at the
	// focused window or at a screen; wayfire binds none of these itself, so
	// the catalogue rows run them through the command plugin.
	case wm.ActionWindowFocusDirection:
		dir, err := directionArg(rest, 0)
		if err != nil {
			return err
		}
		return focusByDirection(act, dir)

	case wm.ActionWindowFocusEdge:
		edge, err := arg(rest, 0, "edge")
		if err != nil {
			return err
		}
		if edge != "left" && edge != "right" {
			return fmt.Errorf("act %s: edge must be left or right, got %q", act, edge)
		}
		return focusAtEdge(act, edge)

	case wm.ActionWindowMoveBy:
		dir, err := directionArg(rest, 0)
		if err != nil {
			return err
		}
		return moveFocusedBy(act, dir)

	case wm.ActionWindowResizeBy:
		step, err := arg(rest, 0, "resize step")
		if err != nil {
			return err
		}
		switch step {
		case "narrower", "wider", "shorter", "taller":
		default:
			return fmt.Errorf("act %s: resize step must be narrower|wider|shorter|taller, got %q", act, step)
		}
		return resizeFocusedBy(act, step)

	case wm.ActionWindowPresetHeight:
		first := false
		if s := strings.TrimSpace(strings.Join(rest, " ")); s != "" {
			if s != "first" {
				return fmt.Errorf("act %s: preset takes no argument or first, got %q", act, s)
			}
			first = true
		}
		return presetFocusedHeight(act, first)

	case wm.ActionWindowCenter:
		return centerFocused(act)

	case wm.ActionAppFocus:
		appID, err := arg(rest, 0, "app id")
		if err != nil {
			return err
		}
		view, ok, err := newestView(func(v wayfireView) bool { return v.AppID == appID })
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("act %s: no window of app %q", act, appID)
		}
		return perform("window-rules/focus-view", map[string]any{"id": view.ID})

	case wm.ActionWorkspaceFocus:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return err
		}
		target, x, y, err := workspaceTarget(ws)
		if err != nil {
			return err
		}
		if target.OutputID <= 0 {
			return fmt.Errorf("act %s: workspace %q is not attached to an output", act, ws)
		}
		outputs, err := readOutputs()
		if err != nil {
			return err
		}
		for _, out := range outputs {
			if out.ID == target.OutputID && out.WsetIndex != target.Index {
				// A handle can name a set the output is not showing, and
				// vswitch only moves inside the current one.
				if err := perform("wsets/set-output-wset", map[string]any{
					"output-id": out.ID, "wset-index": target.Index,
				}); err != nil {
					return err
				}
				break
			}
		}
		return perform("vswitch/set-workspace", map[string]any{
			"x": x, "y": y, "output-id": target.OutputID,
		})

	case wm.ActionWorkspaceCycle:
		delta, err := arg(rest, 0, "delta")
		if err != nil {
			return err
		}
		n, convErr := strconv.Atoi(delta)
		if convErr != nil {
			return fmt.Errorf("act %s: delta must be an integer, got %q", act, delta)
		}
		out, x, y, err := focusedCell()
		if err != nil {
			return err
		}
		gridW, gridH := out.Workspace.GridWidth, out.Workspace.GridHeight
		if gridW <= 0 || gridH <= 0 {
			return fmt.Errorf("act %s: output %q reports no workspace grid", act, out.Name)
		}
		// The grid walks row-major with wrap, which is the order the cells
		// are published in, so one step is one step on the pills.
		steps := gridW * gridH
		flat := (((y*gridW + x + n) % steps) + steps) % steps
		return perform("vswitch/set-workspace", map[string]any{
			"x": flat % gridW, "y": flat / gridW, "output-id": out.ID,
		})

	case wm.ActionWindowMoveToWorkspaceBy:
		delta, err := arg(rest, 0, "delta")
		if err != nil {
			return err
		}
		n, convErr := strconv.Atoi(delta)
		if convErr != nil {
			return fmt.Errorf("act %s: delta must be an integer, got %q", act, delta)
		}
		return moveFocusedWindowWorkspaceBy(act, n)

	case wm.ActionWorkspaceMoveToOutput:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return err
		}
		name, err := arg(rest, 1, "output name")
		if err != nil {
			return err
		}
		wsetIdx, x, y, ok := parseWorkspaceHandle(ws)
		if !ok {
			return fmt.Errorf("act %s: workspace id %q is not set:cell:cell", act, ws)
		}
		target, err := readOutput(name)
		if err != nil {
			return err
		}
		stage, err := readStage()
		if err != nil {
			return err
		}
		return moveCellViewsToOutput(stage, wsetIdx, x, y, target)

	// The screen acts measure direction from the focused screen: focus the
	// window it last held, hand it over, or send the whole cell across.
	case wm.ActionOutputFocusDirection:
		dir, err := directionArg(rest, 0)
		if err != nil {
			return err
		}
		return focusScreenByDirection(act, dir)

	case wm.ActionWindowMoveToOutputBy:
		dir, err := directionArg(rest, 0)
		if err != nil {
			return err
		}
		return moveFocusedToScreen(act, dir)

	case wm.ActionWorkspaceMoveToOutputBy:
		dir, err := directionArg(rest, 0)
		if err != nil {
			return err
		}
		return moveCellToScreen(act, dir)

	case wm.ActionSessionExit:
		pid, err := wayfirePID()
		if err != nil {
			return fmt.Errorf("act %s: %w", act, err)
		}
		return syscall.Kill(pid, syscall.SIGTERM)

	case wm.ActionKeyboardCycleLayout:
		k, err := readKeyboard()
		if err != nil {
			return err
		}
		if len(k.PossibleLayouts) == 0 {
			return fmt.Errorf("act %s: no layouts to cycle", act)
		}
		next := 0
		for i, name := range k.PossibleLayouts {
			if name == k.Layout {
				next = (i + 1) % len(k.PossibleLayouts)
				break
			}
		}
		return perform("wayfire/set-keyboard-state", map[string]any{"layout-index": next})

	case wm.ActionNightLightOn:
		return nightlightStart("wlsunset", "-t", strconv.Itoa(nightlightTemp(rest)), "-T", "30000")

	case wm.ActionNightLightOff:
		nightlightStop("wlsunset")
		return nil

	case wm.ActionInputTouchpad:
		return touchpadAct(rest)

	case wm.ActionBorderColors:
		return setBorderPalette(rest)

	case wm.ActionOutputCycle:
		return cycleOutputs()

	case wm.ActionOutputEnable:
		conn, err := arg(rest, 0, "connector")
		if err != nil {
			return err
		}
		state, err := arg(rest, 1, "on|off")
		if err != nil {
			return err
		}
		mode := ""
		switch state {
		case "on":
			mode = "auto"
		case "off":
			mode = "off"
		default:
			return fmt.Errorf("act %s: state must be on or off, got %q", act, state)
		}
		return perform("wayfire/set-config-options", map[string]any{
			"output:" + conn + "/mode": mode,
		})

	case wm.ActionCursorReassert:
		// apply writes the config and wayfire's reload picks the theme up;
		// there is no imperative call to make before that path exists.
		return fmt.Errorf("act %s: the cursor is a config block wayfire reads on reload", act)
	}

	// A known action this compositor cannot perform names the capability, so
	// a caller that skipped the gate gets told which one to check.
	if capability := act.Capability(); capability != "" {
		return fmt.Errorf("act %s: wayfire does not support %s", act, capability)
	}
	return fmt.Errorf("act: unknown action %q", act)
}

// perform sends one request and lets the shared client turn an error reply
// into the failure, so no action here can mistake a refusal for success.
func perform(method string, data any) error {
	_, err := request(method, data)
	return err
}

// arg names the missing value, so a bad keybind reports it instead of an index.
func arg(args []string, i int, name string) (string, error) {
	if i >= len(args) || strings.TrimSpace(args[i]) == "" {
		return "", fmt.Errorf("act: missing %s", name)
	}
	return args[i], nil
}

func argID(args []string, i int, name string) (int64, error) {
	s, err := arg(args, i, name)
	if err != nil {
		return 0, err
	}
	return windowID(s)
}

// windowID resolves the handle a window action takes: a numeric id, or the
// literal focused a command row spells for the window under the seat. The
// keyword is explicit on purpose: a missing or malformed id still fails, so
// an action can never silently fall back to whatever holds focus.
func windowID(s string) (int64, error) {
	if t := strings.TrimSpace(s); strings.EqualFold(t, "focused") {
		v, ok, err := readFocusedView()
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, fmt.Errorf("act: no window is focused")
		}
		return v.ID, nil
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("act: window id must be numeric or the word focused, got %q", s)
	}
	return n, nil
}

func viewByID(id int64) (wayfireView, error) {
	views, err := readViews()
	if err != nil {
		return wayfireView{}, err
	}
	for _, v := range views {
		if v.ID == id {
			return v, nil
		}
	}
	return wayfireView{}, fmt.Errorf("act: window %d does not exist", id)
}

// newestView prefers the window with the freshest focus timestamp, which is
// the one the summon and app-focus keys mean when several share a title or
// an app id.
func newestView(match func(wayfireView) bool) (wayfireView, bool, error) {
	views, err := readViews()
	if err != nil {
		return wayfireView{}, false, err
	}
	var best wayfireView
	found := false
	for _, v := range views {
		if match(v) && (!found || v.LastFocusTimestamp > best.LastFocusTimestamp) {
			best, found = v, true
		}
	}
	return best, found, nil
}

// focusedCell is the cell the summon target and the cycle step are measured
// from: the focused output's own grid position, read fresh each time.
func focusedCell() (wayfireOutput, int, int, error) {
	name, err := readFocusedOutput()
	if err != nil {
		return wayfireOutput{}, 0, 0, err
	}
	if name == "" {
		return wayfireOutput{}, 0, 0, fmt.Errorf("act: no focused output")
	}
	outputs, err := readOutputs()
	if err != nil {
		return wayfireOutput{}, 0, 0, err
	}
	for _, out := range outputs {
		if out.Name == name {
			return out, out.Workspace.X, out.Workspace.Y, nil
		}
	}
	return wayfireOutput{}, 0, 0, fmt.Errorf("act: focused output %q is gone", name)
}

// workspaceTarget resolves a handle the state frames published to the set
// and cell behind it, failing on an id the compositor would not recognise
// rather than focusing whatever is nearest.
func workspaceTarget(id string) (wayfireWset, int, int, error) {
	wsetIdx, x, y, ok := parseWorkspaceHandle(id)
	if !ok {
		return wayfireWset{}, 0, 0, fmt.Errorf("act: workspace id %q is not set:cell:cell", id)
	}
	wsets, err := readWsets()
	if err != nil {
		return wayfireWset{}, 0, 0, err
	}
	for _, w := range wsets {
		if w.Index == wsetIdx {
			return w, x, y, nil
		}
	}
	return wayfireWset{}, 0, 0, fmt.Errorf("act: workspace set %d does not exist", wsetIdx)
}

// moveViewToCell sends a window to a cell and optionally follows it there.
// A view from another set changes sets first, because vswitch moves inside
// one set only.
func moveViewToCell(id int64, handle string, focus bool) error {
	target, x, y, err := workspaceTarget(handle)
	if err != nil {
		return err
	}
	view, err := viewByID(id)
	if err != nil {
		return err
	}
	if view.WsetIndex != target.Index {
		if err := perform("wsets/send-view-to-wset", map[string]any{
			"view-id": id, "wset-index": target.Index,
		}); err != nil {
			return err
		}
	}
	if err := perform("vswitch/send-view", map[string]any{
		"view-id": id, "x": x, "y": y,
	}); err != nil {
		return err
	}
	if focus {
		return perform("window-rules/focus-view", map[string]any{"id": id})
	}
	return nil
}

// moveFocusedWindowWorkspaceBy steps the window under the seat along its own
// workspace set, the same row-major-with-wrap order the pills and workspace.cycle
// use, and follows it there. Wayfire's grid has no native "send to previous/next
// workspace" method, so the moveWindowPrev/Next rows run this act; niri and
// Hyprland spell the same behaviour natively.
func moveFocusedWindowWorkspaceBy(act wm.Action, n int) error {
	s, err := readStage()
	if err != nil {
		return err
	}
	view, ok, err := readFocusedView()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("act %s: no window is focused", act)
	}
	ws, ok := s.wsetByIndex(view.WsetIndex)
	if !ok {
		return fmt.Errorf("act %s: window %d is on a workspace set that is gone", act, view.ID)
	}
	gridW, gridH := ws.Workspace.GridWidth, ws.Workspace.GridHeight
	if gridW <= 0 || gridH <= 0 {
		return fmt.Errorf("act %s: workspace set %q reports no grid", act, ws.Name)
	}
	x, y, _ := s.cellOfView(view)
	steps := gridW * gridH
	flat := (((y*gridW + x + n) % steps) + steps) % steps
	return moveViewToCell(view.ID, workspaceHandle(ws.Index, flat%gridW, flat/gridW), true)
}

// directionArg reads the argument every directional act takes. A keybind row
// spells it, so a typo names itself instead of walking some other way.
func directionArg(args []string, i int) (string, error) {
	s, err := arg(args, i, "direction")
	if err != nil {
		return "", err
	}
	switch d := strings.ToLower(strings.TrimSpace(s)); d {
	case "left", "right", "up", "down":
		return d, nil
	default:
		return "", fmt.Errorf("act: direction must be left|right|up|down, got %q", s)
	}
}

// directionDelta maps a direction onto its unit vector on the plane.
func directionDelta(dir string) (float64, float64) {
	switch dir {
	case "left":
		return -1, 0
	case "right":
		return 1, 0
	case "up":
		return 0, -1
	case "down":
		return 0, 1
	}
	return 0, 0
}

func centerOf(r wayfireGeometry) (float64, float64) {
	return r.X + r.Width/2, r.Y + r.Height/2
}

// nearestInDirection picks the rect whose centre lies strictly in dir from
// the origin's centre and is closest to it, euclidean on the plane. False
// when nothing lies that way, and the caller names the absence rather than
// moving nowhere.
func nearestInDirection(rects []wayfireGeometry, origin wayfireGeometry, dir string) (int, bool) {
	dx, dy := directionDelta(dir)
	ox, oy := centerOf(origin)
	best, bestD := -1, 0.0
	for i, r := range rects {
		cx, cy := centerOf(r)
		sx, sy := cx-ox, cy-oy
		if sx*dx+sy*dy <= 0 {
			continue
		}
		d := sx*sx + sy*sy
		if best < 0 || d < bestD {
			best, bestD = i, d
		}
	}
	return best, best >= 0
}

// outputInDirection picks the neighbouring screen: its centre strictly in
// dir from the focused output's centre, nearest wins. A lone output can
// never point at another, which is the error the caller names.
func outputInDirection(outputs []wayfireOutput, from wayfireOutput, dir string) (wayfireOutput, bool) {
	rects := make([]wayfireGeometry, len(outputs))
	for i, o := range outputs {
		rects[i] = o.Geometry
	}
	idx, ok := nearestInDirection(rects, from.Geometry, dir)
	if !ok {
		return wayfireOutput{}, false
	}
	return outputs[idx], true
}

// clampF keeps a nudge inside its bounds; an empty range, a window wider
// than its cell, leaves it where it is rather than snapping it blind.
func clampF(v, lo, hi float64) float64 {
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

// candidatesIn gathers the windows a direction act may pick: mapped, not
// minimised, on the origin's own output and cell. A direction across the
// grid border is workspace navigation, not focus. The deck's scatter lives
// in rect(), so a card behind the front one still answers where the eye
// sees it.
func (s *stage) candidatesIn(origin wayfireView) []wayfireView {
	cellX, cellY, _ := s.cellOfView(origin)
	var out []wayfireView
	for _, v := range s.views {
		if !v.Mapped || v.Minimized || v.OutputName != origin.OutputName || v.WsetIndex != origin.WsetIndex {
			continue
		}
		x, y, _ := s.cellOfView(v)
		if x == cellX && y == cellY {
			out = append(out, v)
		}
	}
	return out
}

// windowPlane is what a window-level direction act measures against: the
// view's own rectangle, the output it lies on and the cell rectangle it is
// clamped inside.
type windowPlane struct {
	rect wayfireGeometry
	out  wayfireOutput
	cell wayfireGeometry
}

// planeOf resolves one view against the stage in the same shape cellOfView
// derives a cell, so the clamp and the cell occupancy can never disagree.
func (s *stage) planeOf(v wayfireView) (windowPlane, bool) {
	out, ok := s.outputByName(v.OutputName)
	if !ok {
		return windowPlane{}, false
	}
	cellX, cellY, _ := s.cellOfView(v)
	ws := out.Workspace
	if w, ok := s.wsetByIndex(v.WsetIndex); ok {
		ws = w.Workspace
	}
	cell := wayfireGeometry{
		X:      out.Geometry.X + float64(cellX-ws.X)*out.Geometry.Width,
		Y:      out.Geometry.Y + float64(cellY-ws.Y)*out.Geometry.Height,
		Width:  out.Geometry.Width,
		Height: out.Geometry.Height,
	}
	return windowPlane{rect: v.rect(), out: out, cell: cell}, true
}

// focusedWindow is the window the keybind acts direct at, read fresh: the
// seat's own answer, so a command row means exactly the window the user is
// typing into.
func focusedWindow(act wm.Action) (wayfireView, error) {
	v, ok, err := readFocusedView()
	if err != nil {
		return wayfireView{}, err
	}
	if !ok {
		return wayfireView{}, fmt.Errorf("act %s: no window is focused", act)
	}
	return v, nil
}

// focusedOutput is the screen the seat is on, read fresh for the screen acts
// that measure direction from it.
func focusedOutput(act wm.Action) (wayfireOutput, error) {
	name, err := readFocusedOutput()
	if err != nil {
		return wayfireOutput{}, err
	}
	if name == "" {
		return wayfireOutput{}, fmt.Errorf("act %s: no focused screen", act)
	}
	return readOutput(name)
}

// focusByDirection focuses the window nearest in that direction from the
// focused one, both measured on their rectangles.
func focusByDirection(act wm.Action, dir string) error {
	st, err := readStage()
	if err != nil {
		return err
	}
	origin, err := focusedWindow(act)
	if err != nil {
		return err
	}
	cands := st.candidatesIn(origin)
	rects := make([]wayfireGeometry, len(cands))
	for i, v := range cands {
		rects[i] = v.rect()
	}
	idx, ok := nearestInDirection(rects, origin.rect(), dir)
	if !ok {
		return fmt.Errorf("act %s: no window to the %s", act, dir)
	}
	return perform("window-rules/focus-view", map[string]any{"id": cands[idx].ID})
}

// focusAtEdge focuses the window at the far side of the cell: the leftmost
// centre for first, the rightmost for last. The window already there keeps
// focus, which is the answer a jump key gives everywhere else.
func focusAtEdge(act wm.Action, edge string) error {
	st, err := readStage()
	if err != nil {
		return err
	}
	origin, err := focusedWindow(act)
	if err != nil {
		return err
	}
	cands := st.candidatesIn(origin)
	if len(cands) == 0 {
		return fmt.Errorf("act %s: no window at the %s edge", act, edge)
	}
	best := cands[0]
	bestX, _ := centerOf(best.rect())
	for _, v := range cands[1:] {
		x, _ := centerOf(v.rect())
		if (edge == "left" && x < bestX) || (edge == "right" && x > bestX) {
			best, bestX = v, x
		}
	}
	return perform("window-rules/focus-view", map[string]any{"id": best.ID})
}

// moveFocusedBy slides the window an eighth of the screen that way, clamped
// inside its own cell: one press is one keyboard step, never a jump into the
// neighbouring cell. The slide clears the snap, the way a drag does.
func moveFocusedBy(act wm.Action, dir string) error {
	st, err := readStage()
	if err != nil {
		return err
	}
	origin, err := focusedWindow(act)
	if err != nil {
		return err
	}
	pl, ok := st.planeOf(origin)
	if !ok {
		return fmt.Errorf("act %s: the window's screen is gone", act)
	}
	dx, dy := directionDelta(dir)
	nr := pl.rect
	if dx != 0 {
		nr.X += dx * pl.out.Geometry.Width / 8
	} else {
		nr.Y += dy * pl.out.Geometry.Height / 8
	}
	nr.X = clampF(nr.X, pl.cell.X, pl.cell.X+pl.cell.Width-nr.Width)
	nr.Y = clampF(nr.Y, pl.cell.Y, pl.cell.Y+pl.cell.Height-nr.Height)
	return configurePlaced(origin.ID, nr)
}

// resizeFocusedBy steps the size an eighth of the screen from the top-left
// corner, with a floor so a window can never be pressed out of existence. A
// sized window leaves the snap too: wayfire's snap and the free rectangle
// are two states of one window, and this act picks the second.
func resizeFocusedBy(act wm.Action, step string) error {
	st, err := readStage()
	if err != nil {
		return err
	}
	origin, err := focusedWindow(act)
	if err != nil {
		return err
	}
	pl, ok := st.planeOf(origin)
	if !ok {
		return fmt.Errorf("act %s: the window's screen is gone", act)
	}
	nr := pl.rect
	switch step {
	case "narrower":
		nr.Width = math.Max(64, nr.Width-pl.out.Geometry.Width/8)
	case "wider":
		nr.Width += pl.out.Geometry.Width / 8
	case "shorter":
		nr.Height = math.Max(64, nr.Height-pl.out.Geometry.Height/8)
	case "taller":
		nr.Height += pl.out.Geometry.Height / 8
	}
	return configurePlaced(origin.ID, nr)
}

// presetFocusedHeight cycles the window through its preset heights as a
// fraction of the cell it sits in; first lands on the full height the reset
// key means. The width and the left edge stay put, and the window slides up
// only as far as it must to stay inside its cell.
func presetFocusedHeight(act wm.Action, first bool) error {
	st, err := readStage()
	if err != nil {
		return err
	}
	origin, err := focusedWindow(act)
	if err != nil {
		return err
	}
	pl, ok := st.planeOf(origin)
	if !ok {
		return fmt.Errorf("act %s: the window's screen is gone", act)
	}
	if pl.cell.Height <= 0 {
		return fmt.Errorf("act %s: screen %q reports no height", act, pl.out.Name)
	}
	height := pl.cell.Height
	if !first {
		presets := [3]float64{1, 0.75, 0.5}
		fraction := pl.rect.Height / pl.cell.Height
		nearest, bestD := 0, math.MaxFloat64
		for i, p := range presets {
			if d := math.Abs(fraction - p); d < bestD {
				nearest, bestD = i, d
			}
		}
		height = presets[(nearest+1)%len(presets)] * pl.cell.Height
	}
	nr := pl.rect
	nr.Height = height
	nr.Y = clampF(nr.Y, pl.cell.Y, pl.cell.Y+pl.cell.Height-height)
	return configurePlaced(origin.ID, nr)
}

// centerFocused centres the window on the screen it is on and lets go of the
// snap: the screen rather than the cell, because that is the surface the eye
// reads a centred window against.
func centerFocused(act wm.Action) error {
	st, err := readStage()
	if err != nil {
		return err
	}
	origin, err := focusedWindow(act)
	if err != nil {
		return err
	}
	pl, ok := st.planeOf(origin)
	if !ok {
		return fmt.Errorf("act %s: the window's screen is gone", act)
	}
	nr := pl.rect
	nr.X = pl.out.Geometry.X + (pl.out.Geometry.Width-nr.Width)/2
	nr.Y = pl.out.Geometry.Y + (pl.out.Geometry.Height-nr.Height)/2
	return configurePlaced(origin.ID, nr)
}

// configurePlaced is the one configure every geometry act sends: the new
// rectangle and the cleared snap land in the same transaction, the way
// placeWindow floats, moves and sizes in one.
func configurePlaced(id int64, r wayfireGeometry) error {
	return perform("window-rules/configure-view", map[string]any{
		"id": id,
		"geometry": map[string]any{
			"x":      r.X,
			"y":      r.Y,
			"width":  r.Width,
			"height": r.Height,
		},
		"tiled-edges": 0,
	})
}

// focusScreenByDirection lands the seat on the neighbouring screen through
// the window it last held there: focusing a view is what carries the seat
// across, since wayfire focuses an output by focusing one of its windows. A
// screen showing no window has nothing to hand the seat to.
func focusScreenByDirection(act wm.Action, dir string) error {
	outputs, err := readOutputs()
	if err != nil {
		return err
	}
	from, err := focusedOutput(act)
	if err != nil {
		return err
	}
	target, ok := outputInDirection(outputs, from, dir)
	if !ok {
		return fmt.Errorf("act %s: no screen to the %s", act, dir)
	}
	st, err := readStage()
	if err != nil {
		return err
	}
	found := false
	var best wayfireView
	for _, v := range st.views {
		if !v.Mapped || v.Minimized || v.OutputName != target.Name || v.WsetIndex != target.WsetIndex {
			continue
		}
		x, y, _ := st.cellOfView(v)
		if x != target.Workspace.X || y != target.Workspace.Y {
			continue
		}
		if !found || v.LastFocusTimestamp > best.LastFocusTimestamp {
			best, found = v, true
		}
	}
	if !found {
		return fmt.Errorf("act %s: screen %q shows no window", act, target.Name)
	}
	return perform("window-rules/focus-view", map[string]any{"id": best.ID})
}

// moveFocusedToScreen hands the focused window to the neighbouring screen;
// wayfire re-homes it there the way a cross-output workspace move does.
func moveFocusedToScreen(act wm.Action, dir string) error {
	origin, err := focusedWindow(act)
	if err != nil {
		return err
	}
	outputs, err := readOutputs()
	if err != nil {
		return err
	}
	from, err := focusedOutput(act)
	if err != nil {
		return err
	}
	target, ok := outputInDirection(outputs, from, dir)
	if !ok {
		return fmt.Errorf("act %s: no screen to the %s", act, dir)
	}
	return perform("window-rules/configure-view", map[string]any{
		"id": origin.ID, "output_id": target.ID,
	})
}

// moveCellToScreen sends the focused cell's whole contents to the
// neighbouring screen: the explicit move's traversal with direction standing
// in for the workspace handle.
func moveCellToScreen(act wm.Action, dir string) error {
	out, x, y, err := focusedCell()
	if err != nil {
		return err
	}
	outputs, err := readOutputs()
	if err != nil {
		return err
	}
	target, ok := outputInDirection(outputs, out, dir)
	if !ok {
		return fmt.Errorf("act %s: no screen to the %s", act, dir)
	}
	st, err := readStage()
	if err != nil {
		return err
	}
	return moveCellViewsToOutput(st, out.WsetIndex, x, y, target)
}

// moveCellViewsToOutput sends every view of one cell to the target output.
// The compositor's own cross-output move re-homes the view on the target
// output, which is as close as a per-output grid comes to a workspace
// travelling: the windows go, the cell belongs to the grid they land in.
func moveCellViewsToOutput(st *stage, wsetIdx int64, x, y int, target wayfireOutput) error {
	for _, view := range st.views {
		vx, vy, _ := st.cellOfView(view)
		if view.WsetIndex == wsetIdx && vx == x && vy == y {
			if err := perform("window-rules/configure-view", map[string]any{
				"id": view.ID, "output_id": target.ID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// placeWindow makes the window floating, moves it to the named output,
// sizes it and positions it in one configure-view: the ipc schedules a
// single transaction, so a half-placed window never flashes between steps.
// Coordinates arrive output-relative and go out global, which is the space
// wayfire positions in.
func placeWindow(args []string) error {
	id, x, y, width, height, output, err := placementArgs(args)
	if err != nil {
		return err
	}
	out, err := readOutput(output)
	if err != nil {
		return err
	}
	return perform("window-rules/configure-view", map[string]any{
		"id":        id,
		"output_id": out.ID,
		"geometry": map[string]any{
			"x":      out.Geometry.X + float64(x),
			"y":      out.Geometry.Y + float64(y),
			"width":  float64(width),
			"height": float64(height),
		},
		"tiled-edges": 0,
	})
}

func readOutput(name string) (wayfireOutput, error) {
	outputs, err := readOutputs()
	if err != nil {
		return wayfireOutput{}, err
	}
	for _, out := range outputs {
		if out.Name == name {
			return out, nil
		}
	}
	return wayfireOutput{}, fmt.Errorf("act: output %q does not exist", name)
}

// placementArgs validates the whole rectangle before any request goes out:
// an action arrives from a keybind or a script, and a negative width is the
// caller's bug, not something to hand to the compositor.
func placementArgs(args []string) (id int64, x, y, width, height int, output string, err error) {
	id, err = argID(args, 0, "window id")
	if err != nil {
		return
	}
	names := []string{"x", "y", "width", "height"}
	values := []*int{&x, &y, &width, &height}
	for i := range names {
		raw, argErr := arg(args, i+1, names[i])
		if argErr != nil {
			err = argErr
			return
		}
		*values[i], err = strconv.Atoi(raw)
		if err != nil {
			err = fmt.Errorf("act %s: %s must be an integer, got %q", wm.ActionWindowPlace, names[i], raw)
			return
		}
	}
	if width <= 0 || height <= 0 {
		err = fmt.Errorf("act %s: width and height must be positive, got %dx%d", wm.ActionWindowPlace, width, height)
		return
	}
	output, err = arg(args, 5, "output name")
	return
}

// wayfirePID finds the compositor that owns this session's socket. stat on
// the path names the filesystem node, not the socket inside it, so
// /proc/net/unix is what ties the bound path to the inodes answering on it;
// the owner is found by intersecting those with the process's fds, because
// a live client connection makes the accepted socket list the same path for
// as long as it lasts and the argv check keeps an fd inherited elsewhere
// from being signalled. The inode is what keeps a nested instance on
// another display out of it.
func wayfirePID() (int, error) {
	path := socketPath()
	if path == "" {
		return 0, fmt.Errorf("no wayfire socket")
	}
	candidates, err := listenerSockets(path)
	if err != nil {
		return 0, err
	}
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		proc := "/proc/" + e.Name()
		cmdline, err := os.ReadFile(proc + "/cmdline")
		if err != nil || len(cmdline) == 0 {
			continue
		}
		argv0 := strings.SplitN(string(cmdline), "\x00", 2)[0]
		if filepath.Base(argv0) != "wayfire" {
			continue
		}
		fds, err := os.ReadDir(proc + "/fd")
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(proc + "/fd/" + fd.Name())
			if err == nil && candidates[link] {
				return pid, nil
			}
		}
	}
	return 0, fmt.Errorf("no wayfire process owns %s", path)
}

// listenerSockets reads the sockfs inodes a bound unix path answers on,
// which are the fd names its processes show in /proc. /proc/net/unix lists
// an inode in column seven and the path as the tail of the line.
func listenerSockets(path string) (map[string]bool, error) {
	data, err := os.ReadFile("/proc/net/unix")
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasSuffix(line, " "+path) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		found["socket:["+fields[6]+"]"] = true
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no listener bound to %s", path)
	}
	return found, nil
}

// nightlightTemp parses the colour temperature, defaulting to 4000 K and
// clamping to the range the gamma client accepts, so a stray keybind
// argument can never ask for a value it would reject.
func nightlightTemp(args []string) int {
	t := 4000
	if len(args) > 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(args[0])); err == nil {
			t = v
		}
	}
	if t < 1000 {
		t = 1000
	}
	if t > 25000 {
		t = 25000
	}
	return t
}

// nightlightStart replaces any running backend with a fresh one warmed to
// the temperature. wlsunset with no location computes a polar-night
// trajectory and sits at its low temperature (-t) until killed, so the
// night light is exactly as warm as the user set, day or night; the fixed
// high (-T) only satisfies the validation that high must exceed low. The
// backend is detached (its own session, stdio to /dev/null, released) to
// outlive this short-lived invocation; wayfire restores the gamma through
// wlr-gamma-control when it goes away.
func nightlightStart(argv ...string) error {
	nightlightStop(argv[0])
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, null, null
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// nightlightStop signals every process of this uid whose comm is name,
// which is how nightlight.off stops the backend without a pkill fork.
// comm truncates at 15 characters; the backend name fits, so an exact
// compare is right.
func nightlightStop(name string) {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/comm")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(b)) == name {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
	}
}

type inputDevice struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// touchpadAct flips the pad live: wayfire's input ipc enables and disables
// devices without a config round trip, so status reads the device back
// rather than a state file, and restore has nothing to re-assert because a
// config reload leaves the device alone. The echo guard is shared with the
// siblings: an FN key that reports the press twice would toggle back.
func touchpadAct(args []string) error {
	mode := "toggle"
	if len(args) > 0 && args[0] != "" {
		mode = args[0]
	}
	pads, err := listTouchpads()
	if err != nil {
		return err
	}
	switch mode {
	case "status":
		if touchpadOn(pads) {
			fmt.Fprintln(stdout, "on")
		} else {
			fmt.Fprintln(stdout, "off")
		}
		return nil
	case "restore":
		return nil
	}
	if len(pads) == 0 {
		return fmt.Errorf("act input.touchpad: no touchpad found")
	}
	want := false
	switch mode {
	case "on", "enable":
		want = true
	case "off", "disable":
		want = false
	case "toggle":
		if wm.TouchpadToggleIsEcho(time.Now()) {
			return nil
		}
		want = !touchpadOn(pads)
	default:
		return fmt.Errorf("act input.touchpad: mode must be on|off|toggle|status|restore, got %q", mode)
	}
	for _, pad := range pads {
		if err := perform("input/configure-device", map[string]any{
			"id": pad.ID, "enabled": want,
		}); err != nil {
			return err
		}
	}
	if want {
		touchpadNotify("Touchpad", "On")
	} else {
		touchpadNotify("Touchpad", "Off")
	}
	return nil
}

func listTouchpads() ([]inputDevice, error) {
	raw, err := request("input/list-devices", nil)
	if err != nil {
		return nil, err
	}
	var devices []inputDevice
	if err := json.Unmarshal(raw, &devices); err != nil {
		return nil, fmt.Errorf("wayfire list-devices: bad reply %q", string(raw))
	}
	var pads []inputDevice
	for _, d := range devices {
		if isTouchpad(d) {
			pads = append(pads, d)
		}
	}
	return pads, nil
}

// isTouchpad reads the name because wayfire's device types only say
// pointer, which a mouse is too.
func isTouchpad(d inputDevice) bool {
	if d.Type != "pointer" {
		return false
	}
	name := strings.ToLower(d.Name)
	return strings.Contains(name, "touchpad") ||
		strings.Contains(name, "trackpad") ||
		strings.Contains(name, "synaptics")
}

func touchpadOn(pads []inputDevice) bool {
	for _, p := range pads {
		if !p.Enabled {
			return false
		}
	}
	return true
}

// touchpadNotify shows the toast the FN key gives. A var so the tests stay
// silent, and a swallowed error so a session with no notify-send is fine.
var touchpadNotify = func(title, body string) {
	_ = exec.Command("notify-send", "-a", "Ryoku", title, body).Run()
}

// setBorderPalette pushes the live palette's border colours into the running
// config: the ipc call emits wayfire's reload signal and the decoration
// plugin recolours the frame, without the file underneath the user changing.
func setBorderPalette(args []string) error {
	active, err := arg(args, 0, "active colour")
	if err != nil {
		return err
	}
	inactive, err := arg(args, 1, "inactive colour")
	if err != nil {
		return err
	}
	na, aok := normBorderHex(active)
	ni, iok := normBorderHex(inactive)
	if !aok && !iok {
		return fmt.Errorf("act %s: no usable colour in %q/%q", wm.ActionBorderColors, active, inactive)
	}
	// A fixed border colour is the user's own choice; a wallpaper change must
	// not override it, the store gate the sibling acts share.
	if !loadStore(storePath()).Appearance.BorderFollowsPalette {
		return nil
	}
	// The palette file is what the composer reads: wayfire reloads its config
	// on any write, so the colours have to land in the file as well as on the
	// wire or the next apply would snap back to the store colours.
	if err := writeBorderPalette(na, ni, aok, iok); err != nil {
		return err
	}
	colors := map[string]any{}
	if aok {
		colors["decoration/active_color"] = na + "ff"
	}
	if iok {
		colors["decoration/inactive_color"] = ni + "ff"
	}
	return perform("wayfire/set-config-options", colors)
}

// writeBorderPalette records the live palette's colours for the composer. A
// colour that failed normalisation keeps the file's previous value rather than
// blanking it, so one bad wallpaper hex cannot clear an otherwise good palette.
func writeBorderPalette(active, inactive string, aok, iok bool) error {
	prevA, prevI, _ := borderPaletteColors()
	if !aok {
		active = prevA
	}
	if !iok {
		inactive = prevI
	}
	body, err := json.Marshal(struct {
		Active   string `json:"active"`
		Inactive string `json:"inactive"`
	}{Active: active, Inactive: inactive})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(borderPalettePath()), 0o755); err != nil {
		return err
	}
	return atomicWrite(borderPalettePath(), body, 0o644)
}

// normBorderHex normalises a colour to "#rrggbb", accepting the same
// literal forms the sibling acts do. ok is false for anything that is not
// six hex digits, so a malformed colour is skipped rather than pushed.
func normBorderHex(s string) (string, bool) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) != 6 {
		return "", false
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", false
		}
	}
	return "#" + strings.ToLower(h), true
}

// isInternalOutput tells the built-in panel from an external screen the way
// the display tooling does, so the arrangement cycle knows which is which.
func isInternalOutput(name string) bool {
	u := strings.ToUpper(name)
	return strings.HasPrefix(u, "EDP") || strings.HasPrefix(u, "LVDS") || strings.HasPrefix(u, "DSI")
}

// cycleOutputs steps the arrangement one position: both on, internal only,
// external only, then back. wayfire's on and off are the output's mode
// option, so each step is a config push the compositor applies on the
// reload that push emits; the position lives in a state file because
// wayfire keeps no arrangement state to read. The target set is turned on
// before the other is turned off, so no step flashes every screen dark.
// With only one class of output present there is nothing to arrange, so
// every output is left on and the position reset rather than blanking the
// only screen.
func cycleOutputs() error {
	outputs, err := readOutputs()
	if err != nil {
		return err
	}
	var internal, external []string
	for _, out := range outputs {
		if isInternalOutput(out.Name) {
			internal = append(internal, out.Name)
		} else {
			external = append(external, out.Name)
		}
	}
	sort.Strings(internal)
	sort.Strings(external)

	setMode := func(names []string, mode string) error {
		for _, name := range names {
			if err := perform("wayfire/set-config-options", map[string]any{
				"output:" + name + "/mode": mode,
			}); err != nil {
				return err
			}
		}
		return nil
	}

	if len(internal) == 0 || len(external) == 0 {
		for _, out := range outputs {
			if err := setMode([]string{out.Name}, "auto"); err != nil {
				return err
			}
		}
		return writeCyclePosition(0)
	}

	pos := (readCyclePosition() + 1) % 3
	switch pos {
	case 1: // internal only
		if err := setMode(internal, "auto"); err != nil {
			return err
		}
		if err := setMode(external, "off"); err != nil {
			return err
		}
	case 2: // external only
		if err := setMode(external, "auto"); err != nil {
			return err
		}
		if err := setMode(internal, "off"); err != nil {
			return err
		}
	default: // both
		if err := setMode(internal, "auto"); err != nil {
			return err
		}
		if err := setMode(external, "auto"); err != nil {
			return err
		}
	}
	return writeCyclePosition(pos)
}

// outputCyclePath holds the persistent position of the arrangement cycle,
// since wayfire has no arrangement state of its own to read back.
func outputCyclePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "ryoku", "wayfire-output-cycle")
}

func readCyclePosition() int {
	b, err := os.ReadFile(outputCyclePath())
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n < 0 || n > 2 {
		return 0
	}
	return n
}

func writeCyclePosition(pos int) error {
	if err := os.MkdirAll(filepath.Dir(outputCyclePath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputCyclePath(), []byte(strconv.Itoa(pos)), 0o644)
}
