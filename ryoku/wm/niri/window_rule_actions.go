package main

import "fmt"

// windowRuleAction is one neutral window-rule action and the niri property
// lines windowRuleProps writes for it; nil lines mean the value has no niri
// expression.
type windowRuleAction struct {
	id    string
	props func(value string) []string
}

// windowRuleTable is the one list of window-rule actions niri honours, in the
// order the Hub offers them: the config writer renders from it and caps
// advertises its ids, so the two cannot disagree.
var windowRuleTable = []windowRuleAction{
	{"float", lines("open-floating true")},
	{"tile", lines("open-floating false")},
	{"fullscreen", lines("open-fullscreen true")},
	{"maximize", lines("open-maximized true")},
	{"norounding", lines("geometry-corner-radius 0")},
	{"opacity", func(v string) []string {
		return []string{fmt.Sprintf("opacity %s", kdlFloat(parseFloat(v, 1)))}
	}},
	{"workspace", func(v string) []string { return []string{fmt.Sprintf("open-on-workspace %s", kdlStr(v))} }},
	{"noborder", lines("border {", "    off", "}")},
	{"noshadow", lines("shadow {", "    off", "}")},
	{"blur", lines("background-effect {", "    blur true", "}")},
	{"noblur", lines("background-effect {", "    blur false", "}")},
	{"xray", lines("background-effect {", "    xray true", "}")},
	{"columnwidth", func(v string) []string {
		return []string{fmt.Sprintf("default-column-width { proportion %s; }", kdlFloat(parseFloat(v, 0.5)))}
	}},
	{"minsize", func(v string) []string { return sizeProps("min", v) }},
	{"maxsize", func(v string) []string { return sizeProps("max", v) }},
	{"scrollfactor", func(v string) []string {
		return []string{fmt.Sprintf("scroll-factor %s", kdlNum(parseFloat(v, 1)))}
	}},
	{"tiledstate", lines("tiled-state true")},
	{"babaisfloat", lines("baba-is-float true")},
	{"blockout", lines(`block-out-from "screencast"`)},
}

// windowRuleActions are the table's ids, the list caps advertises.
var windowRuleActions = func() []string {
	ids := make([]string, len(windowRuleTable))
	for i, a := range windowRuleTable {
		ids[i] = a.id
	}
	return ids
}()

// lines renders an action whose properties ignore the rule's value.
func lines(props ...string) func(string) []string {
	return func(string) []string { return props }
}
