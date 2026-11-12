package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// apply and defaults: the write half of the seam for wayfire. apply composes
// wayfire.ini from the shipped defaults, the store and the seed files and
// reports what it could not express; defaults hands the Hub the baseline it
// overlays user values on. A write is followed by a reload of the running
// Wayfire (reloadRunning), so ReloadNeeded stays false: the caller has nothing
// left to do. With --preview apply writes nothing and only reports the losses,
// which is what `ryoku wm use wayfire` shows a user as the cost of switching.

// runApply composes wayfire.ini and prints an ApplyReport. With --preview it
// writes nothing (empty Written) but still walks the store for the unhonored
// list, so a switch can be previewed before it lands.
func runApply(args []string) error {
	preview := false
	storeArg := ""
	for _, a := range args {
		if a == "--preview" {
			preview = true
			continue
		}
		if storeArg == "" {
			storeArg = a
		}
	}
	if storeArg == "" {
		return fmt.Errorf("apply: missing store path")
	}
	s := loadStore(storeArg)

	rep := wm.ApplyReport{
		Provider:     wm.ProviderWayfire,
		Unhonored:    unhonored(storeArg),
		ReloadNeeded: false,
	}
	_, bindReport := resolveBinds(s)
	rep.Unhonored = append(rep.Unhonored, bindReport...)
	if preview {
		return encodeReport(rep)
	}
	if err := writeComposed(s); err != nil {
		return err
	}
	wm.PublishGreeterNumlock(s.Input.NumlockByDefault)
	rep.Written = []string{filepath.Join(wayfireConfigDir(), "wayfire.ini")}
	return encodeReport(rep)
}

// writeComposed is the one way a change reaches Wayfire: compose the whole
// file, write it, and have a running session read it. Values are never pushed
// over set-config-options, which locks each option it sets for the rest of
// the session, after which no reload and so no Hub save can change it again.
func writeComposed(s wayfireStore) error {
	body, err := compose(s)
	if err != nil {
		return err
	}
	if err := writeOverlayIni("wayfire.ini", body); err != nil {
		return err
	}
	return reloadRunning()
}

const reloadConfigMethod = "wayfire/reload-config-metadata"

// reloadRunning makes a running Wayfire read the config just written. Its own
// file watch is attached at startup to the directory it found, and anything
// that replaces ~/.config/wayfire (a checkout deploy, a factory reset) detaches
// it for the rest of the session, after which a save would land on disk and
// nowhere else. The reload method reopens the configured path. Off a session
// there is nothing to reload.
func reloadRunning() error {
	if !live() {
		return nil
	}
	return perform(reloadConfigMethod, map[string]any{})
}

