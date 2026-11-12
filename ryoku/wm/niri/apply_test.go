package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// apply is pure store -> KDL, so these exercise the generated files directly: the
// real niri binary validates them (skipped where niri is absent), the switch
// preview writes nothing, and the unhonored list names the losses a user reads.

func niriBin(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("niri")
	if err != nil {
		t.Skip("niri not installed; skipping validation")
	}
	return p
}

// niriHome points the provider's config and overlay trees at a temp dir and
// returns the niri config dir apply writes into.
func niriHome(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// touchpadDisabled() reads XDG_STATE_HOME; pin it to a clean dir so the
	// generated touchpad block does not depend on the dev box's live toggle.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return niriConfigDir()
}

func writeStore(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "desktop.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// capApply runs apply with stdout captured and returns the decoded report.
func capApply(t *testing.T, args ...string) wm.ApplyReport {
	t.Helper()
	var buf bytes.Buffer
	prev := stdout
	stdout = bufio.NewWriter(&buf)
	defer func() { stdout = prev }()
	if err := runApply(args); err != nil {
		t.Fatalf("runApply(%v): %v", args, err)
	}
	stdout.Flush()
	var rep wm.ApplyReport
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatalf("decode report: %v\n%s", err, buf.String())
	}
	return rep
}

// niri honours desktop.windows at runtime through the watch, not the config
// file, so apply must not list it among the losses a switch would cost. A
// regression here would tell a user the setting vanishes when it does not.
func TestWindowsHonoured(t *testing.T) {
	store := writeStore(t, `{"desktop":{"windows":{"tameMaximizeOnOpen":false}}}`)
	for _, u := range unhonored(store) {
		if strings.HasPrefix(u.Key, "desktop.windows") {
			t.Fatalf("desktop.windows must be honoured, got unhonored %q: %q", u.Key, u.Reason)
		}
	}
	if !defaultStore().Windows.TameMaximizeOnOpen {
		t.Fatal("the correction must default on, so the reported behaviour is fixed out of the box")
	}
}

// TestApplyPublishesGreeterNumlock: a real apply records the keypad choice
// where the login greeter reads it, so a user who turned "Numlock on at login"
// off is not forced back on at the next boot's login screen.
func TestApplyPublishesGreeterNumlock(t *testing.T) {
	niriHome(t)
	file := filepath.Join(t.TempDir(), "greeter-numlock")
	t.Setenv("RYOKU_GREETER_NUMLOCK_FILE", file)
	store := writeStore(t, `{"desktop":{"input":{"numlockByDefault":false}}}`)
	capApply(t, store)
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("apply wrote no greeter numlock hand-off: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "off" {
		t.Fatalf("hand-off = %q, want off", got)
	}
}

// TestApplyWritesKeysoundsConf: apply hands the key-sounds daemon its settings
// as a neutral key=value file, not the namespaced store, because the daemon
// holds no compositor knowledge. The leaves come from wm.niri.keysounds, so a
// change on the Hub's Behaviour tab reaches the running engine on Save.
func TestApplyWritesKeysoundsConf(t *testing.T) {
	niriHome(t)
	store := writeStore(t, `{"wm":{"niri":{"keysounds":{"enabled":false,"profile":"topre","volume":0.25,"release":false}}}}`)
	capApply(t, store)
	b, err := os.ReadFile(keysoundsConfPath())
	if err != nil {
		t.Fatalf("apply wrote no keysounds conf: %v", err)
	}
	got := string(b)
	for _, want := range []string{"enabled=false", "profile=topre", "volume=0.25", "release=false"} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("keysounds conf missing %q\n%s", want, got)
		}
	}
}

// TestKeysoundsDefaultsOn: a box whose provider has never applied (fresh login,
// nothing saved yet) still sounds, matching the Hyprland plugin's defaults.
func TestKeysoundsDefaultsOn(t *testing.T) {
	k := defaultStore().Niri.Keysounds
	if !k.Enabled || !k.Release || k.Profile != "cherry-mx-brown" || k.Volume != 0.6 {
		t.Fatalf("keysounds defaults = %+v, want enabled/release on, cherry-mx-brown at 0.6", k)
	}
}

