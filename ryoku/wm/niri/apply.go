package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	wm "ryoku-wm"
)

// apply and defaults: the write half of the seam for niri. apply renders the
// neutral store into settings.kdl + rebinds.kdl and reports what it could not
// express; defaults hands the Hub the baseline it overlays user values on. niri
// watches its own config, so ReloadNeeded is always false and there is no reload
// action; with --preview apply writes nothing and only reports the losses, which
// is what `ryoku wm use niri` shows a user as the cost of switching.

// runApply renders the store into settings.kdl + rebinds.kdl and prints an
// ApplyReport. With --preview it writes nothing (empty Written) but still walks
// the store for the unhonored list, so a switch can be previewed before it lands.
func runApply(args []string) error {
	preview := false
	storePath := ""
	for _, a := range args {
		if a == "--preview" {
			preview = true
			continue
		}
		if storePath == "" {
			storePath = a
		}
	}
	if storePath == "" {
		return fmt.Errorf("apply: missing store path")
	}

	s := loadStore(storePath)
	binds, bindUnhonored := genBinds(s)

	unh := unhonored(storePath)
	unh = append(unh, bindUnhonored...)

	rep := wm.ApplyReport{Provider: wm.ProviderNiri, Unhonored: unh, ReloadNeeded: false}
	if preview {
		return encodeReport(rep)
	}

	// Both files are always written, empty body included: niri treats a missing
	// include as a hard config error, so skipping one would break the session.
	if err := writeOverlayKdl("settings.kdl", genSettings(s)); err != nil {
		return err
	}
	if err := writeOverlayKdl("rebinds.kdl", []byte(binds)); err != nil {
		return err
	}
	if err := writeKeysounds(s.Niri.Keysounds); err != nil {
		return err
	}
	wm.PublishGreeterNumlock(s.Input.NumlockByDefault)
	rep.Written = []string{
		filepath.Join(niriConfigDir(), "settings.kdl"),
		filepath.Join(niriConfigDir(), "rebinds.kdl"),
	}
	return encodeReport(rep)
}

