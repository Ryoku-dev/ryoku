package main

import (
	"os"
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

// Wayfire names the media keys by their linux input codes, not a derived XF86
// spelling, and the mouse button is a button: a wrong spelling would parse
// silently and never fire. A wheel step and an unknown XF86 keysym stay
// unmatched rather than guessed.
func TestActivatorSpellings(t *testing.T) {
	for chord, want := range map[string]string{
		"XF86AudioRaiseVolume": "KEY_VOLUMEUP",
		"XF86MonBrightnessUp":  "KEY_BRIGHTNESSUP",
		"XF86TouchpadToggle":   "KEY_TOUCHPADTOGGLE",
		"SUPER + Q":            "<super> KEY_Q",
		"SUPER + mouse:272":    "<super> BTN_LEFT",
		"SUPER + CTRL + Left":  "<super> <ctrl> KEY_LEFT",
		"SUPER + KP_1":         "<super> KEY_KP1",
		"SUPER + KP_End":       "<super> KEY_KP1",
		"SHIFT + Print":        "<shift> KEY_SYSRQ",
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

// The resolved emission is the session's truth: each catalogue row lands on its
// option with the shipped chord, families expand to their nine workspace
// members, the tenth is reported, and no activator is claimed twice.
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
		"command.binding_shell_launcher":             {activator: "<super> KEY_SPACE", commandOpt: "command_shell_launcher", cmd: "ryoku-shell launcher"},
		"command.binding_window_close":               {activator: "<super> KEY_Q", extra: "<alt> KEY_F4", commandOpt: "command_window_close"},
		"command.repeatable_binding_media_volume_up": {activator: "KEY_VOLUMEUP", commandOpt: "command_media_volume_up", cmd: "ryoku-volume up"},
		"vswitch.binding_1":                          {activator: "<super> KEY_1 | <super> KEY_KP1"},
		"vswitch.with_win_5":                         {activator: "<super> <alt> KEY_5 | <super> <alt> KEY_KP5"},
		"vswitch.send_win_9":                         {activator: "<super> <shift> KEY_9 | <super> <shift> KEY_KP9"},
		"wm-actions.toggle_fullscreen":               {activator: "<super> KEY_F"},
		"fast-switcher.activate_forward":             {activator: "<alt> KEY_TAB"},
		"move.activate":                              {activator: "<super> BTN_LEFT"},
		"resize.activate":                            {activator: "<super> BTN_RIGHT"},
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
	}
	if opts["vswitch.binding_10"].activator != "" {
		t.Error("wayfire's grid holds nine workspaces; binding_10 must not exist")
	}

	var reasons []string
	for _, u := range report {
		reasons = append(reasons, u.Key+": "+u.Reason)
	}
	joined := strings.Join(reasons, "\n")
	if !strings.Contains(joined, "(default SUPER + T)") || !strings.Contains(joined, "tabs") {
		t.Errorf("an impossible behaviour must be reported:\n%s", joined)
	}
	if !strings.Contains(joined, "SUPER + 0") || !strings.Contains(joined, "no tenth") {
		t.Errorf("the tenth workspace member must be reported:\n%s", joined)
	}
	for _, u := range report {
		if strings.Contains(u.Reason, "already taken") {
			t.Errorf("a clean default store must not lose a claim: %v", u)
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

// An unbound native option is written empty so the baseline layer cannot keep
// it alive; an unbound command row simply never lands.
func TestResolveUnbinds(t *testing.T) {
	s := defaultStore()
	s.Unbinds = []string{"SUPER + mouse:272", "SUPER + Space"}
	out, _ := resolveBinds(s)
	for _, o := range out {
		switch o.option {
		case "activate":
			if o.section == "move" && o.activator != "" {
				t.Errorf("an unbound native option must be written empty, got %q", o.activator)
			}
		case "binding_shell_launcher":
			t.Error("an unbound command row must not be emitted")
		}
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

// The baseline's Alt-Tab pair ships, so the catalogue's window.focusPrevious
// row is not a reason but the option itself.
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
	if v, _ := d.get("fast-switcher", "activate_forward"); v != "<alt> KEY_TAB" {
		t.Errorf("activate_forward = %q", v)
	}
	if defaultBinds()["window.focusPrevious"].reason != "" {
		t.Error("with fast-switcher shipped, window.focusPrevious must be a real option")
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
	if r := byID["workspace.focus"]; r.Chord != "SUPER + {n}" || !r.Rebindable {
		t.Errorf("workspace.focus keeps the family chord: %+v", r)
	}
	// The row renders a ten-key range, so the hint has to say the emission
	// stops at nine, the same story apply's report tells.
	for _, id := range []string{"workspace.focus", "workspace.moveWindow", "workspace.moveWindowSilent",
		"workspace.focus.numpad", "workspace.moveWindow.numpad", "workspace.moveWindowSilent.numpad"} {
		if r := byID[id]; !strings.Contains(r.Hint, "no tenth") && !strings.Contains(r.Hint, "nine") {
			t.Errorf("%s hint = %q; the tenth member has nowhere to go", id, r.Hint)
		}
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