func readGen(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// validateGen writes a tiny includer next to the generated files and runs the
// real niri validate against it, which also proves the missing-include rule
// cannot bite because both files are always present.
func validateGen(t *testing.T, dir string, includes ...string) {
	t.Helper()
	niri := niriBin(t)
	var b strings.Builder
	for _, inc := range includes {
		b.WriteString("include ")
		b.WriteString(kdlStr(inc))
		b.WriteString("\n")
	}
	cfg := filepath.Join(dir, "config.kdl")
	if err := os.WriteFile(cfg, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(niri, "validate", "-c", cfg).CombinedOutput(); err != nil {
		t.Fatalf("niri validate failed: %v\n%s", err, out)
	}
}

func TestApplyEmitsValidKDL(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{"desktop":{
		"appearance":{"gapsOut":10,"borderSize":3,"rounding":8,"activeBorder":"#ff0000","inactiveBorder":"#202020"},
		"input":{"kbLayout":"us","kbVariant":"colemak","kbOptions":"ctrl:nocaps","tapToClick":true,"naturalScroll":true,"sensitivity":0.3,"accelProfile":"flat","repeatRate":30,"repeatDelay":250},
		"cursor":{"theme":"Bibata-Modern-Ice","size":24,"inactiveTimeout":5},
		"env":[{"key":"QT_QPA_PLATFORM","value":"wayland"}],
		"autostart":[{"command":"waybar --config a"}],
		"windowRules":[{"class":"Spotify","action":"float"},{"class":"mpv","action":"opacity","value":"0.9"},{"title":"pip","action":"fullscreen"}],
		"appOverrides":[{"class":"kitty","opacity":0.95,"rounding":6,"borderSize":-1}],
		"keybinds":[{"keys":"SUPER + T","action":"exec","value":"kitty"},{"keys":"SUPER + G","action":"togglefloating"}]
	},"wm":{"niri":{"preferNoCsd":true,"overviewZoom":0.5}}}`)

	rep := capApply(t, store)
	if len(rep.Written) != 2 {
		t.Fatalf("written = %v, want settings.kdl + rebinds.kdl", rep.Written)
	}
	if rep.ReloadNeeded {
		t.Error("niri watches its own config; ReloadNeeded must be false")
	}

	settings := readGen(t, dir, "settings.kdl")
	if !strings.Contains(settings, "geometry-corner-radius 8") {
		t.Errorf("rounding not mapped to geometry-corner-radius\n%s", settings)
	}
	if !strings.Contains(settings, "open-floating true") {
		t.Errorf("float rule not emitted\n%s", settings)
	}
	rebinds := readGen(t, dir, "rebinds.kdl")
	if !strings.Contains(rebinds, `Super+T { spawn-sh "kitty"; }`) {
		t.Errorf("custom exec bind not emitted as spawn-sh\n%s", rebinds)
	}
	if !strings.Contains(rebinds, "Super+G { toggle-window-floating; }") {
		t.Errorf("custom togglefloating bind not emitted\n%s", rebinds)
	}
	validateGen(t, dir, "settings.kdl", "rebinds.kdl")
}

// A near-empty store still boots Ryoku's look: both files are written, carry the
// full baseline, validate, and two applies produce identical bytes.
func TestApplyAlwaysWritesBothAndIsDeterministic(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{}`)

	capApply(t, store)
	s1 := readGen(t, dir, "settings.kdl")
	r1 := readGen(t, dir, "rebinds.kdl")
	if !strings.Contains(s1, "layout {") || !strings.Contains(s1, "cursor {") {
		t.Errorf("baseline look missing from a default apply\n%s", s1)
	}
	if !strings.Contains(r1, "close-window") {
		t.Errorf("default binds missing from a default apply\n%s", r1)
	}

	capApply(t, store)
	if s2 := readGen(t, dir, "settings.kdl"); s2 != s1 {
		t.Error("settings.kdl not deterministic across two applies")
	}
	if r2 := readGen(t, dir, "rebinds.kdl"); r2 != r1 {
		t.Error("rebinds.kdl not deterministic across two applies")
	}
	validateGen(t, dir, "settings.kdl", "rebinds.kdl")
}

func TestPreviewWritesNothing(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{"desktop":{"appearance":{"gapsOut":5},"keybinds":[{"keys":"SUPER + Y","action":"exec","value":"foot"}]}}`)

	rep := capApply(t, store, "--preview")
	if len(rep.Written) != 0 {
		t.Fatalf("preview reported Written = %v, want none", rep.Written)
	}
	for _, name := range []string{"settings.kdl", "rebinds.kdl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("preview wrote %s", name)
		}
	}
}

// The unhonored list is the switch cost: a desktop.* leaf niri cannot express is
// named on its own, and a foreign compositor's whole namespace collapses to one
// aggregated line rather than a per-key dump.
func TestUnhonoredNamesForeignAndSubmap(t *testing.T) {
	niriHome(t)
	store := writeStore(t, `{"desktop":{
		"appearance":{"blurContrast":0.5,"gapsOut":8},
		"keybinds":[{"keys":"SUPER + ALT + 5","action":"submap","value":"resize"}]
	},"wm":{"hyprland":{"plugins":{"hyprbars":{"enabled":true}},"dwindle":{"preserveSplit":true},"anim":{"items":[]}}}}`)

	rep := capApply(t, store, "--preview")

	var blur, submap bool
	foreign := 0
	for _, u := range rep.Unhonored {
		if u.Key == "desktop.appearance.blurContrast" && strings.Contains(u.Reason, "blur") {
			blur = true
		}
		if strings.HasPrefix(u.Key, "wm.hyprland") {
			foreign++
			if !strings.Contains(u.Reason, "Hyprland") || !strings.Contains(u.Reason, "saved") {
				t.Errorf("foreign reason should name Hyprland and say the settings stay saved: %q", u.Reason)
			}
		}
		if u.Key == "desktop.keybinds[0]" && strings.Contains(u.Reason, "shortcut") {
			submap = true
		}
	}
	if !blur {
		t.Error("desktop.appearance.blurContrast must be reported unhonored, naming blur")
	}
	if !submap {
		t.Error("a submap keybind must be reported unhonored as an action a shortcut cannot run")
	}
	if foreign != 1 {
		t.Errorf("wm.hyprland reported %d entries, want exactly one aggregated line", foreign)
	}
}

// A rebind onto another default's chord must resolve to that chord once, with the
// rebind winning, or niri rejects the whole config as a duplicate.
func TestRebindOntoAnotherDefaultKeepsChordOnce(t *testing.T) {
	dir := niriHome(t)
	// Super+Q (close) is rebound onto Super+F, the default fullscreen chord.
	store := writeStore(t, `{"desktop":{"keybindRebinds":{"SUPER + Q":"SUPER + F"}}}`)

	capApply(t, store)
	rebinds := readGen(t, dir, "rebinds.kdl")

	var superF []string
	for _, line := range strings.Split(rebinds, "\n") {
		if strings.Contains(line, "Super+F") {
			superF = append(superF, strings.TrimSpace(line))
		}
	}
	if len(superF) != 1 {
		t.Fatalf("Super+F must appear once, got %d:\n%s", len(superF), rebinds)
	}
	if !strings.Contains(superF[0], "close-window") {
		t.Errorf("rebound close must win Super+F, got %q", superF[0])
	}
	if strings.Contains(rebinds, "fullscreen-window") {
		t.Errorf("the displaced fullscreen default must be dropped, not co-emitted\n%s", rebinds)
	}
	validateGen(t, dir, "rebinds.kdl")
}

// An unbound default chord is gone from the generated block, not left live.
func TestUnbindDropsDefault(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{"desktop":{"unbinds":["SUPER + Q"]}}`)

	capApply(t, store)
	rebinds := readGen(t, dir, "rebinds.kdl")
	if strings.Contains(rebinds, "Super+Q") {
		t.Errorf("unbound Super+Q must not be emitted\n%s", rebinds)
	}
	validateGen(t, dir, "rebinds.kdl")
}

