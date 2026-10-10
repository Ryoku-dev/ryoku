package main

import (
	"fmt"
	"strconv"
	"strings"
)

// The window-rule, per-app override, layer-rule and block-out blocks: the rules
// niri re-evaluates as windows and layer surfaces appear.

// overviewBackdropNamespace is the layer-shell namespace the shell maps its
// blurred overview-wallpaper surface under. The provider and the shell agree on
// this exact string: the shell names the surface, this rule routes it.
const overviewBackdropNamespace = "ryoku-overview-backdrop"

// writeLayerRules lifts the shell's overview-backdrop surface into niri's
// backdrop, then emits each user layer rule. The backdrop rule is always first
// and always emitted: it matches nothing until the shell maps that surface (only
// when the overviewBackdrop capability is live and the user turned it on), and a
// rule that matches nothing is inert. Ryoku keeps its wallpaper on a separate
// opaque surface that already paints every workspace, so unlike a shell whose
// wallpaper IS the backdrop this needs no transparent workspace background to be
// seen.
func writeLayerRules(b *strings.Builder, rules []LayerRule) {
	fmt.Fprintf(b, "layer-rule {\n    match namespace=%s\n    place-within-backdrop true\n}\n\n", kdlStr(overviewBackdropNamespace))
	for _, r := range rules {
		writeUserLayerRule(b, r)
	}
}

// writeUserLayerRule emits the fields niri can set on a matching layer surface.
// Opacity -1 and cornerRadius -1 leave niri's own value alone; blur and shadow
// "inherit" do the same. A rule with no namespace matches nothing useful, so it
// is skipped rather than emitted as a blanket rule.
func writeUserLayerRule(b *strings.Builder, r LayerRule) {
	if strings.TrimSpace(r.Namespace) == "" {
		return
	}
	b.WriteString("layer-rule {\n")
	fmt.Fprintf(b, "    match namespace=%s\n", kdlStr(r.Namespace))
	if r.Opacity >= 0 && r.Opacity <= 1 {
		fmt.Fprintf(b, "    opacity %s\n", kdlNum(r.Opacity))
	}
	if r.CornerRadius >= 0 {
		fmt.Fprintf(b, "    geometry-corner-radius %d\n", r.CornerRadius)
	}
	switch r.Blur {
	case "on":
		b.WriteString("    background-effect {\n        blur true\n    }\n")
	case "off":
		b.WriteString("    background-effect {\n        blur false\n    }\n")
	}
	switch r.Shadow {
	case "on":
		b.WriteString("    shadow {\n        on\n    }\n")
	case "off":
		b.WriteString("    shadow {\n        off\n    }\n")
	}
	if r.BlockOut {
		b.WriteString("    block-out-from \"screencast\"\n")
	}
	if r.BabaIsFloat {
		b.WriteString("    baba-is-float true\n")
	}
	b.WriteString("}\n\n")
}

// writeWindowRules emits the global corner radius and opacity, then each user
// window rule and per-app override niri can express. Rules niri cannot express
// are reported unhonored by apply, not silently dropped here.
func writeWindowRules(b *strings.Builder, a Appearance, rules []WindowRule, apps []AppOverride) {
	if a.Rounding > 0 {
		b.WriteString("window-rule {\n")
		fmt.Fprintf(b, "    geometry-corner-radius %d\n", a.Rounding)
		b.WriteString("    clip-to-geometry true\n")
		b.WriteString("}\n\n")
	}
	writeOpacityRules(b, a)
	for _, r := range builtinWindowRules {
		writeRuleBlock(b, r.class, r.title, r.props)
	}
	for _, r := range rules {
		if props := windowRuleProps(r); len(props) > 0 {
			writeRuleBlock(b, r.Class, r.Title, props)
		}
	}
	for _, ao := range apps {
		if props := appOverrideProps(ao); len(props) > 0 {
			writeRuleBlock(b, ao.Class, ao.Title, props)
		}
	}
}

