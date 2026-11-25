package wm

import (
	"reflect"
	"testing"
)

// The catalogue is the one source both Kit and the Hub read, so a duplicated id
// would list a bind twice or record a rebind against the wrong entry. Every id
// must be unique.
func TestShippedBindsIDsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range ShippedBinds() {
		if seen[b.ID] {
			t.Errorf("duplicate bind id %q", b.ID)
		}
		seen[b.ID] = true
	}
}

// Clash detection compares chords in their normalised form, so a shipped chord
// that is not already its own normal form would never match a user rebind of the
// same combo. Every catalogue chord must normalise to itself.
func TestCatalogueChordsAreNormal(t *testing.T) {
	for _, b := range ShippedBinds() {
		if got := NormChord(b.Chord); got != b.Chord {
			t.Errorf("%s: NormChord(%q) = %q, not itself", b.ID, b.Chord, got)
		}
	}
}

// Categories carry the legend's section order. Pinned because the layout of both
// surfaces depends on it.
func TestCategoriesOrder(t *testing.T) {
	want := []string{"Windows", "Focus", "Move", "Resize", "Workspaces", "Displays", "Apps", "Shell", "Media", "Hardware", "Mouse"}
	if got := Categories(); !reflect.DeepEqual(got, want) {
		t.Errorf("Categories() = %v, want %v", got, want)
	}
}

// A family stands for ten binds on the digit keys 1..9 then 0, and a numpad
// family for KP_1..KP_9 then KP_0. A plain bind expands to just itself.
func TestExpandFamilies(t *testing.T) {
	digits := func(b CatalogBind) []string { return b.Expand() }

	focus := find(t, "workspace.focus")
	wantFocus := []string{
		"SUPER + 1", "SUPER + 2", "SUPER + 3", "SUPER + 4", "SUPER + 5",
		"SUPER + 6", "SUPER + 7", "SUPER + 8", "SUPER + 9", "SUPER + 0",
	}
	if got := digits(focus); !reflect.DeepEqual(got, wantFocus) {
		t.Errorf("workspace.focus expand = %v, want %v", got, wantFocus)
	}

	numpad := find(t, "workspace.focus.numpad")
	wantNumpad := []string{
		"SUPER + KP_1", "SUPER + KP_2", "SUPER + KP_3", "SUPER + KP_4", "SUPER + KP_5",
		"SUPER + KP_6", "SUPER + KP_7", "SUPER + KP_8", "SUPER + KP_9", "SUPER + KP_0",
	}
	if got := digits(numpad); !reflect.DeepEqual(got, wantNumpad) {
		t.Errorf("workspace.focus.numpad expand = %v, want %v", got, wantNumpad)
	}

	plain := find(t, "window.close")
	if got := plain.Expand(); !reflect.DeepEqual(got, []string{"SUPER + Q"}) {
		t.Errorf("window.close expand = %v, want one chord", got)
	}
}

// Every family in the catalogue must expand to exactly ten chords, so a section
// the Hub sizes from the family count is never short a row.
func TestEveryFamilyExpandsToTen(t *testing.T) {
	for _, b := range ShippedBinds() {
		if b.Family && len(b.Expand()) != 10 {
			t.Errorf("%s: family expands to %d, want 10", b.ID, len(b.Expand()))
		}
	}
}

// ExpandChord is the per-member substitution both the digit families and the
// number-pad families ride: {n} onto the digit n%10 so the tenth lands on 0, and
// a KP_{n} family carrying that same digit onto the pad. A chord with no
// placeholder passes through, so a rebind target that never carried one is safe.
func TestExpandChord(t *testing.T) {
	cases := []struct {
		chord string
		n     int
		want  string
	}{
		{"SUPER + {n}", 1, "SUPER + 1"},
		{"SUPER + {n}", 9, "SUPER + 9"},
		{"SUPER + {n}", 10, "SUPER + 0"},
		{"SUPER + CTRL + {n}", 2, "SUPER + CTRL + 2"},
		{"SUPER + KP_{n}", 3, "SUPER + KP_3"},
		{"SUPER + KP_{n}", 10, "SUPER + KP_0"},
		{"SUPER + ALT + KP_{n}", 10, "SUPER + ALT + KP_0"},
		{"SUPER + Q", 5, "SUPER + Q"},
	}
	for _, c := range cases {
		if got := ExpandChord(c.chord, c.n); got != c.want {
			t.Errorf("ExpandChord(%q, %d) = %q, want %q", c.chord, c.n, got, c.want)
		}
	}
}

