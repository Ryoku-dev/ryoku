package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// The neutral chord to wayfire's activator spelling, the catalogue onto
// wayfire's own option vocabulary, and the resolved emission apply lays into
// the composed config. The sibling of niri's config_binds.go: same inputs,
// wayfire's own output language.

// wayfireModifiers maps the neutral modifier tokens to wayfire's bracket
// spellings, lower-cased on the way in.
var wayfireModifiers = map[string]string{
	"super": "<super>",
	"ctrl":  "<ctrl>",
	"alt":   "<alt>",
	"shift": "<shift>",
}

// wayfireKeys maps the neutral keysym names to wayfire's KEY_ codes. Wayfire
// names the media and function keys by their linux input codes, so the XF86
// keysyms are explicit entries rather than a derived spelling, and the number
// pad's NumLock-off keysyms point at the same keypad code they type on: the
// key is the same key whichever level NumLock selects. The family placeholder
// and the wheel tokens have no activator spelling and fall through unmatched.
var wayfireKeys = map[string]string{
	"Return": "KEY_ENTER", "Enter": "KEY_ENTER",
	"Escape": "KEY_ESC", "Esc": "KEY_ESC",
	"Tab": "KEY_TAB", "BackSpace": "KEY_BACKSPACE", "Delete": "KEY_DELETE",
	"Insert": "KEY_INSERT", "Home": "KEY_HOME", "End": "KEY_END",
	"Prior": "KEY_PAGEUP", "Next": "KEY_PAGEDOWN",
	"PageUp": "KEY_PAGEUP", "PageDown": "KEY_PAGEDOWN",
	"Left": "KEY_LEFT", "Right": "KEY_RIGHT", "Up": "KEY_UP", "Down": "KEY_DOWN",
	"space": "KEY_SPACE", "Space": "KEY_SPACE",
	"grave": "KEY_GRAVE", "minus": "KEY_MINUS", "equal": "KEY_EQUAL",
	"comma": "KEY_COMMA", "period": "KEY_DOT", "slash": "KEY_SLASH",
	"backslash": "KEY_BACKSLASH", "semicolon": "KEY_SEMICOLON",
	"apostrophe":  "KEY_APOSTROPHE",
	"bracketleft": "KEY_LEFTBRACE", "bracketright": "KEY_RIGHTBRACE",
	"Print": "KEY_SYSRQ", "Pause": "KEY_PAUSE",
	"Caps_Lock": "KEY_CAPSLOCK", "Num_Lock": "KEY_NUMLOCK",
	"Scroll_Lock": "KEY_SCROLLLOCK",
	"KP_Enter":    "KEY_KPENTER", "KP_Add": "KEY_KPPLUS",
	"KP_Subtract": "KEY_KPMINUS", "KP_Multiply": "KEY_KPASTERISK",
	"KP_Divide": "KEY_KPSLASH", "KP_Decimal": "KEY_KPDOT",
	"KP_Separator": "KEY_KPDOT",

	"XF86AudioRaiseVolume": "KEY_VOLUMEUP", "XF86AudioLowerVolume": "KEY_VOLUMEDOWN",
	"XF86AudioMute": "KEY_MUTE", "XF86AudioPlay": "KEY_PLAY",
	"XF86AudioNext": "KEY_NEXTSONG", "XF86AudioPrev": "KEY_PREVSONG",
	"XF86MonBrightnessUp": "KEY_BRIGHTNESSUP", "XF86MonBrightnessDown": "KEY_BRIGHTNESSDOWN",
	"XF86TouchpadToggle": "KEY_TOUCHPADTOGGLE",
	"XF86TouchpadOn":     "KEY_TOUCHPADON", "XF86TouchpadOff": "KEY_TOUCHPADOFF",

	// The NumLock-off keypad keysyms, on the keypad code they belong to.
	"KP_End": "KEY_KP1", "KP_Down": "KEY_KP2", "KP_Next": "KEY_KP3",
	"KP_Left": "KEY_KP4", "KP_Begin": "KEY_KP5", "KP_Right": "KEY_KP6",
	"KP_Home": "KEY_KP7", "KP_Up": "KEY_KP8", "KP_Prior": "KEY_KP9",
	"KP_Insert": "KEY_KP0",
}

// toWayfireActivator converts a neutral chord ("SUPER + Shift + Left") into the
// activator wayfire parses ("<super> <shift> KEY_LEFT"). ok is false for a
// chord with no wayfire spelling, so apply reports it rather than writing a
// bind that never fires.
func toWayfireActivator(chord string) (string, bool) {
	parts := strings.Split(chord, "+")
	tokens := make([]string, 0, len(parts))
	for i, p := range parts {
		tok := strings.TrimSpace(p)
		if tok == "" {
			return "", false
		}
		if i < len(parts)-1 {
			m, ok := wayfireModifiers[strings.ToLower(tok)]
			if !ok {
				return "", false
			}
			tokens = append(tokens, m)
			continue
		}
		k, ok := wayfireKey(tok)
		if !ok {
			return "", false
		}
		tokens = append(tokens, k)
	}
	return strings.Join(tokens, " "), true
}

