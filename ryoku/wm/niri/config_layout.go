package main

import (
	"fmt"
	"strings"
)

// The layout and animation blocks: the window frame, gaps, preset sizes, shadow,
// tab indicator and background niri reads from the layout node, and the
// per-animation curves the animations node carries.

// writeLayout maps the neutral frame onto niri's border and focus-ring blocks and
// adds the drop shadow, gaps, preset sizes and background niri owns. The user's
// frame choice (border, focus ring, or both) decides which of the two frame
// blocks the sized width and colours land in; the other stays off. niri's border
// defaults to off and only merges on when its block carries an explicit on flag,
// so a bare width would leave the frame invisible. The default and preset widths,
// the centring rules, the urgent colour, the tab indicator, the insert hint, the
// struts and the workspace background are niri exclusives with no neutral key.
func writeLayout(b *strings.Builder, a Appearance, n Niri) {
	b.WriteString("layout {\n")
	fmt.Fprintf(b, "    gaps %d\n", a.GapsOut)
	if c := kdlColor(n.BackgroundColor); c != "" {
		fmt.Fprintf(b, "    background-color %s\n", c)
	}
	if n.DefaultColumnWidth > 0 {
		fmt.Fprintf(b, "    default-column-width { proportion %s; }\n", kdlFloat(n.DefaultColumnWidth))
	}
	if strings.ToLower(strings.TrimSpace(n.DefaultColumnDisplay)) == "tabbed" {
		b.WriteString("    default-column-display \"tabbed\"\n")
	}
	if c := centerFocused(n.CenterFocused); c != "" {
		fmt.Fprintf(b, "    center-focused-column %s\n", kdlStr(c))
	}
	if n.AlwaysCenterSingle {
		b.WriteString("    always-center-single-column\n")
	}
	if n.EmptyWorkspaceAbove {
		b.WriteString("    empty-workspace-above-first\n")
	}
	if len(n.PresetColumnWidths) > 0 {
		b.WriteString("    preset-column-widths {\n")
		for _, w := range n.PresetColumnWidths {
			fmt.Fprintf(b, "        proportion %s\n", kdlFloat(w))
		}
		b.WriteString("    }\n")
	}
	if len(n.PresetWindowHeights) > 0 {
		b.WriteString("    preset-window-heights {\n")
		for _, h := range n.PresetWindowHeights {
			fmt.Fprintf(b, "        proportion %s\n", kdlFloat(h))
		}
		b.WriteString("    }\n")
	}
	if s := n.Struts; s.Left != 0 || s.Right != 0 || s.Top != 0 || s.Bottom != 0 {
		b.WriteString("    struts {\n")
		fmt.Fprintf(b, "        left %d\n", s.Left)
		fmt.Fprintf(b, "        right %d\n", s.Right)
		fmt.Fprintf(b, "        top %d\n", s.Top)
		fmt.Fprintf(b, "        bottom %d\n", s.Bottom)
		b.WriteString("    }\n")
	}
	writeFrame(b, a, n)
	if a.ShadowEnabled {
		b.WriteString("    shadow {\n")
		b.WriteString("        on\n")
		if a.ShadowRange > 0 {
			fmt.Fprintf(b, "        softness %d\n", a.ShadowRange)
		}
		fmt.Fprintf(b, "        spread %d\n", a.ShadowSpread)
		fmt.Fprintf(b, "        offset x=%d y=%d\n", a.ShadowOffsetX, a.ShadowOffsetY)
		if c := kdlColor(a.ShadowColor); c != "" {
			fmt.Fprintf(b, "        color %s\n", c)
		}
		b.WriteString("    }\n")
	}
	if n.TabIndicatorWidth > 0 || n.TabIndicatorHide {
		b.WriteString("    tab-indicator {\n")
		if n.TabIndicatorWidth > 0 {
			fmt.Fprintf(b, "        width %d\n", n.TabIndicatorWidth)
		}
		if n.TabIndicatorHide {
			b.WriteString("        hide-when-single-tab\n")
		}
		b.WriteString("    }\n")
	}
	if !n.InsertHint {
		b.WriteString("    insert-hint {\n        off\n    }\n")
	}
	b.WriteString("}\n\n")
}

// writeFrame emits niri's border and focus-ring blocks per the user's frame
// choice. "border" (the default) draws a border and leaves the ring off,
// "focusRing" the reverse, "both" draws both. The sized width and the neutral
// colours land in whichever block is shown; a solid pair unless the user turned
// the active gradient on, in which case the active frame carries a gradient while
// the inactive frame stays a solid colour.
func writeFrame(b *strings.Builder, a Appearance, n Niri) {
	showBorder := strings.TrimSpace(n.Frame) != "focusRing"
	showRing := n.Frame == "focusRing" || n.Frame == "both"
	writeFrameBlock(b, "border", showBorder, a, n)
	writeFrameBlock(b, "focus-ring", showRing, a, n)
}

