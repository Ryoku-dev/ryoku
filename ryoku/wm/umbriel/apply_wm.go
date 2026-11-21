package main

import (
	"fmt"
	"math"
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
	var unhonored []wm.Unhonored
	// value writes a rendered setting, or reports an invalid saved value.
	value := func(key, table, out, rendered string) {
		if rendered != "" {
			put(table, out, rendered)
		} else {
			unhonored = append(unhonored, wm.Unhonored{Key: key, Reason: "Umbriel cannot express the saved value for this setting."})
		}
		mapped[key] = true
	}

	if v, ok := storeGet(store, "wm.umbriel.warpMouseToFocus"); ok {
		value("wm.umbriel.warpMouseToFocus", "input.cursor", "follows_focus", tomlBool(v))
	}
	if v, ok := storeGet(store, "wm.umbriel.focusFollowsMouseScroll"); ok {
		value("wm.umbriel.focusFollowsMouseScroll", "input.focus", "follows_mouse_max_scroll", boundedNumber("wm.umbriel.focusFollowsMouseScroll", v, 0, 100))
	}
	if v, ok := storeGet(store, "wm.umbriel.workspaceAutoBackAndForth"); ok {
		value("wm.umbriel.workspaceAutoBackAndForth", "workspaces", "back_and_forth", tomlBool(v))
	}
	if v, ok := storeGet(store, "wm.umbriel.modKey"); ok {
		s, _ := v.(string)
		if s != "" {
			s = strconv.Quote(s)
		}
		value("wm.umbriel.modKey", "general", "mod_key", s)
	}
	if v, ok := storeGet(store, "wm.umbriel.dndEdgeViewScrollTriggerWidth"); ok {
		value("wm.umbriel.dndEdgeViewScrollTriggerWidth", "input.drag_edge_scroll", "trigger_zone", boundedNumber("wm.umbriel.dndEdgeViewScrollTriggerWidth", v, 1, 1000))
	}
	if v, ok := storeGet(store, "wm.umbriel.dndEdgeViewScrollDelayMs"); ok {
		value("wm.umbriel.dndEdgeViewScrollDelayMs", "input.drag_edge_scroll", "delay_ms", boundedNumber("wm.umbriel.dndEdgeViewScrollDelayMs", v, 0, 10000))
	}
	if v, ok := storeGet(store, "wm.umbriel.dndEdgeViewScrollMaxSpeed"); ok {
		value("wm.umbriel.dndEdgeViewScrollMaxSpeed", "input.drag_edge_scroll", "max_speed", boundedNumber("wm.umbriel.dndEdgeViewScrollMaxSpeed", v, 1, 100000))
	}

	if v, ok := storeGet(store, "desktop.input.followMouse"); ok {
		f, _ := v.(float64)
		follows := f == 1 || f == 2
		put("input.focus", "follows_mouse", fmt.Sprint(follows))
		mapped["desktop.input.followMouse"] = true
	}

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

// boundedNumber renders a number only when it is a finite value in the
// compositor's documented range; otherwise it reports the setting unhonoured.
func boundedNumber(key string, v any, lo, hi float64) string {
	f, ok := v.(float64)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) || f < lo || f > hi {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func tomlBool(v any) string {
	if b, ok := v.(bool); ok {
		return strconv.FormatBool(b)
	}
	return ""
}