// builtinWindowRules are the desktop's own windows, matched by the app-id or
// title each carries, floated at their designed size the way ryoku/hyprland/
// modules/window_rules.lua floats them. They go out before the user's rules, so
// a user rule on the same match wins (niri takes the last match).
type builtinRule struct {
	class string
	title string
	props []string
}

var builtinWindowRules = []builtinRule{
	// Floating windows open centred. Their own screen-fit maximums clamp these
	// designed sizes before niri places them on smaller outputs.
	{class: "", title: "^Welcome to Ryoku$", props: []string{"open-floating true", "default-column-width { fixed 1180; }", "default-window-height { fixed 760; }"}},
	{class: "", title: "^Rashin$", props: []string{"open-floating true", "default-column-width { fixed 1280; }", "default-window-height { fixed 820; }"}},
	// ryoport's GUI console clients (RDP, VNC): FreeRDP is asked to carry the
	// class below via /wm-class, and xwayland-satellite folds the X11 class
	// into the Wayland app-id, so the patterns match anywhere. Serial and
	// telnet berths ride a real terminal window, whose placement stays the
	// user's.
	{class: "ryoport_console", title: "", props: []string{"open-floating true"}},
	{class: "(v|V)ncviewer", title: "", props: []string{"open-floating true"}},
}

// writeOpacityRules emits the global opacity as a matchless window-rule and the
// inactive opacity as an is-active=false rule niri re-evaluates on focus change.
// Both carry no app match, so a later per-app opacity override still wins for its
// windows: in niri the last matching rule sets the value.
func writeOpacityRules(b *strings.Builder, a Appearance) {
	if a.ActiveOpacity > 0 && a.ActiveOpacity < 1 {
		b.WriteString("window-rule {\n")
		fmt.Fprintf(b, "    opacity %s\n", kdlNum(a.ActiveOpacity))
		b.WriteString("}\n\n")
	}
	if a.InactiveOpacity > 0 && a.InactiveOpacity < 1 {
		b.WriteString("window-rule {\n")
		b.WriteString("    match is-active=false\n")
		fmt.Fprintf(b, "    opacity %s\n", kdlNum(a.InactiveOpacity))
		b.WriteString("}\n\n")
	}
}

func writeRuleBlock(b *strings.Builder, class, title string, props []string) {
	b.WriteString("window-rule {\n")
	var match []string
	if class != "" {
		match = append(match, fmt.Sprintf("app-id=%s", kdlStr(class)))
	}
	if title != "" {
		match = append(match, fmt.Sprintf("title=%s", kdlStr(title)))
	}
	if len(match) > 0 {
		fmt.Fprintf(b, "    match %s\n", strings.Join(match, " "))
	}
	for _, p := range props {
		fmt.Fprintf(b, "    %s\n", p)
	}
	b.WriteString("}\n\n")
}

// windowRuleProps translates one neutral window-rule action into niri property
// lines, or returns nil when niri has no expression for it. The size and
// background-effect actions span several lines, so they carry their own inner
// indentation and writeRuleBlock re-indents each element by one level.
func windowRuleProps(r WindowRule) []string {
	switch r.Action {
	case "float":
		return []string{"open-floating true"}
	case "tile":
		return []string{"open-floating false"}
	case "fullscreen":
		return []string{"open-fullscreen true"}
	case "maximize":
		return []string{"open-maximized true"}
	case "norounding":
		return []string{"geometry-corner-radius 0"}
	case "opacity":
		return []string{fmt.Sprintf("opacity %s", kdlNum(parseFloat(r.Value, 1)))}
	case "workspace":
		return []string{fmt.Sprintf("open-on-workspace %s", kdlStr(r.Value))}
	case "noborder":
		return []string{"border {", "    off", "}"}
	case "columnwidth":
		return []string{fmt.Sprintf("default-column-width { proportion %s; }", kdlNum(parseFloat(r.Value, 0.5)))}
	case "minsize":
		return sizeProps("min", r.Value)
	case "maxsize":
		return sizeProps("max", r.Value)
	case "scrollfactor":
		return []string{fmt.Sprintf("scroll-factor %s", kdlNum(parseFloat(r.Value, 1)))}
	case "tiledstate":
		return []string{"tiled-state true"}
	case "babaisfloat":
		return []string{"baba-is-float true"}
	case "noshadow":
		return []string{"shadow {", "    off", "}"}
	case "blur":
		return []string{"background-effect {", "    blur true", "}"}
	case "noblur":
		return []string{"background-effect {", "    blur false", "}"}
	case "xray":
		return []string{"background-effect {", "    xray true", "}"}
	case "blockout":
		return []string{`block-out-from "screencast"`}
	}
	return nil
}

