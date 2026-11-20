package main

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	wm "ryoku-wm"
)

// The shipped bind table is the template every apply starts from. It is owned
// by the provider, not by the config payload: config.toml includes only the
// generated rebinds.toml, so a rebind cannot leave the old chord active as a
// duplicate the way a second include would.
//
//go:embed binds.toml
var defaultBinds []byte

// familyKeys are the keys a {n} or KP_{n} bind expands to, in catalogue order.
// Umbriel names the number pad by key position rather than digit, so the two
// families use different spellings for the same workspace numbers.
var familyKeys = [10]string{
	"1", "2", "3", "4", "5", "6", "7", "8", "9", "0",
}

var numpadKeys = [10]string{
	"KP_End", "KP_Down", "KP_Next", "KP_Left", "KP_Begin",
	"KP_Right", "KP_Home", "KP_Up", "KP_Prior", "KP_Insert",
}

// umbrielChord turns a legend chord into the spelling Umbriel's keybind table
// uses. Only the differences are rewritten; every other key name is already
// the same. A chord Umbriel has no key for reports false.
func umbrielChord(chord string) (string, bool) {
	parts := strings.Split(chord, "+")
	if len(parts) == 0 {
		return "", false
	}
	out := make([]string, 0, len(parts))
	for i, raw := range parts {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			return "", false
		}
		if i < len(parts)-1 {
			switch strings.ToUpper(tok) {
			case "SUPER", "SUPERKEY", "MOD", "WIN", "META", "LOGO":
				out = append(out, "Mod")
			case "CTRL", "CONTROL":
				out = append(out, "Ctrl")
			case "ALT":
				out = append(out, "Alt")
			case "SHIFT":
				out = append(out, "Shift")
			default:
				return "", false
			}
			continue
		}
		switch tok {
		case "mouse_up":
			out = append(out, "WheelUp")
		case "mouse_down":
			out = append(out, "WheelDown")
		case "comma":
			out = append(out, "Comma")
		case "period":
			out = append(out, "Period")
		case "equal", "minus":
			out = append(out, tok)
		default:
			if strings.HasPrefix(tok, "mouse:") {
				return "", false
			}
			out = append(out, tok)
		}
	}
	return strings.Join(out, "+"), true
}

// chordAliases expands one legend chord into the concrete Umbriel chords it
// stands for. A plain chord has one alias; a family has ten.
func chordAliases(chord string) []string {
	chord = strings.TrimSpace(chord)
	if strings.Contains(chord, "KP_{n}") {
		base := strings.ReplaceAll(chord, "KP_{n}", "%s")
		out := make([]string, 0, len(numpadKeys))
		for _, key := range numpadKeys {
			c, ok := umbrielChord(fmt.Sprintf(base, key))
			if ok {
				out = append(out, c)
			}
		}
		return out
	}
	if strings.Contains(chord, "{n}") {
		base := strings.ReplaceAll(chord, "{n}", "%s")
		out := make([]string, 0, len(familyKeys))
		for _, key := range familyKeys {
			c, ok := umbrielChord(fmt.Sprintf(base, key))
			if ok {
				out = append(out, c)
			}
		}
		return out
	}
	c, ok := umbrielChord(chord)
	if !ok {
		return nil
	}
	return []string{c}
}

// bindLine is one rendered line of the generated [keybinds] table.
type bindLine struct {
	chord string
	value string
}

// resolvedBinds applies the store's rebinds and disabled list to the shipped
// bind template and returns the effective table plus the rows Umbriel cannot
// honour. Both the generated rebinds.toml and the keybind legend are built from
// this one resolution, so a rejected rebind never shows as effective in the Hub.
func resolvedBinds(rebinds map[string]string, disabled []string) ([]bindLine, []wm.Unhonored) {
	// Parse the shipped table first so a rebind onto an existing chord is
	// rejected before it can produce two bindings for the same key.
	var shipped []bindLine
	base := map[string]bool{}
	for _, raw := range strings.Split(string(defaultBinds), "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, `"`) {
			continue
		}
		at := strings.Index(line, `" = `)
		if at < 0 {
			continue
		}
		chord := line[1:at]
		shipped = append(shipped, bindLine{chord: chord, value: strings.TrimSpace(line[at+4:])})
		base[chord] = true
	}

	remap := map[string]string{}
	claimed := map[string]bool{}
	var unhonored []wm.Unhonored
	// Go map order is random; resolve sources in a stable order so two rebinds
	// competing for one target always produce the same winner.
	sources := make([]string, 0, len(rebinds))
	for from := range rebinds {
		sources = append(sources, from)
	}
	sort.Strings(sources)
	for _, from := range sources {
		to := rebinds[from]
		froms, tos := chordAliases(from), chordAliases(to)
		if len(froms) == 0 || len(tos) == 0 || len(froms) != len(tos) {
			unhonored = append(unhonored, wm.Unhonored{Key: "desktop.keybindRebinds." + from, Reason: "Umbriel cannot express this chord, or the new chord does not match the old one's key."})
			continue
		}
		// A source that is not shipped cannot be rebound; reject it so it does
		// not reserve a target another rebind needs.
		missing := false
		for _, chord := range froms {
			if !base[chord] {
				missing = true
				break
			}
		}
		if missing {
			unhonored = append(unhonored, wm.Unhonored{Key: "desktop.keybindRebinds." + from, Reason: "The original chord is not bound on Umbriel."})
			continue
		}
		// A family is one row in the Hub, so a single collision rejects the
		// whole rebind rather than silently remapping its other members.
		collides := false
		for i := range froms {
			if tos[i] != froms[i] && (base[tos[i]] || claimed[tos[i]]) {
				collides = true
				break
			}
		}
		if collides {
			unhonored = append(unhonored, wm.Unhonored{Key: "desktop.keybindRebinds." + from, Reason: "That chord is already bound on Umbriel."})
			continue
		}
		for i := range froms {
			remap[froms[i]] = tos[i]
			claimed[tos[i]] = true
		}
	}
	off := map[string]bool{}
	for _, chord := range disabled {
		for _, alias := range chordAliases(chord) {
			off[alias] = true
		}
	}

	var lines []bindLine
	for _, l := range shipped {
		// The Hub records a disabled chord under its shipped key even after a
		// rebind, so check the original before remapping it.
		if off[l.chord] {
			continue
		}
		if to, ok := remap[l.chord]; ok {
			l.chord = to
		}
		// A rebind target that is itself disabled must not be emitted.
		if off[l.chord] {
			continue
		}
		lines = append(lines, l)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].chord < lines[j].chord })
	return lines, unhonored
}

// renderBinds renders the resolved table as a TOML keybind section.
func renderBinds(rebinds map[string]string, disabled []string) (string, []wm.Unhonored) {
	lines, unhonored := resolvedBinds(rebinds, disabled)
	var b strings.Builder
	b.WriteString("[keybinds]\n")
	for _, l := range lines {
		fmt.Fprintf(&b, "%q = %s\n", l.chord, l.value)
	}
	return b.String(), unhonored
}