func encodeReport(rep wm.ApplyReport) error {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// runDefaults prints the provider's default subtree of the neutral store.
func runDefaults() error {
	tree, err := splitStore(defaultStore())
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(tree)
}

// compose lays the layers over each other, later winning per key: the shipped
// defaults, the store's own sections, then the machine seeds with user.ini
// last. wayfire has no include, so this one composed file is the whole config.
// The shipped defaults are the session's spine and not a layer that may go
// missing: they carry the plugin list, the autostart rows and the baseline's
// own binds, so a compose without them writes a wayfire that boots bare, with
// no shell and no depthdeck, and looks like a render all the same. A missing
// seed stays skippable; a seed is one machine's own addition.
func compose(s wayfireStore) ([]byte, error) {
	defaultsPath := shareDefaultsPath()
	base, err := os.ReadFile(defaultsPath)
	if err != nil {
		return nil, fmt.Errorf("shipped wayfire defaults at %s (ryoku-desktop-wayfire ships them): %w", defaultsPath, err)
	}
	layers := []iniDoc{parseIni(base)}
	layers = append(layers, renderStoreIni(s))
	for _, seed := range []string{"monitors.ini", "keyboard.ini", "user.ini"} {
		if b, err := os.ReadFile(filepath.Join(wayfireConfigDir(), seed)); err == nil {
			layers = append(layers, parseIni(b))
		}
	}
	merged := mergeIni(layers...)
	adjustPlugins(&merged, s)
	applyBinds(&merged, s)
	applyLaunchEnvironment(&merged, s)
	mark := wm.GeneratedConfigBaselineMarker(wm.ProviderWayfire, base)
	body := renderIni(merged)
	out := make([]byte, 0, len(mark)+1+len(body))
	out = append(out, mark...)
	out = append(out, '\n')
	return append(out, body...), nil
}

// renderStoreIni is the store's own layer: every wayfire key the neutral and
// exclusive leaves map to, always in full, so the composed file carries the
// effective config rather than a diff. Colour and duration values arrive here
// already in wayfire's spelling.
func renderStoreIni(s wayfireStore) iniDoc {
	var d iniDoc

	a := s.Appearance
	w := s.Wayfire
	d.set("core", "vwidth", strconv.Itoa(w.Workspace.Columns))
	d.set("core", "vheight", strconv.Itoa(w.Workspace.Rows))
	d.set("core", "preferred_decoration_mode", w.Decoration.Mode)
	d.set("core", "xwayland", w.XWayland.Mode)
	d.set("core", "focus_buttons", w.Focus.Buttons)
	d.set("core", "focus_button_with_modifiers", boolText(w.Focus.WithModifiers))
	d.set("core", "focus_buttons_passthrough", boolText(w.Focus.PassThrough))

	// Tiling keeps Ryoku's dialogs and full-page apps (the Hub, file pickers)
	// floating: they are sized by the built-in window rules, which simple-tile
	// would otherwise override the moment it attaches them.
	tile := "none"
	if w.Tiling.TileByDefault != "none" {
		tile = tileByDefaultMatcher()
	}
	d.set("simple-tile", "tile_by_default", tile)
	d.set("simple-tile", "keep_fullscreen_on_adjacent", boolText(w.Tiling.KeepFullscreenOnAdjacent))
	d.set("simple-tile", "inner_gap_size", strconv.Itoa(a.GapsIn))
	d.set("simple-tile", "outer_horiz_gap_size", strconv.Itoa(a.GapsOut))
	d.set("simple-tile", "outer_vert_gap_size", strconv.Itoa(a.GapsOut))

	d.set("vswitch", "gap", strconv.Itoa(a.GapsWorkspaces))
	d.set("vswitch", "wraparound", w.Workspace.Wraparound)

	d.set("place", "mode", w.Placement.Mode)

	d.set("decoration", "border_size", strconv.Itoa(a.BorderSize))
	d.set("decoration", "title_height", strconv.Itoa(w.Decoration.TitleHeight))
	d.set("decoration", "font", w.Decoration.Font)
	d.set("decoration", "font_color", iniColor(w.Decoration.FontColor))
	d.set("decoration", "font_scale", numText(w.Decoration.FontScale))
	d.set("decoration", "button_order", w.Decoration.ButtonOrder)
	d.set("decoration", "button_scale", numText(w.Decoration.ButtonScale))
	d.set("decoration", "button_padding", strconv.Itoa(w.Decoration.ButtonPadding))
	d.set("decoration", "ignore_views", w.Decoration.IgnoreViews)
	d.set("decoration", "forced_views", w.Decoration.ForcedViews)
	active, inactive := a.ActiveBorder, a.InactiveBorder
	if a.BorderFollowsPalette {
		if pa, pi, ok := borderPaletteColors(); ok {
			if pa != "" {
				active = pa
			}
			if pi != "" {
				inactive = pi
			}
		}
	}
	d.set("decoration", "active_color", iniColor(active))
	d.set("decoration", "inactive_color", iniColor(inactive))

	if wayfirePluginAvailable("ryoku-decor") {
		d.set("ryoku-decor", "radius", strconv.Itoa(a.Rounding))
		d.set("ryoku-decor", "corner_power", numText(a.RoundingPower))
		d.set("ryoku-decor", "border_width", strconv.Itoa(a.BorderSize))
		d.set("ryoku-decor", "active_color", iniColor(active))
		d.set("ryoku-decor", "inactive_color", iniColor(inactive))
		d.set("ryoku-decor", "shadow_enabled", boolText(a.ShadowEnabled))
		d.set("ryoku-decor", "shadow_radius", strconv.Itoa(a.ShadowRange))
		d.set("ryoku-decor", "shadow_spread", strconv.Itoa(a.ShadowSpread))
		d.set("ryoku-decor", "shadow_offset_x", strconv.Itoa(a.ShadowOffsetX))
		d.set("ryoku-decor", "shadow_offset_y", strconv.Itoa(a.ShadowOffsetY))
		d.set("ryoku-decor", "shadow_color", iniShadowColor(a.ShadowColor))
		d.set("ryoku-decor", "ignore_views", w.Decoration.IgnoreViews)
	}

	d.set("blur", "blur_by_default", w.Blur.BlurByDefault)
	d.set("blur", "method", w.Blur.Method)
	d.set("blur", "saturation", numText(w.Blur.Saturation))
	d.set("blur", "alpha_threshold", numText(w.Blur.AlphaThreshold))
	d.set("blur", "alpha_exponent", numText(w.Blur.AlphaExponent))
	for _, method := range []string{"box", "gaussian", "kawase", "bokeh"} {
		d.set("blur", method+"_offset", strconv.Itoa(a.BlurSize))
		d.set("blur", method+"_degrade", strconv.Itoa(w.Blur.Degrade))
		d.set("blur", method+"_iterations", strconv.Itoa(a.BlurPasses))
	}

	d.set("wobbly", "friction", numText(w.Wobbly.Friction))
	d.set("wobbly", "spring_k", numText(w.Wobbly.SpringK))
	d.set("wobbly", "grid_resolution", strconv.Itoa(w.Wobbly.GridResolution))

	d.set("zoom", "modifier", w.Zoom.Modifier)
	d.set("zoom", "speed", numText(w.Zoom.Speed))
	d.set("zoom", "smoothing_duration", strconv.Itoa(w.Zoom.SmoothingMs)+"ms linear")
	d.set("zoom", "interpolation_method", w.Zoom.InterpolationMethod)

	d.set("fast-switcher", "inactive_alpha", numText(w.FastSwitcher.InactiveOpacity))
	in := s.Input
	d.set("input", "xkb_layout", in.KbLayout)
	d.set("input", "xkb_variant", in.KbVariant)
	d.set("input", "xkb_options", in.KbOptions)
	d.set("input", "kb_numlock_default_state", boolText(in.NumlockByDefault))
	d.set("input", "mouse_cursor_speed", numText(in.Sensitivity))
	d.set("input", "mouse_accel_profile", accelText(in.AccelProfile))
	d.set("input", "touchpad_cursor_speed", numText(in.Sensitivity))
	d.set("input", "touchpad_accel_profile", accelText(in.AccelProfile))
	d.set("input", "left_handed_mode", boolText(in.LeftHanded))
	d.set("input", "mouse_natural_scroll", boolText(in.MouseNaturalScroll))
	d.set("input", "mouse_scroll_speed", numText(in.MouseScrollFactor))
	d.set("input", "natural_scroll", boolText(in.NaturalScroll))
	d.set("input", "touchpad_scroll_speed", numText(in.TouchScrollFactor))
	d.set("input", "tap_to_click", boolText(in.TapToClick))
	d.set("input", "tap_and_drag", boolText(in.TapAndDrag))
	d.set("input", "click_method", clickText(in.Clickfinger))
	d.set("input", "middle_emulation", boolText(in.MiddleEmulation))
	d.set("input", "disable_touchpad_while_typing", boolText(in.DisableWhileTyping))
	d.set("input", "kb_repeat_rate", strconv.Itoa(in.RepeatRate))
	d.set("input", "kb_repeat_delay", strconv.Itoa(in.RepeatDelay))
	d.set("input", "cursor_theme", s.Cursor.Theme)
	d.set("input", "cursor_size", strconv.Itoa(s.Cursor.Size))

	// The touchpad workspace swipe is vswipe: both axes flip together with the
	// neutral toggle, the finger count rides along.
	swipe := boolText(in.WorkspaceSwipe)
	d.set("vswipe", "enable_vertical", swipe)
	d.set("vswipe", "enable_horizontal", swipe)
	d.set("vswipe", "fingers", strconv.Itoa(in.SwipeFingers))

	for i, cmd := range s.Autostart {
		d.set("autostart", fmt.Sprintf("ryoku_%02d", i), strings.TrimSpace(cmd.Command))
	}

	d.overlay(genWindowRules(s))

	anim := w.Animation
	d.set("vswitch", "duration", animText(anim.WorkspaceSwitch))
	d.set("animate", "duration", animText(anim.Window))
	d.set("grid", "duration", animText(anim.Snap))
	d.set("animate", "open_animation", windowAnimStyle(anim.OpenStyle, false))
	d.set("animate", "close_animation", windowAnimStyle(anim.CloseStyle, false))
	d.set("animate", "minimize_animation", windowAnimStyle(anim.MinimizeStyle, true))

	dd := s.Wayfire.Depthdeck
	d.set("depthdeck", "enabled", boolText(dd.Enabled))
	d.set("depthdeck", "scatter", boolText(dd.Scatter))
	d.set("depthdeck", "card_edge_scatter", boolText(dd.CardEdgeScatter))
	d.set("depthdeck", "card_scatter_reshuffle", boolText(dd.CardScatterReshuffle))
	d.set("depthdeck", "card_peek_min", numText(dd.CardPeekMin))
	d.set("depthdeck", "card_peek_max", numText(dd.CardPeekMax))
	d.set("depthdeck", "animation_ms", strconv.Itoa(dd.AnimationMs))
	d.set("depthdeck", "layer_1_opacity", numText(dd.Layer1Opacity))
	d.set("depthdeck", "layer_1_scale", numText(dd.Layer1Scale))
	d.set("depthdeck", "layer_2_opacity", numText(dd.Layer2Opacity))
	d.set("depthdeck", "layer_2_scale", numText(dd.Layer2Scale))
	d.set("depthdeck", "max_layers", strconv.Itoa(dd.MaxLayers))
	d.set("depthdeck", "maximized_front_scale", numText(dd.MaximizedFrontScale))

	return d
}

// adjustPlugins enforces the store toggles represented by plugin presence.
// This runs after the merge, so a seed's plugin list is still the shape of the
// list and only an off or unavailable plugin is carved out. Its options section
// goes with it so the generated config describes only plugins Wayfire will load.
func adjustPlugins(d *iniDoc, s wayfireStore) {
	decorAvailable := wayfirePluginAvailable("ryoku-decor")
	sections := d.sections[:0]
	for _, sec := range d.sections {
		disabled := sec.name == "animate" && !s.Appearance.Animations ||
			sec.name == "blur" && !s.Appearance.BlurEnabled ||
			sec.name == "wobbly" && !s.Appearance.WobblyWindows ||
			sec.name == "ryoku-decor" && !decorAvailable
		if !disabled {
			sections = append(sections, sec)
		}
	}
	for i := len(sections); i < len(d.sections); i++ {
		d.sections[i] = iniSection{}
	}
	d.sections = sections

	value, present := d.get("core", "plugins")
	if !present && !decorAvailable {
		return
	}
	listed := strings.Fields(value)
	plugins := make([]string, 0, len(listed)+1)
	haveDecor := false
	for _, p := range listed {
		if p == "animate" && !s.Appearance.Animations {
			continue
		}
		if p == "blur" && !s.Appearance.BlurEnabled {
			continue
		}
		if p == "wobbly" && !s.Appearance.WobblyWindows {
			continue
		}
		if p == "ryoku-decor" {
			if !decorAvailable || haveDecor {
				continue
			}
			haveDecor = true
		}
		plugins = append(plugins, p)
	}
	if decorAvailable && !haveDecor {
		at := len(plugins)
		for i, p := range plugins {
			if p == "autostart" {
				at = i
				break
			}
		}
		plugins = slices.Insert(plugins, at, "ryoku-decor")
	}
	// A list nothing changed keeps the defaults' own spelling.
	if !present || !slices.Equal(plugins, listed) {
		d.set("core", "plugins", strings.Join(plugins, " "))
	}

	if decorAvailable {
		// Keep the stock plugin for title bars and buttons on server-decorated
		// views, but its border would double Ryoku's ring.
		d.set("decoration", "border_size", "0")
	}
}

// applyLaunchEnvironment gives every process Wayfire starts the neutral session
// environment. An `export` in one autostart shell cannot mutate the compositor,
// so the effective variables prefix each autostart and command-plugin launch.
// App roles join the same prefix as BROWSER and TERMINAL, matching the sibling
// providers' environment blocks.
func applyLaunchEnvironment(d *iniDoc, s wayfireStore) {
	parts := make([]string, 0, len(s.Env)+2)
	for _, e := range s.Env {
		if validEnvName(e.Key) && !strings.ContainsRune(e.Value, 0) {
			parts = append(parts, shellWord(e.Key+"="+e.Value))
		}
	}
	for _, role := range []struct {
		key  string
		name string
	}{
		{key: "browser", name: "BROWSER"},
		{key: "terminal", name: "TERMINAL"},
	} {
		if value := strings.TrimSpace(s.Apps[role.key]); value != "" && !strings.ContainsRune(value, 0) {
			parts = append(parts, shellWord(role.name+"="+value))
		}
	}
	if len(parts) == 0 {
		return
	}
	prefix := "env " + strings.Join(parts, " ") + " "
	for i := range d.sections {
		section := &d.sections[i]
		if section.name != "autostart" && section.name != "command" {
			continue
		}
		for j := range section.keys {
			key := &section.keys[j]
			if section.name == "autostart" && key.name == "autostart_wf_shell" {
				continue
			}
			if section.name == "command" && !strings.HasPrefix(key.name, "command_") {
				continue
			}
			if strings.TrimSpace(key.value) != "" {
				key.value = prefix + key.value
			}
		}
	}
}

func validEnvName(name string) bool {
	if name == "" || !isEnvStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if !isEnvStart(c) && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func isEnvStart(c byte) bool {
	return c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func shellWord(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// iniColor normalises a stored colour to wayfire's eight-digit #rrggbbAA
// spelling, the form the set-config-options act already pushes and the config
// parser accepts. An unusable colour falls back to a neutral dark, so one bad
// row cannot poison the file with unparseable text.
func iniColor(s string) string {
	if h, ok := normBorderHex(s); ok {
		return strings.ToUpper(h) + "FF"
	}
	return "#000000FF"
}

// iniShadowColor keeps an explicit alpha and gives a six-digit store colour
// niri's default window-shadow alpha. The shared default is intentionally an
// opaque RGB value, while the rendered shadow must be translucent.
func iniShadowColor(s string) string {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) != 6 && len(h) != 8 {
		return "#00000070"
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "#00000070"
		}
	}
	if len(h) == 6 {
		h += "70"
	}
	return "#" + strings.ToUpper(h)
}

// animText renders an AnimKind as wayfire's duration spelling, the
// "<ms>ms <curve>" its duration options parse. An empty curve falls back to
// wayfire's own default easing rather than writing a bare number the parser
// would reject.
func animText(k AnimKind) string {
	curve := strings.TrimSpace(k.Curve)
	if curve == "" {
		curve = "circle"
	}
	ms := k.DurationMs
	if ms < 0 {
		ms = 0
	}
	return strconv.Itoa(ms) + "ms " + curve
}

func windowAnimStyle(style string, minimize bool) string {
	if minimize {
		switch style {
		case "none", "zoom", "squeezimize":
			return style
		}
		return "squeezimize"
	}
	switch style {
	case "none", "fade", "zoom", "fire", "zap", "spin":
		return style
	}
	return "zoom"
}

// boolText, numText, accelText and clickText spell the neutral scalars the way
// wayfire's options parse them: lower-case booleans, bare decimals, and the
// accel and click profiles whose empty neutral value means "the libinput
// default" rather than nothing at all.
func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func numText(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func accelText(p string) string {
	switch p {
	case "none", "flat", "adaptive":
		return p
	}
	return "default"
}

func clickText(clickfinger bool) string {
	if clickfinger {
		return "clickfinger"
	}
	return "button-areas"
}

// unhonored names every setting the store carries that wayfire cannot express.
// Each desktop.* leaf gets its own short, specific reason, because that list is
// the switch cost a user reads. A foreign compositor's whole namespace collapses
// to one line, since a per-key dump of another compositor's exclusives would
// bury the losses that matter under a hundred that never mattered.
func unhonored(storePath string) []wm.Unhonored {
	ns, ok := readNeutralStore(storePath)
	if !ok {
		return nil
	}
	var out []wm.Unhonored
	out = append(out, unhonoredLeaves(ns.Desktop["appearance"], "appearance", appearanceEmitted(), appearanceReason)...)
	out = append(out, unhonoredLeaves(ns.Desktop["input"], "input", inputEmitted, inputReason)...)
	out = append(out, unhonoredLeaves(ns.Desktop["cursor"], "cursor", cursorEmitted, cursorReason)...)
	out = append(out, unhonoredWindowRules(ns.Desktop["windowRules"])...)
	out = append(out, unhonoredKeybinds(ns.Desktop["keybinds"])...)
	out = append(out, unhonoredAppOverrides(ns.Desktop["appOverrides"])...)
	out = append(out, unhonoredDesktopMisc(ns.Desktop)...)
	out = append(out, unhonoredForeign(ns.WM)...)
	return out
}

// desktopHandled are the desktop.* keys this provider either emits or reports at
// a finer grain. Env and app roles prefix every Wayfire launch, while windows
// remains a whole top-level loss. The rebind tables are honoured by resolveBinds.
var desktopHandled = map[string]bool{
	"appearance": true, "input": true, "cursor": true,
	"windowRules": true, "appOverrides": true, "autostart": true,
	"keybinds": true, "keybindRebinds": true, "unbinds": true, "keybindDisabled": true,
	"env": true, "apps": true,
}

var appearanceEmittedWithoutDecor = map[string]bool{
	"gapsIn": true, "gapsOut": true, "gapsWorkspaces": true,
	"borderSize": true, "activeOpacity": true,
	"activeBorder": true, "inactiveBorder": true, "borderFollowsPalette": true,
	"animations": true, "blurEnabled": true, "blurSize": true, "blurPasses": true,
	"wobblyWindows": true,
}

var appearanceEmittedWithDecor = map[string]bool{
	"gapsIn": true, "gapsOut": true, "gapsWorkspaces": true,
	"borderSize": true, "activeOpacity": true,
	"activeBorder": true, "inactiveBorder": true, "borderFollowsPalette": true,
	"animations": true, "blurEnabled": true, "blurSize": true, "blurPasses": true,
	"wobblyWindows": true,
	"rounding":      true, "roundingPower": true,
	"shadowEnabled": true, "shadowRange": true, "shadowColor": true,
	"shadowOffsetX": true, "shadowOffsetY": true, "shadowSpread": true,
}

func appearanceEmitted() map[string]bool {
	if wayfirePluginAvailable("ryoku-decor") {
		return appearanceEmittedWithDecor
	}
	return appearanceEmittedWithoutDecor
}

func appearanceReason(leaf string) string {
	switch {
	case leaf == "rounding" || leaf == "roundingPower" ||
		leaf == "shadowEnabled" || leaf == "shadowRange" || leaf == "shadowColor" ||
		leaf == "shadowOffsetX" || leaf == "shadowOffsetY" || leaf == "shadowSpread":
		return "Ryoku's window frames for Wayfire are missing; reinstall Ryoku's Wayfire desktop to get them back."
	case strings.HasPrefix(leaf, "shadow"):
		return "Ryoku's window frames for Wayfire have no setting for this shadow option."
	case leaf == "inactiveOpacity":
		return "Wayfire can set one opacity when a window opens, but cannot change it when focus moves."
	case strings.HasSuffix(leaf, "Opacity") || leaf == "fullscreenOpacity":
		return "Wayfire can't change a window's opacity while it is open."
	case strings.HasPrefix(leaf, "glow"):
		return "Wayfire has no glow."
	case strings.HasPrefix(leaf, "dim"):
		return "Wayfire can't dim windows."
	case strings.HasPrefix(leaf, "blur"):
		return "Wayfire's blur has no matching setting."
	case leaf == "layout":
		return "Wayfire tiles every window one way and can't use Hyprland's other layouts."
	case leaf == "windowStyle":
		return "Wayfire has no window-open styles; set the window-open animation on the Animations page instead."
	case leaf == "animatedBorder" || leaf == "borderAngleSpeed":
		return "Wayfire's border colour can't move."
	}
	return "Wayfire has no matching appearance setting."
}

var inputEmitted = map[string]bool{
	"kbLayout": true, "kbVariant": true, "kbOptions": true, "numlockByDefault": true,
	"sensitivity": true, "accelProfile": true, "leftHanded": true,
	"mouseNaturalScroll": true, "mouseScrollFactor": true, "naturalScroll": true,
	"touchScrollFactor": true, "tapToClick": true,
	"tapAndDrag": true, "clickfinger": true, "middleEmulation": true,
	"disableWhileTyping": true, "repeatRate": true, "repeatDelay": true,
	"workspaceSwipe": true, "swipeFingers": true,
}

func inputReason(leaf string) string {
	switch leaf {
	case "followMouse":
		return "Wayfire focuses a window when you click it, but can't focus the window under the pointer."
	case "middleClickPaste":
		return "Wayfire can't turn off pasting with the middle button."
	case "swipeInvert", "swipeCreateNew", "swipeDistance":
		return "Wayfire's workspace swipe has no setting for direction, new workspaces or distance."
	}
	return "Wayfire has no matching input setting."
}

var cursorEmitted = map[string]bool{
	"theme": true, "size": true,
}

func cursorReason(leaf string) string {
	return "Wayfire can set the pointer's look and size, but can't hide it after a while or while you type."
}

// unhonoredLeaves reports each present leaf of a desktop.* object that wayfire
// does not emit, in a stable order, using reason for the user-facing why.
func unhonoredLeaves(raw json.RawMessage, section string, emitted map[string]bool, reason func(string) string) []wm.Unhonored {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		if !emitted[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]wm.Unhonored, 0, len(keys))
	for _, k := range keys {
		out = append(out, wm.Unhonored{Key: "desktop." + section + "." + k, Reason: reason(k)})
	}
	return out
}

func unhonoredWindowRules(raw json.RawMessage) []wm.Unhonored {
	if len(raw) == 0 {
		return nil
	}
	var rules []WindowRule
	if json.Unmarshal(raw, &rules) != nil {
		return nil
	}
	var out []wm.Unhonored
	for i, r := range rules {
		if _, why := renderRule(r.Class, r.Title, r.Action, r.Value); why != "" {
			out = append(out, wm.Unhonored{
				Key:    fmt.Sprintf("desktop.windowRules[%d]", i),
				Reason: why,
			})
		}
	}
	return out
}

// unhonoredKeybinds reports each stored bind wayfire cannot express, asking
// keybindWhy the same question genWayfireBinds asks before it emits, so a row
// is reported exactly when it is not written.
func unhonoredKeybinds(raw json.RawMessage) []wm.Unhonored {
	if len(raw) == 0 {
		return nil
	}
	var binds []Keybind
	if json.Unmarshal(raw, &binds) != nil {
		return nil
	}
	var out []wm.Unhonored
	for i, k := range binds {
		why := keybindWhy(k)
		if why == "" {
			continue
		}
		out = append(out, wm.Unhonored{
			Key:    fmt.Sprintf("desktop.keybinds[%d]", i),
			Reason: why,
		})
	}
	return out
}

func unhonoredAppOverrides(raw json.RawMessage) []wm.Unhonored {
	if len(raw) == 0 {
		return nil
	}
	var apps []AppOverride
	if json.Unmarshal(raw, &apps) != nil {
		return nil
	}
	var out []wm.Unhonored
	for i, a := range apps {
		if a.Opacity >= 0 && a.Opacity <= 1 {
			if _, ok := wayfireOverride(a); !ok {
				out = append(out, wm.Unhonored{
					Key:    fmt.Sprintf("desktop.appOverrides[%d]", i),
					Reason: "Wayfire's window rules need an app ID or a title to match.",
				})
				continue
			}
		}
		lost := overrideLost(a)
		if len(lost) == 0 {
			continue
		}
		out = append(out, wm.Unhonored{
			Key:    fmt.Sprintf("desktop.appOverrides[%d]", i),
			Reason: "Wayfire can't set " + joinWords(lost) + " for one app.",
		})
	}
	return out
}

// unhonoredDesktopMisc reports a top-level desktop.* key with no handler.
// Environment and app roles are handled by applyLaunchEnvironment; windows has
// no runtime corrector in Wayfire.
func unhonoredDesktopMisc(desktop map[string]json.RawMessage) []wm.Unhonored {
	keys := make([]string, 0)
	for k := range desktop {
		if !desktopHandled[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]wm.Unhonored, 0, len(keys))
	for _, k := range keys {
		out = append(out, wm.Unhonored{Key: "desktop." + k, Reason: desktopMiscReason(k)})
	}
	return out
}

func desktopMiscReason(key string) string {
	if key == "windows" {
		return "Wayfire can't put apps that open maximised back in a tile without taking them out of tiling."
	}
	return "Wayfire has no setting for this."
}

// unhonoredForeign collapses each non-wayfire namespace that carries content
// into a single line, keyed by the namespace and derived from the store so a
// third compositor works the same way.
func unhonoredForeign(wmns map[string]json.RawMessage) []wm.Unhonored {
	names := make([]string, 0, len(wmns))
	for name, raw := range wmns {
		if name == wm.ProviderWayfire {
			continue
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(raw, &m) != nil || len(m) == 0 {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]wm.Unhonored, 0, len(names))
	for _, name := range names {
		out = append(out, wm.Unhonored{
			Key:    "wm." + name,
			Reason: fmt.Sprintf("These %s settings have no Wayfire equivalent. They stay saved and come back if you switch back.", titleName(name)),
		})
	}
	return out
}

func titleName(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// joinWords lists words the way a sentence does: "a", "a and b", "a, b and c".
func joinWords(words []string) string {
	if len(words) < 2 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}
