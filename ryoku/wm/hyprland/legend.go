package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// The Hyprland bind legend. Hyprland's shipped shortcuts ARE the neutral Ryoku
// catalogue, registered in ~/.config/hypr/modules/binds.lua and any sibling
// module that binds through the K() rebind helper. This file parses those Lua
// binds, matches each against the catalogue by chord, and reports the FULL
// effective legend as []wm.BindRow: every catalogue row (marked unhonored where
// Hyprland cannot perform it), then any Lua bind the catalogue does not know as a
// Hyprland-exclusive row, then the user's own custom binds from the store. The
// cheatsheet and the Hub read this one list instead of parsing Lua themselves, so
// the surfaces never drift from the config the desktop loads.

// hyprUnhonored names each catalogue bind Hyprland cannot honour and why. Every
// such bind stays in the legend rather than vanishing, so the sheet reads
// honestly. A catalogue bind that is neither registered in binds.lua nor named
// here falls through to a generic reason, which the parity test rejects: a bind
// dropped from the Lua must be given its reason here on purpose, never silently.
var hyprUnhonored = map[string]string{
	"window.presetHeight":    "Use the resize shortcut instead.",
	"column.first":           "Hyprland has no columns.",
	"column.last":            "Hyprland has no columns.",
	"resize.resetHeight":     "Use the resize shortcut instead.",
	"workspace.reorderUp":    "Hyprland's workspaces are numbered and can't be reordered.",
	"workspace.reorderDown":  "Hyprland's workspaces are numbered and can't be reordered.",
	"shell.inhibitShortcuts": "Hyprland can't take shortcuts back from an app that grabbed them.",
}

