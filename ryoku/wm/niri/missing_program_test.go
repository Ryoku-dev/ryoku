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

// No shipped bind needs an optional program. The search bar stays bound, and
// the old Hyakki window chord is not emitted.
func TestSearchBarStaysBoundWithoutOptionalPrograms(t *testing.T) {
	withoutProgram(t, "hyakki-app")
	s := loadStore("")
	kdl, _ := genBinds(s)
	if !strings.Contains(kdl, "\n    Alt+Space {") {
		t.Fatalf("search bar went missing:\n%s", kdl)
	}
	if strings.Contains(kdl, "hyakki") || strings.Contains(kdl, "dev.ryoku.Hyakki") {
		t.Fatalf("Hyakki chord still emitted:\n%s", kdl)
	}
	for _, r := range bindRows(s) {
		if r.ID == "shell.hyakki" {
			t.Fatal("shell.hyakki row still in the legend")
		}
		if r.ID == "shell.ask" && r.Label != "Search" {
			t.Fatalf("search label = %q", r.Label)
		}
	}
}

func TestSearchBarBoundWithoutOni(t *testing.T) {
	withoutProgram(t, "oni")
	kdl, _ := genBinds(loadStore(""))
	if !strings.Contains(kdl, "\n    Alt+Space {") {
		t.Fatalf("search bar unbound without oni:\n%s", kdl)
	}
	if strings.Contains(kdl, "hyakki-app") {
		t.Fatalf("Hyakki window bind still emitted:\n%s", kdl)
	}
}