// runDefaults must carry the niri exclusives under wm.niri, or SettingDomains
// advertises a namespace the Hub finds empty and gates rows with nothing behind.
func TestDefaultsCarryNiriNamespace(t *testing.T) {
	prev := aliveCheck
	aliveCheck = func(string) bool { return false } // offline, deterministic
	defer func() { aliveCheck = prev }()

	var buf bytes.Buffer
	prevOut := stdout
	stdout = bufio.NewWriter(&buf)
	defer func() { stdout = prevOut }()
	if err := runDefaults(); err != nil {
		t.Fatalf("runDefaults: %v", err)
	}
	stdout.Flush()

	var tree struct {
		Desktop map[string]json.RawMessage `json:"desktop"`
		WM      struct {
			Niri map[string]json.RawMessage `json:"niri"`
		} `json:"wm"`
	}
	if err := json.Unmarshal(buf.Bytes(), &tree); err != nil {
		t.Fatalf("decode defaults: %v\n%s", err, buf.String())
	}
	for _, k := range []string{"appearance", "input", "cursor"} {
		if _, ok := tree.Desktop[k]; !ok {
			t.Errorf("defaults missing desktop.%s", k)
		}
	}
	if len(tree.WM.Niri) == 0 {
		t.Fatal("wm.niri must carry the exclusive sections")
	}
	if _, ok := tree.WM.Niri["preferNoCsd"]; !ok {
		t.Error("wm.niri must include preferNoCsd")
	}
}

