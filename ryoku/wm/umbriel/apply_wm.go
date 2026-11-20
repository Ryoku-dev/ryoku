package main

import (
	"fmt"
	"strconv"

	wm "ryoku-wm"
)

// wmSettings maps Umbriel's exclusive settings and the neutral focus row into
// its config. Rows without a writer stay unhonoured with a full sentence.
func wmSettings(store map[string]any) (map[string]map[string]string, []wm.Unhonored, map[string]bool) {
	sections := map[string]map[string]string{}
	mapped := map[string]bool{}
	put := func(table, key, value string) {
		if sections[table] == nil {
			sections[table] = map[string]string{}
		}
		sections[table][key] = value
	}

	if v, ok := storeGet(store, "wm.umbriel.warpMouseToFocus"); ok {
		if s := tomlBool(v); s != "" {
			put("input.cursor", "follows_focus", s)
		}
		mapped["wm.umbriel.warpMouseToFocus"] = true
	}
	if v, ok := storeGet(store, "wm.umbriel.focusFollowsMouseScroll"); ok {
		if n := tomlNumber(v); n != "" {
			put("input.focus", "follows_mouse_max_scroll", n)
		}
		mapped["wm.umbriel.focusFollowsMouseScroll"] = true
	}
	if v, ok := storeGet(store, "wm.umbriel.workspaceAutoBackAndForth"); ok {
		if s := tomlBool(v); s != "" {
			put("workspaces", "back_and_forth", s)
		}
		mapped["wm.umbriel.workspaceAutoBackAndForth"] = true
	}
	if v, ok := storeGet(store, "wm.umbriel.modKey"); ok {
		if s, ok := v.(string); ok && s != "" {
			put("general", "mod_key", strconv.Quote(s))
		}
		mapped["wm.umbriel.modKey"] = true
	}
	if v, ok := storeGet(store, "wm.umbriel.dndEdgeViewScrollTriggerWidth"); ok {
		if n := tomlNumber(v); n != "" {
			put("input.drag_edge_scroll", "trigger_zone", n)
		}
		mapped["wm.umbriel.dndEdgeViewScrollTriggerWidth"] = true
	}
	if v, ok := storeGet(store, "wm.umbriel.dndEdgeViewScrollDelayMs"); ok {
		if n := tomlNumber(v); n != "" {
			put("input.drag_edge_scroll", "delay_ms", n)
		}
		mapped["wm.umbriel.dndEdgeViewScrollDelayMs"] = true
	}
	if v, ok := storeGet(store, "wm.umbriel.dndEdgeViewScrollMaxSpeed"); ok {
		if n := tomlNumber(v); n != "" {
			put("input.drag_edge_scroll", "max_speed", n)
		}
		mapped["wm.umbriel.dndEdgeViewScrollMaxSpeed"] = true
	}

	if v, ok := storeGet(store, "desktop.input.followMouse"); ok {
		f, _ := v.(float64)
		follows := f == 1 || f == 2
		put("input.focus", "follows_mouse", fmt.Sprint(follows))
		mapped["desktop.input.followMouse"] = true
	}

	unhonored := []wm.Unhonored{}

	return sections, unhonored, mapped
}

func tomlNumber(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	default:
		return ""
	}
}

func tomlBool(v any) string {
	if b, ok := v.(bool); ok {
		return strconv.FormatBool(b)
	}
	return ""
}
