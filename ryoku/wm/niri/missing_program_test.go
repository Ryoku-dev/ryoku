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

// With the Hyakki app absent its window chord is not emitted, a rebind the
// user recorded for it does not resurrect it, the legend row keeps its place
// and names the program, and the Hyakki bar, which needs no program, stays bound.
func TestAbsentProgramDropsItsBind(t *testing.T) {
	withoutProgram(t, "hyakki-app")
	s := loadStore("")
	s.KeybindRebinds = map[string]string{"SUPER + ALT + Space": "SUPER + CTRL + SHIFT + F12"}
	kdl, report := genBinds(s)
	for _, chord := range []string{"Super+Alt+Space", "Super+Ctrl+Shift+F12"} {
		if strings.Contains(kdl, "\n    "+chord+" {") {
			t.Fatalf("%s is bound without hyakki-app:\n%s", chord, kdl)
		}
	}
	if !strings.Contains(kdl, "\n    Alt+Space {") {
		t.Fatalf("the Hyakki bar went missing although it needs no program:\n%s", kdl)
	}
	reported := false
	for _, u := range report {
		if strings.Contains(u.Key, "SUPER + ALT + Space") && u.Reason != "" {
			reported = true
		}
	}
	if !reported {
		t.Fatalf("apply did not report the dropped Hyakki bind: %+v", report)
	}
	for _, r := range bindRows(s) {
		if r.ID == "shell.hyakki" {
			if r.Unhonored == "" {
				t.Fatalf("shell.hyakki row reason = %q", r.Unhonored)
			}
			return
		}
	}
	t.Fatal("shell.hyakki row vanished from the legend")
}

// The Hyakki bar opens with Hyakki off: Web and Downloads need no daemon.
func TestHyakkiBarBoundWithoutOni(t *testing.T) {
	withoutProgram(t, "oni")
	kdl, _ := genBinds(loadStore(""))
	if !strings.Contains(kdl, "\n    Alt+Space {") {
		t.Fatalf("the Hyakki bar is unbound without oni:\n%s", kdl)
	}
	if !strings.Contains(kdl, `spawn "ryoku-summon" "--app-id" "dev.ryoku.Hyakki" "hyakki-app";`) {
		t.Fatalf("the Hyakki window bind does not use its stable app id:\n%s", kdl)
	}
}