// wayfireKey spells the chord's final token: a modifier on its own (the bare
// "SUPER" chord), a mapped keysym, a letter or digit, a mouse button, or a
// function key. Wheel steps are deliberately absent: wayfire binds keys and
// buttons, and a scroll has no activator spelling.
func wayfireKey(tok string) (string, bool) {
	if m, ok := wayfireModifiers[strings.ToLower(tok)]; ok {
		return m, true
	}
	if k, ok := wayfireKeys[tok]; ok {
		return k, true
	}
	if b, ok := wayfireButtons[tok]; ok {
		return b, true
	}
	upper := strings.ToUpper(tok)
	if len(tok) == 1 {
		c := tok[0]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			return "KEY_" + upper, true
		}
	}
	if strings.HasPrefix(tok, "F") && len(tok) > 1 && len(tok) <= 3 {
		if _, err := strconv.Atoi(tok[1:]); err == nil {
			return "KEY_" + upper, true
		}
	}
	if strings.HasPrefix(tok, "KP_") {
		if d := strings.TrimPrefix(tok, "KP_"); len(d) == 1 && d[0] >= '0' && d[0] <= '9' {
			return "KEY_KP" + d, true
		}
	}
	return "", false
}

// wayfireButtons maps the neutral mouse tokens to wayfire's button codes.
var wayfireButtons = map[string]string{
	"mouse:272": "BTN_LEFT",
	"mouse:273": "BTN_RIGHT",
	"mouse:274": "BTN_MIDDLE",
}

// keybindWhy is the reason a store row has no wayfire spelling, or "" when it
// does. An unset chord is not a loss: the row is simply empty, the way the
// sibling writers treat it.
func keybindWhy(k Keybind) string {
	if strings.TrimSpace(k.Keys) == "" {
		return ""
	}
	if k.Action != "exec" {
		return fmt.Sprintf("wayfire binds shell commands; it cannot bind the %q action.", k.Action)
	}
	if strings.TrimSpace(k.Value) == "" {
		return "an exec bind needs a command to run."
	}
	if _, ok := toWayfireActivator(k.Keys); !ok {
		return fmt.Sprintf("wayfire cannot bind the chord %q.", k.Keys)
	}
	return ""
}

// wayfireBind is how wayfire expresses one catalogue entry. A row with a reason
// has no wayfire expression at all and the legend lists it unhonored rather
// than hiding it. A row with cmd is a command-plugin row: it lands under
// [command] as binding_<key> plus command_<key> (repeatable_binding_<key> when
// a held key should repeat it). A row with section and key is a native option
// wayfire already understands. extra is a second activator the shipped config
// carried on the same option and keeps. hint replaces the catalogue's own copy
// in the legend when the emission differs from what the id promises.
type wayfireBind struct {
	section, key string
	cmd          string
	repeatable   bool
	extra        string
	reason       string
	hint         string
}

