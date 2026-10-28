package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// Every shipped catalogue id must resolve to exactly one of a niri action or a
// reason: an id with neither is a silent drop (a bind that vanishes from the
// session), an id with both is ambiguous. And no mapping may name an id the
// catalogue does not carry, or apply would translate a bind nothing ships.
func TestEveryCatalogueIdMappedOrReasoned(t *testing.T) {
	defs := defaultBinds()
	ids := map[string]bool{}
	for _, cb := range wm.ShippedBinds() {
		ids[cb.ID] = true
		nb, ok := defs[cb.ID]
		if !ok {
			t.Errorf("catalogue id %q has no niri mapping", cb.ID)
			continue
		}
		if (nb.action != "") == (nb.reason != "") {
			t.Errorf("catalogue id %q: want exactly one of action/reason (action=%q reason=%q)", cb.ID, nb.action, nb.reason)
		}
	}
	for id := range defs {
		if !ids[id] {
			t.Errorf("defaultBinds maps %q with no catalogue entry", id)
		}
	}
}

func TestDirectionalFocusFallsThroughToAdjacentMonitor(t *testing.T) {
	defs := defaultBinds()
	for id, want := range map[string]string{
		"focus.left":  "focus-column-or-monitor-left",
		"focus.right": "focus-column-or-monitor-right",
		"focus.up":    "focus-window-or-monitor-up",
		"focus.down":  "focus-window-or-monitor-down",
	} {
		if got := defs[id].action; got != want {
			t.Errorf("%s action = %q, want %q", id, got, want)
		}
	}

	rendered, _ := genBinds(loadStore(""))
	for _, want := range []string{
		"Super+Left { focus-column-or-monitor-left; }",
		"Super+Right { focus-column-or-monitor-right; }",
		"Super+Up { focus-window-or-monitor-up; }",
		"Super+Down { focus-window-or-monitor-down; }",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("generated binds missing %q", want)
		}
	}
}

func TestRecoveryBindsBypassShortcutInhibitor(t *testing.T) {
	rendered, _ := genBinds(loadStore(""))
	for _, want := range []string{
		`Super+Escape allow-inhibiting=false { spawn "ryoku-shell" "quicksettings"; }`,
		`Super+Shift+Escape allow-inhibiting=false { toggle-keyboard-shortcuts-inhibit; }`,
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("generated binds missing inhibitor recovery %q", want)
		}
	}
}

func TestMicMuteKeyDoesNotRepeat(t *testing.T) {
	if !defaultBinds()["media.micMute"].noRepeat {
		t.Fatal("media.micMute must ignore key repeat or one press can toggle twice")
	}
	rendered, _ := genBinds(loadStore(""))
	if want := `XF86AudioMicMute allow-when-locked=true repeat=false`; !strings.Contains(rendered, want) {
		t.Errorf("generated binds missing %q", want)
	}
}

func emittedChords(s niriStore) []string {
	out, _ := resolveBinds(s)
	chords := make([]string, len(out))
	for i, o := range out {
		chords[i] = o.chord
	}
	return chords
}

// Each number-pad family binds the ten keypad digits and the NumLock-off twin of
// each, twenty emitted binds, so a workspace shortcut on the keypad fires whether
// or not NumLock is on. Those twins are emitted binds only: the legend still
// lists one row per family, never twenty keypad entries.
func TestNumpadFamilyEmitsTwentyBinds(t *testing.T) {
	chords := emittedChords(loadStore(""))
	// The three families are disjoint by modifier prefix, so a prefix count is a
	// clean per-family total.
	for name, prefix := range map[string]string{
		"focus":  "Super+KP_",
		"move":   "Super+Alt+KP_",
		"silent": "Super+Shift+KP_",
	} {
		n := 0
		for _, c := range chords {
			if strings.HasPrefix(c, prefix) {
				n++
			}
		}
		if n != 20 {
			t.Errorf("numpad %s family emitted %d binds, want 20", name, n)
		}
	}
	kp := 0
	for _, c := range chords {
		if strings.Contains(c, "KP_") {
			kp++
		}
	}
	if kp != 60 {
		t.Errorf("emitted KP_ binds = %d, want 60", kp)
	}

	numpadRows := 0
	for _, r := range bindRows(loadStore("")) {
		if strings.HasSuffix(r.ID, ".numpad") {
			numpadRows++
		}
	}
	if numpadRows != 3 {
		t.Errorf("numpad legend rows = %d, want 3", numpadRows)
	}
}

