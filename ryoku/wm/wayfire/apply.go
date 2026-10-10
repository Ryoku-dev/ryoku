package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// apply and defaults: the write half of the seam for wayfire. apply composes
// wayfire.ini from the shipped defaults, the store and the seed files and
// reports what it could not express; defaults hands the Hub the baseline it
// overlays user values on. wayfire watches its own config file
// (workarounds/auto_reload_config, shipped on in the defaults layer), so
// ReloadNeeded is always false and there is no reload action; with --preview
// apply writes nothing and only reports the losses, which is what
// `ryoku wm use wayfire` shows a user as the cost of switching.

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
	body, err := compose(s)
	if err != nil {
		return err
	}
	if err := writeOverlayIni("wayfire.ini", body); err != nil {
		return err
	}
	wm.PublishGreeterNumlock(s.Input.NumlockByDefault)
	rep.Written = []string{filepath.Join(wayfireConfigDir(), "wayfire.ini")}
	return encodeReport(rep)
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
	base, err := os.ReadFile(shareDefaultsPath)
	if err != nil {
		return nil, fmt.Errorf("shipped wayfire defaults at %s (ryoku-desktop-wayfire ships them): %w", shareDefaultsPath, err)
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
	return renderIni(merged), nil
}

// renderStoreIni is the store's own layer: every wayfire key the neutral and
// exclusive leaves map to, always in full, so the composed file carries the
// effective config rather than a diff. Colour and duration values arrive here
// already in wayfire's spelling.
func renderStoreIni(s wayfireStore) iniDoc {
	var d iniDoc

	a := s.Appearance
	d.set("decoration", "border_size", strconv.Itoa(a.BorderSize))
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

	in := s.Input
	d.set("input", "xkb_layout", in.KbLayout)
	d.set("input", "xkb_variant", in.KbVariant)
	d.set("input", "xkb_options", in.KbOptions)
	d.set("input", "kb_numlock_default_state", boolText(in.NumlockByDefault))
	d.set("input", "mouse_cursor_speed", numText(in.Sensitivity))
	d.set("input", "mouse_accel_profile", accelText(in.AccelProfile))
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

	anim := s.Wayfire.Animation
	d.set("vswitch", "duration", animText(anim.WorkspaceSwitch))
	d.set("animate", "duration", animText(anim.Window))
	d.set("grid", "duration", animText(anim.Snap))

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

// adjustPlugins enforces the two store toggles that are plugin presences rather
// than options: animations off drops animate from core/plugins, blur off drops
// blur. This runs after the merge, so a seed's plugin list is still the shape
// of the list and only the store's off switch can carve a plugin out of it; a
// store that leaves a plugin on never forces one back in.
func adjustPlugins(d *iniDoc, s wayfireStore) {
	if s.Appearance.Animations && s.Appearance.BlurEnabled {
		return
	}
	value, ok := d.get("core", "plugins")
	if !ok {
		return
	}
	keep := make([]string, 0, 16)
	for _, p := range strings.Fields(value) {
		if p == "animate" && !s.Appearance.Animations {
			continue
		}
		if p == "blur" && !s.Appearance.BlurEnabled {
			continue
		}
		keep = append(keep, p)
	}
	d.set("core", "plugins", strings.Join(keep, " "))
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
	case "flat", "adaptive":
		return p
	}
	return "default"
}

func clickText(clickfinger bool) string {
	if clickfinger {
		return "clickfinger"
	}
	return "buttonareas"
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
	out = append(out, unhonoredLeaves(ns.Desktop["appearance"], "appearance", appearanceEmitted, appearanceReason)...)
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
// a finer grain; a top-level key outside this set is reported whole. env, apps
// and windows have no wayfire spelling, so they stay out and get a reason of
// their own below. The rebind tables are honoured: resolveBinds folds them into
// the emission, so they leave this list with the old reason.
var desktopHandled = map[string]bool{
	"appearance": true, "input": true, "cursor": true,
	"windowRules": true, "appOverrides": true, "autostart": true,
	"keybinds": true, "keybindRebinds": true, "unbinds": true,
}

var appearanceEmitted = map[string]bool{
	"borderSize": true, "activeBorder": true, "inactiveBorder": true,
	"borderFollowsPalette": true, "animations": true, "blurEnabled": true,
}

func appearanceReason(leaf string) string {
	switch {
	case leaf == "gapsIn" || leaf == "gapsOut":
		return "wayfire windows float; there are no gaps between them."
	case leaf == "rounding" || leaf == "roundingPower":
		return "wayfire's decorations are square; there is no corner radius."
	case strings.HasSuffix(leaf, "Opacity") || leaf == "fullscreenOpacity":
		return "wayfire has no window opacity."
	case strings.HasPrefix(leaf, "shadow"):
		return "wayfire has no window shadows."
	case strings.HasPrefix(leaf, "glow"):
		return "wayfire has no glow."
	case strings.HasPrefix(leaf, "dim"):
		return "wayfire has no window dimming."
	case strings.HasPrefix(leaf, "blur"):
		return "wayfire's blur is tuned by method and iterations in the shipped config, not by size and passes."
	case leaf == "layout":
		return "wayfire has no tiling layout; windows float and snap."
	case leaf == "windowStyle":
		return "wayfire has no window-open style presets; set the window-open animation on the Animations page instead."
	case leaf == "wobblyWindows":
		return "wayfire has no wobbly-windows effect."
	case leaf == "animatedBorder" || leaf == "borderAngleSpeed":
		return "wayfire's border colour is static; there is no rotating-border animation."
	}
	return "wayfire has no matching appearance control."
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
		return "wayfire has no focus-follows-mouse mode."
	case "middleClickPaste":
		return "wayfire has no middle-click-paste switch."
	case "swipeInvert", "swipeCreateNew", "swipeDistance":
		return "wayfire's workspace swipe has no inversion, create-new or distance setting."
	}
	return "wayfire has no matching input control."
}

var cursorEmitted = map[string]bool{
	"theme": true, "size": true,
}

func cursorReason(leaf string) string {
	return "wayfire never hides the cursor, so there is no timeout or keypress hide."
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
					Reason: "wayfire window rules need an app id or title to match on.",
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
			Reason: "wayfire cannot set per-window " + strings.Join(lost, ", ") + ".",
		})
	}
	return out
}

// unhonoredDesktopMisc reports a top-level desktop.* key with no handler, each
// with the wayfire-specific why: env and apps ride the session rather than the
// config, windows has no runtime corrector here, and the rebind tables would
// have to rewrite the seed files' own bindings.
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
	switch key {
	case "env", "apps":
		return "wayfire takes its environment from the session, not from the store."
	case "windows":
		return "wayfire lets apps open themselves maximised; there is no tame-on-open."
	}
	return "wayfire has no setting for this."
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
			Reason: fmt.Sprintf("These %s-only settings have no wayfire equivalent. They stay in the store and return if you switch back.", titleName(name)),
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