// niri can express drop shadows and window opacity, so the Hub shadow and opacity
// controls must reach the config; the leaves niri has no field for stay in the
// unhonored list, each naming the field it is missing.
func TestAppearanceShadowAndOpacityEmitted(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{"desktop":{"appearance":{
		"shadowEnabled":true,"shadowRange":30,"shadowColor":"#101010",
		"shadowPower":3,"shadowSharp":true,"shadowScale":0.9,
		"activeOpacity":0.8,"inactiveOpacity":0.6,"fullscreenOpacity":0.5
	}}}`)

	rep := capApply(t, store)
	settings := readGen(t, dir, "settings.kdl")

	for _, want := range []string{"shadow {", "softness 30", `color "#101010"`} {
		if !strings.Contains(settings, want) {
			t.Errorf("shadow not emitted, missing %q\n%s", want, settings)
		}
	}
	if !strings.Contains(settings, "opacity 0.8") {
		t.Errorf("global active opacity not emitted\n%s", settings)
	}
	if !strings.Contains(settings, "match is-active=false") || !strings.Contains(settings, "opacity 0.6") {
		t.Errorf("inactive opacity not emitted as an is-active=false rule\n%s", settings)
	}

	reason := map[string]string{}
	for _, u := range rep.Unhonored {
		reason[u.Key] = u.Reason
	}
	unhonored := map[string]string{
		"desktop.appearance.shadowPower":       "shadow",
		"desktop.appearance.shadowSharp":       "shadow",
		"desktop.appearance.shadowScale":       "shadow",
		"desktop.appearance.fullscreenOpacity": "full-screen",
	}
	for key, term := range unhonored {
		if !strings.Contains(reason[key], term) {
			t.Errorf("%s reason %q must name the missing niri feature (%q)", key, reason[key], term)
		}
	}
	for _, key := range []string{
		"desktop.appearance.shadowEnabled", "desktop.appearance.shadowRange",
		"desktop.appearance.shadowColor", "desktop.appearance.activeOpacity",
		"desktop.appearance.inactiveOpacity",
	} {
		if reason[key] != "" {
			t.Errorf("%s is emitted; it must not be reported unhonored (%q)", key, reason[key])
		}
	}
	validateGen(t, dir, "settings.kdl", "rebinds.kdl")
}

// The opacity precedence a user sees is the emission order, since niri lets the
// last matching rule win: the global floor first, focus state over it, then a
// specific app over both. A reorder would silently invert what the user sees.
func TestOpacityRuleOrder(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{"desktop":{
		"appearance":{"activeOpacity":0.9,"inactiveOpacity":0.7},
		"appOverrides":[{"class":"mpv","opacity":0.4,"rounding":-1,"borderSize":-1}]
	}}`)

	capApply(t, store)
	settings := readGen(t, dir, "settings.kdl")

	global := strings.Index(settings, "opacity 0.9")
	inactive := strings.Index(settings, "match is-active=false")
	perApp := strings.Index(settings, `app-id="mpv"`)
	if global < 0 || inactive < 0 || perApp < 0 {
		t.Fatalf("all three opacity tiers must be present\n%s", settings)
	}
	if !(global < inactive && inactive < perApp) {
		t.Errorf("order must be global < is-active=false < per-app, got %d,%d,%d\n%s", global, inactive, perApp, settings)
	}
	validateGen(t, dir, "settings.kdl", "rebinds.kdl")
}