// defaultBinds is the whole catalogue mapped onto wayfire's vocabulary: what
// wayfire performs natively, what it runs through the command plugin, and what
// it honestly cannot do. An id absent here is left off entirely, which would be
// a bug, so the tests assert full coverage.
func defaultBinds() map[string]wayfireBind {
	return map[string]wayfireBind{
		// Windows
		"window.close": {
			key: "window_close", extra: "<alt> KEY_F4",
			cmd: wm.IrisCloseCheck + " && qs -c shell ipc call closeConfirm trigger || ryoku-wm-wayfire act window.close focused",
		},
		"window.fullscreen":    {section: "wm-actions", key: "toggle_fullscreen"},
		"window.float":         {key: "window_float", cmd: "ryoku-wm-wayfire act window.float focused"},
		"window.pin":           {section: "wm-actions", key: "toggle_sticky"},
		"window.resize":        {reason: "wayfire has no resize mode; Super, Ctrl and the arrows step the size instead."},
		"window.presetHeight":  {key: "window_preset_height", cmd: "ryoku-wm-wayfire act window.presetHeight"},
		"column.tabbed":        {reason: "wayfire windows have no tabs; each view keeps its own cell."},
		"column.maximize":      {section: "wm-actions", key: "toggle_maximize"},
		"column.center":        {key: "column_center", cmd: "ryoku-wm-wayfire act window.center"},
		"window.focusPrevious": {section: "fast-switcher", key: "activate_forward"},

		// Focus. Each direction picks the nearest window whose centre lies
		// that way from the focused one, both measured on their rectangles;
		// first and last jump to the edge of the cell.
		"focus.left":   {key: "focus_left", cmd: "ryoku-wm-wayfire act window.focusDirection left"},
		"focus.right":  {key: "focus_right", cmd: "ryoku-wm-wayfire act window.focusDirection right"},
		"focus.up":     {key: "focus_up", cmd: "ryoku-wm-wayfire act window.focusDirection up"},
		"focus.down":   {key: "focus_down", cmd: "ryoku-wm-wayfire act window.focusDirection down"},
		"column.first": {key: "column_first", cmd: "ryoku-wm-wayfire act window.focusEdge left"},
		"column.last":  {key: "column_last", cmd: "ryoku-wm-wayfire act window.focusEdge right"},

		// Move. One press slides the window an eighth of the screen that way,
		// kept inside its own cell; the slide clears the snap.
		"move.left":         {key: "move_left", cmd: "ryoku-wm-wayfire act window.moveBy left"},
		"move.right":        {key: "move_right", cmd: "ryoku-wm-wayfire act window.moveBy right"},
		"move.up":           {key: "move_up", cmd: "ryoku-wm-wayfire act window.moveBy up"},
		"move.down":         {key: "move_down", cmd: "ryoku-wm-wayfire act window.moveBy down"},
		"column.mergeLeft":  {reason: "wayfire has no merge-between-windows action."},
		"column.mergeRight": {reason: "wayfire has no merge-between-windows action."},

		// Resize. One press steps the size an eighth of the screen from the
		// top-left corner, with a floor so a window cannot crush itself out
		// of reach; the step clears the snap too.
		"resize.narrower":    {key: "resize_narrower", cmd: "ryoku-wm-wayfire act window.resizeBy narrower"},
		"resize.wider":       {key: "resize_wider", cmd: "ryoku-wm-wayfire act window.resizeBy wider"},
		"resize.shorter":     {key: "resize_shorter", cmd: "ryoku-wm-wayfire act window.resizeBy shorter"},
		"resize.taller":      {key: "resize_taller", cmd: "ryoku-wm-wayfire act window.resizeBy taller"},
		"resize.resetHeight": {key: "resize_reset_height", cmd: "ryoku-wm-wayfire act window.presetHeight first", hint: "Give the window the full height of its workspace"},

		// Workspaces. The grid's nine cells take the number keys and the
		// number pad natively: each family lands on the same option, so both
		// spellings switch together.
		"workspace.focus":                   {section: "vswitch", key: "binding_{n}", hint: "Focus workspace 1 to 9; wayfire's grid holds nine, so the 0 key has no tenth to focus"},
		"workspace.moveWindow":              {section: "vswitch", key: "with_win_{n}", hint: "Send the window to workspace 1 to 9; there is no tenth to send it to"},
		"workspace.moveWindowSilent":        {section: "vswitch", key: "send_win_{n}", hint: "Send the window to workspace 1 to 9 without focusing it; there is no tenth"},
		"workspace.focus.numpad":            {section: "vswitch", key: "binding_{n}", hint: "Focus workspace 1 to 9 from the number pad; wayfire's grid holds nine, so the 0 key has no tenth to focus"},
		"workspace.moveWindow.numpad":       {section: "vswitch", key: "with_win_{n}", hint: "Send the window to workspace 1 to 9 from the number pad; there is no tenth to send it to"},
		"workspace.moveWindowSilent.numpad": {section: "vswitch", key: "send_win_{n}", hint: "Send the window to workspace 1 to 9 from the number pad without focusing it; there is no tenth"},
		"workspace.prev":                    {key: "workspace_prev", cmd: "ryoku-wm-wayfire act workspace.cycle -1"},
		"workspace.next":                    {key: "workspace_next", cmd: "ryoku-wm-wayfire act workspace.cycle 1"},
		"workspace.prevWheel":               {reason: "wayfire binds keys and mouse buttons, not wheel steps."},
		"workspace.nextWheel":               {reason: "wayfire binds keys and mouse buttons, not wheel steps."},
		"workspace.moveWindowPrev":          {key: "workspace_move_window_prev", cmd: "ryoku-wm-wayfire act window.moveToWorkspaceBy -1"},
		"workspace.moveWindowNext":          {key: "workspace_move_window_next", cmd: "ryoku-wm-wayfire act window.moveToWorkspaceBy 1"},
		"workspace.reorderUp":               {reason: "wayfire's workspace order is fixed by the grid."},
		"workspace.reorderDown":             {reason: "wayfire's workspace order is fixed by the grid."},
		"workspace.hideWindow":              {reason: "wayfire has no scratchpad workspace."},
		"workspace.scratchpad":              {reason: "wayfire has no scratchpad workspace."},
		"workspace.overview":                {key: "workspace_overview", cmd: "ryoku-shell overview"},
		"workspace.overviewDesktops":        {key: "workspace_overview_desktops", cmd: "ryoku-shell overview", hint: "Opens the overview; once inside, Alt+Left and Alt+Right step across desktops"},

		// Displays. The screen in that direction: focus the window it last
		// held, hand the focused window over, or send the whole cell across.
		"display.focus.left":          {key: "display_focus_left", cmd: "ryoku-wm-wayfire act output.focusDirection left", hint: "Focus the last window of the screen to the left; with one screen there is nowhere to go"},
		"display.focus.right":         {key: "display_focus_right", cmd: "ryoku-wm-wayfire act output.focusDirection right", hint: "Focus the last window of the screen to the right; with one screen there is nowhere to go"},
		"display.focus.up":            {key: "display_focus_up", cmd: "ryoku-wm-wayfire act output.focusDirection up", hint: "Focus the last window of the screen above; with one screen there is nowhere to go"},
		"display.focus.down":          {key: "display_focus_down", cmd: "ryoku-wm-wayfire act output.focusDirection down", hint: "Focus the last window of the screen below; with one screen there is nowhere to go"},
		"display.moveWindow.left":     {key: "display_move_window_left", cmd: "ryoku-wm-wayfire act window.moveToOutputBy left"},
		"display.moveWindow.right":    {key: "display_move_window_right", cmd: "ryoku-wm-wayfire act window.moveToOutputBy right"},
		"display.moveWindow.up":       {key: "display_move_window_up", cmd: "ryoku-wm-wayfire act window.moveToOutputBy up"},
		"display.moveWindow.down":     {key: "display_move_window_down", cmd: "ryoku-wm-wayfire act window.moveToOutputBy down"},
		"display.moveWorkspace.left":  {key: "display_move_workspace_left", cmd: "ryoku-wm-wayfire act workspace.moveToOutputBy left"},
		"display.moveWorkspace.right": {key: "display_move_workspace_right", cmd: "ryoku-wm-wayfire act workspace.moveToOutputBy right"},
		"display.moveWorkspace.up":    {key: "display_move_workspace_up", cmd: "ryoku-wm-wayfire act workspace.moveToOutputBy up"},
		"display.moveWorkspace.down":  {key: "display_move_workspace_down", cmd: "ryoku-wm-wayfire act workspace.moveToOutputBy down"},
		"display.cycle":               {key: "display_cycle", cmd: "ryoku-wm-wayfire act output.cycle"},

		// Apps
		"app.terminal": {key: "app_terminal", cmd: "ryoku-app terminal"},
		"app.files":    {key: "app_files", cmd: "ryoku-app files"},
		"app.browser":  {key: "app_browser", cmd: "ryoku-app browser"},
		"app.editor":   {key: "app_editor", cmd: "ryoku-app editor"},
		"app.notes":    {key: "app_notes", cmd: "ryoku-app notes"},
		"app.yazi":     {key: "app_yazi", cmd: "kitty -e yazi"},
		"app.ryotunes": {key: "app_ryotunes", cmd: "ryotunes"},

		// Shell
		"shell.launcher":          {key: "shell_launcher", cmd: "ryoku-shell launcher"},
		"shell.ask":               {key: "shell_ask", cmd: "ryoku-shell ask"},
		"shell.rashin":            {key: "shell_rashin", cmd: "ryoku-summon Rashin flock -n -o /tmp/rashin-app.lock rashin-app"},
		"shell.cheatsheet":        {key: "shell_cheatsheet", cmd: "pkill -x -f 'qs -c keys' 2>/dev/null || " + wm.QmlEnv + " flock -n -o /tmp/ryoku-keys.lock qs -c keys"},
		"shell.lock":              {key: "shell_lock", cmd: "ryoku-shell lock"},
		"shell.quicksettings":     {key: "shell_quicksettings", cmd: "ryoku-shell quicksettings"},
		"shell.wallpaper":         {key: "shell_wallpaper", cmd: "ryogami wallpaper ui"},
		"shell.wallpaperRandom":   {key: "shell_wallpaper_random", cmd: "ryogami wallpaper random"},
		"shell.ryovm":             {key: "shell_ryovm", cmd: "ryoku-summon ryovm " + wm.QmlEnv + " flock -n -o /tmp/ryovm.lock qs -c ryovm"},
		"shell.clipboard":         {key: "shell_clipboard", cmd: "ryoku-shell clipboard"},
		"shell.visualizer":        {key: "shell_visualizer", cmd: "ryoku-shell visualizer"},
		"shell.visualizerOverlay": {key: "shell_visualizer_overlay", cmd: "ryoku-shell visualizer-overlay"},
		"shell.visualizerPlace":   {key: "shell_visualizer_place", cmd: "ryoku-shell visualizer-place"},
		"shell.voice":             {key: "shell_voice", cmd: "ryoku-shell voice"},
		"shell.settings":          {key: "shell_settings", cmd: "ryoku-shell hub open"},
		"shell.screenshot":        {key: "shell_screenshot", cmd: wm.QmlEnv + " flock -n -o /tmp/ryoshot.lock qs -c ryoshot"},
		"shell.screenshotPrint":   {key: "shell_screenshot_print", cmd: wm.QmlEnv + " flock -n -o /tmp/ryoshot.lock qs -c ryoshot"},
		"shell.screenshotMonitor": {key: "shell_screenshot_monitor", cmd: wm.QmlEnv + " flock -n -o /tmp/ryoshot.lock env RYOSHOT_MODE=monitor qs -c ryoshot"},
		"shell.colorPicker":       {key: "shell_color_picker", cmd: "hyprpicker -a"},
		"shell.restartAudio":      {key: "shell_restart_audio", cmd: "ryoku-restart-audio"},
		"shell.inhibitShortcuts":  {reason: "wayfire has no keyboard-shortcuts inhibit."},

		// Media. The row repeats while the key is held, the way the shipped
		// config spelled its own volume rows.
		"media.volumeUp":   {key: "media_volume_up", cmd: "ryoku-volume up", repeatable: true},
		"media.volumeDown": {key: "media_volume_down", cmd: "ryoku-volume down", repeatable: true},
		"media.mute":       {key: "media_mute", cmd: "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle"},
		"media.play":       {key: "media_play", cmd: "playerctl play-pause"},
		"media.next":       {key: "media_next", cmd: "playerctl next"},
		"media.prev":       {key: "media_prev", cmd: "playerctl previous"},

		// Hardware
		"hardware.brightnessUp":   {key: "hardware_brightness_up", cmd: "ryoku-cmd-brightness +5", repeatable: true},
		"hardware.brightnessDown": {key: "hardware_brightness_down", cmd: "ryoku-cmd-brightness -5", repeatable: true},
		"hardware.touchpadToggle": {key: "hardware_touchpad_toggle", cmd: "ryoku-wm-wayfire act input.touchpad toggle"},
		"hardware.touchpadOn":     {key: "hardware_touchpad_on", cmd: "ryoku-wm-wayfire act input.touchpad on"},
		"hardware.touchpadOff":    {key: "hardware_touchpad_off", cmd: "ryoku-wm-wayfire act input.touchpad off"},

		// Mouse
		"mouse.move":   {section: "move", key: "activate"},
		"mouse.resize": {section: "resize", key: "activate"},
	}
}