// sizeProps parses a "WxH" value into niri's min/max width and height lines. A
// zero or absent dimension is left out, so a rule can bound one axis alone. A
// value that parses to neither returns nil, so apply reports it rather than
// emitting a match with no effect.
func sizeProps(prefix, value string) []string {
	w, h := parseWH(value)
	var props []string
	if w > 0 {
		props = append(props, fmt.Sprintf("%s-width %d", prefix, w))
	}
	if h > 0 {
		props = append(props, fmt.Sprintf("%s-height %d", prefix, h))
	}
	if len(props) == 0 {
		return nil
	}
	return props
}

// parseWH splits a "800x600" style value into width and height, tolerating the
// unicode times sign and whitespace a hand-edit might carry.
func parseWH(s string) (int, int) {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == 'x' || r == 'X' || r == '*' || r == ' ' || r == '\u00d7'
	})
	if len(parts) != 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return w, h
}

// appOverrideProps renders the per-app fields niri can express; -1 and "inherit"
// mean leave alone. A per-app border rule merges over the base border, so a width
// alone would not turn one on where the base is off, and a stored 0 would draw a
// zero-width line rather than none: the on flag draws the sized border, off drops
// it, matching how the layout border block resolves. Forced blur rides a
// background-effect the same way the global blur rule does.
func appOverrideProps(a AppOverride) []string {
	var props []string
	if a.Opacity >= 0 && a.Opacity <= 1 {
		props = append(props, fmt.Sprintf("opacity %s", kdlNum(a.Opacity)))
	}
	if a.Rounding >= 0 {
		props = append(props, fmt.Sprintf("geometry-corner-radius %d", a.Rounding))
	}
	switch {
	case a.BorderSize > 0:
		props = append(props, "border {", "    on", fmt.Sprintf("    width %d", a.BorderSize), "}")
	case a.BorderSize == 0:
		props = append(props, "border {", "    off", "}")
	}
	switch a.Blur {
	case "on":
		props = append(props, "background-effect {", "    blur true", "}")
	case "off":
		props = append(props, "background-effect {", "    blur false", "}")
	}
	return props
}

func parseFloat(s string, fallback float64) float64 {
	var f float64
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%g", &f); err != nil {
		return fallback
	}
	return f
}

// writeBlockOut hides each named app from screencasts with its own window-rule.
// "screencast" keeps the app out of screen shares and recordings while leaving
// the user's own screenshots working. The list is empty by default, so this is
// never a blanket switch: only the app-ids the user names are ever blanked.
func writeBlockOut(b *strings.Builder, list string) {
	for _, id := range splitList(list) {
		b.WriteString("window-rule {\n")
		fmt.Fprintf(b, "    match app-id=%s\n", kdlStr(id))
		b.WriteString("    block-out-from \"screencast\"\n")
		b.WriteString("}\n\n")
	}
}

// splitList splits a comma-separated field into trimmed, non-empty tokens.
func splitList(s string) []string {
	var out []string
	for _, tok := range strings.Split(s, ",") {
		if t := strings.TrimSpace(tok); t != "" {
			out = append(out, t)
		}
	}
	return out
}