// A number-pad chord a user brings in gets the same NumLock-off twin the shipped
// families do, so it fires whichever way NumLock sits. A custom bind and a rebind
// each emit their canonical KP_<digit> chord and its twin, while the legend still
// carries the one canonical chord.
func TestNumpadTwinForCustomAndRebind(t *testing.T) {
	has := func(chords []string, want string) bool {
		for _, c := range chords {
			if c == want {
				return true
			}
		}
		return false
	}

	// A custom bind on SUPER + KP_5 emits Super+KP_5 and its twin Super+KP_Begin.
	cs := loadStore("")
	cs.Keybinds = []Keybind{{Keys: "SUPER + KP_5", Action: "exec", Value: "kitty"}}
	custom := emittedChords(cs)
	if !has(custom, "Super+KP_5") || !has(custom, "Super+KP_Begin") {
		t.Errorf("custom KP_5 bind: want Super+KP_5 and Super+KP_Begin, got %v", custom)
	}

	// A rebind of window.close onto SUPER + KP_1 takes both keypad faces.
	rs := loadStore("")
	rs.KeybindRebinds = map[string]string{"SUPER + Q": "SUPER + KP_1"}
	out, _ := resolveBinds(rs)
	rebind := make([]string, len(out))
	for i, o := range out {
		rebind[i] = o.chord
	}
	if !has(rebind, "Super+KP_1") || !has(rebind, "Super+KP_End") {
		t.Errorf("rebound close to KP_1: want Super+KP_1 and Super+KP_End, got %v", rebind)
	}
	// Both faces carry the rebound action, so the rebind owns the physical key
	// whichever way NumLock sits, not a leftover workspace bind.
	for _, o := range out {
		if (o.chord == "Super+KP_1" || o.chord == "Super+KP_End") && o.action != defaultBinds()["window.close"].action {
			t.Errorf("chord %q action = %q, want the window.close action", o.chord, o.action)
		}
	}

	// The legend keeps one canonical row; the twin is an emitted bind only.
	n := 0
	for _, r := range bindRows(rs) {
		if r.ID == "window.close" {
			n++
			if r.Chord != "SUPER + KP_1" {
				t.Errorf("window.close chord = %q, want SUPER + KP_1", r.Chord)
			}
		}
	}
	if n != 1 {
		t.Errorf("window.close legend rows = %d, want 1", n)
	}
}

// A family rebind is stored as one {n} entry and moves all ten members at once:
// the digit stays and only the modifier set changes. The provider resolves the
// family before expanding, so every workspace chord shifts to the new modifiers
// and the shipped bare-digit chord is never emitted. A keypad family carries both
// NumLock faces. The legend keeps its one {n} row, marked rebindable, with the
// default it was recorded over.
func TestFamilyRebindMovesWholeFamily(t *testing.T) {
	s := loadStore("")
	s.KeybindRebinds = map[string]string{
		"SUPER + {n}":    "SUPER + CTRL + {n}",
		"SUPER + KP_{n}": "SUPER + ALT + KP_{n}",
	}
	set := map[string]bool{}
	for _, c := range emittedChords(s) {
		set[c] = true
	}

	for _, d := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"} {
		if !set["Super+Ctrl+"+d] {
			t.Errorf("focus family rebind: missing Super+Ctrl+%s", d)
		}
		if set["Super+"+d] {
			t.Errorf("focus family rebind: shipped Super+%s still emitted", d)
		}
	}

	// The keypad family moved onto Super+Alt+KP_ and kept both NumLock faces.
	if !set["Super+Alt+KP_1"] || !set["Super+Alt+KP_End"] {
		t.Error("keypad family rebind: want both Super+Alt+KP_1 and Super+Alt+KP_End")
	}
	if set["Super+KP_1"] {
		t.Error("keypad family rebind: shipped Super+KP_1 still emitted")
	}

	var row wm.BindRow
	for _, r := range bindRows(s) {
		if r.ID == "workspace.focus" {
			row = r
			break
		}
	}
	if row.ID == "" {
		t.Fatal("workspace.focus row missing")
	}
	if row.Default != "SUPER + {n}" {
		t.Errorf("default = %q, want SUPER + {n}", row.Default)
	}
	if row.Chord != "SUPER + CTRL + {n}" {
		t.Errorf("chord = %q, want SUPER + CTRL + {n}", row.Chord)
	}
	if !row.Rebindable {
		t.Error("family row should be Rebindable")
	}
	if want := wm.DisplayKeys("SUPER + CTRL + {n}"); !reflect.DeepEqual(row.Keys, want) {
		t.Errorf("keys = %v, want %v", row.Keys, want)
	}
}

