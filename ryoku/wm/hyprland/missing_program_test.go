package main

import (
	"os"
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

// With the Rashin app absent its chord is switched off in the K() table, so
// binds.lua never registers it, and a rebind the user recorded for it is
// overruled the same way: the lookup is keyed by the shipped chord. The Rashin
// bar needs no program and stays registered.
func TestAbsentProgramSwitchesItsBindOff(t *testing.T) {
	withoutProgram(t, "rashin-app")
	got := string(renderRebinds(Overrides{KeybindRebinds: map[string]string{"SUPER + ALT + Space": "SUPER + A"}}))
	if !strings.Contains(got, `["SUPER + ALT + Space"] = false`) {
		t.Fatalf("Rashin window chord still registered without rashin-app:\n%s", got)
	}
	if strings.Contains(got, `"SUPER + A"`) {
		t.Fatalf("a rebind of an absent program's bind leaked into the table:\n%s", got)
	}
	if strings.Contains(got, `["ALT + Space"] = false`) {
		t.Fatalf("the Rashin bar needs no program, yet it was switched off:\n%s", got)
	}
}

func TestRashinBarStaysOnWithoutRashind(t *testing.T) {
	withoutProgram(t, "rashind")
	if got := string(renderRebinds(Overrides{})); strings.Contains(got, `["ALT + Space"] = false`) {
		t.Fatalf("the Rashin bar was switched off without rashind:\n%s", got)
	}
}

func TestPresentProgramLeavesTheTableAlone(t *testing.T) {
	if got := string(renderRebinds(Overrides{})); strings.Contains(got, "= false") {
		t.Fatalf("a bind was switched off with every program present:\n%s", got)
	}
}

func TestAbsentProgramRowSaysSo(t *testing.T) {
	withoutProgram(t, "rashin-app")
	var row wm.BindRow
	for _, c := range wm.ShippedBinds() {
		if c.ID == "shell.rashin" {
			row = catalogRow(c, true, nil, nil)
		}
	}
	if row.Unhonored != "Rashin is not installed." {
		t.Fatalf("shell.rashin row reason = %q", row.Unhonored)
	}
	if len(row.Keys) == 0 {
		t.Fatal("the row dropped its chord; it should keep it and say why it is off")
	}
}

func TestRashinBindAndRuleUseStableAppID(t *testing.T) {
	binds, err := os.ReadFile(repoModules() + "/binds.lua")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(binds), "ryoku-summon --app-id dev.ryoku.rashin rashin-app") {
		t.Fatalf("Rashin bind does not use the stable app id:\n%s", binds)
	}
	rules, err := os.ReadFile(repoModules() + "/window_rules.lua")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rules), `class = "^(dev\\.ryoku\\.rashin)$"`) {
		t.Fatalf("Rashin rule does not use the stable app id:\n%s", rules)
	}
}