// runBinds prints the effective bind legend as a JSON array of wm.BindRow, the
// seam's binds verb. It reads the shipped Lua modules and the neutral store from
// the same config home every other verb uses, so the chords reflect what the
// session actually emits. Any store path the client passes is ignored: the
// provider owns where the store lives, the same as apply.
func runBinds(_ []string) error {
	rows := buildBindRows(hyprModulesDir(), loadStore(desktopStorePath()))
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// hyprModulesDir is the live module tree the desktop loads, where binds.lua and
// its siblings sit.
func hyprModulesDir() string { return filepath.Join(hyprConfigDir(), "modules") }

// buildBindRows walks the catalogue in legend order, matching each shipped bind
// to a parsed Lua bind by normalised chord; a match takes the catalogue's own
// label and the user's rebind, an unmatched bind takes its unhonored reason. Any
// Lua bind the catalogue does not claim follows as a Hyprland-exclusive row, then
// the store's custom binds. modulesDir and o are arguments so the test can build
// against the repo tree and a constructed store.
func buildBindRows(modulesDir string, o Overrides) []wm.BindRow {
	parsed := parseBindsDir(modulesDir)
	bound := make(map[string]parsedBind, len(parsed))
	for _, p := range parsed {
		n := wm.NormChord(p.combo)
		if _, seen := bound[n]; !seen {
			bound[n] = p
		}
	}
	consumed := make(map[string]bool, len(parsed))
	off := wm.DisabledSet(o.KeybindDisabled)

	rows := make([]wm.BindRow, 0, len(parsed)+len(o.Keybinds)+8)
	for _, c := range wm.ShippedBinds() {
		matched := matchCatalog(c, bound, consumed)
		rows = append(rows, catalogRow(c, matched, o.KeybindRebinds, off))
	}

	// Lua binds the catalogue does not document are Hyprland's own. None ship
	// today (binds.lua is the shared set), but a module could add one, so it is
	// reported under the compositor's name rather than dropped.
	for _, p := range parsed {
		n := wm.NormChord(p.combo)
		if consumed[n] {
			continue
		}
		consumed[n] = true
		rows = append(rows, exclusiveRow(p, off))
	}

	for i, k := range o.Keybinds {
		if row, ok := customRow(i, k); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// matchCatalog reports whether the Lua binds the catalogue entry, and marks the
// parsed binds it covers as consumed so they are not re-reported as exclusives. A
// family is matched when every one of its ten chords is bound; a numpad family
// also consumes the NumLock-off twin of each chord.
func matchCatalog(c wm.CatalogBind, bound map[string]parsedBind, consumed map[string]bool) bool {
	var required, extra []string
	if !c.Family {
		required = []string{wm.NormChord(c.Chord)}
	} else {
		for _, ex := range c.Expand() {
			required = append(required, wm.NormChord(ex))
			if c.Numpad {
				for _, al := range wm.NumpadAliases(ex) {
					extra = append(extra, wm.NormChord(al))
				}
			}
		}
	}
	matched := true
	for _, ch := range required {
		if _, ok := bound[ch]; ok {
			consumed[ch] = true
		} else {
			matched = false
		}
	}
	for _, ch := range extra {
		if _, ok := bound[ch]; ok {
			consumed[ch] = true
		}
	}
	return matched
}

// catalogRow renders one catalogue entry as a BindRow: the catalogue's own label
// and hint, its default chord, the effective chord after the user's rebind, and
// the display tokens of that chord. An unmatched entry keeps its place but takes
// the reason Hyprland cannot honour it.
func catalogRow(c wm.CatalogBind, matched bool, rebinds map[string]string, off map[string]bool) wm.BindRow {
	// A family resolves through its family-level rebind, keeping the {n} so the
	// row carries the effective placeholder chord and DisplayKeys renders the
	// range. A plain bind takes the user's rebind when set.
	chord := c.Chord
	if c.Family {
		chord, _ = wm.FamilyRebind(c.Chord, rebinds)
	} else if to, ok := rebinds[c.Chord]; ok {
		if t := strings.TrimSpace(to); t != "" {
			chord = t
		}
	}
	row := wm.BindRow{
		ID:         c.ID,
		Category:   c.Category,
		Label:      c.Label,
		Hint:       c.Hint,
		Keys:       wm.DisplayKeys(chord),
		Default:    c.Chord,
		Chord:      chord,
		Kind:       c.Kind,
		Rebindable: rebindableCombo(c.Chord),
		Locked:     c.Locked,
		Disabled:   off[c.Chord],
	}
	// A switched-off bind holds no chord, so the sheet must not print the one it
	// would have had. The row stays, which is the point: the action is still
	// findable and can be given a chord again.
	if row.Disabled {
		row.Keys = nil
		row.Chord = ""
	}
	// A rebind that moved this bind onto the number pad works in one NumLock state
	// only: Hyprland's rebind path is a single K() lookup, so unlike a shipped
	// family it registers one keysym, not both faces. Name that limit in the hint
	// so the row reads honestly rather than firing in one state with no reason.
	if note := numpadRebindHint(chord); note != "" {
		if row.Hint != "" {
			row.Hint += ". " + note
		} else {
			row.Hint = note
		}
	}
	if !matched {
		reason, ok := hyprUnhonored[c.ID]
		if !ok {
			reason = "Not bound on Hyprland."
		}
		row.Unhonored = reason
	}
	// The Lua still names the bind, so it reads as matched; the K() table is
	// what keeps it off while its program is absent, and the row says so.
	if reason := c.Missing(hasProgram); reason != "" {
		row.Unhonored = reason
	}
	return row
}

// numpadOffNames is the set of NumLock-off number-pad keysyms, derived from the
// wm seam's digit->off mapping so this file names no keysym the catalogue does
// not already own.
var numpadOffNames = func() map[string]bool {
	off := map[string]bool{}
	for d := 0; d <= 9; d++ {
		digit := "KP_" + strconv.Itoa(d)
		for _, alias := range wm.NumpadAliases(digit) {
			for _, tok := range strings.Split(alias, " + ") {
				if tok != digit {
					off[tok] = true
				}
			}
		}
	}
	return off
}()

// numpadRebindHint names the NumLock limitation of a chord a rebind moved onto
// the number pad. Hyprland's rebind path is one K() lookup, so unlike the shipped
// families (which bind both keypad faces in Lua) it registers a single keysym: a
// digit fires with NumLock on, its NumLock-off twin with NumLock off. It returns
// "" when the chord holds no number-pad key.
func numpadRebindHint(chord string) string {
	if len(wm.NumpadAliases(chord)) > 0 {
		return "Works with NumLock on"
	}
	for _, tok := range strings.Split(chord, " + ") {
		if numpadOffNames[tok] {
			return "Works with NumLock off"
		}
	}
	return ""
}

// rebindableCombo reports whether the Hub may record a new chord over a bind: a
// plain keyboard chord can be, a pointer button or a dedicated media/hardware key
// cannot. Matches the niri provider so the two read the same rule.
func rebindableCombo(combo string) bool {
	if strings.TrimSpace(combo) == "" {
		return false
	}
	for _, tok := range strings.Split(combo, " + ") {
		if strings.HasPrefix(tok, "mouse") || strings.HasPrefix(tok, "XF86") {
			return false
		}
	}
	return true
}

// exclusiveRow renders a Lua bind the catalogue does not document as a row under
// the compositor's own name, its label the Lua comment (or a description read off
// the dispatcher when the line carried none).
func exclusiveRow(p parsedBind, off map[string]bool) wm.BindRow {
	label := strings.TrimSpace(p.comment)
	if label == "" {
		label = describeDispatcher(p.dispatcher)
	}
	row := wm.BindRow{
		ID:         "hyprland." + chordSlug(p.combo),
		Category:   "Hyprland",
		Label:      capitalize(label),
		Keys:       wm.DisplayKeys(p.combo),
		Default:    p.combo,
		Chord:      p.combo,
		Kind:       wm.BindWM,
		Rebindable: rebindableCombo(p.combo),
		Disabled:   off[p.combo],
	}
	if row.Disabled {
		row.Keys = nil
		row.Chord = ""
	}
	return row
}

// customRow renders one of the store's own shortcuts as a Custom row. The label
// reads the action in plain words; Hyprland honours the exec and the three window
// actions genKeybind emits and reports any other action as unhonored rather than
// dropping it. A degenerate empty-exec bind names nothing, so it is left out.
func customRow(i int, k Keybind) (wm.BindRow, bool) {
	if (k.Action == "exec" || k.Action == "") && strings.TrimSpace(k.Value) == "" {
		return wm.BindRow{}, false
	}
	row := wm.BindRow{
		ID:         fmt.Sprintf("custom.%d", i),
		Category:   "Custom",
		Label:      wm.CustomBindLabel(k.Action, k.Value),
		Keys:       wm.DisplayKeys(k.Keys),
		Default:    k.Keys,
		Chord:      k.Keys,
		Kind:       wm.BindCustom,
		Rebindable: false,
	}
	if reason, ok := customUnhonored(k); ok {
		row.Unhonored = reason
	}
	return row, true
}

// customUnhonored mirrors genKeybind: Hyprland emits the exec and the three
// window actions; any other action produces no bind, so the row is reported with
// a reason instead of silently doing nothing.
func customUnhonored(k Keybind) (string, bool) {
	switch k.Action {
	case "exec", "", "close", "fullscreen", "togglefloating":
		return "", false
	}
	return "Hyprland can't run this action from a shortcut.", true
}

// parsedBind is one Lua bind after evaluation: the chord it registers (a family
// loop is already expanded to its concrete chords), its trailing comment, and the
// dispatcher text for the rare exclusive with no comment.
type parsedBind struct {
	combo      string
	comment    string
	dispatcher string
}

var (
	reBindK   = regexp.MustCompile(`hl\.bind\(\s*K\((.*?)\)\s*,\s*(.+)$`)
	reTrailC  = regexp.MustCompile(`\s--\s+(.+?)\s*$`)
	reLuaList = regexp.MustCompile(`^local\s+([A-Za-z_]\w*)\s*=\s*\{(.*)\}\s*$`)
	reIndex   = regexp.MustCompile(`^([A-Za-z_]\w*)\[([A-Za-z_]\w*)\]$`)
	reExecCmd = regexp.MustCompile(`exec_cmd\("([^"]*)"`)
)

// parseBindsDir parses every Lua module in dir that registers binds through K().
// Files are read in name order for a stable legend; a module that binds no K()
// shortcut (a submap or switch file) contributes nothing.
func parseBindsDir(dir string) []parsedBind {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []parsedBind
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".lua") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if !strings.Contains(string(b), "hl.bind(K(") {
			continue
		}
		out = append(out, parseBindsSrc(string(b))...)
	}
	return out
}

// parseBindsSrc walks one module a line at a time. Only hl.bind(K(...)) lines
// count, so a submap or switch bind that does not go through the rebind helper is
// skipped. A `for i = 1, 10` loop expands to its ten iterations, so the workspace
// families land as their concrete chords: the digit, the number pad, and the
// NumLock-off twin drawn from the kp_off table declared just above the loop.
func parseBindsSrc(src string) []parsedBind {
	var out []parsedBind
	tables := map[string][]string{}
	inLoop := false
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := reLuaList.FindStringSubmatch(trimmed); m != nil {
			tables[m[1]] = parseStringList(m[2])
			continue
		}
		if strings.HasPrefix(trimmed, "for ") && strings.HasSuffix(trimmed, " do") {
			inLoop = true
			continue
		}
		if inLoop && trimmed == "end" {
			inLoop = false
			continue
		}
		if !strings.Contains(trimmed, "hl.bind(K(") {
			continue
		}
		m := reBindK.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		comment := ""
		if cm := reTrailC.FindStringSubmatch(line); cm != nil {
			comment = cm[1]
		}
		for _, combo := range resolveChords(m[1], inLoop, tables) {
			if combo == "" {
				continue
			}
			out = append(out, parsedBind{combo: combo, comment: comment, dispatcher: m[2]})
		}
	}
	return out
}

