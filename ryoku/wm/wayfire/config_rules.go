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
		return "on created if " + cond + " then set sticky true", ""
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
		for _, c := range coords {
			n, err := strconv.Atoi(c)
			if err != nil || n < 0 {
				return "", fmt.Sprintf("wayfire workspace rules need grid coordinates \"x y\", not the value %q.", value)
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

// genWindowRules renders the neutral rules and per-app overrides into the
// window-rules section. Rows with no wayfire spelling are left to apply's
// unhonored walk, which names them; this only lays down the rows that landed.
func genWindowRules(s wayfireStore) iniDoc {
	var d iniDoc
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