// The pointer scroll-factor controls map onto niri's mouse and touchpad blocks,
// so a non-default factor reaches the right block and drops off the unhonored
// list.
func TestScrollFactorsEmitted(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{"desktop":{"input":{"mouseScrollFactor":1.5,"touchScrollFactor":0.8}}}`)

	rep := capApply(t, store)
	settings := readGen(t, dir, "settings.kdl")

	tp := strings.Index(settings, "touchpad {")
	ms := strings.Index(settings, "mouse {")
	touch := strings.Index(settings, "scroll-factor 0.8")
	mouse := strings.Index(settings, "scroll-factor 1.5")
	if !(tp >= 0 && tp < touch && touch < ms && ms < mouse) {
		t.Errorf("scroll-factor mapping wrong: want touchpad 0.8 then mouse 1.5\n%s", settings)
	}
	for _, u := range rep.Unhonored {
		if u.Key == "desktop.input.mouseScrollFactor" || u.Key == "desktop.input.touchScrollFactor" {
			t.Errorf("%s is emitted now; must not be reported unhonored", u.Key)
		}
	}
	validateGen(t, dir, "settings.kdl", "rebinds.kdl")
}

// The Hub edits preset column widths as one text field, so the store carries a
// comma-separated string; an older store or a hand-edit may still carry the JSON
// array, and a user may type percents. All three must reach niri as one
// proportion line per width, or the setting silently drops on the next save.
func TestPresetColumnWidthsStringAndArray(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"string", `"0.4, 0.6"`},
		{"array", `[0.4, 0.6]`},
		{"percent", `"40%, 60%"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := niriHome(t)
			store := writeStore(t, `{"desktop":{},"wm":{"niri":{"presetColumnWidths":`+tc.value+`}}}`)
			capApply(t, store)
			settings := readGen(t, dir, "settings.kdl")
			for _, want := range []string{"proportion 0.4", "proportion 0.6"} {
				if !strings.Contains(settings, want) {
					t.Errorf("%s: settings.kdl missing %q\n%s", tc.name, want, settings)
				}
			}
		})
	}
}

// A width cycle needs at least two widths; with one, Super+R has nothing to step
// to. A one-width store falls back to the default cycle in the generated config
// and is named unhonored so the user sees why; a real cycle is emitted as given
// and reported nowhere. Both are validated by the real niri.
func TestPresetColumnWidthsCycleFloor(t *testing.T) {
	reported := func(rep wm.ApplyReport) bool {
		for _, u := range rep.Unhonored {
			if u.Key == "wm.niri.presetColumnWidths" {
				return true
			}
		}
		return false
	}

	dirOne := niriHome(t)
	repOne := capApply(t, writeStore(t, `{"wm":{"niri":{"presetColumnWidths":"0.5"}}}`))
	genOne := readGen(t, dirOne, "settings.kdl")
	if !strings.Contains(genOne, "proportion 0.33333") || !strings.Contains(genOne, "proportion 0.66667") {
		t.Fatalf("a one-width preset must fall back to the default cycle:\n%s", genOne)
	}
	if !reported(repOne) {
		t.Fatalf("a one-width preset must be reported unhonored; got %+v", repOne.Unhonored)
	}
	validateGen(t, dirOne, "settings.kdl", "rebinds.kdl")

	dirTwo := niriHome(t)
	repTwo := capApply(t, writeStore(t, `{"wm":{"niri":{"presetColumnWidths":"0.4, 0.6"}}}`))
	genTwo := readGen(t, dirTwo, "settings.kdl")
	if !strings.Contains(genTwo, "proportion 0.4") || !strings.Contains(genTwo, "proportion 0.6") {
		t.Fatalf("a valid two-width cycle must be emitted as given:\n%s", genTwo)
	}
	if reported(repTwo) {
		t.Fatalf("a valid cycle must not be reported unhonored: %+v", repTwo.Unhonored)
	}
	validateGen(t, dirTwo, "settings.kdl", "rebinds.kdl")
}

