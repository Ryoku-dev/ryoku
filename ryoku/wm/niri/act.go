package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	wm "ryoku-wm"
)

// One neutral action id in, one niri request out. The only file allowed to
// spell a niri action name, which is what keeps the Hyprland provider a sibling
// file rather than a second shell.
//
// niri takes actions over the same socket as its queries, so an action is a
// struct here rather than a command string. The field names are niri's own and
// are checked by the compositor: an unknown action or a mistyped field comes
// back as an error rather than being ignored, which is why act_test pins them.
//
// Arity is checked rather than trusted: an action arrives from a keybind or a
// script, and a niri window action with a null id silently applies to the
// focused window.

// action wraps one niri action in the request envelope.
func action(name string, args any) map[string]any {
	return map[string]any{"Action": map[string]any{name: args}}
}

// niri resolves a workspace by stable id, by index or by name. Ids come back
// from the state frames and survive the renumbering that makes an index
// unreliable on a compositor where workspaces appear and disappear, so a
// numeric handle is always an id here. A non-numeric one is a configured name.
func workspaceRef(id string) map[string]any {
	if n, err := strconv.ParseUint(strings.TrimSpace(id), 10, 64); err == nil {
		return map[string]any{"Id": n}
	}
	return map[string]any{"Name": id}
}

func windowID(id string) (uint64, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(id), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("act: window id must be numeric, got %q", id)
	}
	return n, nil
}

func runAct(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("act: missing action id")
	}
	act := wm.Action(args[0])
	rest := args[1:]
	if !live() {
		return fmt.Errorf("act %s: no live niri session", act)
	}

	switch act {
	case wm.ActionCursorReassert:
		// The cursor is a config block on niri: regenerating the tree is the
		// re-assert, and niri watches its config file.
		return runApply([]string{storePath()})

	case wm.ActionWindowFocus:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		return perform(action("FocusWindow", map[string]any{"id": id}))

	case wm.ActionWindowClose:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		return perform(action("CloseWindow", map[string]any{"id": id}))

	case wm.ActionWindowFullscreen:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		return perform(action("FullscreenWindow", map[string]any{"id": id}))

	case wm.ActionWindowFloat:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		return perform(action("ToggleWindowFloating", map[string]any{"id": id}))

	case wm.ActionWindowMoveToWorkspace:
		id, err := argID(rest, 0, "window id")
		if err != nil {
			return err
		}
		ws, err := arg(rest, 1, "workspace id")
		if err != nil {
			return err
		}
		// Follows the window, matching what the same action does on Hyprland.
		return perform(action("MoveWindowToWorkspace", map[string]any{
			"window_id": id,
			"reference": workspaceRef(ws),
			"focus":     true,
		}))

	case wm.ActionWindowSummon:
		title, err := arg(rest, 0, "window title")
		if err != nil {
			return err
		}
		return summon(title)

	case wm.ActionAppFocus:
		appID, err := arg(rest, 0, "app id")
		if err != nil {
			return err
		}
		id, err := newestWindowOf(appID)
		if err != nil {
			return err
		}
		return perform(action("FocusWindow", map[string]any{"id": id}))

	case wm.ActionWorkspaceFocus:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return err
		}
		return perform(action("FocusWorkspace", map[string]any{"reference": workspaceRef(ws)}))

	case wm.ActionWorkspaceCycle:
		delta, err := arg(rest, 0, "delta")
		if err != nil {
			return err
		}
		n, convErr := strconv.Atoi(delta)
		if convErr != nil {
			return fmt.Errorf("act %s: delta must be an integer, got %q", act, delta)
		}
		return cycleWorkspace(n)

	case wm.ActionWorkspaceMoveToOutput:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return err
		}
		out, err := arg(rest, 1, "output name")
		if err != nil {
			return err
		}
		return perform(action("MoveWorkspaceToMonitor", map[string]any{
			"output":    out,
			"reference": workspaceRef(ws),
		}))

	case wm.ActionSessionExit:
		// The confirmation prompt is niri's own overlay; the desktop already
		// asked before calling this.
		return perform(action("Quit", map[string]any{"skip_confirmation": true}))

	case wm.ActionOutputPower:
		state, err := arg(rest, 0, "on|off")
		if err != nil {
			return err
		}
		if state != "on" && state != "off" {
			return fmt.Errorf("act %s: state must be on or off, got %q", act, state)
		}
		// niri powers every output together. Refuse a named one rather than
		// blanking more screens than the caller asked for.
		if len(rest) > 1 && strings.TrimSpace(rest[1]) != "" {
			return fmt.Errorf("act %s: niri powers all outputs together, so %q cannot be targeted alone", act, rest[1])
		}
		if state == "on" {
			return perform(action("PowerOnMonitors", map[string]any{}))
		}
		return perform(action("PowerOffMonitors", map[string]any{}))

	case wm.ActionKeyboardCycleLayout:
		return perform(action("SwitchLayout", map[string]any{"layout": "Next"}))

	case wm.ActionOverviewToggle:
		return perform(action("ToggleOverview", map[string]any{}))

	case wm.ActionNightLightOn:
		return nightlightStart("gammastep", "-m", "wayland", "-O", strconv.Itoa(nightlightTemp(rest)))

	case wm.ActionNightLightOff:
		nightlightStop("gammastep")
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
		switch state {
		case "on":
			return perform(outputRequest(conn, "On"))
		case "off":
			return perform(outputRequest(conn, "Off"))
		}
		return fmt.Errorf("act %s: state must be on or off, got %q", act, state)
	}

	// A known action this compositor cannot perform names the capability, so a
	// caller that skipped the gate gets told which one to check.
	if capability := act.Capability(); capability != "" {
		return fmt.Errorf("act %s: niri does not support %s", act, capability)
	}
	return fmt.Errorf("act: unknown action %q", act)
}

