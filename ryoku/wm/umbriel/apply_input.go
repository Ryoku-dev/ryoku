package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// inputSettings maps the neutral Mouse and touchpad store to Umbriel's input
// config. It returns the TOML fragment, the rows that cannot be honoured, and
// the store keys it consumed so apply does not report them twice.
func inputSettings(store map[string]any) (map[string]map[string]string, []wm.Unhonored, map[string]bool) {
	sections := map[string]map[string]string{}
	mapped := map[string]bool{}
	var unhonored []wm.Unhonored

	put := func(table, key, value string) {
		if sections[table] == nil {
			sections[table] = map[string]string{}
		}
		sections[table][key] = value
	}
	val := func(key string) (any, bool) { return storeGet(store, key) }
	consume := func(key, table, out string, format func(any) string) {
		if v, ok := val(key); ok {
			if rendered := format(v); rendered != "" {
				put(table, out, rendered)
			} else {
				unhonored = append(unhonored, wm.Unhonored{Key: key, Reason: "Umbriel cannot express the saved value for this input setting."})
			}
			mapped[key] = true
		}
	}
	num := func(v any) string {
		switch x := v.(type) {
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		case int:
			return strconv.Itoa(x)
		default:
			return ""
		}
	}
	boolean := func(v any) string {
		if b, ok := v.(bool); ok {
			return strconv.FormatBool(b)
		}
		return ""
	}
	quoted := func(v any) string {
		if s, ok := v.(string); ok {
			return strconv.Quote(s)
		}
		return ""
	}

	// Keyboard. The Hub composes layout, style and options into these three
	// strings; Umbriel accepts them verbatim, including a comma-separated layout
	// list and the grp:* switch option. The repeat and Num Lock rows are plain
	// scalars.
	consume("desktop.input.kbLayout", "input.keyboard", "layout", quoted)
	consume("desktop.input.kbVariant", "input.keyboard", "variant", quoted)
	consume("desktop.input.kbOptions", "input.keyboard", "options", quoted)
	consume("desktop.input.repeatRate", "input.keyboard", "repeat_rate", num)
	consume("desktop.input.repeatDelay", "input.keyboard", "repeat_delay", num)
	consume("desktop.input.numlockByDefault", "input.keyboard", "numlock_toggle", boolean)

	// Mouse.
	consume("desktop.input.sensitivity", "input.mouse", "sensitivity", num)
	consume("desktop.input.sensitivity", "input.touchpad", "sensitivity", num)
	if v, ok := val("desktop.input.accelProfile"); ok {
		s, _ := v.(string)
		if s == "flat" || s == "adaptive" {
			put("input.mouse", "accel_profile", quoted(v))
			put("input.touchpad", "accel_profile", quoted(v))
		}
		mapped["desktop.input.accelProfile"] = true
	}
	consume("desktop.input.leftHanded", "input.mouse", "left_handed", boolean)
	consume("desktop.input.leftHanded", "input.touchpad", "left_handed", boolean)
	consume("desktop.input.mouseNaturalScroll", "input.mouse", "natural_scroll", boolean)
	consume("desktop.input.middleClickPaste", "input", "middle_click_paste", boolean)

	// Touchpad. The on/off row is driven by the input.touchpad action, which
	// writes touchpad.toml, so apply leaves that key alone.
	consume("desktop.input.tapToClick", "input.touchpad", "tap", boolean)
	consume("desktop.input.tapAndDrag", "input.touchpad", "tap_and_drag", boolean)
	consume("desktop.input.disableWhileTyping", "input.touchpad", "disable_while_typing", boolean)
	consume("desktop.input.touchScrollFactor", "input.touchpad", "scroll_factor", num)
	if v, ok := val("desktop.input.clickfinger"); ok {
		if b, ok := v.(bool); ok {
			if b {
				put("input.touchpad", "click_method", `"clickfinger"`)
			} else {
				put("input.touchpad", "click_method", `"button_areas"`)
			}
			mapped["desktop.input.clickfinger"] = true
		}
	}

	// Touchpad natural scrolling owns the Umbriel key. A separate gesture invert
	// would need another key, so it is reported unhonoured below.
	consume("desktop.input.naturalScroll", "input.touchpad", "natural_scroll", boolean)

	// Pointer.
	consume("desktop.cursor.hideOnKeyPress", "input.cursor", "hide_when_typing", boolean)
	if v, ok := val("desktop.cursor.inactiveTimeout"); ok {
		if f, ok := v.(float64); ok && f >= 0 && f <= 3600000 {
			put("input.cursor", "hide_timeout_ms", strconv.Itoa(int(f*1000+0.5)))
		}
		mapped["desktop.cursor.inactiveTimeout"] = true
	}

	return sections, unhonored, mapped
}

func storeGet(store map[string]any, key string) (any, bool) {
	var cur any = store
	for _, part := range strings.Split(key, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func renderInputToml(sections map[string]map[string]string) string {
	var b strings.Builder
	for _, table := range []string{"input", "input.keyboard", "input.mouse", "input.touchpad", "input.cursor", "input.focus", "input.drag_edge_scroll", "general", "workspaces"} {
		values := sections[table]
		if len(values) == 0 {
			continue
		}
		fmt.Fprintf(&b, "[%s]\n", table)
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s = %s\n", k, values[k])
		}
		b.WriteString("\n")
	}
	return b.String()
}