// The niri surface the Window Manager page controls must reach the config: every
// new leaf emits its line, the neutral shadow spread/offset and middle-click
// toggle are honoured rather than reported lost, the tabbed toggle is bound so
// the tab-indicator is reachable, and the whole tree still validates.
func TestWindowManagerLeavesEmitted(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{"desktop":{
		"appearance":{"shadowEnabled":true,"shadowSpread":4,"shadowOffsetX":2,"shadowOffsetY":8},
		"input":{"middleClickPaste":false}
	},"wm":{"niri":{
		"defaultColumnWidth":0.65,
		"centerFocusedColumn":"on-overflow",
		"urgentColor":"#ff0000",
		"tabIndicatorWidth":6,
		"tabIndicatorHideSingle":true,
		"insertHint":false,
		"animationSlowdown":1.5,
		"blockOutApps":"org.keepassxc.KeePassXC, dev.secret.App"
	}}}`)

	rep := capApply(t, store)
	settings := readGen(t, dir, "settings.kdl")
	rebinds := readGen(t, dir, "rebinds.kdl")

	for _, want := range []string{
		"default-column-width { proportion 0.65; }",
		`center-focused-column "on-overflow"`,
		`urgent-color "#ff0000"`,
		"spread 4",
		"offset x=2 y=8",
		"tab-indicator {",
		"width 6",
		"hide-when-single-tab",
		"insert-hint {",
		"slowdown 1.5",
		"disable-primary",
		`block-out-from "screencast"`,
		`app-id="org.keepassxc.KeePassXC"`,
		`app-id="dev.secret.App"`,
	} {
		if !strings.Contains(settings, want) {
			t.Errorf("settings.kdl missing %q\n%s", want, settings)
		}
	}
	if !strings.Contains(rebinds, "Super+T { toggle-column-tabbed-display; }") {
		t.Errorf("tabbed-column toggle must be bound so the tab indicator is reachable\n%s", rebinds)
	}

	// The neutral shadow shape and the middle-click toggle are now expressible,
	// so they must drop off the switch-cost list rather than read as losses.
	for _, u := range rep.Unhonored {
		switch u.Key {
		case "desktop.appearance.shadowSpread", "desktop.appearance.shadowOffsetX",
			"desktop.appearance.shadowOffsetY", "desktop.input.middleClickPaste":
			t.Errorf("%s is emitted now; must not be reported unhonored (%q)", u.Key, u.Reason)
		}
	}
	validateGen(t, dir, "settings.kdl", "rebinds.kdl")
}

// The lone-window centring rule is a bare flag with no value, so the store
// toggle maps to the presence or absence of the always-center-single-column
// node, and both states validate. The overview-backdrop layer-rule is always
// emitted, since the provider cannot know whether the shell will map the
// surface, and a rule that matches nothing is inert.
func TestOverviewBackdropAndSingleColumnCentring(t *testing.T) {
	off := niriHome(t)
	capApply(t, writeStore(t, `{"wm":{"niri":{"alwaysCenterSingleColumn":false}}}`))
	dis := readGen(t, off, "settings.kdl")
	if strings.Contains(dis, "always-center-single-column") {
		t.Errorf("single-column centring off must omit the node\n%s", dis)
	}
	if !strings.Contains(dis, `match namespace="ryoku-overview-backdrop"`) ||
		!strings.Contains(dis, "place-within-backdrop true") {
		t.Errorf("overview-backdrop layer-rule missing\n%s", dis)
	}
	validateGen(t, off, "settings.kdl", "rebinds.kdl")

	on := niriHome(t)
	capApply(t, writeStore(t, `{"wm":{"niri":{"alwaysCenterSingleColumn":true}}}`))
	en := readGen(t, on, "settings.kdl")
	if !strings.Contains(en, "always-center-single-column") {
		t.Errorf("single-column centring on must emit the bare flag\n%s", en)
	}
	validateGen(t, on, "settings.kdl", "rebinds.kdl")
}

// "DYNAMIC" is a store role, not a theme on disk: the loaded store must carry
// the concrete wallpaper-following theme into the KDL, or niri opens a theme
// that does not exist and the pointer falls back to a bitmap.
func TestLoadStoreResolvesDynamicCursor(t *testing.T) {
	store := writeStore(t, `{"desktop":{"cursor":{"theme":"DYNAMIC","size":18}}}`)
	s := loadStore(store)
	if s.Cursor.Theme != wm.CursorThemeMaterial {
		t.Fatalf("loaded theme = %q, want %q", s.Cursor.Theme, wm.CursorThemeMaterial)
	}
	var b strings.Builder
	writeCursor(&b, s.Cursor)
	if !strings.Contains(b.String(), `xcursor-theme "`+wm.CursorThemeMaterial+`"`) {
		t.Fatalf("KDL does not carry the resolved theme:\n%s", b.String())
	}
}

// The whole new surface, every niri exclusive and neutral blur key exercised at
// once, must round-trip through the real niri validator. Skipped where niri is
// absent, so it proves the generated blocks are grammatical wherever the binary
// is on PATH.
func TestNewSurfaceValidatesThroughNiri(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{
		"desktop":{
			"appearance":{"blurEnabled":true,"blurPasses":4,"blurNoise":0.03,"blurXray":true,"blurPopups":true,"activeOpacity":0.95,"inactiveOpacity":0.8,"rounding":8},
			"input":{"followMouse":1,"middleClickPaste":false},
			"appOverrides":[{"class":"kitty","opacity":0.9,"rounding":6,"borderSize":-1,"blur":"on"}],
			"windowRules":[
				{"class":"foo","action":"columnwidth","value":"0.65"},
				{"class":"bar","action":"minsize","value":"800x600"},
				{"class":"baz","action":"maxsize","value":"1200x900"},
				{"class":"qux","action":"scrollfactor","value":"1.5"},
				{"class":"a","action":"tiledstate"},
				{"class":"b","action":"babaisfloat"},
				{"class":"c","action":"noshadow"},
				{"class":"d","action":"xray"},
				{"class":"e","action":"blockout"}
			]
		},
		"wm":{"niri":{
			"frame":"both","borderGradient":true,"gradientFrom":"#e0563b","gradientTo":"#9b3226","gradientAngle":135,"gradientRelativeTo":"workspace-view",
			"backgroundColor":"#0b0e14","presetWindowHeights":"0.4, 0.6","defaultColumnDisplay":"tabbed","emptyWorkspaceAboveFirst":true,
			"backdropColor":"#101010","workspaceShadow":true,"workspaceShadowSoftness":50,"workspaceShadowSpread":12,"workspaceShadowOffsetY":14,"workspaceShadowColor":"#00000050",
			"hotkeyOverlayHideNotBound":true,"recentWindows":false,
			"warpMouseToFocus":"center-xy","focusFollowsMouseScroll":10,"workspaceAutoBackAndForth":true,"modKey":"Alt","modKeyNested":"Ctrl","disablePowerKey":true,
			"dndEdgeViewScrollTriggerWidth":40,"dndEdgeWorkspaceMaxSpeed":1600,
			"anim":{
				"workspaceSwitch":{"mode":"spring","dampingRatio":1.0,"stiffness":900,"epsilon":0.0001},
				"windowOpen":{"mode":"ease","durationMs":200,"curve":"ease-out-expo"},
				"windowClose":{"mode":"off"}
			},
			"layerRules":[
				{"namespace":"waybar","opacity":0.9,"cornerRadius":12,"blur":"on","shadow":"on","blockOut":true,"babaIsFloat":true},
				{"namespace":"notifications","opacity":-1,"cornerRadius":-1,"blur":"off","shadow":"off"}
			]
		}}
	}`)
	capApply(t, store)
	validateGen(t, dir, "settings.kdl", "rebinds.kdl")
}