// wayfireExclusive is one bind wayfire's own shipped config carries outside the
// catalogue. Its chord seeds the claim, so a custom bind or a rebind onto one
// is reported rather than firing twice over wayfire's own row, and the legend
// lists it under wayfire's name.
type wayfireExclusive struct {
	section, key string
	chord        string
	label        string
	hint         string
}

// wayfireExclusives is the shipped baseline's own binds: the grid, the
// window-taking workspace switches, the zoom modifier and the reverse window
// cycle. The chords mirror wayfire/wayfire.ini, which a test pins, so the
// legend and the claim can never drift from the config they describe.
func wayfireExclusives() []wayfireExclusive {
	return []wayfireExclusive{
		{section: "zoom", key: "modifier", chord: "SUPER", label: "Zoom", hint: "Hold Super and roll the wheel to magnify the desktop"},
		{section: "grid", key: "slot_bl", chord: "SUPER + CTRL + KP_1", label: "Tile bottom-left"},
		{section: "grid", key: "slot_b", chord: "SUPER + CTRL + KP_2", label: "Tile bottom"},
		{section: "grid", key: "slot_br", chord: "SUPER + CTRL + KP_3", label: "Tile bottom-right"},
		{section: "grid", key: "slot_l", chord: "SUPER + CTRL + KP_4", label: "Tile left"},
		{section: "grid", key: "slot_c", chord: "SUPER + CTRL + KP_5", label: "Tile centre"},
		{section: "grid", key: "slot_r", chord: "SUPER + CTRL + KP_6", label: "Tile right"},
		{section: "grid", key: "slot_tl", chord: "SUPER + CTRL + KP_7", label: "Tile top-left"},
		{section: "grid", key: "slot_t", chord: "SUPER + CTRL + KP_8", label: "Tile top"},
		{section: "grid", key: "slot_tr", chord: "SUPER + CTRL + KP_9", label: "Tile top-right"},
		{section: "grid", key: "restore", chord: "SUPER + CTRL + KP_0", label: "Restore the default geometry"},
		{section: "vswitch", key: "with_win_left", chord: "SUPER + CTRL + SHIFT + Left", label: "Send window to the workspace left"},
		{section: "vswitch", key: "with_win_down", chord: "SUPER + CTRL + SHIFT + Down", label: "Send window to the workspace down"},
		{section: "vswitch", key: "with_win_up", chord: "SUPER + CTRL + SHIFT + Up", label: "Send window to the workspace up"},
		{section: "vswitch", key: "with_win_right", chord: "SUPER + CTRL + SHIFT + Right", label: "Send window to the workspace right"},
		{section: "fast-switcher", key: "activate_backward", chord: "ALT + SHIFT + Tab", label: "Reverse window cycle"},
	}
}

