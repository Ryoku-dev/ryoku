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

// With the Hyakki app absent its chord is switched off in the K() table, so
// binds.lua never registers it, and a rebind the user recorded for it is
// overruled the same way: the lookup is keyed by the shipped chord. The Hyakki
// bar needs no program and stays registered.
func TestAbsentProgramSwitchesItsBindOff(t *testing.T) {
	withoutProgram(t, "hyakki-app")
	got := string(renderRebinds(Overrides{KeybindRebinds: map[string]string{"SUPER + ALT + Space": "SUPER + A"}}))
	if !strings.Contains(got, `["SUPER + ALT + Space"] = false`) {
		t.Fatalf("Hyakki window chord still registered without hyakki-app:\n%s", got)
	}
	if strings.Contains(got, `"SUPER + A"`) {
		t.Fatalf("a rebind of an absent program's bind leaked into the table:\n%s", got)
	}
	if strings.Contains(got, `["ALT + Space"] = false`) {
		t.Fatalf("the Hyakki bar needs no program, yet it was switched off:\n%s", got)
	}
}

func TestHyakkiBarStaysOnWithoutOni(t *testing.T) {
	withoutProgram(t, "oni")
	if got := string(renderRebinds(Overrides{})); strings.Contains(got, `["ALT + Space"] = false`) {
		t.Fatalf("the Hyakki bar was switched off without oni:\n%s", got)
	}
}

func TestPresentProgramLeavesTheTableAlone(t *testing.T) {
	if got := string(renderRebinds(Overrides{})); strings.Contains(got, "= false") {
		t.Fatalf("a bind was switched off with every program present:\n%s", got)
	}
}

func TestAbsentProgramRowSaysSo(t *testing.T) {
	withoutProgram(t, "hyakki-app")
	var row wm.BindRow
	for _, c := range wm.ShippedBinds() {
		if c.ID == "shell.hyakki" {
			row = catalogRow(c, true, nil, nil)
		}
	}
	if row.Unhonored == "" {
		t.Fatalf("shell.hyakki row reason = %q", row.Unhonored)
	}
	if len(row.Keys) == 0 {
		t.Fatal("the row dropped its chord; it should keep it and say why it is off")
	}
}
