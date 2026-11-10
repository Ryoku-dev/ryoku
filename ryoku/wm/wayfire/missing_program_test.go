package main

import (
	"os"
	"testing"
)

// Every test in this package renders against a box that has every optional
// program, so a runner without Rashin cannot change what the fixtures expect.
func TestMain(m *testing.M) {
	hasProgram = func(string) bool { return true }
	os.Exit(m.Run())
}

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

// With Rashin absent the Ask command bind is left out of the composed config,
// a rebind the user recorded for it does not resurrect it, the legend row keeps
// its place and names the program, and the window bind, whose program is
// present, stays bound.
func TestAbsentProgramDropsItsBind(t *testing.T) {
	withoutProgram(t, "rashind")
	s := defaultStore()
	s.KeybindRebinds = map[string]string{"ALT + Space": "SUPER + CTRL + SHIFT + F12"}
	got := composedValues(s)
	for _, option := range []string{"command.binding_shell_ask", "command.command_shell_ask"} {
		if _, bound := got[option]; bound {
			t.Fatalf("%s = %q is written without rashind", option, got[option])
		}
	}
	if got["command.binding_shell_rashin"] != "<super> <alt> KEY_SPACE" {
		t.Fatalf("the Rashin window bind = %q although rashin-app is present", got["command.binding_shell_rashin"])
	}
	if got["command.command_shell_rashin"] != "ryoku-summon --app-id dev.ryoku.rashin rashin-app" {
		t.Fatalf("the Rashin window bind does not use its stable app id: %q", got["command.command_shell_rashin"])
	}
	_, report := resolveBinds(s)
	reported := false
	for _, u := range report {
		if u.Key == "desktop.keybinds (default ALT + Space)" && u.Reason == "rashind is not installed." {
			reported = true
		}
	}
	if !reported {
		t.Fatalf("apply did not report the dropped Ask bind: %+v", report)
	}
	for _, r := range bindRows(s) {
		if r.ID == "shell.ask" {
			if r.Unhonored != "rashind is not installed." {
				t.Fatalf("shell.ask row reason = %q", r.Unhonored)
			}
			return
		}
	}
	t.Fatal("shell.ask row vanished from the legend")
}

func TestPresentProgramKeepsAsk(t *testing.T) {
	got := composedValues(defaultStore())
	if got["command.binding_shell_ask"] != "<alt> KEY_SPACE" {
		t.Fatalf("Ask bind = %q with rashind present", got["command.binding_shell_ask"])
	}
}