// wayfireWorkspaceCount is the workspace grid's cells: the number keys hold
// one bind each and the tenth member of a family has nowhere to go.
const wayfireWorkspaceCount = 9

// outBind is one resolved bind, ready for the ini writer: the option to write
// (a native option, or the command plugin's binding_ with its command_ beside
// it) and the activator it accepts. An empty activator with an empty cmd is a
// native option being switched off.
type outBind struct {
	section    string
	option     string
	commandOpt string
	activator  string
	extra      string
	cmd        string
}

// effectiveChord resolves a default's emitted chord: the user's rebind when
// set, otherwise the shipped chord. The bool reports whether a rebind applied.
func effectiveChord(def string, rebinds map[string]string) (string, bool) {
	if v, ok := rebinds[def]; ok {
		if t := strings.TrimSpace(v); t != "" {
			return t, true
		}
	}
	return def, false
}

// resolveBinds expands the catalogue into the emitted bind set and folds the
// store's custom binds, rebinds and unbinds into it, resolving every wayfire
// activator to one winner: wayfire's own shipped bind, then a custom bind,
// then a rebound default, then a static default. A family expands to its nine
// workspace members; a workspace family's tenth member is reported instead of
// bound, and an unbound native option is written empty so it dies rather than
// surviving from the baseline layer. report names each behaviour wayfire could
// not honour and each chord that lost its claim.
func resolveBinds(s wayfireStore) ([]outBind, []wm.Unhonored) {
	unbind := map[string]bool{}
	for _, c := range s.Unbinds {
		if c = strings.TrimSpace(c); c != "" {
			unbind[c] = true
		}
	}

	claimed := map[string]string{}
	for _, ex := range wayfireExclusives() {
		if act, ok := toWayfireActivator(ex.chord); ok && claimed[act] == "" {
			claimed[act] = "wayfire's own " + ex.label
		}
	}

	defs := defaultBinds()
	var report []wm.Unhonored
	var out []outBind
	// A number-pad chord wants its NumLock-off twin too. Twins are held back so
	// every explicit chord claims first; on wayfire both keysyms land on the
	// same keypad code, so a twin of an already-emitted chord is a silent no-op
	// rather than a second write.
	type pendingTwin struct {
		reportKey string
		hubChord  string
		ob        outBind
	}
	var twins []pendingTwin
	emit := func(reportKey, owner, hubChord string, ob outBind, recordTwin bool) {
		activator, ok := toWayfireActivator(hubChord)
		if !ok {
			return
		}
		if taker := claimed[activator]; taker != "" {
			if taker != owner {
				report = append(report, wm.Unhonored{
					Key:    reportKey,
					Reason: fmt.Sprintf("the chord is already taken by %s.", taker),
				})
			}
			return
		}
		claimed[activator] = owner
		ob.activator = activator
		out = append(out, ob)
		if recordTwin && len(wm.NumpadAliases(hubChord)) > 0 {
			twins = append(twins, pendingTwin{reportKey, hubChord, ob})
		}
	}

	// Priority 1: a user custom bind wins every chord it takes. A row that
	// fails keybindWhy is skipped here and named by unhonoredKeybinds, which
	// asks the same question, so the two lists can never disagree.
	for i, k := range s.Keybinds {
		if strings.TrimSpace(k.Keys) == "" {
			continue
		}
		if keybindWhy(k) != "" {
			continue
		}
		ob := outBind{
			section:    "command",
			commandOpt: fmt.Sprintf("command_ryoku_%d", i),
			option:     fmt.Sprintf("binding_ryoku_%d", i),
			cmd:        strings.TrimSpace(k.Value),
		}
		if k.Release {
			ob.option = fmt.Sprintf("release_binding_ryoku_%d", i)
		}
		emit(fmt.Sprintf("desktop.keybinds[%d]", i), "a custom bind", k.Keys, ob, true)
	}

	cat := wm.ShippedBinds()

	// Behaviours wayfire cannot perform: report each once, unless the user
	// removed the chord anyway.
	for _, cb := range cat {
		wb := defs[cb.ID]
		if wb.reason == "" || unbind[cb.Chord] {
			continue
		}
		report = append(report, wm.Unhonored{
			Key:    fmt.Sprintf("desktop.keybinds (default %s)", cb.Chord),
			Reason: wb.reason,
		})
	}

	// The tenth member of a workspace family has no home: wayfire's grid is
	// three by three, so member 10 is reported once instead of bound to
	// nothing.
	for _, cb := range cat {
		wb := defs[cb.ID]
		if !cb.Family || wb.reason != "" || unbind[wm.ExpandChord(cb.Chord, 10)] {
			continue
		}
		report = append(report, wm.Unhonored{
			Key:    fmt.Sprintf("desktop.keybinds (default %s)", wm.ExpandChord(cb.Chord, 10)),
			Reason: "wayfire's workspace grid holds nine workspaces; there is no tenth.",
		})
	}

	// An unbound native option must still be written, empty, so the baseline
	// layer cannot keep it alive behind the user's back; seen once per option.
	seenOff := map[string]bool{}
	offNative := func(ob outBind) {
		k := ob.section + "." + ob.option
		if seenOff[k] {
			return
		}
		seenOff[k] = true
		out = append(out, ob)
	}

	// Priority 2 then 3: rebound defaults claim before static ones, so a rebind
	// onto another default's chord wins and the static default is dropped.
	claimDefaults := func(rebound bool) {
		for _, cb := range cat {
			wb := defs[cb.ID]
			if wb.reason != "" {
				continue
			}
			base := outBind{section: wb.section, extra: wb.extra, cmd: wb.cmd}
			switch {
			case wb.cmd != "":
				if base.section == "" {
					base.section = "command"
				}
				prefix := "binding_"
				if wb.repeatable {
					prefix = "repeatable_binding_"
				}
				base.option = prefix + wb.key
				base.commandOpt = "command_" + wb.key
			default:
				base.option = wb.key
			}
			// A family resolves through its family-level rebind first, so one
			// stored entry moves all its members; a per-member legacy rebind
			// (keyed on the shipped concrete chord) still wins below.
			famChord, famRebound := cb.Chord, false
			if cb.Family {
				famChord, famRebound = wm.FamilyRebind(cb.Chord, s.KeybindRebinds)
			}
			for idx, chord := range cb.Expand() {
				n := idx + 1
				if cb.Family && n > wayfireWorkspaceCount {
					continue
				}
				member := base
				if cb.Family {
					member.option = strings.ReplaceAll(base.option, "{n}", strconv.Itoa(n))
					member.commandOpt = strings.ReplaceAll(base.commandOpt, "{n}", strconv.Itoa(n))
				}
				if unbind[chord] {
					if wb.cmd == "" {
						offNative(member)
					}
					continue
				}
				to, isRebound := effectiveChord(chord, s.KeybindRebinds)
				if !isRebound && famRebound {
					to = wm.ExpandChord(famChord, n)
					isRebound = true
				}
				if isRebound != rebound {
					continue
				}
				emit(fmt.Sprintf("desktop.keybinds (default %s)", chord), "another shipped bind", to, member, true)
			}
		}
	}
	claimDefaults(true)
	claimDefaults(false)

	// Every explicit chord is claimed now, so lay down the NumLock-off twin of
	// each number-pad bind. A twin never displaces a bind a user set, and its
	// report stays silent: on wayfire it is the same keypad code again.
	for _, t := range twins {
		for _, alias := range wm.NumpadAliases(t.hubChord) {
			activator, ok := toWayfireActivator(alias)
			if !ok {
				continue
			}
			if claimed[activator] != "" {
				continue
			}
			claimed[activator] = "another shipped bind"
			ob := t.ob
			ob.activator = activator
			out = append(out, ob)
		}
	}

	return out, report
}