// A per-member rebind keyed on a shipped concrete chord still wins over a family
// rebind, so a legacy per-key entry set before the family feature keeps working.
// Member three moves to its own target while the rest of the family follows the
// family rebind.
func TestFamilyRebindKeepsLegacyMember(t *testing.T) {
	s := loadStore("")
	s.KeybindRebinds = map[string]string{
		"SUPER + {n}": "SUPER + CTRL + {n}",
		"SUPER + 3":   "SUPER + F5",
	}
	set := map[string]bool{}
	for _, c := range emittedChords(s) {
		set[c] = true
	}
	if !set["Super+F5"] {
		t.Error("legacy per-member rebind of SUPER + 3 should win, want Super+F5")
	}
	if set["Super+Ctrl+3"] {
		t.Error("member three took the family chord instead of its per-member rebind")
	}
	if !set["Super+Ctrl+1"] || !set["Super+Ctrl+2"] || !set["Super+Ctrl+4"] {
		t.Error("the rest of the family should follow the family rebind")
	}
}

// The legend leads with every catalogue row in catalogue order and, with no
// custom binds, is nothing but those rows. The shape is the wm.BindRow contract,
// so it must round-trip through JSON into []wm.BindRow unchanged.
func TestBindRowsCatalogueFirst(t *testing.T) {
	rows := bindRows(loadStore(""))
	cat := wm.ShippedBinds()
	if len(rows) != len(cat) {
		t.Fatalf("rows = %d, want %d (default store carries no customs or exclusives)", len(rows), len(cat))
	}
	for i, cb := range cat {
		if rows[i].ID != cb.ID {
			t.Fatalf("row %d id = %q, want %q", i, rows[i].ID, cb.ID)
		}
		if rows[i].Default != cb.Chord {
			t.Errorf("row %q default = %q, want %q", cb.ID, rows[i].Default, cb.Chord)
		}
		if rows[i].Kind == wm.BindCustom {
			t.Errorf("catalogue row %q must not be kind custom", cb.ID)
		}
	}

	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var back []wm.BindRow
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("decode into []wm.BindRow: %v", err)
	}
	if !reflect.DeepEqual(back, rows) {
		t.Error("rows did not survive a JSON round trip into []wm.BindRow")
	}
}

// window.resize is the one catalogue row niri reworks: Hyprland's keyboard resize
// submap becomes stepping the column through its preset widths, so both the label
// and the hint carry niri's own copy rather than the neutral description.
func TestWindowResizeCopyOverride(t *testing.T) {
	var row wm.BindRow
	for _, r := range bindRows(loadStore("")) {
		if r.ID == "window.resize" {
			row = r
			break
		}
	}
	if row.ID == "" {
		t.Fatal("window.resize row missing")
	}
	if row.Label != "Step column width" {
		t.Errorf("label = %q, want Step column width", row.Label)
	}
	if row.Hint != "Cycle the column through its preset widths" {
		t.Errorf("hint = %q, want the niri preset-widths copy", row.Hint)
	}
}

