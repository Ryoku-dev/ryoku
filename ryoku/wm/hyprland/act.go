package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	wm "ryoku-wm"
)

// One neutral action id in, one Hyprland dispatcher out. The only file allowed
// to spell a dispatcher name, which is what makes the niri provider a sibling
// file rather than a second shell.
//
// Everything speaks the Lua config provider's API, because that is the only
// parser Ryoku ships: `hyprctl dispatch <expr>` evaluates as hl.dispatch(<expr>),
// so a classic `dispatch workspace 9` is a Lua syntax error, and `hyprctl
// keyword` refuses outright ("keyword can't work with non-legacy parsers"). Live
// config changes therefore go through eval and hl.config.
//
// Arity is checked rather than trusted: an action arrives from a keybind or a
// script, and a dispatcher called with an empty selector silently applies to the
// focused window.

func runAct(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("act: missing action id")
	}
	action := wm.Action(args[0])
	rest := args[1:]
	if !live() {
		return fmt.Errorf("act %s: no live Hyprland session", action)
	}

	switch action {
	case wm.ActionWindowFocus:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.focus({ window = ` + luaStr("address:"+id) + ` })`)

	case wm.ActionWindowClose:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.window.close({ window = ` + luaStr("address:"+id) + ` })`)

	case wm.ActionWindowFullscreen:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.window.fullscreen({ window = ` + luaStr("address:"+id) + ` })`)

	case wm.ActionWindowFloat:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.window.float({ action = "toggle", window = ` + luaStr("address:"+id) + ` })`)

	case wm.ActionWindowPlace:
		return placeWindow(rest)

	case wm.ActionWindowMoveToWorkspace:
		id, err := arg(rest, 0, "window id")
		if err != nil {
			return err
		}
		ws, err := arg(rest, 1, "workspace id")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.window.move({ workspace = ` + luaStr(ws) + `, window = ` + luaStr("address:"+id) + ` })`)

	case wm.ActionWindowSummon:
		title, err := arg(rest, 0, "window title")
		if err != nil {
			return err
		}
		return summon(title)

	case wm.ActionAppFocus:
		class, err := arg(rest, 0, "app id")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.focus({ window = ` + luaStr("class:"+class) + ` })`)

	case wm.ActionWorkspaceFocus:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.focus({ workspace = ` + luaStr(ws) + ` })`)

	case wm.ActionWorkspaceCycle:
		delta, err := arg(rest, 0, "delta")
		if err != nil {
			return err
		}
		n, convErr := strconv.Atoi(delta)
		if convErr != nil {
			return fmt.Errorf("act %s: delta must be an integer, got %q", action, delta)
		}
		// r+N walks relative to the current workspace, the same selector the
		// shipped scroll binds use.
		sel := "r+" + strconv.Itoa(n)
		if n < 0 {
			sel = "r-" + strconv.Itoa(-n)
		}
		return dispatch(`hl.dsp.focus({ workspace = ` + luaStr(sel) + ` })`)

	case wm.ActionWorkspaceMoveToOutput:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return err
		}
		out, err := arg(rest, 1, "output name")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.workspace.move({ workspace = ` + luaStr(ws) + `, monitor = ` + luaStr(out) + ` })`)

	case wm.ActionWorkspaceToggleSpecial:
		name, err := arg(rest, 0, "special workspace name")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.workspace.toggle_special(` + luaStr(name) + `)`)

	case wm.ActionWindowHide:
		id, err := arg(rest, 0, "window id or focused")
		if err != nil {
			return err
		}
		if id == "focused" {
			// The workspace helper owns the focused case: a silent move still
			// pulls focus into the scratchpad, so it closes the special
			// workspace again and refocuses where the window came from.
			return runWorkspaceHelper("hide")
		}
		return dispatch(`hl.dsp.window.move({ workspace = "special:` + scratchName + `", window = ` + luaStr("address:"+id) + `, follow = false })`)

	case wm.ActionSessionExit:
		return dispatch(`hl.dsp.exit()`)

	case wm.ActionOutputPower:
		state, err := arg(rest, 0, "on|off")
		if err != nil {
			return err
		}
		if state != "on" && state != "off" {
			return fmt.Errorf("act %s: state must be on or off, got %q", action, state)
		}
		expr := `hl.dsp.dpms({ action = ` + luaStr(state)
		if len(rest) > 1 && rest[1] != "" {
			expr += `, monitor = ` + luaStr(rest[1])
		}
		return dispatch(expr + ` })`)

	case wm.ActionKeyboardCycleLayout:
		// A top-level hyprctl command, not a keyword, so it works under the Lua
		// parser unchanged. all, so a split or external board cannot drift.
		_, err := ctl("switchxkblayout", "all", "next")
		return err

	case wm.ActionSubmapEnter:
		name, err := arg(rest, 0, "submap name")
		if err != nil {
			return err
		}
		return dispatch(`hl.dsp.submap(` + luaStr(name) + `)`)

	case wm.ActionSubmapReset:
		return dispatch(`hl.dsp.submap("reset")`)

	case wm.ActionConfigReload:
		// config-only leaves monitors alone; a full reload re-applies output
		// config and can black a screen mid-update.
		if len(rest) > 0 && rest[0] == "config-only" {
			_, err := ctl("reload", "config-only")
			return err
		}
		_, err := ctl("reload")
		return err

	case wm.ActionConfigAutoreload:
		state, err := arg(rest, 0, "on|off")
		if err != nil {
			return err
		}
		// The knob is phrased as a disable, so invert here not in every caller.
		disable := "true"
		if state == "on" {
			disable = "false"
		}
		return evalLua(`hl.config({ misc = { disable_autoreload = ` + disable + ` } })`)

	case wm.ActionCursorSet:
		theme, err := arg(rest, 0, "cursor theme")
		if err != nil {
			return err
		}
		size, err := arg(rest, 1, "cursor size")
		if err != nil {
			return err
		}
		_, err = ctl("setcursor", theme, size)
		return err

	case wm.ActionCursorReassert:
		o := loadStore(desktopStorePath())
		_, err := ctl("setcursor", o.Cursor.Theme, strconv.Itoa(o.Cursor.Size))
		return err

	case wm.ActionCursorVisibility:
		state, err := arg(rest, 0, "hidden|visible")
		if err != nil {
			return err
		}
		if state != "hidden" && state != "visible" {
			return fmt.Errorf("act %s: state must be hidden or visible, got %q", action, state)
		}
		previous, err := cursorVisibility()
		if err != nil {
			return err
		}
		invisible := state == "hidden"
		if _, err := fmt.Fprintln(stdout, previous); err != nil {
			return err
		}
		return evalLua(fmt.Sprintf(`hl.config({ cursor = { invisible = %t } })`, invisible))

	case wm.ActionScreenShader:
		// A shader NAME, not a path: callers must not know the config layout.
		// Empty clears.
		name := ""
		if len(rest) > 0 {
			name = strings.TrimSpace(rest[0])
		}
		return evalLua(`hl.config({ decoration = { screen_shader = ` + luaStr(shaderPath(name)) + ` } })`)

	case wm.ActionFocusFollowsMouse:
		mode, err := arg(rest, 0, "follow mode")
		if err != nil {
			return err
		}
		n, convErr := strconv.Atoi(mode)
		if convErr != nil {
			return fmt.Errorf("act %s: mode must be 0..3, got %q", action, mode)
		}
		// Print the previous mode first so the caller restores it exactly;
		// follow_mouse is a mode, not a flag, so a hardcoded restore would
		// overwrite a deliberate non-default.
		if prev := followMouseMode(); prev != "" {
			fmt.Fprintln(stdout, prev)
		}
		return evalLua(`hl.config({ input = { follow_mouse = ` + strconv.Itoa(n) + ` } })`)

	case wm.ActionWorkspaceLayout:
		ws, err := arg(rest, 0, "workspace id")
		if err != nil {
			return err
		}
		layout, err := arg(rest, 1, "layout name")
		if err != nil {
			return err
		}
		return evalLua(`hl.workspace_rule({ workspace = ` + luaStr(ws) + `, layout = ` + luaStr(layout) + ` })`)

	case wm.ActionBorderColors:
		active, err := arg(rest, 0, "active colour")
		if err != nil {
			return err
		}
		inactive, err := arg(rest, 1, "inactive colour")
		if err != nil {
			return err
		}
		// A fixed border colour is the user's own choice; a wallpaper change must
		// not override it, so the live push is skipped when the store pins the
		// colours. genConfig already keeps col.active_border in settings.lua then.
		if !loadStore(desktopStorePath()).Appearance.BorderFollowsPalette {
			return nil
		}
		return setBorderColors(active, inactive)

	case wm.ActionGameMode:
		state, err := arg(rest, 0, "on|off")
		if err != nil {
			return err
		}
		switch state {
		case "on":
			return evalLua(`dofile(` + luaStr(filepath.Join(hyprModulesDir(), "game_mode.lua")) + `)`)
		case "off":
			// The caller clears the persisted request before restoring defaults.
			_, err := ctl("reload")
			return err
		}
		return fmt.Errorf("act %s: state must be on or off, got %q", action, state)

	case wm.ActionNightLightOn:
		return nightlightStart("hyprsunset", "-t", strconv.Itoa(nightlightTemp(rest)))

	case wm.ActionNightLightOff:
		nightlightStop("hyprsunset")
		return nil

	case wm.ActionInputTouchpad:
		return touchpad(rest)

	case wm.ActionOutputCycle:
		// Super+P's job: ryoku-monitor owns the mirror/extend arrangement and
		// pushes it live, so the seam steps it rather than re-deriving the layout.
		return runMonitor("toggle")

	case wm.ActionOutputEnable:
		conn, err := arg(rest, 0, "connector")
		if err != nil {
			return err
		}
		state, err := arg(rest, 1, "on|off")
		if err != nil {
			return err
		}
		// keyword is rejected by the Lua parser, so a connector is toggled
		// through eval against the hl.monitor API, the same call the display
		// tooling uses. Only `disabled` is sent: hl.monitor copies the output's
		// existing rule and reapplies just the fields it is given, so naming a
		// mode, position or scale here would reset the layout the user set and
		// turn a re-enable into a "preferred mode at auto position" rewrite.
		switch state {
		case "on":
			return evalLua(`hl.monitor({ output = ` + luaStr(conn) + `, disabled = false })`)
		case "off":
			return evalLua(`hl.monitor({ output = ` + luaStr(conn) + `, disabled = true })`)
		}
		return fmt.Errorf("act %s: state must be on or off, got %q", action, state)
	}

	return fmt.Errorf("act: unknown action %q", action)
}

// summon raises an already-open window to the current workspace and focuses it,
// matched by exact title. Ported from ryoku-summon: a single-instance app
// strands its window on whatever workspace it first opened on, and every
// Quickshell window shares one app id, so the title is the only handle. The live
// title is matched first and the initial title second, so a window that retitles
// itself is still found. No match is an error, which is what lets the keybind
// fall through to launching the app; a found window is always focused, so a
// partial raise never triggers a duplicate launch.
func summon(title string) error {
	raw, err := query("clients")
	if err != nil {
		return err
	}
	var clients []struct {
		Address      string `json:"address"`
		Title        string `json:"title"`
		InitialTitle string `json:"initialTitle"`
	}
	if err := json.Unmarshal(raw, &clients); err != nil {
		return err
	}
	addr := ""
	for _, c := range clients {
		if c.Title == title {
			addr = c.Address
			break
		}
	}
	if addr == "" {
		for _, c := range clients {
			if c.InitialTitle == title {
				addr = c.Address
				break
			}
		}
	}
	if addr == "" {
		return fmt.Errorf("act %s: no window titled %q", wm.ActionWindowSummon, title)
	}
	sel := luaStr("address:" + addr)
	// The active workspace is best effort, matching the script: the window is
	// only moved when the current workspace reads, then always focused.
	if raw, err := query("activeworkspace"); err == nil {
		var ws struct {
			ID int `json:"id"`
		}
		if json.Unmarshal(raw, &ws) == nil {
			_ = dispatch(`hl.dsp.window.move({ workspace = ` + strconv.Itoa(ws.ID) + `, window = ` + sel + ` })`)
		}
	}
	_ = dispatch(`hl.dsp.focus({ window = ` + sel + ` })`)
	return nil
}

// touchpad locks or unlocks every touchpad the FN touchpad key drives. Ported
// from ryoku-cmd-touchpad: the Lua config provider rejects `hyprctl keyword`, so
// device state is flipped through `hyprctl eval` against the same hl.device API
// the config uses, and the intended state is kept in a flag file because
// Hyprland exposes no per-device readback and re-enables every pad on a reload.
func touchpad(args []string) error {
	mode := "toggle"
	if len(args) > 0 && args[0] != "" {
		mode = args[0]
	}
	switch mode {
	case "status":
		if touchpadOff() {
			fmt.Fprintln(stdout, "off")
		} else {
			fmt.Fprintln(stdout, "on")
		}
		return nil
	case "on", "enable":
		return touchpadSet(true)
	case "off", "disable":
		return touchpadSet(false)
	case "toggle":
		if wm.TouchpadToggleIsEcho(time.Now()) {
			return nil
		}
		// A stored off means re-enable; otherwise lock it.
		return touchpadSet(touchpadOff())
	case "restore":
		// Login and reload re-enable every pad (the state is a runtime override,
		// not config), so a stored off is pushed back. A reload stays silent; at
		// login the lock is named once, so a pad left off is never a mystery.
		if !touchpadOff() {
			return nil
		}
		// Best-effort like the ported script: a transient device-list failure at
		// login must not turn an unattended re-assert into an error.
		if found, _ := setPads(false); found && len(args) > 1 && args[1] == "login" {
			touchpadNotify("Touchpad is locked off", "Press the touchpad key (Fn+F10 on most laptops) to turn it back on")
		}
		return nil
	}
	return fmt.Errorf("act input.touchpad: mode must be on|off|toggle|status|restore, got %q", mode)
}

// touchpadSet flips every pad and records the intent, matching the ported
// script: a machine with no touchpad says so instead of claiming a state.
func touchpadSet(enable bool) error {
	found, err := setPads(enable)
	if err != nil {
		return err
	}
	if !found {
		touchpadNotify("Touchpad", "No touchpad found")
		return nil
	}
	if enable {
		_ = os.Remove(touchpadStatePath())
		touchpadNotify("Touchpad", "On")
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(touchpadStatePath()), 0o755)
	_ = os.WriteFile(touchpadStatePath(), nil, 0o644)
	touchpadNotify("Touchpad", "Off")
	return nil
}

// setPads flips every touchpad to enabled. found is false when the machine has
// no touchpad. A single device that refuses is swallowed, so one bad pad does
// not leave the rest half-flipped.
func setPads(enable bool) (found bool, err error) {
	names, err := touchpadNames()
	if err != nil {
		return false, err
	}
	val := "false"
	if enable {
		val = "true"
	}
	for _, name := range names {
		_ = evalLua(`hl.device({ name = ` + luaStr(name) + `, enabled = ` + val + ` })`)
		found = true
	}
	return found, nil
}

// touchpadNames lists every touchpad libinput exposes. Its device name carries
// the "touchpad" suffix (asuf1204:00-2808:0201-touchpad), which the
// trackpoint/mouse half lacks, so the name is the filter.
func touchpadNames() ([]string, error) {
	raw, err := query("devices")
	if err != nil {
		return nil, err
	}
	var dev struct {
		Mice []struct {
			Name string `json:"name"`
		} `json:"mice"`
	}
	if err := json.Unmarshal(raw, &dev); err != nil {
		return nil, err
	}
	var names []string
	for _, m := range dev.Mice {
		if strings.Contains(strings.ToLower(m.Name), "touchpad") {
			names = append(names, m.Name)
		}
	}
	return names, nil
}

// touchpadOff reports whether an intended touchpad-off is stored.
func touchpadOff() bool {
	_, err := os.Stat(touchpadStatePath())
	return err == nil
}

// touchpadStatePath is the flag file holding the intended touchpad-off across a
// reload, since Hyprland re-enables every pad when it rereads its config.
func touchpadStatePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "ryoku", "touchpad.disabled")
}

// touchpadNotify shows the toast the ported script sent. A var so the pad tests
// stay silent, and a swallowed error so a session with no notify-send is fine.
var touchpadNotify = func(title, body string) {
	_ = exec.Command("notify-send", "-a", "Ryoku", title, body).Run()
}

// eval, not keyword: under the Lua config provider a reload re-runs
// decoration.lua and reverts col.active_border to the login value.
func setBorderColors(active, inactive string) error {
	var parts []string
	if rgb := hyprRGB(active); rgb != "" {
		parts = append(parts, `["col.active_border"]=`+luaStr(rgb))
	}
	if rgb := hyprRGB(inactive); rgb != "" {
		parts = append(parts, `["col.inactive_border"]=`+luaStr(rgb))
	}
	if len(parts) == 0 {
		return fmt.Errorf("act %s: no usable colour in %q/%q", wm.ActionBorderColors, active, inactive)
	}
	return evalLua("hl.config({general={" + strings.Join(parts, ",") + "}})")
}

// Empty for a non-hex value, so a missing role is skipped not mis-set.
func hyprRGB(hex string) string {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) != 6 {
		return ""
	}
	return "rgb(" + h + ")"
}

// shaderPath turns a shader name into the file the compositor loads. Empty name
// yields an empty string, which clears the shader.
func shaderPath(name string) string {
	if name == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "hypr", "shaders", name+".glsl")
}

// followMouseMode reports the mode the user configured, which is what the
// launcher hands back when it closes. It reads the settings store rather than
// the live option: a prior freeze that never restored (a crash mid-launch)
// leaves follow_mouse at 0 live, and reading that back would "restore" the
// stuck value instead of the user's setting. The store is the durable truth
// and its default matches the shipped input.lua.
func followMouseMode() string {
	return strconv.Itoa(loadStore(desktopStorePath()).Input.FollowMouse)
}

func cursorVisibility() (string, error) {
	out, err := ctl("getoption", "cursor:invisible", "-j")
	if err != nil {
		return "", err
	}
	var option struct {
		Bool bool `json:"bool"`
		Int  int  `json:"int"`
	}
	if err := json.Unmarshal(out, &option); err != nil {
		return "", fmt.Errorf("read cursor visibility: %w", err)
	}
	if option.Bool || option.Int != 0 {
		return "hidden", nil
	}
	return "visible", nil
}

// placeWindow applies the neutral output-local rectangle without ever falling
// back to the focused window. Hyprland's move coordinates are global logical
// coordinates, so the target output's logical origin is the only conversion.
// Client geometry and monitor origins share that coordinate space; applying the
// output scale again would double-scale a fractional output.
func placeWindow(args []string) error {
	id, x, y, width, height, output, err := windowPlacementArgs(args)
	if err != nil {
		return err
	}

	monitors, err := readMonitors()
	if err != nil {
		return fmt.Errorf("act %s: read outputs: %w", wm.ActionWindowPlace, err)
	}
	var target *hyprMonitor
	for i := range monitors {
		if monitors[i].Name == output {
			target = &monitors[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("act %s: output %q does not exist", wm.ActionWindowPlace, output)
	}
	if target.Scale <= 0 {
		return fmt.Errorf("act %s: output %q reported invalid scale %g", wm.ActionWindowPlace, output, target.Scale)
	}

	raw, err := query("clients")
	if err != nil {
		return fmt.Errorf("act %s: read windows: %w", wm.ActionWindowPlace, err)
	}
	var clients []struct {
		Address  string `json:"address"`
		Floating bool   `json:"floating"`
	}
	if err := json.Unmarshal(raw, &clients); err != nil {
		return fmt.Errorf("act %s: decode windows: %w", wm.ActionWindowPlace, err)
	}
	floating, found := false, false
	for _, client := range clients {
		if client.Address == id {
			floating, found = client.Floating, true
			break
		}
	}
	if !found {
		return fmt.Errorf("act %s: window id %q does not exist", wm.ActionWindowPlace, id)
	}

	sel := luaStr("address:" + id)
	if !floating {
		if err := dispatch(`hl.dsp.window.float({ action = "toggle", window = ` + sel + ` })`); err != nil {
			return fmt.Errorf("act %s: make window %q floating: %w", wm.ActionWindowPlace, id, err)
		}
	}
	if err := dispatch(`hl.dsp.window.move({ monitor = ` + luaStr(output) + `, window = ` + sel + ` })`); err != nil {
		return fmt.Errorf("act %s: move window %q to output %q: %w", wm.ActionWindowPlace, id, output, err)
	}
	if err := dispatch(`hl.dsp.window.resize({ x = ` + strconv.Itoa(width) + `, y = ` + strconv.Itoa(height) + `, exact = true, window = ` + sel + ` })`); err != nil {
		return fmt.Errorf("act %s: resize window %q: %w", wm.ActionWindowPlace, id, err)
	}
	globalX, globalY := target.X+x, target.Y+y
	if err := dispatch(`hl.dsp.window.move({ x = ` + strconv.Itoa(globalX) + `, y = ` + strconv.Itoa(globalY) + `, window = ` + sel + ` })`); err != nil {
		return fmt.Errorf("act %s: position window %q: %w", wm.ActionWindowPlace, id, err)
	}
	return nil
}

func windowPlacementArgs(args []string) (id string, x, y, width, height int, output string, err error) {
	id, err = arg(args, 0, "window id")
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

// arg names the missing value, so a bad keybind reports it instead of an index.
func arg(args []string, i int, name string) (string, error) {
	if i >= len(args) || strings.TrimSpace(args[i]) == "" {
		return "", fmt.Errorf("act: missing %s", name)
	}
	return args[i], nil
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
// temperature. The backend is detached (its own session, stdio to /dev/null,
// released) so it outlives this short-lived invocation and holds the gamma
// until it is killed; Hyprland restores the gamma when it goes away.
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

// scratchName is the special workspace the shipped scratchpad binds use; the
// workspace helper names it the same way.
const scratchName = "scratch"

// runWorkspaceHelper runs the shipped workspace helper the scratchpad binds
// call, so window.hide on the focused window behaves exactly like Super+H.
var runWorkspaceHelper = func(args ...string) error {
	return exec.Command(filepath.Join(hyprConfigDir(), "scripts", "ryoku-workspace"), args...).Run()
}
