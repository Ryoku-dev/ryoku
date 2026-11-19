package main

import (
	"testing"
)

func withoutProgram(t *testing.T, name string) {
	t.Helper()
	prev := hasProgram
	hasProgram = func(p string) bool { return p != name }
	t.Cleanup(func() { hasProgram = prev })
}

func composedValues(s wayfireStore) map[string]string {
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

// With the Rashin app absent its window bind is left out of the composed
// config, a rebind the user recorded for it does not resurrect it, the legend
// row keeps its place and names the program, and the Rashin bar, which needs
// no program, stays bound.
func TestAbsentProgramDropsItsBind(t *testing.T) {
	withoutProgram(t, "rashin-app")
	s := defaultStore()
	s.KeybindRebinds = map[string]string{"SUPER + ALT + Space": "SUPER + CTRL + SHIFT + F12"}
	got := composedValues(s)
	for _, option := range []string{"command.binding_shell_rashin", "command.command_shell_rashin"} {
		if _, bound := got[option]; bound {
			t.Fatalf("%s = %q is written without rashin-app", option, got[option])
		}
	}
	if got["command.binding_shell_ask"] != "<alt> KEY_SPACE" {
		t.Fatalf("the Rashin bar = %q although it needs no program", got["command.binding_shell_ask"])
	}
	_, report := resolveBinds(s)
	reported := false
	for _, u := range report {
		if u.Key == "desktop.keybinds (default SUPER + ALT + Space)" && u.Reason == "Rashin is not installed." {
			reported = true
		}
	}
	if !reported {
		t.Fatalf("apply did not report the dropped Rashin bind: %+v", report)
	}
	for _, r := range bindRows(s) {
		if r.ID == "shell.rashin" {
			if r.Unhonored != "Rashin is not installed." {
				t.Fatalf("shell.rashin row reason = %q", r.Unhonored)
			}
			return
		}
	}
	t.Fatal("shell.rashin row vanished from the legend")
}

// The Rashin bar opens with Rashin off: Web and Downloads need no daemon.
func TestRashinBarBoundWithoutRashind(t *testing.T) {
	withoutProgram(t, "rashind")
	got := composedValues(defaultStore())
	if got["command.binding_shell_ask"] != "<alt> KEY_SPACE" {
		t.Fatalf("Rashin bar bind = %q without rashind", got["command.binding_shell_ask"])
	}
	if got["command.command_shell_rashin"] != "ryoku-summon --app-id dev.ryoku.rashin rashin-app" {
		t.Fatalf("the Rashin window bind does not use its stable app id: %q", got["command.command_shell_rashin"])
	}
}