// A rebind moves the chord the session emits and the display keys the sheet shows,
// but never the default: the default is the key the rebind is recorded over, so
// the Hub can always offer to reset it.
func TestRebindMovesChordNotDefault(t *testing.T) {
	s := loadStore("")
	s.KeybindRebinds = map[string]string{"SUPER + Q": "SUPER + X"}

	var row wm.BindRow
	for _, r := range bindRows(s) {
		if r.ID == "window.close" {
			row = r
			break
		}
	}
	if row.ID == "" {
		t.Fatal("window.close row missing")
	}
	if row.Default != "SUPER + Q" {
		t.Errorf("default = %q, want SUPER + Q", row.Default)
	}
	if row.Chord != "SUPER + X" {
		t.Errorf("chord = %q, want SUPER + X", row.Chord)
	}
	if want := wm.DisplayKeys("SUPER + X"); !reflect.DeepEqual(row.Keys, want) {
		t.Errorf("keys = %v, want %v", row.Keys, want)
	}

	out, _ := resolveBinds(s)
	got := ""
	for _, o := range out {
		if o.action == defaultBinds()["window.close"].action {
			got = o.chord
		}
	}
	if got != "Super+X" {
		t.Errorf("emitted close-window chord = %q, want Super+X", got)
	}
}

// The keysym names niri's xkb parser accepts, pinned so a rename cannot silently
// emit a chord niri rejects and cost a user their session. Verified live against
// niri validate (see TestRebindsKdlValidates); these lock the translation.
func TestNiriKeyAcceptsNames(t *testing.T) {
	cases := map[string]string{
		"KP_0": "KP_0", "KP_1": "KP_1", "KP_9": "KP_9",
		"KP_End": "KP_End", "KP_Down": "KP_Down", "KP_Next": "KP_Next",
		"KP_Left": "KP_Left", "KP_Begin": "KP_Begin", "KP_Right": "KP_Right",
		"KP_Home": "KP_Home", "KP_Up": "KP_Up", "KP_Prior": "KP_Prior",
		"KP_Insert": "KP_Insert", "KP_Enter": "KP_Enter", "KP_Add": "KP_Add",
		"KP_Subtract": "KP_Subtract", "KP_Multiply": "KP_Multiply",
		"KP_Divide": "KP_Divide", "KP_Decimal": "KP_Decimal",
		"Prior": "Page_Up", "Next": "Page_Down",
		"bracketleft": "bracketleft", "bracketright": "bracketright",
		"Home": "Home", "End": "End", "Print": "Print",
		"grave": "grave", "comma": "comma", "Tab": "Tab", "Return": "Return",
		"a": "A", "z": "Z",
	}
	for in, want := range cases {
		got, ok := niriKey(in)
		if !ok || got != want {
			t.Errorf("niriKey(%q) = %q,%v; want %q,true", in, got, ok, want)
		}
	}
	if _, ok := niriKey("mouse:272"); ok {
		t.Error("niriKey(mouse:272) should fail: a pointer button has no keysym")
	}
}

// The rebinds.kdl the default store renders must validate as real niri config:
// every action name, every keysym and every property has to be one niri accepts,
// or a deployed box boots with no session. Skipped where niri is not installed.
func TestRebindsKdlValidates(t *testing.T) {
	niriBin, err := exec.LookPath("niri")
	if err != nil {
		t.Skip("niri not on PATH")
	}
	src := "../../niri"
	if _, err := os.Stat(filepath.Join(src, "config.kdl")); err != nil {
		t.Skipf("shipped niri tree missing: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "niri")
	copyNiriTree(t, src, dst)

	s := defaultStore()
	binds, _ := genBinds(s)
	if err := os.WriteFile(filepath.Join(dst, "rebinds.kdl"), []byte(binds), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "settings.kdl"), genSettings(s), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(niriBin, "validate", "-c", filepath.Join(dst, "config.kdl")).CombinedOutput()
	if err != nil {
		t.Fatalf("niri validate rejected the rendered config: %v\n%s", err, out)
	}
}

// copyNiriTree copies the flat shipped niri config tree so the test can drop its
// own settings.kdl and rebinds.kdl beside the real includes.
func copyNiriTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