// resolveChords evaluates the K() argument into one chord, or ten when it sits in
// a workspace loop. The argument is either a quoted literal ("ALT + Tab") or a
// Lua concat of mod, string pieces, the loop digit `key`, and a kp_off[i] lookup.
func resolveChords(arg string, inLoop bool, tables map[string][]string) []string {
	arg = strings.TrimSpace(arg)
	if !inLoop {
		return []string{evalConcat(arg, 0, tables)}
	}
	out := make([]string, 0, 10)
	for i := 1; i <= 10; i++ {
		out = append(out, evalConcat(arg, i, tables))
	}
	return out
}

func evalConcat(arg string, i int, tables map[string][]string) string {
	if strings.HasPrefix(arg, "\"") && !strings.Contains(arg, "..") {
		return unquoteLua(arg)
	}
	var b strings.Builder
	for _, part := range strings.Split(arg, "..") {
		p := strings.TrimSpace(part)
		switch {
		case p == "mod":
			b.WriteString("SUPER")
		case strings.HasPrefix(p, "\""):
			b.WriteString(unquoteLua(p))
		case p == "key":
			b.WriteString(strconv.Itoa(i % 10))
		case p == "i":
			b.WriteString(strconv.Itoa(i))
		default:
			if idx := reIndex.FindStringSubmatch(p); idx != nil {
				n := i
				if idx[2] == "key" {
					n = i % 10
				}
				if list, ok := tables[idx[1]]; ok && n >= 1 && n <= len(list) {
					b.WriteString(list[n-1])
				}
			} else {
				b.WriteString(p)
			}
		}
	}
	return b.String()
}

