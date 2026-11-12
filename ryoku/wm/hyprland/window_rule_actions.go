package main

import "fmt"

// windowRuleAction is one neutral window-rule action and the hl.window_rule
// property genWindowRule writes for it.
type windowRuleAction struct {
	id   string
	prop func(value string) string
}

// windowRuleTable is the one list of window-rule actions Hyprland honours, in
// the order the Hub offers them: genWindowRule renders from it and caps
// advertises its ids, so the two cannot disagree.
var windowRuleTable = []windowRuleAction{
	{"float", flagProp("float")},
	{"tile", flagProp("tile")},
	{"pin", flagProp("pin")},
	{"fullscreen", flagProp("fullscreen")},
	{"maximize", flagProp("maximize")},
	{"center", flagProp("center")},
	{"pseudo", flagProp("pseudo")},
	{"keepaspectratio", flagProp("keep_aspect_ratio")},
	{"size", func(v string) string {
		w, h := parseWxH(v)
		return fmt.Sprintf("size = { %d, %d }", w, h)
	}},
	{"move", func(v string) string {
		x, y := parseWxH(v)
		return fmt.Sprintf("move = { %d, %d }", x, y)
	}},
	{"workspace", func(v string) string { return fmt.Sprintf("workspace = %s", luaStr(v)) }},
	{"norounding", func(string) string { return "rounding = 0" }},
	{"noborder", func(string) string { return "border_size = 0" }},
	{"opacity", func(v string) string { return fmt.Sprintf("opacity = %s", luaNum(parseFloat(v, 1))) }},
	{"opaque", flagProp("opaque")},
	{"noblur", flagProp("no_blur")},
	{"noshadow", flagProp("no_shadow")},
	{"nodim", flagProp("no_dim")},
	{"noanim", flagProp("no_anim")},
	{"xray", flagProp("xray")},
	{"nofocus", flagProp("no_focus")},
	{"stayfocused", flagProp("stay_focused")},
	{"suppressevent", func(v string) string {
		return fmt.Sprintf("suppress_event = %s", luaStr(oneOf(v, "maximize", "fullscreen", "activate", "activatefocus")))
	}},
	{"idleinhibit", func(v string) string {
		return fmt.Sprintf("idle_inhibit = %s", luaStr(oneOf(v, "always", "focus", "fullscreen")))
	}},
	{"immediate", flagProp("immediate")},
}

// windowRuleActions are the table's ids, the list caps advertises.
var windowRuleActions = func() []string {
	ids := make([]string, len(windowRuleTable))
	for i, a := range windowRuleTable {
		ids[i] = a.id
	}
	return ids
}()

func windowRuleProp(id string) (func(string) string, bool) {
	for _, a := range windowRuleTable {
		if a.id == id {
			return a.prop, true
		}
	}
	return nil, false
}

// flagProp renders a boolean hl.window_rule field; a wrong field name here
// breaks the whole generated file.
func flagProp(field string) func(string) string {
	return func(string) string { return field + " = true" }
}

// oneOf keeps v when it is one of the allowed values and falls back to the
// first otherwise.
func oneOf(v string, allowed ...string) string {
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return allowed[0]
}