func perform(req any) error {
	_, err := request(req)
	return err
}

// outputRequest is niri's top-level Output request, not an Action: it turns one
// named connector on or off. The action is a unit variant, so it serialises as
// a bare string ("On" | "Off"), the shape act_test pins.
func outputRequest(name, action string) map[string]any {
	return map[string]any{"Output": map[string]any{"output": name, "action": action}}
}

// touchpadAct locks or unlocks the touchpad the FN touchpad key drives. niri has
// no runtime input IPC, so the lock is recorded in a state file and re-emitted
// into settings.kdl by writeInput; niri watches its config, so re-running the
// apply path here takes effect live and survives the next login.
func touchpadAct(args []string) error {
	mode := "toggle"
	if len(args) > 0 && args[0] != "" {
		mode = args[0]
	}
	off := touchpadDisabled()
	switch mode {
	case "status":
		if off {
			fmt.Fprintln(stdout, "off")
		} else {
			fmt.Fprintln(stdout, "on")
		}
		return nil
	case "on", "enable":
		off = false
	case "off", "disable":
		off = true
	case "toggle":
		off = !off
	case "restore":
		// The intent already lives in the config niri watches, so restore only
		// re-emits it. Silent, since it runs unattended at login.
	default:
		return fmt.Errorf("act input.touchpad: mode must be on|off|toggle|status|restore, got %q", mode)
	}
	if off {
		if err := os.MkdirAll(filepath.Dir(touchpadStatePath()), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(touchpadStatePath(), nil, 0o644); err != nil {
			return err
		}
	} else if err := os.Remove(touchpadStatePath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := writeOverlayKdl("settings.kdl", genSettings(loadStore(storePath()))); err != nil {
		return err
	}
	if mode != "restore" {
		if off {
			touchpadNotify("Touchpad", "Off")
		} else {
			touchpadNotify("Touchpad", "On")
		}
	}
	return nil
}

// touchpadStatePath records the niri provider's intended touchpad-off. niri has
// no runtime input IPC, so writeInput emits `off` under touchpad while this file
// exists, which is what carries the intent across the config reload niri does on
// its own.
func touchpadStatePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "ryoku", "touchpad-off")
}

// touchpadDisabled reports whether that intent is recorded, so writeInput can
// emit the off line without knowing where the file lives.
func touchpadDisabled() bool {
	_, err := os.Stat(touchpadStatePath())
	return err == nil
}

