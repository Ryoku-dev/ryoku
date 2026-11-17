package main

import (
	"fmt"
	"strconv"
	"strings"
)

// The top-level miscellany: the server-side-decoration hint, the hotkey overlay,
// the screenshot path, the gestures block, the overview, the recent-windows
// switcher, plus the environment and autostart blocks.

// writeMisc emits the niri-exclusive top-level settings: server-side decorations,
// the hotkey overlay, the screenshot path, the gestures, the overview and the
// recent-windows switcher.
func writeMisc(b *strings.Builder, n Niri) {
	if n.PreferNoCSD {
		b.WriteString("prefer-no-csd\n\n")
	}
	if n.HotkeyOverlaySkip || n.HotkeyOverlayHide {
		b.WriteString("hotkey-overlay {\n")
		if n.HotkeyOverlaySkip {
			b.WriteString("    skip-at-startup\n")
		}
		if n.HotkeyOverlayHide {
			b.WriteString("    hide-not-bound\n")
		}
		b.WriteString("}\n\n")
	}
	if strings.TrimSpace(n.ScreenshotPath) != "" {
		fmt.Fprintf(b, "screenshot-path %s\n\n", kdlStr(n.ScreenshotPath))
	}
	writeGestures(b, n)
	writeOverview(b, n)
	if !n.RecentWindows {
		b.WriteString("recent-windows {\n    off\n}\n\n")
	}
}

// writeGestures emits the gestures block: the hot-corner switch plus the two
// drag-and-drop edge-scroll gestures niri can tune. The dnd blocks appear only
// when the user moved them off niri's own defaults, so a stock session keeps the
// bare hot-corners block.
func writeGestures(b *strings.Builder, n Niri) {
	var body strings.Builder
	if !n.HotCorners {
		body.WriteString("    hot-corners {\n        off\n    }\n")
	}
	if n.DndViewTriggerWidth != 30 || n.DndViewDelayMs != 100 || n.DndViewMaxSpeed != 1500 {
		body.WriteString("    dnd-edge-view-scroll {\n")
		fmt.Fprintf(&body, "        trigger-width %d\n", n.DndViewTriggerWidth)
		fmt.Fprintf(&body, "        delay-ms %d\n", n.DndViewDelayMs)
		fmt.Fprintf(&body, "        max-speed %d\n", n.DndViewMaxSpeed)
		body.WriteString("    }\n")
	}
	if n.DndWsTriggerHeight != 50 || n.DndWsDelayMs != 100 || n.DndWsMaxSpeed != 1500 {
		body.WriteString("    dnd-edge-workspace-switch {\n")
		fmt.Fprintf(&body, "        trigger-height %d\n", n.DndWsTriggerHeight)
		fmt.Fprintf(&body, "        delay-ms %d\n", n.DndWsDelayMs)
		fmt.Fprintf(&body, "        max-speed %d\n", n.DndWsMaxSpeed)
		body.WriteString("    }\n")
	}
	if body.Len() == 0 {
		return
	}
	b.WriteString("gestures {\n")
	b.WriteString(body.String())
	b.WriteString("}\n\n")
}

// writeOverview emits the overview block: the zoom, the backdrop colour and the
// workspace shadow. Each part appears only when it moves niri off its own
// default, so a stock session keeps the bare zoom line.
func writeOverview(b *strings.Builder, n Niri) {
	var body strings.Builder
	if n.OverviewZoom > 0 {
		fmt.Fprintf(&body, "    zoom %s\n", kdlNum(n.OverviewZoom))
	}
	if c := kdlColor(n.BackdropColor); c != "" {
		fmt.Fprintf(&body, "    backdrop-color %s\n", c)
	}
	writeWorkspaceShadow(&body, n)
	if body.Len() == 0 {
		return
	}
	b.WriteString("overview {\n")
	b.WriteString(body.String())
	b.WriteString("}\n\n")
}

// writeWorkspaceShadow writes the overview workspace-shadow. niri draws it by
// default, so an on shadow at niri's own softness, spread and offset stays silent
// and only a disabled or retuned shadow reaches the config.
func writeWorkspaceShadow(b *strings.Builder, n Niri) {
	if !n.WorkspaceShadow {
		b.WriteString("    workspace-shadow {\n        off\n    }\n")
		return
	}
	tuned := n.WorkspaceShadowSoft != 40 || n.WorkspaceShadowSprd != 10 ||
		n.WorkspaceShadowY != 10 || strings.TrimSpace(n.WorkspaceShadowColor) != ""
	if !tuned {
		return
	}
	b.WriteString("    workspace-shadow {\n")
	b.WriteString("        on\n")
	fmt.Fprintf(b, "        softness %d\n", n.WorkspaceShadowSoft)
	fmt.Fprintf(b, "        spread %d\n", n.WorkspaceShadowSprd)
	fmt.Fprintf(b, "        offset x=0 y=%d\n", n.WorkspaceShadowY)
	if c := kdlColor(n.WorkspaceShadowColor); c != "" {
		fmt.Fprintf(b, "        color %s\n", c)
	}
	b.WriteString("    }\n")
}

// writeEnvironment folds the user env vars and the browser/terminal app roles into
// one environment block, so the CLI and xdg-open honour the same choice the
// keybinds launch.
func writeEnvironment(b *strings.Builder, env []EnvVar, apps map[string]string, c Cursor) {
	type kv struct{ k, v string }
	var rows []kv
	for _, e := range env {
		if strings.TrimSpace(e.Key) == "" {
			continue
		}
		rows = append(rows, kv{e.Key, e.Value})
	}
	if v := strings.TrimSpace(apps["browser"]); v != "" {
		rows = append(rows, kv{"BROWSER", v})
	}
	if v := strings.TrimSpace(apps["terminal"]); v != "" {
		rows = append(rows, kv{"TERMINAL", v})
	}
	// Spawned apps must wear the picked cursor, not the session default. The
	// environment.d baseline names a fixed Bibata set; without this export a
	// wallpaper-following (or any diverged) pick only reaches niri's own
	// pointer, and every app window keeps the static default that never
	// recolours. This is the niri twin of Hyprland's settings.lua cursor env.
	if c.Theme != "" {
		rows = append(rows, kv{"XCURSOR_THEME", c.Theme})
	}
	if c.Size > 0 {
		rows = append(rows, kv{"XCURSOR_SIZE", strconv.Itoa(c.Size)})
	}
	if len(rows) == 0 {
		return
	}
	b.WriteString("environment {\n")
	for _, r := range rows {
		fmt.Fprintf(b, "    %s %s\n", r.k, kdlStr(r.v))
	}
	b.WriteString("}\n\n")
}

// writeAutostart runs each user autostart command through the shell, matching how
// the Hyprland provider execs them.
func writeAutostart(b *strings.Builder, auto []Autostart) {
	wrote := false
	for _, a := range auto {
		if strings.TrimSpace(a.Command) == "" {
			continue
		}
		fmt.Fprintf(b, "spawn-sh-at-startup %s\n", kdlStr(a.Command))
		wrote = true
	}
	if wrote {
		b.WriteString("\n")
	}
}
