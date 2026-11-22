package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

func runAct(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("act: missing action id")
	}
	id := wm.Action(args[0])
	rest := args[1:]

	switch id {
	case wm.ActionWindowFocus:
		if len(rest) < 1 {
			return fmt.Errorf("window.focus: missing window id")
		}
		return action("window-focus", rest[0])

	case wm.ActionWindowClose:
		if len(rest) < 1 {
			return fmt.Errorf("window.close: missing window id")
		}
		return action("window-close", rest[0])

	case wm.ActionWindowFullscreen:
		if len(rest) < 1 {
			return fmt.Errorf("window.fullscreen: missing window id")
		}
		return action("window-toggle-fullscreen", rest[0])

	case wm.ActionWindowFloat:
		if len(rest) < 1 {
			return fmt.Errorf("window.float: missing window id")
		}
		return action("window-toggle-floating", rest[0])

	case wm.ActionWorkspaceFocus:
		if len(rest) < 1 {
			return fmt.Errorf("workspace.focus: missing workspace id")
		}
		return focusWorkspace(rest[0])

	case wm.ActionWorkspaceCycle:
		if len(rest) < 1 {
			return fmt.Errorf("workspace.cycle: missing delta")
		}
		n, err := strconv.Atoi(rest[0])
		if err != nil || n == 0 {
			return fmt.Errorf("workspace.cycle: bad delta %q", rest[0])
		}
		raw, err := query("workspaces")
		if err != nil {
			return err
		}
		var ws []uWorkspace
		if err := json.Unmarshal(raw, &ws); err != nil {
			return err
		}
		focus := ""
		rawOut, err := query("outputs")
		if err != nil {
			return err
		}
		var outs []uOutput
		if err := json.Unmarshal(rawOut, &outs); err != nil {
			return err
		}
		for _, o := range outs {
			if o.Focused {
				focus = o.Name
				break
			}
		}
		// Cycle only within the focused output's workspaces. Names repeat
		// across outputs, so a global index can step the wrong screen.
		if focus == "" {
			return fmt.Errorf("workspace.cycle: no focused output")
		}
		mine := make([]uWorkspace, 0, len(ws))
		for _, w := range ws {
			if w.Output == focus {
				mine = append(mine, w)
			}
		}
		at := -1
		for i, w := range mine {
			if w.Active {
				at = i
				break
			}
		}
		if at < 0 {
			return fmt.Errorf("workspace.cycle: no active workspace on the focused output")
		}
		if n < -at || n >= len(mine)-at {
			return fmt.Errorf("workspace.cycle: no workspace %d steps away", n)
		}
		target := mine[at+n]
		value := target.Name
		if target.Output != "" {
			value += "/" + target.Output
		}
		return action("workspace-switch", value)

	case wm.ActionSessionExit:
		return action("session-quit", "skip-confirmation")

	case wm.ActionOutputPower:
		if len(rest) < 1 || (rest[0] != "on" && rest[0] != "off") {
			return fmt.Errorf("output.power: expected on|off")
		}
		name := "dpms-on"
		if rest[0] == "off" {
			name = "dpms-off"
		}
		value := ""
		if len(rest) > 1 {
			value = rest[1]
		}
		return action(name, value)

	case wm.ActionOutputFocusDirection:
		if len(rest) < 1 {
			return fmt.Errorf("output.focusDirection: missing direction")
		}
		return action("output-focus-"+rest[0], "")

	case wm.ActionWindowFocusDirection:
		if len(rest) < 1 {
			return fmt.Errorf("window.focusDirection: missing direction")
		}
		return action("window-focus-"+rest[0], "")

	case wm.ActionWindowMoveToOutputBy:
		if len(rest) < 1 {
			return fmt.Errorf("window.moveToOutputBy: missing direction")
		}
		return action("window-move-to-output-"+rest[0], "")

	case wm.ActionWindowSummon:
		if len(rest) < 1 {
			return fmt.Errorf("window.summon: missing title")
		}
		return summon(rest[0])

	case wm.ActionOverviewToggle:
		return action("overview-toggle", "")

	case wm.ActionOutputEnable:
		if len(rest) != 2 || (rest[1] != "on" && rest[1] != "off") {
			return fmt.Errorf("output.enable: expected <output> on|off")
		}
		name := "output-enable"
		if rest[1] == "off" {
			name = "output-disable"
		}
		return action(name, rest[0])

	case wm.ActionOutputCycle:
		return cycleOutputs()

	case wm.ActionKeyboardCycleLayout:
		return action("keyboard-layout-next", "")

	case wm.ActionWindowMoveToWorkspace:
		if len(rest) < 2 {
			return fmt.Errorf("window.moveToWorkspace: missing window id or workspace")
		}
		// Umbriel moves the focused window only, so focus the target first.
		if err := action("window-focus", rest[0]); err != nil {
			return err
		}
		return action("window-move-to-workspace", rest[1])

	case wm.ActionOutputRefresh:
		return refreshOutput(rest)

	case wm.ActionInputTouchpad:
		return touchpadAction(rest)

	default:
		return fmt.Errorf("act %s: not supported by the experimental Umbriel provider", id)
	}
}

func focusWorkspace(id string) error {
	raw, err := query("workspaces")
	if err != nil {
		return err
	}
	var ws []uWorkspace
	if err := json.Unmarshal(raw, &ws); err != nil {
		return err
	}
	for _, w := range ws {
		if w.ID == id {
			value := w.Name
			if w.Output != "" {
				value += "/" + w.Output
			}
			return action("workspace-switch", value)
		}
	}
	return fmt.Errorf("workspace.focus: unknown workspace %q", id)
}