// joinBinds folds the resolved rows onto their slots: the number-pad family
// lands on the same option as the digit family, so several rows can share one
// key now and the last write must not erase the others. Activators join in
// first-seen order, a slot whose rows are all empty stays empty so it is
// still written off, and a row with a unique option passes through as it is,
// extra chord included.
func joinBinds(out []outBind) []outBind {
	joined := make([]outBind, 0, len(out))
	at := map[string]int{}
	for _, o := range out {
		k := o.section + "\x00" + o.option
		if i, seen := at[k]; seen {
			if o.activator != "" {
				joined[i].activator = strings.TrimSpace(joined[i].activator + " | " + o.activator)
			}
			continue
		}
		at[k] = len(joined)
		joined = append(joined, o)
	}
	return joined
}

// applyBinds lays the resolved binds into the composed config, after every
// layer, so the chords the legend will claim are the chords the session emits:
// the seeds and the baseline can no longer hide one.
func applyBinds(d *iniDoc, s wayfireStore) {
	out, _ := resolveBinds(s)
	for _, o := range joinBinds(out) {
		if o.commandOpt != "" {
			d.set(o.section, o.commandOpt, o.cmd)
		}
		value := o.activator
		if o.activator != "" && o.extra != "" {
			value = o.activator + " | " + o.extra
		}
		d.set(o.section, o.option, value)
	}
}