// A family is rebound as a unit: the store keeps the {n} and changes only the
// modifier set, so one entry moves all ten members. FamilyRebind reads that entry
// and guards its shape, so a value that drops the placeholder, moves the digit,
// or carries a non-modifier token is refused rather than collapsing ten workspace
// keys onto one chord.
func TestFamilyRebind(t *testing.T) {
	cases := []struct {
		name    string
		def     string
		rebinds map[string]string
		want    string
		ok      bool
	}{
		{"digit moved", "SUPER + {n}", map[string]string{"SUPER + {n}": "SUPER + CTRL + {n}"}, "SUPER + CTRL + {n}", true},
		{"numpad moved", "SUPER + KP_{n}", map[string]string{"SUPER + KP_{n}": "SUPER + ALT + KP_{n}"}, "SUPER + ALT + KP_{n}", true},
		{"no entry", "SUPER + {n}", nil, "SUPER + {n}", false},
		{"identity", "SUPER + {n}", map[string]string{"SUPER + {n}": "SUPER + {n}"}, "SUPER + {n}", false},
		{"placeholder dropped", "SUPER + {n}", map[string]string{"SUPER + {n}": "SUPER + CTRL"}, "SUPER + {n}", false},
		{"digit turned numpad", "SUPER + {n}", map[string]string{"SUPER + {n}": "SUPER + CTRL + KP_{n}"}, "SUPER + {n}", false},
		{"non-modifier token", "SUPER + {n}", map[string]string{"SUPER + {n}": "SUPER + X + {n}"}, "SUPER + {n}", false},
	}
	for _, c := range cases {
		got, ok := FamilyRebind(c.def, c.rebinds)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: FamilyRebind(%q) = %q,%v; want %q,%v", c.name, c.def, got, ok, c.want, c.ok)
		}
	}
}

// The number pad sends a different keysym with NumLock off, so a provider binds
// both faces of each digit. NumpadAliases must map every digit to a distinct
// NumLock-off keysym, and leave a chord with no number-pad digit alone.
func TestNumpadAliasesRoundTrip(t *testing.T) {
	want := map[string]string{
		"SUPER + KP_1": "SUPER + KP_End",
		"SUPER + KP_2": "SUPER + KP_Down",
		"SUPER + KP_3": "SUPER + KP_Next",
		"SUPER + KP_4": "SUPER + KP_Left",
		"SUPER + KP_5": "SUPER + KP_Begin",
		"SUPER + KP_6": "SUPER + KP_Right",
		"SUPER + KP_7": "SUPER + KP_Home",
		"SUPER + KP_8": "SUPER + KP_Up",
		"SUPER + KP_9": "SUPER + KP_Prior",
		"SUPER + KP_0": "SUPER + KP_Insert",
	}
	distinct := map[string]bool{}
	for _, chord := range find(t, "workspace.focus.numpad").Expand() {
		got := NumpadAliases(chord)
		if len(got) != 1 || got[0] != want[chord] {
			t.Fatalf("NumpadAliases(%q) = %v, want [%q]", chord, got, want[chord])
		}
		if distinct[got[0]] {
			t.Errorf("NumpadAliases produced %q twice", got[0])
		}
		distinct[got[0]] = true
	}
	if len(distinct) != 10 {
		t.Errorf("aliases not a bijection: %d distinct, want 10", len(distinct))
	}

	if got := NumpadAliases("SUPER + Q"); len(got) != 0 {
		t.Errorf("NumpadAliases on a non-numpad chord = %v, want empty", got)
	}
	if got := NumpadAliases("SUPER + KP_{n}"); len(got) != 0 {
		t.Errorf("NumpadAliases on the family placeholder = %v, want empty", got)
	}
}

// The sheet shows readable tokens, not raw keysyms. Pin the placeholders and the
// awkward keysyms the catalogue actually carries.
func TestDisplayKeys(t *testing.T) {
	cases := []struct {
		chord string
		want  []string
	}{
		{"SUPER + KP_1", []string{"Super", "Num 1"}},
		{"SUPER + {n}", []string{"Super", "1 \u2026 0"}},
		{"SUPER + KP_{n}", []string{"Super", "Num 1 \u2026 0"}},
		{"SUPER + Prior", []string{"Super", "Page Up"}},
		{"SUPER + Next", []string{"Super", "Page Down"}},
		{"SUPER + grave", []string{"Super", "`"}},
		{"SUPER + comma", []string{"Super", ","}},
		{"SUPER + minus", []string{"Super", "-"}},
		{"SUPER + equal", []string{"Super", "="}},
		{"SUPER + bracketleft", []string{"Super", "["}},
		{"SUPER + bracketright", []string{"Super", "]"}},
		{"SUPER + mouse:272", []string{"Super", "LMB"}},
		{"SUPER + mouse:273", []string{"Super", "RMB"}},
		{"SUPER + mouse_up", []string{"Super", "Scroll Up"}},
		{"SUPER + mouse_down", []string{"Super", "Scroll Down"}},
		{"XF86AudioRaiseVolume", []string{"Vol +"}},
		{"XF86AudioLowerVolume", []string{"Vol -"}},
		{"XF86AudioMute", []string{"Mute"}},
		{"XF86AudioMicMute", []string{"Mic mute"}},
		{"XF86AudioPlay", []string{"Play"}},
		{"XF86AudioPause", []string{"Pause"}},
		{"XF86AudioNext", []string{"Next"}},
		{"XF86AudioPrev", []string{"Prev"}},
		{"XF86MonBrightnessUp", []string{"Brightness +"}},
		{"XF86MonBrightnessDown", []string{"Brightness -"}},
		{"XF86TouchpadToggle", []string{"Touchpad"}},
		{"XF86TouchpadOn", []string{"Touchpad On"}},
		{"XF86TouchpadOff", []string{"Touchpad Off"}},
		{"SUPER + SHIFT + Escape", []string{"Super", "Shift", "Esc"}},
		{"SUPER + Return", []string{"Super", "Enter"}},
		{"SUPER + Space", []string{"Super", "Space"}},
		{"ALT + Tab", []string{"Alt", "Tab"}},
		{"Print", []string{"Print"}},
	}
	for _, tc := range cases {
		if got := DisplayKeys(tc.chord); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("DisplayKeys(%q) = %v, want %v", tc.chord, got, tc.want)
		}
	}
}