// touchpadAction implements the Fn touchpad keys and the Hub toggle. Umbriel
// 0.1.0 has no runtime input override (config-get/config-set are newer docs),
// so the intent is persisted in a watched optional include, like niri's
// config-only route. status prints on|off so a caller can restore exactly.
func touchpadAction(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("input.touchpad: expected on|off|toggle|status|restore")
	}
	dir := umbrielConfigDir()
	if dir == "" {
		return fmt.Errorf("input.touchpad: no Umbriel config dir")
	}
	path := filepath.Join(dir, "touchpad.toml")
	current := true
	if body, err := os.ReadFile(path); err == nil && strings.Contains(string(body), "enabled = false") {
		current = false
	}
	switch args[0] {
	case "status":
		if current {
			fmt.Fprintln(stdout, "on")
		} else {
			fmt.Fprintln(stdout, "off")
		}
		return nil
	case "restore":
		// A caller that saved `status` can pass that value back so a temporary
		// change cannot leave the pad in the wrong state after a restore.
		if len(args) > 1 {
			switch args[1] {
			case "on":
				current = true
			case "off":
				current = false
			default:
				return fmt.Errorf("input.touchpad: restore state must be on|off, got %q", args[1])
			}
			body := fmt.Sprintf("# Written by ryoku wm act input.touchpad. Do not edit.\n[input.touchpad]\nenabled = %t\n", current)
			return os.WriteFile(path, []byte(body), 0o644)
		}
		return nil
	case "on":
		current = true
	case "off":
		current = false
	case "toggle":
		current = !current
	default:
		return fmt.Errorf("input.touchpad: unknown action %q", args[0])
	}
	body := fmt.Sprintf("# Written by ryoku wm act input.touchpad. Do not edit.\n[input.touchpad]\nenabled = %t\n", current)
	return os.WriteFile(path, []byte(body), 0o644)
}

// cycleOutputs steps between "all outputs" and "one output", the display
// toggle key's job. It uses Umbriel's logical output actions, not DPMS, so a
// disabled output leaves the desktop layout rather than just going dark.
func cycleOutputs() error {
	raw, err := query("outputs")
	if err != nil {
		return err
	}
	var outs []uOutput
	if err := json.Unmarshal(raw, &outs); err != nil {
		return err
	}
	enabled := 0
	keep := ""
	for _, o := range outs {
		if o.Enabled {
			enabled++
			if keep == "" {
				keep = o.Name
			}
		}
	}
	if enabled <= 1 {
		for _, o := range outs {
			if !o.Enabled {
				if err := action("output-enable", o.Name); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, o := range outs {
		if o.Enabled && o.Name != keep {
			if err := action("output-disable", o.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func summon(title string) error {
	raw, err := query("windows")
	if err != nil {
		return err
	}
	var wins []uWindow
	if err := json.Unmarshal(raw, &wins); err != nil {
		return err
	}
	for _, w := range wins {
		if w.Title == title {
			return action("window-focus", w.ID)
		}
	}
	return fmt.Errorf("window.summon: no window titled %q", title)
}

// refreshOutput picks the advertised mode closest to the requested refresh and
// publishes it through monitors.toml, the file Umbriel watches for layout
// changes. There is no live refresh action in Umbriel 0.1.0's IPC.
func refreshOutput(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("output.refresh: expected <output> <hz>")
	}
	hz, err := strconv.ParseFloat(strings.TrimSpace(args[1]), 64)
	if err != nil || hz <= 0 {
		return fmt.Errorf("output.refresh: bad refresh %q", args[1])
	}
	raw, err := query("outputs")
	if err != nil {
		return err
	}
	var outs []uOutput
	if err := json.Unmarshal(raw, &outs); err != nil {
		return err
	}
	found := false
	layout := make([]wm.OutputLayout, 0, len(outs))
	for _, o := range outs {
		mode := ""
		for _, m := range o.Modes {
			full := fmt.Sprintf("%dx%d", m.Width, m.Height)
			if m.RefreshMHz > 0 {
				full += fmt.Sprintf("@%g", float64(m.RefreshMHz)/1000)
			}
			if m.Current {
				mode = full
			}
		}
		item := wm.OutputLayout{
			Name: o.Name, Enabled: o.Enabled, Mode: mode, Scale: o.Scale,
			X: o.Position.X, Y: o.Position.Y, Transform: transformOf(o.Transform),
		}
		if o.Name == args[0] {
			found = true
			modes := make([]string, 0, len(o.Modes))
			for _, m := range o.Modes {
				full := fmt.Sprintf("%dx%d", m.Width, m.Height)
				if m.RefreshMHz > 0 {
					full += fmt.Sprintf("@%g", float64(m.RefreshMHz)/1000)
				}
				modes = append(modes, full)
			}
			best, ok := wm.ClosestRefreshMode(mode, modes, hz)
			if !ok {
				return fmt.Errorf("output.refresh: output %q has no mode near %g Hz", args[0], hz)
			}
			item.Mode = best
		}
		layout = append(layout, item)
	}
	if !found {
		return fmt.Errorf("output.refresh: output %q does not exist", args[0])
	}
	dir := umbrielConfigDir()
	if dir == "" {
		return fmt.Errorf("output.refresh: no Umbriel config dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, "monitors.toml"), monitorsToml(layout), 0o644)
}