// niri 26.04's window blur renders translucent windows opaque, so Ryoku no
// longer models global blur: every desktop.appearance.blur* key is reported
// unhonored, each naming the opaque-blur reason. Per-app blur stays honoured (a
// deliberately targeted surface), so an appOverride blur is not a reported loss.
func TestGlobalBlurUnhonored(t *testing.T) {
	niriHome(t)
	store := writeStore(t, `{"desktop":{
		"appearance":{"blurEnabled":true,"blurPasses":4,"blurNoise":0.03,"blurXray":true,"blurPopups":true,"blurContrast":0.5,"blurSize":8,"blurSpecial":true},
		"appOverrides":[{"class":"kitty","opacity":-1,"rounding":-1,"borderSize":-1,"blur":"off"}]
	}}`)
	rep := capApply(t, store, "--preview")

	reason := map[string]string{}
	for _, u := range rep.Unhonored {
		reason[u.Key] = u.Reason
	}
	for _, k := range []string{"blurEnabled", "blurPasses", "blurNoise", "blurXray", "blurPopups", "blurContrast", "blurSize", "blurSpecial"} {
		r := reason["desktop.appearance."+k]
		if r == "" {
			t.Errorf("desktop.appearance.%s must be reported unhonored now that niri has no window blur", k)
			continue
		}
		if !strings.Contains(strings.ToLower(r), "solid") {
			t.Errorf("desktop.appearance.%s reason must explain that blur makes windows solid: %q", k, r)
		}
	}
	for _, u := range rep.Unhonored {
		if strings.HasPrefix(u.Key, "desktop.appOverrides") && strings.Contains(strings.ToLower(u.Reason), "blur") {
			t.Errorf("per-app blur is honoured; must not be reported as a loss: %q", u.Reason)
		}
	}
}

