package main

import (
	"fmt"
	"strconv"
	"strings"
)

// wayfire's window-rules language, rendered from the neutral rows:
// "on <signal> if <condition> then <action>". The condition is wayfire's own
// expression language, the action its set from view-action-interface. This file
// owns that spelling, the sibling of niri's config_rules.go; renderRule's
// second return is the same reason apply prints when a row has no wayfire
// spelling, so the emission and the switch cost can never disagree.

// ruleCondition renders the neutral match as wayfire's condition. Both criteria
// set means both must hold, which wayfire's expression parser takes with "&"
// (the same join its animate filters use); no criteria at all means the row
// cannot be anchored, since wayfire's grammar wants a condition after "if".
func ruleCondition(class, title string) (string, bool) {
	switch {
	case class != "" && title != "":
		return fmt.Sprintf("app_id is %s & title is %s", ruleStr(class), ruleStr(title)), true
	case class != "":
		return fmt.Sprintf("app_id is %s", ruleStr(class)), true
	case title != "":
		return fmt.Sprintf("title is %s", ruleStr(title)), true
	}
	return "", false
}

// ruleStr quotes a match value for the rule language. strconv.Quote's escapes
// cover the quotes and backslashes a title can carry; wayfire's lexer reads the
// same forms.
func ruleStr(s string) string {
	return strconv.Quote(s)
}

// renderRule turns one neutral rule into wayfire's language, or into the reason
// it has none. The signal is "created": the neutral actions are open-time
// properties, the same moments niri's open-* rules fire on.
func renderRule(class, title, action, value string) (string, string) {
	cond, ok := ruleCondition(class, title)
	if !ok {
		return "", "wayfire window rules need an app id or title to match on."
	}
	switch action {
	case "maximize":
		return "on created if " + cond + " then maximize", ""
	case "pin":
		return "on created if " + cond + " then sticky", ""
	case "opacity":
		v, ok := ruleOpacity(value)
		if !ok {
			return "", fmt.Sprintf("wayfire cannot apply %q to the value %q.", action, value)
		}
		return "on created if " + cond + " then set alpha " + v, ""
	case "workspace":
		v := strings.TrimSpace(value)
		coords := strings.Fields(v)
		if len(coords) != 2 {
			return "", fmt.Sprintf("wayfire workspace rules need grid coordinates \"x y\", not the value %q.", value)
		}
		for i, c := range coords {
			n, err := strconv.Atoi(c)
			limit := 5
			if i == 1 {
				limit = 2
			}
			if err != nil || n < 0 || n >= limit {
				return "", fmt.Sprintf("wayfire workspace rules need coordinates inside the 5x2 grid, not the value %q.", value)
			}
		}
		return "on created if " + cond + " then assign_workspace " + v, ""
	}
	return "", fmt.Sprintf("wayfire window rules have no %q action.", action)
}

// wayfireRule is renderRule for a whole row; ok is false when the row has no
// spelling, which is exactly when the report names the reason.
func wayfireRule(r WindowRule) (string, bool) {
	rule, why := renderRule(r.Class, r.Title, r.Action, r.Value)
	return rule, why == ""
}

// ruleOpacity validates a neutral opacity value and renders it the way wayfire's
// alpha action parses it: a bare decimal in [0,1], never exponent notation.
func ruleOpacity(v string) (string, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 0 || f > 1 {
		return "", false
	}
	return strconv.FormatFloat(f, 'f', -1, 64), true
}

// wayfireOverride renders the one per-app appearance field wayfire expresses:
// opacity, as an alpha rule on the same condition. Everything else the override
// carries is reported unhonored per override by unhonoredAppOverrides.
func wayfireOverride(a AppOverride) (string, bool) {
	rule, why := renderRule(a.Class, a.Title, "opacity", strconv.FormatFloat(a.Opacity, 'f', -1, 64))
	return rule, why == ""
}

