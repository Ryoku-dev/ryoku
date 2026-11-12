package main

import (
	"fmt"
	"strconv"
	"strings"
)

// windowRuleAction is one neutral window-rule action and the wayfire action
// renderRule writes after "then", or the reason the value has no spelling.
type windowRuleAction struct {
	id   string
	then func(value string) (string, string)
}

// windowRuleTable is the one list of window-rule actions wayfire honours, in
// the order the Hub offers them: renderRule renders from it and caps advertises
// its ids, so the two cannot disagree.
var windowRuleTable = []windowRuleAction{
	{"pin", fixed("sticky")},
	{"maximize", fixed("maximize")},
	{"opacity", func(v string) (string, string) {
		alpha, ok := ruleOpacity(v)
		if !ok {
			return "", fmt.Sprintf("Wayfire needs an opacity from 0 to 1, not %q.", v)
		}
		return "set alpha " + alpha, ""
	}},
	{"workspace", ruleWorkspace},
}

// windowRuleActions are the table's ids, the list caps advertises.
var windowRuleActions = func() []string {
	ids := make([]string, len(windowRuleTable))
	for i, a := range windowRuleTable {
		ids[i] = a.id
	}
	return ids
}()

// fixed renders an action that takes no value.
func fixed(action string) func(string) (string, string) {
	return func(string) (string, string) { return action, "" }
}

// ruleWorkspace places the window on wayfire's 5x2 workspace grid, which takes
// "x y" coordinates rather than a number or name.
func ruleWorkspace(value string) (string, string) {
	v := strings.TrimSpace(value)
	coords := strings.Fields(v)
	if len(coords) != 2 {
		return "", fmt.Sprintf("Wayfire places windows on a grid of workspaces, so it needs a column and a row like \"1 0\", not %q.", value)
	}
	for i, c := range coords {
		n, err := strconv.Atoi(c)
		limit := 5
		if i == 1 {
			limit = 2
		}
		if err != nil || n < 0 || n >= limit {
			return "", fmt.Sprintf("Wayfire's workspace grid is 5 across and 2 down, so %q is outside it.", value)
		}
	}
	return "assign_workspace " + v, ""
}
