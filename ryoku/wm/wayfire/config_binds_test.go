package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// Every catalogue id must land in defaultBinds: either an option wayfire can
// write or an honest reason the legend reports. An id absent here would leave
// a shipped bind off the sheet entirely.
func TestEveryCatalogueIdMappedOrReasoned(t *testing.T) {
	defs := defaultBinds()
	for _, cb := range wm.ShippedBinds() {
		wb, ok := defs[cb.ID]
		if !ok {
			t.Errorf("%s is missing from defaultBinds", cb.ID)
			continue
		}
		switch {
		case wb.reason != "":
			if wb.section != "" || wb.key != "" || wb.cmd != "" {
				t.Errorf("%s carries a reason and an expression at once", cb.ID)
			}
		case wb.cmd != "":
			if wb.key == "" {
				t.Errorf("%s is a command row without a key", cb.ID)
			}
		default:
			if wb.section == "" || wb.key == "" {
				t.Errorf("%s has neither an option nor a reason", cb.ID)
			}
		}
	}
	for id := range defs {
		found := false
		for _, cb := range wm.ShippedBinds() {
			if cb.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("defaultBinds carries %s, which the catalogue does not", id)
		}
	}
}

// Wayfire names keys by linux input codes, including upstream's KEY_PRINT and
// KEY_PLAYPAUSE spellings, and a mouse button remains a button. A wrong code
// parses silently and never fires. Wheel steps and unknown XF86 keysyms stay
// unmatched rather than guessed.
func TestActivatorSpellings(t *testing.T) {
	for chord, want := range map[string]string{
		"XF86AudioRaiseVolume": "KEY_VOLUMEUP",
		"XF86AudioPlay":        "KEY_PLAYPAUSE",
		"XF86AudioMicMute":     "KEY_MICMUTE",
		"XF86MonBrightnessUp":  "KEY_BRIGHTNESSUP",
		"XF86TouchpadToggle":   "KEY_TOUCHPAD_TOGGLE",
		"XF86AudioPrev":        "KEY_PREVIOUSSONG",
		"SUPER":                "<super>",
		"SUPER + Q":            "<super> KEY_Q",
		"SUPER + mouse:272":    "<super> BTN_LEFT",
		"SUPER + CTRL + Left":  "<super> <ctrl> KEY_LEFT",
		"SUPER + KP_1":         "<super> KEY_KP1",
		"SUPER + KP_End":       "<super> KEY_KP1",
		"SHIFT + Print":        "<shift> KEY_PRINT",
		"SUPER + bracketleft":  "<super> KEY_LEFTBRACE",
		"SUPER + mouse_up":     "",
		"XF86Whatever":         "",
	} {
		got, ok := toWayfireActivator(chord)
		if want == "" {
			if ok {
				t.Errorf("toWayfireActivator(%q) = %q, want no spelling", chord, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("toWayfireActivator(%q) = %q, %v; want %q", chord, got, ok, want)
		}
	}
}

// Wayfire resolves activator keys through libevdev's code names and drops a
// binding whose key it cannot name, silently, when the config loads. Every
// spelling the keysym table emits must be a kernel key name.
func TestKeysymTableUsesKernelKeyNames(t *testing.T) {
	header, err := os.ReadFile("/usr/include/linux/input-event-codes.h")
	if err != nil {
		t.Skipf("kernel input header unavailable: %v", err)
	}
	known := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^#define\s+((?:KEY|BTN)_[A-Z0-9_]+)`).FindAllSubmatch(header, -1) {
		known[string(m[1])] = true
	}
	for keysym, code := range wayfireKeys {
		if !known[code] {
			t.Errorf("%s maps to %s, which is not a kernel key name", keysym, code)
		}
	}
}

// The resolved emission is the session's truth: each catalogue row lands on its
// option with the shipped chord, families expand to all ten workspace members,
// and no activator is claimed twice.
func TestResolveEmitsCatalogue(t *testing.T) {
	out, report := resolveBinds(defaultStore())
	claimed := map[string]string{}
	for _, o := range out {
		if o.activator == "" {
			continue
		}
		if taker, dup := claimed[o.activator]; dup {
			t.Errorf("%q claimed twice: %s and %s", o.activator, taker, o.option)
		}
		claimed[o.activator] = o.option
	}
	// The pins read the rows as apply writes them: a family and its
	// number-pad twin share one slot with both activators joined.
	opts := map[string]outBind{}
	for _, o := range joinBinds(out) {
		opts[o.section+"."+o.option] = o
	}

	for option, want := range map[string]outBind{
		"command.binding_shell_launcher":                    {activator: "<super> KEY_SPACE", commandOpt: "command_shell_launcher", cmd: "ryoku-shell launcher"},
		"command.binding_window_close":                      {activator: "<super> KEY_Q", extra: "<alt> KEY_F4", commandOpt: "command_window_close"},
		"command.repeatable_binding_media_volume_up":        {activator: "KEY_VOLUMEUP", commandOpt: "command_media_volume_up", cmd: "ryoku-volume up"},
		"command.always_binding_media_play":                 {activator: "KEY_PLAYPAUSE", extra: "KEY_PLAYCD", commandOpt: "command_media_play", cmd: "ryoku-media play-pause"},
		"command.always_binding_media_pause":                {activator: "KEY_PAUSECD", commandOpt: "command_media_pause", cmd: "ryoku-media pause"},
		"command.always_binding_media_mute":                 {activator: "KEY_MUTE", commandOpt: "command_media_mute", cmd: "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle"},
		"command.always_binding_media_mic_mute":             {activator: "KEY_MICMUTE", commandOpt: "command_media_mic_mute", cmd: "wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle"},
		"command.repeatable_binding_hardware_brightness_up": {activator: "KEY_BRIGHTNESSUP", commandOpt: "command_hardware_brightness_up", cmd: "ryoku-cmd-brightness +5"},
		"command.always_binding_hardware_touchpad_toggle":   {activator: "KEY_TOUCHPAD_TOGGLE", commandOpt: "command_hardware_touchpad_toggle", cmd: "ryoku-wm-wayfire act input.touchpad toggle"},
		"command.binding_shell_screenshot_print":            {activator: "KEY_PRINT", commandOpt: "command_shell_screenshot_print"},
		"command.binding_workspace_hide_window":             {activator: "<super> KEY_H", commandOpt: "command_workspace_hide_window", cmd: "ryoku-wm-wayfire act window.hide focused"},
		"command.binding_workspace_scratchpad":              {activator: "<super> <alt> KEY_H", commandOpt: "command_workspace_scratchpad", cmd: "ryoku-wm-wayfire act workspace.toggleSpecial scratchpad"},
		"command.binding_window_preset_width":               {activator: "<super> KEY_R", commandOpt: "command_window_preset_width", cmd: "ryoku-wm-wayfire act window.presetWidth"},
		"vswitch.binding_1":                                 {activator: "<super> KEY_1 | <super> KEY_KP1"},
		"vswitch.with_win_5":                                {activator: "<super> <alt> KEY_5 | <super> <alt> KEY_KP5"},
		"vswitch.send_win_9":                                {activator: "<super> <shift> KEY_9 | <super> <shift> KEY_KP9"},
		"vswitch.binding_10":                                {activator: "<super> KEY_0 | <super> KEY_KP0"},
		"wm-actions.toggle_fullscreen":                      {activator: "<super> KEY_F"},
		"command.binding_window_float":                      {activator: "<super> KEY_A", commandOpt: "command_window_float", cmd: "ryoku-wm-wayfire act window.float focused"},
		"command.binding_column_maximize":                   {activator: "<super> KEY_D", commandOpt: "command_column_maximize", cmd: "ryoku-wm-wayfire act window.maximize", mirrorSection: "simple-tile", mirrorOption: "key_toggle_maximized"},
		"fast-switcher.activate":                            {activator: "<alt> KEY_TAB"},
		"shortcuts-inhibit.break_grab":                      {activator: "<super> <shift> KEY_ESC"},
		"move.activate":                                     {activator: "<super> BTN_LEFT"},
		"resize.activate":                                   {activator: "<super> BTN_RIGHT"},
	} {
		got, ok := opts[option]
		if !ok {
			t.Errorf("%s not emitted", option)
			continue
		}
		if got.activator != want.activator {
			t.Errorf("%s activator = %q, want %q", option, got.activator, want.activator)
		}
		if want.extra != "" && got.extra != want.extra {
			t.Errorf("%s extra = %q, want %q", option, got.extra, want.extra)
		}
		if want.commandOpt != "" && got.commandOpt != want.commandOpt {
			t.Errorf("%s commandOpt = %q, want %q", option, got.commandOpt, want.commandOpt)
		}
		if want.cmd != "" && got.cmd != want.cmd {
			t.Errorf("%s cmd = %q, want %q", option, got.cmd, want.cmd)
		}
		if want.mirrorOption != "" &&
			(got.mirrorSection != want.mirrorSection || got.mirrorOption != want.mirrorOption) {
			t.Errorf("%s mirror = [%s] %s, want [%s] %s",
				option, got.mirrorSection, got.mirrorOption, want.mirrorSection, want.mirrorOption)
		}
	}

	var reasons []string
	for _, u := range report {
		reasons = append(reasons, u.Key+": "+u.Reason)
	}
	joined := strings.Join(reasons, "\n")
	if !strings.Contains(joined, "(default SUPER + T)") || !strings.Contains(joined, "tabbed") {
		t.Errorf("an impossible behaviour must be reported:\n%s", joined)
	}
	wantUnhonored := map[string]string{
		"desktop.keybinds (default SUPER + T)":            "no tabbed layout",
		"desktop.keybinds (default SUPER + bracketleft)":  "no keyboard merge action",
		"desktop.keybinds (default SUPER + bracketright)": "no keyboard merge action",
		"desktop.keybinds (default SUPER + mouse_up)":     "axis callbacks",
		"desktop.keybinds (default SUPER + mouse_down)":   "axis callbacks",
		"desktop.keybinds (default SUPER + CTRL + Prior)": "fixed cells",
		"desktop.keybinds (default SUPER + CTRL + Next)":  "fixed cells",
	}
	if len(report) != len(wantUnhonored) {
		t.Errorf("default unhonored report has %d rows, want %d: %v", len(report), len(wantUnhonored), report)
	}
	for _, u := range report {
		want, ok := wantUnhonored[u.Key]
		if !ok {
			t.Errorf("unexpected default unhonored row: %+v", u)
			continue
		}
		if !strings.Contains(u.Reason, want) {
			t.Errorf("%s reason = %q, want it to mention %q", u.Key, u.Reason, want)
		}
	}
	for _, u := range report {
		if strings.Contains(u.Reason, "already taken") {
			t.Errorf("a clean default store must not lose a claim: %v", u)
		}
	}
}

func TestTiledAndFloatingNativeBindsStayTogether(t *testing.T) {
	values := func(s wayfireStore) map[string]string {
		d := &iniDoc{}
		applyBinds(d, s)
		out := map[string]string{}
		for _, section := range d.sections {
			for _, key := range section.keys {
				out[section.name+"."+key.name] = key.value
			}
		}
		return out
	}

	got := values(defaultStore())
	if got["command.binding_window_float"] != "<super> KEY_A" {
		t.Errorf("float toggle = %q, want the float act's centred float/tile toggle", got["command.binding_window_float"])
	}
	for _, option := range []string{"simple-tile.key_toggle_maximized", "command.binding_column_maximize"} {
		if got[option] != "<super> KEY_D" {
			t.Errorf("%s = %q, want the shared tiled/floating maximize chord", option, got[option])
		}
	}

	unbound := defaultStore()
	unbound.Unbinds = []string{"SUPER + D"}
	got = values(unbound)
	for _, option := range []string{"simple-tile.key_toggle_maximized", "command.binding_column_maximize"} {
		if got[option] != "" {
			t.Errorf("unbound %s = %q, want both maximize handlers disabled", option, got[option])
		}
	}
}

// The claim order is wayfire's own shipped bind, then a user custom, then a
// rebound default: a custom on a shipped chord wins it and the displaced
// default says so instead of firing twice, and a rebind moves the chord the
// legend will show.
func TestResolvePriority(t *testing.T) {
	s := defaultStore()
	s.Keybinds = []Keybind{{Keys: "SUPER + Q", Action: "exec", Value: "notify-send hi"}}
	out, report := resolveBinds(s)
	foundCustom, foundClose := false, false
	for _, o := range out {
		if o.option == "binding_ryoku_0" && o.activator == "<super> KEY_Q" {
			foundCustom = true
		}
		if o.option == "binding_window_close" {
			foundClose = true
		}
	}
	if !foundCustom {
		t.Error("a custom bind must win its chord")
	}
	if foundClose {
		t.Error("the displaced default must not be emitted beside it")
	}
	if !strings.Contains(strings.Join(reportStrings(report), "\n"), "already taken by a custom bind") {
		t.Errorf("the displaced default must be reported: %v", report)
	}

	// A custom onto wayfire's own shipped chord loses to the shipped bind.
	s = defaultStore()
	s.Keybinds = []Keybind{{Keys: "SUPER + CTRL + KP_4", Action: "exec", Value: "x"}}
	_, report = resolveBinds(s)
	if !strings.Contains(strings.Join(reportStrings(report), "\n"), "wayfire's own Tile left") {
		t.Errorf("a chord wayfire's own config holds must stay reported: %v", report)
	}

	// A rebind moves the emitted chord.
	s = defaultStore()
	s.KeybindRebinds = map[string]string{"SUPER + L": "SUPER + SHIFT + P"}
	out, _ = resolveBinds(s)
	found := false
	for _, o := range out {
		if o.option == "binding_shell_lock" {
			found = true
			if o.activator != "<super> <shift> KEY_P" {
				t.Errorf("rebound shell lock emitted %q", o.activator)
			}
		}
	}
	if !found {
		t.Error("the rebound lock row must still be emitted")
	}
}

// Native unbinds are explicitly switched off; command rows disappear. Release
// customs use command's release list, not the regular binding list.
func TestResolveUnbindsAndRelease(t *testing.T) {
	s := defaultStore()
	s.Unbinds = []string{"SUPER + mouse:272", "SUPER + Space"}
	s.Keybinds = []Keybind{{Keys: "SUPER + Z", Action: "exec", Value: "notify-send released", Release: true}}
	out, _ := resolveBinds(s)
	foundRelease := false
	for _, o := range out {
		switch o.option {
		case "activate":
			if o.section == "move" && o.activator != "" {
				t.Errorf("an unbound native option must be written empty, got %q", o.activator)
			}
		case "binding_shell_launcher":
			t.Error("an unbound command row must not be emitted")
		case "release_binding_ryoku_0":
			foundRelease = o.section == "command" && o.activator == "<super> KEY_Z" &&
				o.commandOpt == "command_ryoku_0" && o.cmd == "notify-send released"
		}
	}
	if !foundRelease {
		t.Error("a release custom must emit command's release binding pair")
	}
}

// The exclusives describe the shipped baseline, so a change there has to be a
// deliberate edit here too: every row's chord must be the config's own.
func TestExclusivesMatchBaseline(t *testing.T) {
	raw, err := os.ReadFile("../../wayfire/wayfire.ini")
	if err != nil {
		t.Fatal(err)
	}
	d := parseIni(raw)
	for _, ex := range wayfireExclusives() {
		value, ok := d.get(ex.section, ex.key)
		if !ok {
			t.Errorf("baseline carries no [%s] %s", ex.section, ex.key)
			continue
		}
		activator, ok := toWayfireActivator(ex.chord)
		if !ok {
			t.Errorf("%s: %s has no wayfire spelling", ex.label, ex.chord)
			continue
		}
		for _, tok := range strings.Fields(activator) {
			if !containsField(value, tok) {
				t.Errorf("[%s] %s = %q does not bind %s (missing %s)", ex.section, ex.key, value, ex.chord, tok)
			}
		}
	}
}

// The baseline's Alt-Tab pair uses the v0.11 `activate` and
// `activate_backward` options, so the catalogue points at the real forward key.
func TestFastSwitcherShips(t *testing.T) {
	raw, err := os.ReadFile("../../wayfire/wayfire.ini")
	if err != nil {
		t.Fatal(err)
	}
	d := parseIni(raw)
	plugins, ok := d.get("core", "plugins")
	if !ok || !strings.Contains(plugins, "fast-switcher") {
		t.Errorf("plugins = %q; the Alt-Tab row needs fast-switcher", plugins)
	}
	if v, _ := d.get("fast-switcher", "activate"); v != "<alt> KEY_TAB" {
		t.Errorf("activate = %q", v)
	}
	if defaultBinds()["window.focusPrevious"].reason != "" {
		t.Error("with fast-switcher shipped, window.focusPrevious must be a real option")
	}
}

// Native catalogue rows depend on plugins whose v0.11 options they target.
// Keeping the roster assertion beside the mappings prevents a valid-looking
// option from landing in a section no loaded plugin reads.
func TestNativeBindPluginsShip(t *testing.T) {
	raw, err := os.ReadFile("../../wayfire/wayfire.ini")
	if err != nil {
		t.Fatal(err)
	}
	d := parseIni(raw)
	plugins, _ := d.get("core", "plugins")
	for _, plugin := range []string{"command", "fast-switcher", "grid", "move", "resize", "shortcuts-inhibit", "simple-tile", "vswitch", "wm-actions", "wsets", "zoom"} {
		if !containsField(plugins, plugin) {
			t.Errorf("core.plugins = %q; missing %s for a shipped bind", plugins, plugin)
		}
	}
}

func containsField(value, field string) bool {
	for _, f := range strings.Fields(value) {
		if f == field {
			return true
		}
	}
	return false
}

func reportStrings(report []wm.Unhonored) []string {
	out := make([]string, 0, len(report))
	for _, u := range report {
		out = append(out, u.Key+": "+u.Reason)
	}
	return out
}

// The legend leads with the shipped catalogue, in the order the sheet presents
// it, then wayfire's own shipped binds under wayfire's name. A behaviour
// without a wayfire expression keeps its chord and carries the reason, and is
// not rebindable: recording a chord over something that does not exist would
// move nothing. A media key stays fixed the way every provider fixes it.
func TestBindRowsCatalogueAndExclusives(t *testing.T) {
	rows := bindRows(defaultStore())
	cat := wm.ShippedBinds()
	if len(rows) < len(cat)+len(wayfireExclusives()) {
		t.Fatalf("got %d rows, want catalogue plus exclusives at least", len(rows))
	}
	for i, cb := range cat {
		if rows[i].ID != cb.ID {
			t.Fatalf("row %d = %s, want %s", i, rows[i].ID, cb.ID)
		}
	}

	byID := map[string]wm.BindRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if r := byID["focus.left"]; r.Unhonored != "" || !r.Rebindable {
		t.Errorf("focus.left = unhonored %q, rebindable %v; it binds directionally", r.Unhonored, r.Rebindable)
	}
	if r := byID["window.close"]; r.Unhonored != "" || !r.Rebindable || r.Chord != "SUPER + Q" {
		t.Errorf("window.close = %+v", r)
	}
	if r := byID["window.resize"]; r.Unhonored != "" || r.Label != "Step window width" ||
		!strings.Contains(r.Hint, "full") {
		t.Errorf("Wayfire width preset row = %+v", r)
	}
	if r := byID["window.float"]; r.Unhonored != "" || !r.Rebindable {
		t.Errorf("window.float must expose simple-tile's native attach/detach toggle: %+v", r)
	}
	if r := byID["column.maximize"]; r.Unhonored != "" || !r.Rebindable {
		t.Errorf("column.maximize must cover tiled and floating views: %+v", r)
	}
	if r := byID["workspace.focus"]; r.Chord != "SUPER + {n}" || !r.Rebindable {
		t.Errorf("workspace.focus keeps the ten-member family chord: %+v", r)
	}
	for _, id := range []string{"workspace.focus", "workspace.moveWindow", "workspace.moveWindowSilent",
		"workspace.focus.numpad", "workspace.moveWindow.numpad", "workspace.moveWindowSilent.numpad"} {
		if r := byID[id]; strings.Contains(r.Hint, "nine") || strings.Contains(r.Hint, "no tenth") {
			t.Errorf("%s still claims the tenth workspace is missing: %q", id, r.Hint)
		}
	}
	if r := byID["shell.inhibitShortcuts"]; r.Unhonored != "" ||
		r.Label != "Restore desktop shortcuts" || !strings.Contains(r.Hint, "shortcut grab") {
		t.Errorf("shortcut inhibitor row = %+v", r)
	}
	if r := byID["workspace.hideWindow"]; r.Unhonored != "" {
		t.Errorf("wsets-backed hide window row = %+v", r)
	}
	if r := byID["media.volumeUp"]; r.Rebindable {
		t.Error("a dedicated media key must stay fixed")
	}
	// A rebind reaches the row the Hub will show.
	rebound := defaultStore()
	rebound.KeybindRebinds = map[string]string{"SUPER + L": "SUPER + SHIFT + P"}
	for _, r := range bindRows(rebound) {
		if r.ID == "shell.lock" && r.Chord != "SUPER + SHIFT + P" {
			t.Errorf("rebound shell.lock chord = %q", r.Chord)
		}
	}

	// wayfire's own rows follow the catalogue, fixed, with the second chord of
	// a multi-chord option noted rather than hidden.
	if rows[len(cat)].Category != "Wayfire" {
		t.Errorf("the first wayfire exclusive sits at %d with category %q", len(cat), rows[len(cat)].Category)
	}
	got := byID["wayfire.grid.slot_l"]
	if got.Label != "Tile left" || got.Rebindable || got.Hint != "" {
		t.Errorf("wayfire.grid.slot_l = %+v", got)
	}
	if strings.Contains(byID["wayfire.zoom.modifier"].Hint, "also") {
		t.Errorf("zoom hint = %q, it takes one modifier", byID["wayfire.zoom.modifier"].Hint)
	}
}

// A custom row is honoured only where wayfire can act on it, and the reason it
// shows is the same one apply reports, through the one keybindWhy. An empty
// chord is not a row at all.
func TestBindRowsCustoms(t *testing.T) {
	s := defaultStore()
	s.Keybinds = []Keybind{
		{Keys: "SUPER + T", Action: "exec", Value: "alacritty"},
		{Keys: "SUPER + Q", Action: "close"},
		{Keys: "mouse_up", Action: "exec", Value: "wheel"},
		{Keys: "SUPER + Y", Action: "exec", Value: ""},
		{Keys: "  ", Action: "exec", Value: "ghost"},
	}
	rows := bindRows(s)
	var customs []wm.BindRow
	for _, r := range rows {
		if strings.HasPrefix(r.ID, "custom.") {
			customs = append(customs, r)
		}
	}
	if len(customs) != 4 {
		t.Fatalf("got %d custom rows, want the four with a chord", len(customs))
	}
	switch {
	case customs[0].Label != "Run: alacritty" || customs[0].Unhonored != "":
		t.Errorf("exec row = %+v", customs[0])
	case !strings.Contains(customs[1].Unhonored, `cannot bind the "close" action`):
		t.Errorf("non-exec row unhonored = %q", customs[1].Unhonored)
	case !strings.Contains(customs[2].Unhonored, "cannot bind the chord"):
		t.Errorf("wheel row unhonored = %q", customs[2].Unhonored)
	case !strings.Contains(customs[3].Unhonored, "needs a command"):
		t.Errorf("empty command row unhonored = %q", customs[3].Unhonored)
	}
}
