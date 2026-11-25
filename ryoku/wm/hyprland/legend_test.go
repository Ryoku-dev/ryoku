package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// repoModules is the real shipped module tree, resolved from this test file so a
// bind removed from ryoku/hyprland/modules/binds.lua is seen by the parity gate.
func repoModules() string { return filepath.Join("..", "..", "hyprland", "modules") }

// The legend parser records chords, so pin shortcut-inhibitor flags in Lua.
func TestRecoveryBindsBypassShortcutInhibitor(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoModules(), "binds.lua"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	bindLine := func(chord string) string {
		t.Helper()
		start := strings.Index(src, chord)
		if start < 0 {
			t.Fatalf("recovery bind %s is missing", chord)
		}
		line := src[start:]
		if end := strings.IndexByte(line, '\n'); end >= 0 {
			line = line[:end]
		}
		return line
	}

	escape := bindLine(`K(mod .. " + Escape")`)
	if !strings.Contains(escape, "dont_inhibit = true") {
		t.Errorf("Escape does not bypass shortcut inhibition: %s", escape)
	}
	closeWindow := bindLine(`K(mod .. " + Q")`)
	if strings.Contains(closeWindow, "dont_inhibit = true") {
		t.Errorf("Q bypasses shortcut inhibition: %s", closeWindow)
	}
}

func TestFullscreenFocusRoutesAcrossMonitor(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoModules(), "binds.lua"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if !strings.Contains(body, "movefocus_cycles_fullscreen = true") {
		t.Error("binds.lua must keep same-workspace focus working from fullscreen windows")
	}
	for _, direction := range []string{"left", "right", "up", "down"} {
		if !strings.Contains(body, `function() focus_direction("`+direction+`") end`) {
			t.Errorf("fullscreen %s bind does not route through focus_direction", direction)
		}
	}
}

func comboSet(binds []parsedBind) map[string]bool {
	set := make(map[string]bool, len(binds))
	for _, b := range binds {
		set[b.combo] = true
	}
	return set
}

// A fixture with the numpad loop must parse to the concrete chords: the plain
// digit, the KP_ digit, and the NumLock-off twin drawn from kp_off, plus the
// literal binds around it.
func TestParseFixtureChords(t *testing.T) {
	const src = `local mod = "SUPER"
local K = require("modules.rebind")

hl.bind(K(mod .. " + Q"), hl.dsp.window.close()) -- close
hl.bind(K("ALT + Tab"), hl.dsp.focus({ last = true })) -- last

local kp_off = { "KP_End", "KP_Down", "KP_Next", "KP_Left", "KP_Begin", "KP_Right", "KP_Home", "KP_Up", "KP_Prior", "KP_Insert" }
for i = 1, 10 do
    local key = i % 10
    hl.bind(K(mod .. " + " .. key), hl.dsp.exec_cmd("ws focus " .. i)) -- focus
    hl.bind(K(mod .. " + KP_" .. key), hl.dsp.exec_cmd("ws focus " .. i)) -- focus kp
    hl.bind(K(mod .. " + " .. kp_off[i]), hl.dsp.exec_cmd("ws focus " .. i)) -- focus kp off
end
`
	set := comboSet(parseBindsSrc(src))
	for _, want := range []string{
		"SUPER + Q", "ALT + Tab",
		"SUPER + 3", "SUPER + 0", // digit, 10 -> 0
		"SUPER + KP_3", "SUPER + KP_0", // number pad, 10 -> KP_0
		"SUPER + KP_Next", "SUPER + KP_Insert", // twin of KP_3 and KP_0
	} {
		if !set[want] {
			t.Errorf("fixture parse missing chord %q; got %v", want, set)
		}
	}
}

// The number-pad loop in the real binds.lua yields sixty binds: three families
// on KP_1..KP_0 and the same three on the NumLock-off twins.
func TestNumpadLoopYields60(t *testing.T) {
	parsed := parseBindsDir(repoModules())
	kp := 0
	for _, b := range parsed {
		if containsKP(b.combo) {
			kp++
		}
	}
	if kp != 60 {
		t.Fatalf("number-pad binds = %d, want 60", kp)
	}
}

func containsKP(s string) bool {
	for i := 0; i+3 <= len(s); i++ {
		if s[i:i+3] == "KP_" {
			return true
		}
	}
	return false
}

// Every catalogue bind must be either matched in the real binds.lua or given a
// specific reason in hyprUnhonored. A generic reason means a bind fell out of the
// Lua with no reason recorded, which fails here. No Lua bind is left over as a
// spurious Hyprland-exclusive row.
func TestCatalogueParity(t *testing.T) {
	rows := buildBindRows(repoModules(), defaultOverrides())
	byID := make(map[string]wm.BindRow, len(rows))
	exclusive, custom := 0, 0
	for _, r := range rows {
		switch r.Category {
		case "Hyprland":
			exclusive++
		case "Custom":
			custom++
		default:
			byID[r.ID] = r
		}
	}

	unhonored := 0
	for _, c := range wm.ShippedBinds() {
		r, ok := byID[c.ID]
		if !ok {
			t.Errorf("catalogue id %q produced no row", c.ID)
			continue
		}
		if r.Unhonored == "" {
			continue
		}
		unhonored++
		want, listed := hyprUnhonored[c.ID]
		if !listed {
			t.Errorf("catalogue id %q is unhonored with a generic reason %q: it is neither bound in binds.lua nor given a specific reason", c.ID, r.Unhonored)
			continue
		}
		if r.Unhonored != want {
			t.Errorf("catalogue id %q reason = %q, want %q", c.ID, r.Unhonored, want)
		}
	}

	if unhonored != len(hyprUnhonored) {
		t.Errorf("unhonored rows = %d, want %d (%v)", unhonored, len(hyprUnhonored), hyprUnhonored)
	}
	if exclusive != 0 {
		t.Errorf("Hyprland-exclusive rows = %d, want 0 (every shipped bind is catalogued)", exclusive)
	}
	if custom != 0 {
		t.Errorf("custom rows on an empty store = %d, want 0", custom)
	}
	if len(rows) < 99 {
		t.Errorf("legend rows = %d, want >= 99", len(rows))
	}

	// Every id named in hyprUnhonored must actually be unhonored, so a stale
	// reason for a bind that is now bound is caught.
	for id := range hyprUnhonored {
		if r, ok := byID[id]; ok && r.Unhonored == "" {
			t.Errorf("hyprUnhonored lists %q but its row is honored; drop the stale reason", id)
		}
	}
}

