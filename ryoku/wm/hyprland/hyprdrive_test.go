package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	wm "ryoku-wm"
	"strings"
)

const hdBlock = hyprdriveBegin + "\nbody\n" + hyprdriveEnd + "\n"

func TestSpliceHyprdriveAppendsAfterTheUsersLines(t *testing.T) {
	got := spliceHyprdrive("-- mine\nhl.bind(\"SUPER + X\", f)", hdBlock)
	want := "-- mine\nhl.bind(\"SUPER + X\", f)\n\n" + hdBlock
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSpliceHyprdriveReplacesOnlyTheBlock(t *testing.T) {
	text := "a\n" + hyprdriveBegin + "\nold\n" + hyprdriveEnd + "\nb\n"
	got := spliceHyprdrive(text, hdBlock)
	if got != "a\n"+hdBlock+"b\n" {
		t.Fatalf("got %q", got)
	}
}

func TestSpliceHyprdriveRemovesTheBlockAndKeepsTheRest(t *testing.T) {
	text := "a\n\n" + hyprdriveBegin + "\nold\n" + hyprdriveEnd + "\nb\n"
	if got := spliceHyprdrive(text, ""); got != "a\nb\n" {
		t.Fatalf("got %q", got)
	}
	if got := spliceHyprdrive("a\n", ""); got != "a\n" {
		t.Fatalf("a file with no block changed: %q", got)
	}
}

func TestSpliceHyprdriveWithoutEndMarkerRunsToTheEnd(t *testing.T) {
	text := "a\n" + hyprdriveBegin + "\ntruncated"
	if got := spliceHyprdrive(text, hdBlock); got != "a\n"+hdBlock {
		t.Fatalf("got %q", got)
	}
}

func TestHyprdriveBlockIsEmptyWhileOff(t *testing.T) {
	if got := genHyprdriveBlock(defaultOverrides()); got != "" {
		t.Fatalf("an off plugin wrote %q", got)
	}
}

func TestBindOnDefaultsToOffOnlyWhenItTakesABindOver(t *testing.T) {
	b := hdBind{ID: "show-desktop", Chords: []string{"SUPER + D"}}
	if on, decided := bindOn(Hyprdrive{}, b, nil); !on || decided {
		t.Fatalf("a free chord: on=%v decided=%v", on, decided)
	}
	conflict := []hdConflict{{Chord: "SUPER + D", Label: "Maximise"}}
	if on, _ := bindOn(Hyprdrive{}, b, conflict); on {
		t.Fatal("a chord Ryoku uses was taken without asking")
	}
	if on, decided := bindOn(Hyprdrive{Binds: map[string]bool{"show-desktop": true}}, b, conflict); !on || !decided {
		t.Fatal("an accepted takeover stayed off")
	}
}

func TestConflictsFollowAMovedChord(t *testing.T) {
	b := hdBind{ID: "show-desktop", Chords: []string{"SUPER + D"}}
	taken := map[string]string{wm.NormChord("SUPER + D"): "Maximise"}
	if got := b.conflictsOf(Hyprdrive{}, taken); len(got) != 1 || got[0].Label != "Maximise" {
		t.Fatalf("got %v", got)
	}
	moved := Hyprdrive{Chords: map[string]string{"show-desktop": "SUPER + ALT + D"}}
	if got := b.conflictsOf(moved, taken); len(got) != 0 {
		t.Fatalf("a moved chord still conflicts: %v", got)
	}
}

func TestTakeoverConfigKeysFollowTheirSwitch(t *testing.T) {
	cfg := hyprdriveConfig(Hyprdrive{
		Binds:  map[string]bool{"tap-super": false},
		Config: map[string]any{"hyprdrive:tap_super": true, "hyprdrive:scale": 0.8, "other:key": 1.0},
	}, map[string]string{})
	if cfg["hyprdrive:tap_super"] != false {
		t.Fatal("a page value turned the Super tap back on behind its switch")
	}
	if cfg["hyprdrive:scale"] != 0.8 {
		t.Fatal("a page value was dropped")
	}
	if _, ok := cfg["other:key"]; ok {
		t.Fatal("a key outside the plugin's namespace was written")
	}
}

func TestRecordLayoutKeepsOneLinePerDesktop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layouts")
	for _, step := range []struct {
		id     int
		layout string
	}{{3, "scrolling"}, {1, "mosaic"}, {3, "mosaic"}} {
		if err := recordLayout(path, step.id, step.layout); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "1 mosaic\n3 mosaic\n" {
		t.Fatalf("got %q", got)
	}
}

func TestNextLayoutCycles(t *testing.T) {
	for from, to := range map[string]string{"dwindle": "scrolling", "scrolling": "mosaic", "mosaic": "dwindle", "master": "dwindle"} {
		if got := nextLayout(from); got != to {
			t.Fatalf("%s -> %s, want %s", from, got, to)
		}
	}
}

// withHyprdriveCopy lays a user-tier hyprdrive.so built for a pinned compositor,
// so genHyprdriveBlock emits a whole block without a running Hyprland.
func withHyprdriveCopy(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".local", "lib", "hyprland", "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := parseABI("aaaa_aq_0.15_hu_0.14_hg_0.5_hc_0.1_hlg_0.6")
	if err := os.WriteFile(filepath.Join(dir, "hyprdrive.so"), []byte("hyprdrive"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hyprdrive.abi"), []byte(target.String()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	liveOnce.Do(func() {})
	was := liveCached
	liveCached = compositorInfo{abi: target}
	t.Cleanup(func() { liveCached = was })
}

func TestMosaicIsSplitUntilThePluginLoads(t *testing.T) {
	o := defaultOverrides()
	o.Appearance.Layout = layoutMosaic
	o.Plugins.Hyprdrive.Enabled = true
	if s := genLua(o, false); strings.Contains(s, "mosaic") {
		t.Fatalf("settings.lua names mosaic before the plugin is loaded:\n%s", s)
	}
	if s := fullConfigLua(o, false); strings.Contains(s, `layout = "mosaic"`) || !strings.Contains(s, `layout = "dwindle"`) {
		t.Fatalf("the full preview should hold Split for Mosaic:\n%s", s)
	}
	if s := liveLua(o); !strings.Contains(s, `if p.name == "hyprdrive" then hl.config({ general = { layout = "mosaic" } })`) {
		t.Fatalf("the live preview should set Mosaic behind the loaded check:\n%s", s)
	}
	o.Plugins.Hyprdrive.Enabled = false
	if s := liveLua(o); strings.Contains(s, "mosaic") {
		t.Fatalf("Mosaic with Hyprdrive off should be Split:\n%s", s)
	}
}

func TestHyprdriveBlockSetsMosaicAfterTheLoad(t *testing.T) {
	withHyprdriveCopy(t)
	o := defaultOverrides()
	o.Plugins.Hyprdrive.Enabled = true
	if s := genHyprdriveBlock(o); strings.Contains(s, `layout = "mosaic"`) {
		t.Fatalf("Split chosen, yet the block sets Mosaic:\n%s", s)
	}
	o.Appearance.Layout = layoutMosaic
	s := genHyprdriveBlock(o)
	load, set := strings.Index(s, "hl.plugin.load"), strings.Index(s, `hl.config({ general = { layout = "mosaic" } })`)
	guard := strings.Index(s, "if ryoku_hyprdrive_loaded() then")
	if load < 0 || set < 0 || guard < 0 || !(load < guard && guard < set) {
		t.Fatalf("Mosaic must be set inside the loaded guard, after the load:\n%s", s)
	}
}

// Turning Hyprdrive on and then off gives the user's file back byte for byte,
// a trailing blank line included.
func TestSpliceHyprdriveOnThenOffKeepsTheFile(t *testing.T) {
	for _, text := range []string{"", "a\n", "a\n\n", "a\n\n\n", "-- mine\nend\n\n"} {
		on := spliceHyprdrive(text, hdBlock)
		if got := spliceHyprdrive(on, ""); got != text {
			t.Fatalf("%q: on then off gave %q", text, got)
		}
		if again := spliceHyprdrive(spliceHyprdrive(on, ""), hdBlock); again != on {
			t.Fatalf("%q: a second on differs: %q vs %q", text, again, on)
		}
	}
	// no newline at the end: it gains one, nothing else
	if got := spliceHyprdrive(spliceHyprdrive("a", hdBlock), ""); got != "a\n" {
		t.Fatalf("got %q", got)
	}
}

// Every Hyprdrive row on the Settings page has a default in the store: the Hub
// hides a row whose value is undefined, and the block would leave the key out.
func TestEveryHyprdriveRowHasADefault(t *testing.T) {
	raw, err := os.ReadFile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Tab string `json:"tab"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	defaults := hyprdriveDefaults()
	const prefix = "wm.hyprland.plugins.hyprdrive.config."
	n := 0
	for _, r := range rows {
		if r.Tab != "Hyprdrive" || !strings.HasPrefix(r.Key, prefix) {
			continue
		}
		n++
		if _, ok := defaults[strings.TrimPrefix(r.Key, prefix)]; !ok {
			t.Errorf("%s has no default", r.Key)
		}
	}
	if n < 50 {
		t.Fatalf("only %d Hyprdrive rows found", n)
	}
}

func TestHyprdriveBlockHealsAMissingPlugin(t *testing.T) {
	withHyprdriveCopy(t)
	o := defaultOverrides()
	o.Plugins.Hyprdrive.Enabled = true
	s := genHyprdriveBlock(o)
	guard, heal, end := strings.Index(s, "if ryoku_hyprdrive_loaded() then"), strings.Index(s, "\nelse\n  hl.exec_cmd("), strings.LastIndex(s, "\nend\n")
	if guard < 0 || heal < guard || end < heal {
		t.Fatalf("the heal must be the guard's else branch:\n%s", s)
	}
	cmd := hyprdriveHealCmd("/p/hyprdrive.so")
	// it reloads only once the plugin is held, so a plugin that will not load cannot loop
	if !strings.Contains(cmd, `grep -q "^Plugin hyprdrive " && hyprctl reload`) || !strings.Contains(cmd, `hyprctl plugin load "/p/hyprdrive.so"`) {
		t.Fatalf("heal command: %s", cmd)
	}
}

// The shortcuts that ship on are the overview's Super+Tab and the layout cycle;
// the rest ship off, and a switch the user flips still wins.
func TestShippedShortcutDefaults(t *testing.T) {
	want := map[string]bool{"tap-super": false, "overview-tab": true, "show-desktop": false, "scroll-desktops": false, "layout-cycle": true, "swipe-overview": false}
	h := hyprdriveDefaultsOverride()
	for _, b := range hyprdriveBinds {
		on, decided := bindOn(h, b, nil)
		if decided || on != want[b.ID] {
			t.Errorf("%s: on=%v decided=%v, want on=%v undecided", b.ID, on, decided, want[b.ID])
		}
	}
	h.Binds = map[string]bool{"tap-super": true}
	for _, b := range hyprdriveBinds {
		if b.ID == "tap-super" {
			if on, _ := bindOn(h, b, nil); !on {
				t.Error("the user's own switch should win over the shipped default")
			}
		}
	}
	// one that is on by default still waits when it would take a Ryoku shortcut over
	for _, b := range hyprdriveBinds {
		if b.ID == "overview-tab" {
			if on, _ := bindOn(hyprdriveDefaultsOverride(), b, []hdConflict{{Chord: "SUPER + Tab", Label: "x"}}); on {
				t.Error("a conflicting shortcut must stay off until accepted")
			}
		}
	}
}

func hyprdriveDefaultsOverride() Hyprdrive {
	return Hyprdrive{Config: hyprdriveDefaults()}
}

// A chord recorded on the Hyprdrive page replaces the shipped one in the block,
// and the block frees that chord, not the old one.
func TestRecordedChordReplacesTheShippedOne(t *testing.T) {
	withHyprdriveCopy(t)
	o := defaultOverrides()
	o.Plugins.Hyprdrive.Enabled = true
	o.Plugins.Hyprdrive.Binds = map[string]bool{"layout-cycle": true}
	o.Plugins.Hyprdrive.Chords = map[string]string{"layout-cycle": "SUPER + ALT + M"}
	s := genHyprdriveBlock(o)
	if !strings.Contains(s, `hl.bind("SUPER + ALT + M"`) || !strings.Contains(s, `hl.unbind("SUPER + ALT + M")`) {
		t.Fatalf("the recorded chord is not bound:\n%s", s)
	}
	if strings.Contains(s, `"SUPER + SHIFT + L"`) {
		t.Fatalf("the shipped chord is still in the block:\n%s", s)
	}
	o.Plugins.Hyprdrive.Chords = map[string]string{"layout-cycle": ""}
	if s := genHyprdriveBlock(o); !strings.Contains(s, `hl.bind("SUPER + SHIFT + L"`) {
		t.Fatalf("a cleared chord should give the shipped one back:\n%s", s)
	}
}