func encodeReport(rep wm.ApplyReport) error {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// runDefaults prints the provider's default subtree of the neutral store.
//
// Niri's KeyboardLayouts IPC exposes human-readable names such as
// "English (US)", not XKB identifiers such as "us". Those names are suitable
// for status displays, but must never seed desktop.input.kbLayout: the Hub
// reads this subtree as the unsaved baseline and writes it back into the
// config's xkb rule, where only an identifier is valid.
func runDefaults() error {
	s := defaultStore()
	tree, err := splitStore(s)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(tree)
}

// unhonored names every setting the store carries that niri cannot express. Each
// desktop.* leaf gets its own short, specific reason, because that list is the
// switch cost a user reads. A foreign compositor's whole namespace collapses to
// one line, since a per-key dump of another compositor's exclusives would bury
// the losses that matter under a hundred that never mattered.
func unhonored(storePath string) []wm.Unhonored {
	ns, ok := readNeutralStore(storePath)
	if !ok {
		return nil
	}
	var out []wm.Unhonored
	out = append(out, unhonoredLeaves(ns.Desktop["appearance"], "appearance", appearanceEmitted, appearanceReason)...)
	out = append(out, unhonoredLeaves(ns.Desktop["input"], "input", inputEmitted, inputReason)...)
	out = append(out, unhonoredFocusMode(ns.Desktop["input"])...)
	out = append(out, unhonoredLeaves(ns.Desktop["cursor"], "cursor", cursorEmitted, cursorReason)...)
	out = append(out, unhonoredWindowRules(ns.Desktop["windowRules"])...)
	out = append(out, unhonoredAppOverrides(ns.Desktop["appOverrides"])...)
	out = append(out, unhonoredDesktopMisc(ns.Desktop)...)
	out = append(out, presetWidthsUnhonored(ns)...)
	out = append(out, unhonoredForeign(ns.WM)...)
	return out
}

// presetWidthsUnhonored names a stored preset-width cycle that cannot drive
// Super+R: it needs at least two widths, and loadStore has already fallen back
// to the default so the bind still steps. Reported only when the user actually
// set fewer than two, so an unset store (which keeps the default) says nothing.
func presetWidthsUnhonored(ns neutralStore) []wm.Unhonored {
	raw, ok := ns.WM["niri"]
	if !ok {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	stored, present := m["presetColumnWidths"]
	if !present {
		return nil
	}
	var p Proportions
	if json.Unmarshal(stored, &p) != nil || len(p) >= 2 {
		return nil
	}
	return []wm.Unhonored{{
		Key:    "wm.niri.presetColumnWidths",
		Reason: "A width cycle needs at least two widths, so the default is used.",
	}}
}

// desktopHandled are the desktop.* keys this provider either emits or reports at
// a finer grain; a top-level key outside this set is reported whole. windows is
// honoured at runtime by the watch stream rather than the config file, so it
// belongs here even though apply writes nothing for it.
var desktopHandled = map[string]bool{
	"appearance": true, "input": true, "cursor": true, "env": true,
	"windowRules": true, "appOverrides": true, "autostart": true,
	"keybinds": true, "keybindRebinds": true, "unbinds": true, "apps": true,
	"keybindDisabled": true,
	"windows":         true,
}

var appearanceEmitted = map[string]bool{
	"gapsIn": true, "gapsOut": true, "borderSize": true, "rounding": true,
	"activeBorder": true, "inactiveBorder": true, "borderFollowsPalette": true, "animations": true,
	"activeOpacity": true, "inactiveOpacity": true,
	"shadowEnabled": true, "shadowRange": true, "shadowColor": true,
	"shadowSpread": true, "shadowOffsetX": true, "shadowOffsetY": true,
}

func appearanceReason(leaf string) string {
	switch {
	case strings.HasPrefix(leaf, "blur"):
		return "Niri's window blur makes see-through windows solid, so Ryoku leaves it off."
	case leaf == "shadowPower":
		return "Niri's shadows have no sharpness setting."
	case leaf == "shadowSharp":
		return "Niri's shadows are always soft."
	case leaf == "shadowScale":
		return "Niri can't change a shadow's size."
	case strings.HasPrefix(leaf, "glow"):
		return "Niri has no glow."
	case strings.HasPrefix(leaf, "dim"):
		return "Niri can't dim windows."
	case leaf == "fullscreenOpacity":
		return "Niri can't give full-screen windows their own opacity."
	case leaf == "roundingPower":
		return "Niri's rounded corners have one fixed shape."
	case leaf == "layout":
		return "Niri arranges windows in its own scrolling row."
	case leaf == "windowStyle":
		return "Niri has no window-open styles; set the window-open animation on the Animations page instead."
	case leaf == "wobblyWindows":
		return "Niri has no wobbly windows."
	case leaf == "animatedBorder" || leaf == "borderAngleSpeed":
		return "Niri's border gradient can't spin."
	}
	return "Niri has no matching appearance setting."
}

var inputEmitted = map[string]bool{
	"kbLayout": true, "kbVariant": true, "kbOptions": true, "numlockByDefault": true,
	"followMouse": true, "sensitivity": true, "accelProfile": true, "leftHanded": true,
	"mouseNaturalScroll": true, "mouseScrollFactor": true, "naturalScroll": true,
	"touchScrollFactor": true, "tapToClick": true, "middleClickPaste": true,
	"tapAndDrag": true, "clickfinger": true, "middleEmulation": true,
	"disableWhileTyping": true, "repeatRate": true, "repeatDelay": true,
}

func inputReason(leaf string) string {
	switch leaf {
	case "workspaceSwipe", "swipeFingers", "swipeInvert", "swipeCreateNew", "swipeDistance":
		return "Niri's touchpad workspace swipe is built in, with no setting for fingers, distance, direction or new workspaces."
	}
	return "Niri has no matching input setting."
}

// unhonoredFocusMode names the one focus mode niri cannot keep: it always focuses a clicked window.
func unhonoredFocusMode(raw json.RawMessage) []wm.Unhonored {
	var in struct {
		FollowMouse int `json:"followMouse"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &in) != nil || in.FollowMouse != 3 {
		return nil
	}
	return []wm.Unhonored{{Key: "desktop.input.followMouse", Reason: "Niri always moves focus to a window you click."}}
}

var cursorEmitted = map[string]bool{
	"theme": true, "size": true, "inactiveTimeout": true, "hideOnKeyPress": true,
}

func cursorReason(leaf string) string {
	return "Niri has no matching pointer setting."
}

// unhonoredLeaves reports each present leaf of a desktop.* object that niri does
// not emit, in a stable order, using reason for the user-facing why.
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
		if r.Action == "" || windowRuleProps(r) != nil {
			continue
		}
		out = append(out, wm.Unhonored{
			Key:    fmt.Sprintf("desktop.windowRules[%d]", i),
			Reason: "Niri can't apply this to a window.",
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
		var lost []string
		if a.Shadow == "off" {
			lost = append(lost, "the shadow")
		}
		if a.Dim == "off" {
			lost = append(lost, "dimming")
		}
		if a.Anim == "off" {
			lost = append(lost, "animations")
		}
		if a.Opaque == "on" {
			lost = append(lost, "full opacity")
		}
		if len(lost) > 0 {
			out = append(out, wm.Unhonored{
				Key:    fmt.Sprintf("desktop.appOverrides[%d]", i),
				Reason: "Niri can't set " + joinWords(lost) + " for one app.",
			})
		}
	}
	return out
}

// unhonoredDesktopMisc reports a top-level desktop.* key with no handler. A
// display-shaped key points at the Displays page, which owns layout, so a
// skipped monitor block is visible rather than silently lost.
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
		reason := "Niri has no setting for this."
		l := strings.ToLower(k)
		if strings.Contains(l, "display") || strings.Contains(l, "monitor") || strings.Contains(l, "output") {
			reason = "Arrange displays on the Displays page instead."
		}
		out = append(out, wm.Unhonored{Key: "desktop." + k, Reason: reason})
	}
	return out
}

// unhonoredForeign collapses each non-niri namespace that carries content into a
// single line, keyed by the namespace and derived from the store so a third
// compositor works the same way.
func unhonoredForeign(wmns map[string]json.RawMessage) []wm.Unhonored {
	names := make([]string, 0, len(wmns))
	for name, raw := range wmns {
		if name == wm.ProviderNiri {
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
			Reason: fmt.Sprintf("These %s settings have no Niri equivalent. They stay saved and come back if you switch back.", titleName(name)),
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