// A numpad family is matched by its KP_ chords and consumes its twins, so it
// reports honored, not unhonored. It is rebindable as a unit: the Hub records one
// chord and the store keeps the {n}, moving all ten members together.
func TestNumpadFamilyMatched(t *testing.T) {
	rows := buildBindRows(repoModules(), defaultOverrides())
	for _, id := range []string{"workspace.focus.numpad", "workspace.moveWindow.numpad", "workspace.moveWindowSilent.numpad"} {
		found := false
		for _, r := range rows {
			if r.ID == id {
				found = true
				if r.Unhonored != "" {
					t.Errorf("%q reported unhonored %q, want matched", id, r.Unhonored)
				}
				if !r.Rebindable {
					t.Errorf("%q is a family; Rebindable should be true so the Hub can offer to change it", id)
				}
			}
		}
		if !found {
			t.Errorf("row %q missing", id)
		}
	}
}

// A family resolves through its family-level rebind: the row shows the effective
// {n} chord and stays rebindable and matched, while the default keeps the shipped
// {n} so the Hub can offer to reset it. matchCatalog reads the catalogue default,
// unchanged by the rebind, so the row is never wrongly reported unhonored.
func TestCatalogFamilyRebind(t *testing.T) {
	o := defaultOverrides()
	o.KeybindRebinds = map[string]string{"SUPER + {n}": "SUPER + CTRL + {n}"}
	var row wm.BindRow
	for _, r := range buildBindRows(repoModules(), o) {
		if r.ID == "workspace.focus" {
			row = r
			break
		}
	}
	if row.ID == "" {
		t.Fatal("workspace.focus row missing")
	}
	if row.Default != "SUPER + {n}" {
		t.Errorf("Default = %q, want SUPER + {n}", row.Default)
	}
	if row.Chord != "SUPER + CTRL + {n}" {
		t.Errorf("Chord = %q, want SUPER + CTRL + {n}", row.Chord)
	}
	if !row.Rebindable {
		t.Error("family row should be Rebindable")
	}
	if row.Unhonored != "" {
		t.Errorf("family should stay matched after a rebind, got unhonored %q", row.Unhonored)
	}
	if last := row.Keys[len(row.Keys)-1]; last != "1 \u2026 0" {
		t.Errorf("last key token = %q, want the family range", last)
	}
}

// A rebind moves the effective Chord and its display Keys while the Default (the
// key the rebind is recorded over) stays the shipped chord.
func TestRebindMovesChordNotDefault(t *testing.T) {
	o := defaultOverrides()
	o.KeybindRebinds = map[string]string{"SUPER + Q": "SUPER + X"}
	rows := buildBindRows(repoModules(), o)

	var row wm.BindRow
	for _, r := range rows {
		if r.ID == "window.close" {
			row = r
		}
	}
	if row.ID == "" {
		t.Fatal("window.close row missing")
	}
	if row.Default != "SUPER + Q" {
		t.Errorf("Default = %q, want SUPER + Q", row.Default)
	}
	if row.Chord != "SUPER + X" {
		t.Errorf("Chord = %q, want SUPER + X", row.Chord)
	}
	if len(row.Keys) == 0 || row.Keys[len(row.Keys)-1] != "X" {
		t.Errorf("Keys = %v, want to end in X", row.Keys)
	}
}

// A custom store bind Hyprland cannot emit is listed with a reason, not dropped;
// one it can emit is honored. A degenerate empty exec is left out entirely.
func TestCustomBinds(t *testing.T) {
	o := defaultOverrides()
	o.Keybinds = []Keybind{
		{Keys: "SUPER + Y", Action: "exec", Value: "kitty"},
		{Keys: "SUPER + Z", Action: "workspace", Value: "3"},
		{Keys: "SUPER + U", Action: "exec", Value: ""},
	}
	rows := buildBindRows(repoModules(), o)

	byID := map[string]wm.BindRow{}
	for _, r := range rows {
		if r.Category == "Custom" {
			byID[r.ID] = r
		}
	}
	if len(byID) != 2 {
		t.Fatalf("custom rows = %d, want 2 (empty exec dropped)", len(byID))
	}
	run := byID["custom.0"]
	if run.Label != "Run: kitty" || run.Unhonored != "" {
		t.Errorf("custom.0 = %+v, want label \"Run: kitty\" honored", run)
	}
	ws := byID["custom.1"]
	if ws.Label != "Focus workspace 3" || ws.Unhonored == "" {
		t.Errorf("custom.1 = %+v, want label \"Focus workspace 3\" with a reason", ws)
	}
}