// bindRows is the legend apply claims: the shipped catalogue resolved against
// the user's rebinds, wayfire's own shipped binds under wayfire's name, then
// the store's custom rows. A row wayfire cannot perform keeps its chord and
// carries its reason, so the sheet can never promise a bind the session does
// not have; a behaviour with no wayfire expression is also not rebindable,
// since recording a new chord over something that does not exist would move
// nothing.
func bindRows(s wayfireStore) []wm.BindRow {
	defs := defaultBinds()
	rows := make([]wm.BindRow, 0, 128)

	for _, cb := range wm.ShippedBinds() {
		wb := defs[cb.ID]
		// A family keeps its {n} and resolves through its family-level rebind,
		// so the row carries the effective {n} chord and DisplayKeys renders
		// the range. A plain bind takes the user's rebind when set.
		eff, _ := effectiveChord(cb.Chord, s.KeybindRebinds)
		if cb.Family {
			eff, _ = wm.FamilyRebind(cb.Chord, s.KeybindRebinds)
		}
		hint := cb.Hint
		if wb.hint != "" {
			hint = wb.hint
		}
		row := wm.BindRow{
			ID:         cb.ID,
			Category:   cb.Category,
			Label:      cb.Label,
			Hint:       hint,
			Keys:       wm.DisplayKeys(eff),
			Default:    cb.Chord,
			Chord:      eff,
			Kind:       cb.Kind,
			Rebindable: wb.reason == "" && rebindable(cb),
			Locked:     cb.Locked,
		}
		if wb.reason != "" {
			row.Unhonored = wb.reason
		}
		rows = append(rows, row)
	}

	// wayfire's own shipped binds, titled with wayfire's name: the grid and
	// the workspace switches that carry a window, the zoom modifier and the
	// reverse window cycle. They belong to the baseline, so they rebind
	// nowhere.
	for _, ex := range wayfireExclusives() {
		rows = append(rows, wm.BindRow{
			ID:         "wayfire." + ex.section + "." + ex.key,
			Category:   "Wayfire",
			Label:      ex.label,
			Hint:       ex.hint,
			Keys:       wm.DisplayKeys(ex.chord),
			Default:    ex.chord,
			Chord:      ex.chord,
			Kind:       wm.BindCustom,
			Rebindable: false,
		})
	}

	for i, k := range s.Keybinds {
		chord := strings.TrimSpace(k.Keys)
		if chord == "" {
			continue // an empty row is not a bind, the way apply treats it
		}
		// The same question keybindWhy asks apply's report, so the legend and
		// the switch cost can never disagree on a custom row.
		rows = append(rows, wm.BindRow{
			ID:         fmt.Sprintf("custom.%d", i),
			Category:   "Custom",
			Label:      customLabel(k),
			Keys:       wm.DisplayKeys(chord),
			Default:    chord,
			Chord:      chord,
			Kind:       wm.BindCustom,
			Rebindable: false,
			Unhonored:  keybindWhy(k),
		})
	}

	return rows
}

// customLabel names a store row for the legend. wayfire's only expressible
// custom is a command, so an exec row names what it runs; any other action
// keeps its own name beside the reason it has no wayfire spelling.
func customLabel(k Keybind) string {
	if k.Action == "exec" || k.Action == "" {
		if v := strings.TrimSpace(k.Value); v != "" {
			return "Run: " + v
		}
		return "Run command"
	}
	return "Action: " + k.Action
}

// rebindable reports whether the Hub may let a user record a new chord over
// this bind. A workspace family is rebindable as a unit: the Hub records one
// chord and the store keeps the {n} placeholder, so all nine members move
// together. A media or hardware chord rides a dedicated key, so it stays
// fixed.
func rebindable(cb wm.CatalogBind) bool {
	for _, tok := range strings.Split(cb.Chord, " + ") {
		if strings.HasPrefix(tok, "mouse") || strings.HasPrefix(tok, "XF86") {
			return false
		}
	}
	return true
}

// runBinds prints the effective bind legend as a JSON array of wm.BindRow,
// read through the store so the chords reflect what the session actually
// emits.
func runBinds(args []string) error {
	storePath := ""
	if len(args) > 0 {
		storePath = args[0]
	}
	rows := bindRows(loadStore(storePath))
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}
