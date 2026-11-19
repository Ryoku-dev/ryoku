package main

import (
	"strings"
	"testing"
)

func withoutProgram(t *testing.T, name string) {
	t.Helper()
	prev := hasProgram
	hasProgram = func(p string) bool { return p != name }
	t.Cleanup(func() { hasProgram = prev })
}

// With the Rashin app absent its window chord is not emitted, a rebind the
// user recorded for it does not resurrect it, the legend row keeps its place
// and names the program, and the Rashin bar, which needs no program, stays bound.
func TestAbsentProgramDropsItsBind(t *testing.T) {
	withoutProgram(t, "rashin-app")
	s := loadStore("")
	s.KeybindRebinds = map[string]string{"SUPER + ALT + Space": "SUPER + CTRL + SHIFT + F12"}
	kdl, report := genBinds(s)
	for _, chord := range []string{"Super+Alt+Space", "Super+Ctrl+Shift+F12"} {
		if strings.Contains(kdl, "\n    "+chord+" {") {
			t.Fatalf("%s is bound without rashin-app:\n%s", chord, kdl)
		}
	}
	if !strings.Contains(kdl, "\n    Alt+Space {") {
		t.Fatalf("the Rashin bar went missing although it needs no program:\n%s", kdl)
	}
	reported := false
	for _, u := range report {
		if strings.Contains(u.Key, "SUPER + ALT + Space") && u.Reason == "Rashin is not installed." {
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
	kdl, _ := genBinds(loadStore(""))
	if !strings.Contains(kdl, "\n    Alt+Space {") {
		t.Fatalf("the Rashin bar is unbound without rashind:\n%s", kdl)
	}
	if !strings.Contains(kdl, `spawn "ryoku-summon" "--app-id" "dev.ryoku.rashin" "rashin-app";`) {
		t.Fatalf("the Rashin window bind does not use its stable app id:\n%s", kdl)
	}
}