// overrideLost names the stored per-app fields with no wayfire spelling, using
// the sibling sentinels: numbers at -1 and strings unset mean leave alone, so
// only a value the user actually chose counts as a loss. An opacity past 1 is
// corrupt rather than unset and lands here too, instead of reaching a parser
// that would reject it.
func overrideLost(a AppOverride) []string {
	var lost []string
	if a.Opacity > 1 {
		lost = append(lost, "opacity")
	}
	if a.Rounding >= 0 {
		lost = append(lost, "rounding")
	}
	if a.BorderSize >= 0 {
		lost = append(lost, "border")
	}
	if a.Blur != "" && a.Blur != "inherit" {
		lost = append(lost, "blur")
	}
	if a.Shadow != "" && a.Shadow != "inherit" {
		lost = append(lost, "shadow")
	}
	if a.Dim != "" && a.Dim != "inherit" {
		lost = append(lost, "dim")
	}
	if a.Anim != "" && a.Anim != "inherit" {
		lost = append(lost, "animation")
	}
	if a.Opaque != "" && a.Opaque != "inherit" {
		lost = append(lost, "opaque")
	}
	return lost
}

// A simple-tile matcher decides whether a window ever joins the tiling tree.
// Once a view is attached, the plugin ignores ordinary untile and geometry
// requests, so Ryoku's designed floating windows must be excluded before their
// created rules run. Wayfire's condition language has exact and substring
// comparisons rather than regular expressions; alternatives from the Hyprland
// rules are consequently listed as separate rows.
type wayfireBuiltinRule struct {
	key       string
	condition string
	action    string
	floating  bool
}

const (
	// The Hub is a full page fitted to the work area, as Hyprland fits it on
	// open. Wayfire's geometry rules only measure the whole output, so a
	// percentage page slides under whatever the shell reserves (a frame edge,
	// a bar) and hides the Hub's header; maximize is the one rule form that
	// tracks the work area on every output. The other profiles keep
	// Hyprland's 92% by 88% screen-fit ceiling, whose insets clear the shell's
	// reserved edges, and Wayfire's place plugin centres floating windows that
	// carry no explicit geometry.
	pageFit         = "maximize"
	largeGeometry   = "set geometry_ppt 4 6 92 88"
	mediumGeometry  = "set geometry_ppt 15 17 70 66"
	compactGeometry = "set geometry_ppt 20 24 60 52"
	phoneGeometry   = "set geometry_ppt 31 6 38 88"
)

func settingsWindowAction(v SettingsWindow) string {
	v = normalizeSettingsWindow(v)
	if v.Mode != "custom" {
		return pageFit
	}
	width, height := v.Width*100, v.Height*100
	x, y := (100-width)/2, (100-height)/2
	return fmt.Sprintf(
		"set geometry_ppt %s %s %s %s",
		strconv.FormatFloat(x, 'f', -1, 64),
		strconv.FormatFloat(y, 'f', -1, 64),
		strconv.FormatFloat(width, 'f', -1, 64),
		strconv.FormatFloat(height, 'f', -1, 64),
	)
}