// setBorderPalette records the live palette's border colours and regenerates
// settings.kdl so niri re-reads it and recolours the frame, the niri twin of
// Hyprland's eval push. niri has no runtime config IPC, so this writes the
// palette file writeFrame reads and re-runs the apply path, exactly as the
// touchpad lock does. A no-op when the store fixes the border, so a wallpaper
// change never overrides a chosen colour.
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
	s := loadStore(storePath())
	if !s.Appearance.BorderFollowsPalette {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(borderPalettePath()), 0o755); err != nil {
		return err
	}
	pal := struct {
		Active   string `json:"active"`
		Inactive string `json:"inactive"`
	}{Active: na, Inactive: ni}
	body, err := json.Marshal(pal)
	if err != nil {
		return err
	}
	if err := atomicWrite(borderPalettePath(), body, 0o644); err != nil {
		return err
	}
	return writeOverlayKdl("settings.kdl", genSettings(s))
}

// normBorderHex normalises a colour to "#rrggbb", accepting the same literal
// forms the Hyprland act does ("#rrggbb" or "rrggbb"). ok is false for anything
// that is not six hex digits, so a malformed colour is skipped rather than
// written into the config.
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

// touchpadNotify shows the toast the FN key gives, matching the Hyprland side. A
// var so the tests stay silent, and a swallowed error so a session with no
// notify-send is fine.
var touchpadNotify = func(title, body string) {
	_ = exec.Command("notify-send", "-a", "Ryoku", title, body).Run()
}

// isInternalOutput tells the built-in panel from an external screen the same way
// the display tooling does, so the arrangement cycle knows which is which.
func isInternalOutput(name string) bool {
	u := strings.ToUpper(name)
	return strings.HasPrefix(u, "EDP") || strings.HasPrefix(u, "LVDS") || strings.HasPrefix(u, "DSI")
}

// cycleOutputs steps the arrangement one position: both on, internal only,
// external only, then back. niri has no arrangement state to read, so the
// position is kept in a state file and each step applies the on/off set over
// IPC. The target set is turned on before the other is turned off, so no step
// flashes every screen dark. With only one class of output present there is
// nothing to arrange, so every output is left on and the position reset rather
// than blanking the only screen.
func cycleOutputs() error {
	raw, err := request("Outputs")
	if err != nil {
		return err
	}
	var byName map[string]niriOutput
	if err := decode(raw, "Outputs", &byName); err != nil {
		return err
	}
	var internal, external []string
	for name := range byName {
		if isInternalOutput(name) {
			internal = append(internal, name)
		} else {
			external = append(external, name)
		}
	}
	sort.Strings(internal)
	sort.Strings(external)

	setState := func(names []string, on bool) error {
		verb := "Off"
		if on {
			verb = "On"
		}
		for _, name := range names {
			if err := perform(outputRequest(name, verb)); err != nil {
				return err
			}
		}
		return nil
	}

	if len(internal) == 0 || len(external) == 0 {
		for name := range byName {
			if err := perform(outputRequest(name, "On")); err != nil {
				return err
			}
		}
		return writeCyclePosition(0)
	}

	pos := (readCyclePosition() + 1) % 3
	switch pos {
	case 1: // internal only
		if err := setState(internal, true); err != nil {
			return err
		}
		if err := setState(external, false); err != nil {
			return err
		}
	case 2: // external only
		if err := setState(external, true); err != nil {
			return err
		}
		if err := setState(internal, false); err != nil {
			return err
		}
	default: // both
		if err := setState(internal, true); err != nil {
			return err
		}
		if err := setState(external, true); err != nil {
			return err
		}
	}
	return writeCyclePosition(pos)
}

// outputCyclePath holds the persistent position of the arrangement cycle, since
// niri has no arrangement state of its own to read back.
func outputCyclePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "ryoku", "niri-output-cycle")
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

// cycleWorkspace walks niri's vertical workspace order one step at a time.
// There is no relative reference to pass, and stepping is what niri's own binds
// do, so the loop is the action rather than a workaround.
func cycleWorkspace(delta int) error {
	name := "FocusWorkspaceDown"
	if delta < 0 {
		name, delta = "FocusWorkspaceUp", -delta
	}
	for range delta {
		if err := perform(action(name, map[string]any{})); err != nil {
			return err
		}
	}
	return nil
}