func writeFrameBlock(b *strings.Builder, node string, show bool, a Appearance, n Niri) {
	fmt.Fprintf(b, "    %s {\n", node)
	if show && a.BorderSize > 0 {
		b.WriteString("        on\n")
		fmt.Fprintf(b, "        width %d\n", a.BorderSize)
	} else {
		b.WriteString("        off\n")
	}
	if show {
		writeFrameColors(b, a, n)
	}
	b.WriteString("    }\n")
}

// writeFrameColors emits the active and inactive frame colours, or the active
// gradient plus a solid inactive colour when the user turned the gradient on. The
// urgent colour is always a solid. When the border follows the palette and the
// palette file holds usable colours, those drive the solid active and inactive
// colours so the frame tracks the wallpaper the way Hyprland's border does. The
// gradient rows stay as they are: an explicit user gradient wins over the solid
// palette colour for the active frame whenever the gradient toggle is on.
func writeFrameColors(b *strings.Builder, a Appearance, n Niri) {
	if n.BorderGradient && kdlColor(n.GradientFrom) != "" && kdlColor(n.GradientTo) != "" {
		fmt.Fprintf(b, "        active-gradient from=%s to=%s angle=%d relative-to=%s\n",
			kdlColor(n.GradientFrom), kdlColor(n.GradientTo), n.GradientAngle, kdlStr(gradientRelativeTo(n.GradientRelativeTo)))
		if c := kdlColor(a.InactiveBorder); c != "" {
			fmt.Fprintf(b, "        inactive-color %s\n", c)
		}
	} else {
		active, inactive := a.ActiveBorder, a.InactiveBorder
		if a.BorderFollowsPalette {
			if pa, pi, ok := borderPaletteColors(); ok {
				active, inactive = pa, pi
			}
		}
		if c := kdlColor(active); c != "" {
			fmt.Fprintf(b, "        active-color %s\n", c)
		}
		if c := kdlColor(inactive); c != "" {
			fmt.Fprintf(b, "        inactive-color %s\n", c)
		}
	}
	if c := kdlColor(n.UrgentColor); c != "" {
		fmt.Fprintf(b, "        urgent-color %s\n", c)
	}
}

// gradientRelativeTo guards the stored anchor against niri's two accepted words,
// so a stray value falls back to the window anchor rather than a parse error.
func gradientRelativeTo(s string) string {
	if strings.TrimSpace(s) == "workspace-view" {
		return "workspace-view"
	}
	return "window"
}

// writeAnimations turns every animation off, or renders the per-kind tree the
// user tuned. A kind left on "default" emits nothing so niri keeps its own
// animation; the slowdown factor still applies over the top. niri runs at full
// speed by default, so an untouched tree stays silent.
func writeAnimations(b *strings.Builder, a Appearance, n Niri) {
	if !a.Animations {
		b.WriteString("animations {\n    off\n}\n\n")
		return
	}
	var body strings.Builder
	if n.AnimationSlowdown > 0 && n.AnimationSlowdown != 1 {
		fmt.Fprintf(&body, "    slowdown %s\n", kdlNum(n.AnimationSlowdown))
	}
	kinds := []struct {
		node string
		spec AnimSpec
	}{
		{"workspace-switch", n.Anim.WorkspaceSwitch},
		{"window-open", n.Anim.WindowOpen},
		{"window-close", n.Anim.WindowClose},
		{"horizontal-view-movement", n.Anim.HorizontalViewMovement},
		{"window-movement", n.Anim.WindowMovement},
		{"window-resize", n.Anim.WindowResize},
		{"config-notification-open-close", n.Anim.ConfigNotificationOpenClose},
		{"screenshot-ui-open", n.Anim.ScreenshotUiOpen},
		{"overview-open-close", n.Anim.OverviewOpenClose},
	}
	for _, k := range kinds {
		writeAnimKind(&body, k.node, k.spec)
	}
	if body.Len() == 0 {
		return
	}
	b.WriteString("animations {\n")
	b.WriteString(body.String())
	b.WriteString("}\n\n")
}

// writeAnimKind renders one animation kind. "off" disables it, "spring" and
// "ease" carry the two families niri accepts; "default" (and anything unknown)
// emits nothing so niri keeps its own animation. The spring damping-ratio and
// epsilon go out as float literals, which niri's parser requires.
func writeAnimKind(b *strings.Builder, node string, s AnimSpec) {
	switch s.Mode {
	case "off":
		fmt.Fprintf(b, "    %s {\n        off\n    }\n", node)
	case "spring":
		fmt.Fprintf(b, "    %s {\n        spring damping-ratio=%s stiffness=%d epsilon=%s\n    }\n",
			node, kdlFloat(s.DampingRatio), s.Stiffness, kdlFloat(s.Epsilon))
	case "ease":
		fmt.Fprintf(b, "    %s {\n        duration-ms %d\n        curve %s\n    }\n",
			node, s.DurationMs, kdlStr(s.Curve))
	}
}

// centerFocused guards the stored value against niri's three accepted words; an
// unknown value omits the line so niri keeps its own default.
func centerFocused(s string) string {
	switch v := strings.ToLower(strings.TrimSpace(s)); v {
	case "never", "always", "on-overflow":
		return v
	}
	return ""
}
