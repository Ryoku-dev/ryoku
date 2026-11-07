package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// dock_cli.go is the agent-facing `ryoku-shell dock ...` surface for Ryoku's own
// universal dock. It edits shell.json `dock.*` through the settings store, the
// sole writer of shell.json.

// dockEdges are the screen edges the dock can anchor to; "auto" tracks the edge
// opposite the bar.
var dockEdges = map[string]bool{"auto": true, "top": true, "bottom": true, "left": true, "right": true}

// settingsFrame snapshots the current in-memory shell.json frame (a deep copy
// under the store lock), so a read never races the sole writer.
func (d *daemon) settingsFrame() map[string]any {
	if d.settings == nil {
		return map[string]any{}
	}
	d.settings.mu.Lock()
	defer d.settings.mu.Unlock()
	return deepCopyMap(d.settings.raw)
}

// defaultDock is the shipped dock store, mirroring services/Config.qml so a
// partial dock object in shell.json never drops a field a mutation did not set.
func defaultDock() map[string]any {
	return map[string]any{
		"enabled": false, "edge": "auto", "edgeGap": dockEdgeGapDefault,
		"autohide": false, "revealOnEmpty": true, "pinned": []string{},
		"magnify": true, "frost": true, "shadow": true, "labels": true, "media": false,
	}
}

// currentDock reads the dock store, preserving any unknown fields and filling in
// missing shipped defaults, so writing it back is always a complete object.
func (d *daemon) currentDock() map[string]any {
	out := map[string]any{}
	if dock, ok := d.settingsFrame()["dock"].(map[string]any); ok {
		for k, v := range dock {
			out[k] = v
		}
	}
	for k, v := range defaultDock() {
		if _, ok := out[k]; !ok {
			out[k] = v
		}
	}
	return out
}

func (d *daemon) dockCLI(args []string) string {
	if len(args) < 1 {
		return "err dock: expected show|hide|edge|autohide|pin|unpin|list"
	}
	switch args[0] {
	case "show":
		return d.dockSet("enabled", true)
	case "hide":
		return d.dockSet("enabled", false)
	case "edge":
		if len(args) != 2 || !dockEdges[args[1]] {
			return "err dock: edge expects auto|top|bottom|left|right"
		}
		return d.dockSet("edge", args[1])
	case "autohide":
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			return "err dock: autohide expects on|off"
		}
		return d.dockSet("autohide", args[1] == "on")
	case "pin":
		if len(args) != 2 {
			return "err dock: pin needs an app id"
		}
		return d.dockPin(args[1], true)
	case "unpin":
		if len(args) != 2 {
			return "err dock: unpin needs an app id"
		}
		return d.dockPin(args[1], false)
	case "list":
		return d.dockList(hasJSONFlag(args[1:]))
	}
	return "err dock: unknown verb " + args[0]
}

func hasJSONFlag(args []string) bool {
	for _, a := range args {
		if a == "--json" {
			return true
		}
	}
	return false
}

// toStringSlice reads a decoded JSON array into a string slice, dropping any
// non-string element.
func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func removeID(ids []string, id string) []string {
	out := make([]string, 0, len(ids))
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}

func (d *daemon) dockSet(key string, value any) string {
	if d.settings == nil {
		return "err dock: settings not ready"
	}
	dock := d.currentDock()
	dock[key] = value
	b, _ := json.Marshal(dock)
	if err := d.settings.patch("dock", b); err != nil {
		return "err dock: " + err.Error()
	}
	return "ok"
}

func (d *daemon) dockPin(app string, add bool) string {
	if d.settings == nil {
		return "err dock: settings not ready"
	}
	dock := d.currentDock()
	pinned := toStringSlice(dock["pinned"])
	if add {
		if inSet(pinned, app) {
			return "ok"
		}
		pinned = append(pinned, app)
	} else {
		if !inSet(pinned, app) {
			return "err dock: " + app + " is not pinned"
		}
		pinned = removeID(pinned, app)
	}
	dock["pinned"] = pinned
	b, _ := json.Marshal(dock)
	if err := d.settings.patch("dock", b); err != nil {
		return "err dock: " + err.Error()
	}
	return "ok"
}

func (d *daemon) dockList(asJSON bool) string {
	dock := d.currentDock()
	view := map[string]any{
		"enabled":  dock["enabled"],
		"edge":     dock["edge"],
		"autohide": dock["autohide"],
		"pinned":   toStringSlice(dock["pinned"]),
	}
	if asJSON {
		b, _ := json.Marshal(view)
		return string(b)
	}
	return fmt.Sprintf("enabled: %v\nedge: %v\nautohide: %v\npinned: %s",
		view["enabled"], view["edge"], view["autohide"], strings.Join(toStringSlice(view["pinned"]), ", "))
}