// newestWindowOf resolves an app id to a window, because niri focuses by id
// only. The most recently focused match wins, which is what a caller that knows
// only what it launched means by "focus it".
func newestWindowOf(appID string) (uint64, error) {
	wins, err := readWindows()
	if err != nil {
		return 0, err
	}
	best := niriWindow{}
	found := false
	for _, w := range wins {
		if !strings.EqualFold(w.AppID, appID) {
			continue
		}
		if !found || w.FocusTimestamp.after(best.FocusTimestamp) {
			best, found = w, true
		}
	}
	if !found {
		return 0, fmt.Errorf("act %s: no window with app id %q", wm.ActionAppFocus, appID)
	}
	return best.ID, nil
}

// summon raises an already-open window to the current workspace and focuses it,
// matched by exact title. niri focuses by id, so the title is resolved to a
// window here and the newest match wins, mirroring newestWindowOf. Ported from
// ryoku-summon: a single-instance app strands its window on the workspace it
// first opened on, and every Quickshell window shares one app id, so the title
// is the only handle. No match is an error, which lets the keybind fall through
// to launching the app. MoveWindowToWorkspace pulls the window onto the focused
// workspace and focuses it in one request, the shape window.moveToWorkspace pins.
func summon(title string) error {
	id, err := newestWindowByTitle(title)
	if err != nil {
		return err
	}
	ws, err := focusedWorkspaceID()
	if err != nil {
		return err
	}
	return perform(action("MoveWindowToWorkspace", map[string]any{
		"window_id": id,
		"reference": map[string]any{"Id": ws},
		"focus":     true,
	}))
}

// newestWindowByTitle resolves an exact window title to a window id, newest
// focused match first: the title twin of newestWindowOf.
func newestWindowByTitle(title string) (uint64, error) {
	wins, err := readWindows()
	if err != nil {
		return 0, err
	}
	best := niriWindow{}
	found := false
	for _, w := range wins {
		if w.Title != title {
			continue
		}
		if !found || w.FocusTimestamp.after(best.FocusTimestamp) {
			best, found = w, true
		}
	}
	if !found {
		return 0, fmt.Errorf("act %s: no window titled %q", wm.ActionWindowSummon, title)
	}
	return best.ID, nil
}

// focusedWorkspaceID reads the id of the workspace niri currently has focused,
// so summon can pull a window onto it by stable id rather than index.
func focusedWorkspaceID() (uint64, error) {
	wss, err := readWorkspaces()
	if err != nil {
		return 0, err
	}
	for _, ws := range wss {
		if ws.IsFocused {
			return ws.ID, nil
		}
	}
	return 0, fmt.Errorf("act %s: no focused workspace", wm.ActionWindowSummon)
}

// arg names the missing value, so a bad keybind reports it instead of an index.
func arg(args []string, i int, name string) (string, error) {
	if i >= len(args) || strings.TrimSpace(args[i]) == "" {
		return "", fmt.Errorf("act: missing %s", name)
	}
	return args[i], nil
}

func argID(args []string, i int, name string) (uint64, error) {
	s, err := arg(args, i, name)
	if err != nil {
		return 0, err
	}
	return windowID(s)
}

// decode unwraps one query payload, which niri nests under the request name.
func decode(raw json.RawMessage, key string, dst any) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("niri %s: %w", key, err)
	}
	body, ok := envelope[key]
	if !ok {
		return fmt.Errorf("niri %s: missing from reply", key)
	}
	return json.Unmarshal(body, dst)
}

// nightlightTemp parses the colour temperature, defaulting to 4000 K and
// clamping to the range the gamma client accepts, so a stray keybind argument
// can never ask for a value it would reject.
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

// nightlightStart replaces any running backend with a fresh one warmed to the
// temperature. gammastep -m wayland -O sets the temperature over
// wlr-gamma-control and pauses until killed, so it is detached (its own session,
// stdio to /dev/null, released) to outlive this short-lived invocation; niri
// restores the gamma when it goes away.
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

// nightlightStop signals every process of this uid whose comm is name, which is
// how nightlight.off stops the backend without a pkill fork. comm truncates at
// 15 characters; the backend name fits, so an exact compare is right.
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