// NormChord collapses spelling differences so clash detection compares combos,
// not strings: modifier case, order, and stray spacing wash out. The key keeps
// its own case, since keysyms like grave and KP_1 are case-sensitive.
func TestNormChordCollapsesSpelling(t *testing.T) {
	cases := []struct{ in, want string }{
		{"super + shift + P", "SUPER + SHIFT + P"},
		{"SHIFT+SUPER+P", "SUPER + SHIFT + P"},
		{"super  +  alt  +  ctrl + Left", "SUPER + CTRL + ALT + Left"},
		{"SUPER + Q", "SUPER + Q"},
	}
	for _, tc := range cases {
		if got := NormChord(tc.in); got != tc.want {
			t.Errorf("NormChord(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The client decodes what the provider reports. Two rows exercise the field set
// and the omitempty flags; a blank answer must decode to an empty, non-nil slice
// so the caller ranges over it without a guard.
func TestDecodeBinds(t *testing.T) {
	fixture := `[
		{"id":"window.close","category":"Windows","label":"Close window","keys":["Super","Q"],"default":"SUPER + Q","chord":"SUPER + Q","kind":"wm","rebindable":true},
		{"id":"media.mute","category":"Media","label":"Mute","keys":["Mute"],"default":"XF86AudioMute","chord":"XF86AudioMute","kind":"shell","locked":true}
	]`
	rows, err := decodeBinds([]byte(fixture))
	if err != nil {
		t.Fatalf("decodeBinds: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("decoded %d rows, want 2", len(rows))
	}
	if rows[0].ID != "window.close" || rows[0].Kind != BindWM || !rows[0].Rebindable {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if !reflect.DeepEqual(rows[0].Keys, []string{"Super", "Q"}) {
		t.Errorf("row 0 keys = %v", rows[0].Keys)
	}
	if rows[0].Unhonored != "" {
		t.Errorf("row 0 unhonored = %q, want empty", rows[0].Unhonored)
	}
	if rows[1].ID != "media.mute" || rows[1].Kind != BindShell || !rows[1].Locked {
		t.Errorf("row 1 = %+v", rows[1])
	}

	empty, err := decodeBinds([]byte("  \n"))
	if err != nil {
		t.Fatalf("decodeBinds empty: %v", err)
	}
	if empty == nil {
		t.Fatal("empty answer decoded to nil, want empty slice")
	}
	if len(empty) != 0 {
		t.Errorf("empty answer decoded to %d rows", len(empty))
	}
}

func find(t *testing.T, id string) CatalogBind {
	t.Helper()
	for _, b := range ShippedBinds() {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("bind %q not in catalogue", id)
	return CatalogBind{}
}

// A bind that launches an optional app loses its chord when the app is gone,
// and the legend names the program; every other bind is untouched by the lookup.
func TestMissingNamesTheAbsentProgram(t *testing.T) {
	absent := func(string) bool { return false }
	present := func(string) bool { return true }
	var needy, plain int
	for _, b := range ShippedBinds() {
		if b.Needs == "" {
			plain++
			if got := b.Missing(absent); got != "" {
				t.Errorf("%s needs nothing but reads missing: %q", b.ID, got)
			}
			continue
		}
		needy++
		if got := b.Missing(present); got != "" {
			t.Errorf("%s has its program but reads missing: %q", b.ID, got)
		}
		if b.NeedsName == "" || b.NeedsName == b.Needs {
			t.Errorf("%s names its program %q, not the app the user installs", b.ID, b.Needs)
		}
		if got := b.Missing(absent); got != b.NeedsName+" is not installed." {
			t.Errorf("%s missing reason = %q", b.ID, got)
		}
	}
	if needy == 0 || plain == 0 {
		t.Fatalf("catalogue has %d needy and %d plain binds; the test needs both", needy, plain)
	}
}
