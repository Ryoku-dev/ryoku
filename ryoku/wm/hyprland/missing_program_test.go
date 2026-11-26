package main

import (
	"strings"
	"testing"

	wm "ryoku-wm"
)

func withoutProgram(t *testing.T, name string) {
	t.Helper()
	prev := hasProgram
	hasProgram = func(p string) bool { return p != name }
	t.Cleanup(func() { hasProgram = prev })
}

// No shipped bind needs an optional program. An absent program must not
// switch the search bar off, and the old Hyakki chord is not a shipped row.
func TestSearchBarStaysBoundWithoutOptionalPrograms(t *testing.T) {
	withoutProgram(t, "hyakki-app")
	got := string(renderRebinds(Overrides{}))
	if strings.Contains(got, `["ALT + Space"] = false`) || strings.Contains(got, "hyakki") {
		t.Fatalf("search bar or a Hyakki chord was switched off:\n%s", got)
	}
	var ask bool
	for _, c := range wm.ShippedBinds() {
		if c.ID == "shell.hyakki" {
			t.Fatal("shell.hyakki is still shipped")
		}
		row := catalogRow(c, true, nil, nil)
		if row.Unhonored != "" {
			t.Errorf("%s unhonored = %q", c.ID, row.Unhonored)
		}
		if c.ID == "shell.ask" {
			ask = true
			if row.Label != "Search" {
				t.Errorf("search label = %q", row.Label)
			}
		}
	}
	if !ask {
		t.Fatal("shell.ask vanished")
	}
}

func TestSearchBarStaysOnWithoutOni(t *testing.T) {
	withoutProgram(t, "oni")
	if got := string(renderRebinds(Overrides{})); strings.Contains(got, `["ALT + Space"] = false`) {
		t.Fatalf("search bar switched off without oni:\n%s", got)
	}
}

func TestPresentProgramLeavesTheTableAlone(t *testing.T) {
	if got := string(renderRebinds(Overrides{})); strings.Contains(got, "= false") {
		t.Fatalf("a bind was switched off with every program present:\n%s", got)
	}
}