var wayfireBuiltinRules = []wayfireBuiltinRule{
	{key: "system_pavucontrol", condition: `app_id is "pavucontrol"`, floating: true},
	{key: "system_network", condition: `app_id is "nm-connection-editor"`, floating: true},
	{key: "system_bluetooth", condition: `app_id is "blueman-manager"`, floating: true},
	{key: "system_polkit_kde", condition: `app_id is "org.kde.polkit-kde-authentication-agent-1"`, floating: true},
	{key: "system_portal_gtk", condition: `app_id is "xdg-desktop-portal-gtk"`, floating: true},
	{key: "system_polkit", condition: `app_id is "hyprpolkitagent"`, floating: true},

	{key: "picker_open_file", condition: `title is "Open File"`, floating: true},
	{key: "picker_save_file", condition: `title is "Save File"`, floating: true},
	{key: "picker_save_as", condition: `title is "Save As"`, floating: true},
	{key: "picker_choose_files", condition: `title is "Choose Files"`, floating: true},
	{key: "picker_open_folder", condition: `title is "Open Folder"`, floating: true},
	{key: "picture_in_picture", condition: `title contains "Picture-in-Picture"`, floating: true},
	{key: "picture_in_picture_chromium", condition: `title contains "Picture in picture"`, floating: true},

	{key: "ghosttype", condition: `app_id is "Ghosttype-app"`, floating: true},
	{key: "spotify_lower", condition: `app_id is "spotify"`, floating: true},
	{key: "spotify_upper", condition: `app_id is "Spotify"`, floating: true},
	{key: "nautilus", condition: `app_id is "org.gnome.Nautilus"`, action: largeGeometry, floating: true},
	{key: "hub", condition: `title is "Ryoku Settings"`, action: pageFit, floating: true},
	{key: "rashin", condition: `title is "Rashin"`, action: largeGeometry, floating: true},
	{key: "phone_tools", condition: `title contains "ryoku-phone-"`, action: phoneGeometry, floating: true},
	{key: "update", condition: `app_id is "dev.ryoku.update"`, action: largeGeometry, floating: true},
	{key: "ryostore_title", condition: `title is "Ryostore"`, action: largeGeometry, floating: true},
	{key: "ryoport", condition: `app_id is "dev.ryoku.ryoport"`, action: largeGeometry, floating: true},
	{key: "ryoport_ssh", condition: `app_id is "dev.ryoku.ryoport_ssh"`, action: compactGeometry, floating: true},
	{key: "ryoport_spicy", condition: `app_id is "spicy"`, floating: true},
	{key: "ryoport_console", condition: `app_id is "dev.ryoku.ryoport_console"`, floating: true},
	{key: "ryoport_vnc_lower", condition: `app_id is "vncviewer"`, floating: true},
	{key: "ryoport_vnc_upper", condition: `app_id is "Vncviewer"`, floating: true},
	{key: "ryostore_app", condition: `app_id is "ryostore"`, action: mediumGeometry, floating: true},
	{key: "rashin_setup", condition: `app_id is "ryoku-rashin-setup"`, action: mediumGeometry, floating: true},
	{key: "looking_glass", condition: `app_id is "looking-glass-client"`, action: largeGeometry, floating: true},
	{key: "qemu_lower", condition: `app_id is "qemu"`, action: largeGeometry, floating: true},
	{key: "qemu_upper", condition: `app_id is "Qemu"`, action: largeGeometry, floating: true},
	{key: "welcome", condition: `title is "Welcome to Ryoku"`, action: largeGeometry, floating: true},
	{key: "ryotunes", condition: `app_id is "ryotunes"`, floating: true},
	{key: "ryotunes_qml", condition: `app_id is "org.quickshell" & title is "Ryotunes"`, floating: true},

	// Chromium's native-Wayland share indicator has no usable pointer geometry.
	// Hyprland parks it on special:sharebar; Wayfire's closest equivalent is the
	// last cell in Ryoku's 5x2 grid. It opens away from the current desktop and
	// remains reachable if Chromium ever fixes the surface.
	{key: "chromium_sharebar", condition: `app_id is "" & title contains "is sharing"`, action: "assign_workspace 4 1", floating: true},
}

var defaultTileByMatcher = buildTileByDefaultMatcher()

func buildTileByDefaultMatcher() string {
	conditions := make([]string, 0, len(wayfireBuiltinRules))
	for _, r := range wayfireBuiltinRules {
		if r.floating {
			conditions = append(conditions, "("+r.condition+")")
		}
	}
	return "!(" + strings.Join(conditions, " | ") + ")"
}

func tileByDefaultMatcher() string {
	return defaultTileByMatcher
}

func genBuiltinWindowRules(d *iniDoc, settingsWindow SettingsWindow) {
	for _, r := range wayfireBuiltinRules {
		action := r.action
		if r.key == "hub" {
			action = settingsWindowAction(settingsWindow)
		}
		if action != "" {
			d.set("window-rules", "ryoku_builtin_"+r.key,
				"on created if "+r.condition+" then "+action)
		}
	}
}

// genWindowRules renders the neutral rules and per-app overrides into the
// window-rules section. Wayfire has no focus-change event, so activeOpacity is
// the one baseline alpha assigned when each toplevel opens; inactiveOpacity
// remains an explicit reported loss. User and per-app rules follow the baseline
// so their more specific alpha value wins.
func genWindowRules(s wayfireStore) iniDoc {
	var d iniDoc
	genBuiltinWindowRules(&d, s.Wayfire.SettingsWindow)
	if a := s.Appearance.ActiveOpacity; a >= 0 && a < 1 {
		d.set("window-rules", "ryoku_active_opacity",
			"on created if type is \"toplevel\" then set alpha "+strconv.FormatFloat(a, 'f', -1, 64))
	}
	for i, r := range s.WindowRules {
		if rule, ok := wayfireRule(r); ok {
			d.set("window-rules", fmt.Sprintf("ryoku_rule_%d", i), rule)
		}
	}
	for i, a := range s.AppOverrides {
		if a.Opacity >= 0 && a.Opacity <= 1 {
			if rule, ok := wayfireOverride(a); ok {
				d.set("window-rules", fmt.Sprintf("ryoku_override_%d", i), rule)
			}
		}
	}
	return d
}