// niri always focuses a clicked window, so the keyboard-only mode is a reported
// loss, while the click-with-scrolling mode is what niri does anyway.
func TestKeyboardOnlyFocusUnhonored(t *testing.T) {
	niriHome(t)
	for mode, want := range map[string]bool{"1": false, "2": false, "3": true} {
		store := writeStore(t, `{"desktop":{"input":{"followMouse":`+mode+`}}}`)
		rep := capApply(t, store, "--preview")
		reason := ""
		for _, u := range rep.Unhonored {
			if u.Key == "desktop.input.followMouse" {
				reason = u.Reason
			}
		}
		if (reason != "") != want {
			t.Errorf("followMouse %s: reported unhonored = %q, want reported %v", mode, reason, want)
		}
		if want && !strings.Contains(reason, "click") {
			t.Errorf("followMouse %s: reason must say niri focuses a clicked window: %q", mode, reason)
		}
	}
}

// niri's border follows the live palette (config_layout reads the recorded
// colours), so a following or a pinned border is honoured, never a reported loss.
func TestBorderPaletteChoiceIsHonoured(t *testing.T) {
	niriHome(t)
	for _, follow := range []string{"true", "false"} {
		store := writeStore(t, `{"desktop":{"appearance":{"borderFollowsPalette":`+follow+`,"activeBorder":"#112233"}}}`)
		rep := capApply(t, store, "--preview")
		for _, u := range rep.Unhonored {
			if strings.Contains(strings.ToLower(u.Key), "border") {
				t.Errorf("borderFollowsPalette %s: %s reported unhonored: %q", follow, u.Key, u.Reason)
			}
		}
	}
}

// The imprecise windowStyle reason now points a niri user at the Animations page,
// where the window-open animation lives, while the rotating-gradient keys stay
// correctly unhonored, naming niri's static border gradient.
func TestWindowStyleReasonPointsAtAnimations(t *testing.T) {
	niriHome(t)
	store := writeStore(t, `{"desktop":{"appearance":{"windowStyle":"dwindle","animatedBorder":true,"borderAngleSpeed":2}}}`)
	rep := capApply(t, store, "--preview")

	reason := map[string]string{}
	for _, u := range rep.Unhonored {
		reason[u.Key] = u.Reason
	}
	if r := reason["desktop.appearance.windowStyle"]; !strings.Contains(r, "Animations") {
		t.Errorf("windowStyle should point at the Animations page: %q", r)
	}
	if r := reason["desktop.appearance.animatedBorder"]; !strings.Contains(r, "gradient") {
		t.Errorf("animatedBorder should name niri's static gradient: %q", r)
	}
	if r := reason["desktop.appearance.borderAngleSpeed"]; !strings.Contains(r, "gradient") {
		t.Errorf("borderAngleSpeed should name niri's static gradient: %q", r)
	}
}

func TestApplyKeepsSettingsOnNormalColumn(t *testing.T) {
	dir := niriHome(t)
	store := writeStore(t, `{"wm":{"niri":{"settingsWindow":{"mode":"custom","width":0.5,"height":0.5}}}}`)
	capApply(t, store)
	settings := readGen(t, dir, "settings.kdl")
	// The retired settingsWindow value must not size the Settings window. The
	// only Settings rule is the builtin floating one at its designed 1280x800;
	// the retired value adds no second rule of its own.
	want := "window-rule {\n" +
		"    match title=\"^Ryoku Settings$\"\n" +
		"    open-floating true\n" +
		"    default-column-width { fixed 1280; }\n" +
		"    default-window-height { fixed 800; }\n" +
		"}\n"
	if !strings.Contains(settings, want) {
		t.Fatalf("the retired settingsWindow value must not change the Settings builtin rule:\n%s", settings)
	}
	if strings.Count(settings, `match title="^Ryoku Settings$"`) != 1 {
		t.Fatalf("the retired settingsWindow value must not add a second Settings rule:\n%s", settings)
	}
	validateGen(t, dir, "settings.kdl", "rebinds.kdl")
}