func unquoteLua(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		return s[1 : len(s)-1]
	}
	return s
}

func parseStringList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if v := unquoteLua(strings.TrimSpace(part)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// describeDispatcher is the fallback label for a Lua bind that shipped with no
// trailing comment. Only reached for a compositor-exclusive bind, so it stays a
// short read off the dispatcher rather than a full vocabulary.
func describeDispatcher(d string) string {
	if m := reExecCmd.FindStringSubmatch(d); m != nil {
		return m[1]
	}
	switch {
	case strings.Contains(d, "window.close"):
		return "close window"
	case strings.Contains(d, "window.fullscreen"):
		return "fullscreen"
	case strings.Contains(d, "window.center"):
		return "centre window"
	case strings.Contains(d, "group"):
		return "group"
	case strings.Contains(d, "window.move"):
		return "move window"
	case strings.Contains(d, "focus"):
		return "focus"
	}
	return strings.TrimSpace(d)
}

// chordSlug turns a chord into a stable id fragment: lowercased, every run of
// non-alphanumeric characters collapsed to a single dot.
func chordSlug(chord string) string {
	var b strings.Builder
	prevDot := false
	for _, r := range strings.ToLower(chord) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDot = false
		} else if !prevDot {
			b.WriteByte('.')
			prevDot = true
		}
	}
	return strings.Trim(b.String(), ".")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}
