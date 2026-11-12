package main

import "testing"

func TestPresetRename(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	entries := []presetEntry{{GPU: "0000:01:00.0", ID: "power_limit", Value: "60"}}
	if err := saveCustomPresets([]preset{{Name: "Night", Entries: entries}, {Name: "Games"}}); err != nil {
		t.Fatal(err)
	}
	if err := presetRename("Night", "Quiet"); err == nil {
		t.Error("a built-in name must be refused")
	}
	if err := presetRename("Night", "Games"); err == nil {
		t.Error("a name in use must be refused")
	}
	if err := presetRename("Nope", "Other"); err == nil {
		t.Error("a missing preset must be reported")
	}
	if err := presetRename("Night", "Evening"); err != nil {
		t.Fatal(err)
	}
	got := loadCustomPresets()
	if len(got) != 2 || got[0].Name != "Evening" || len(got[0].Entries) != 1 || got[0].Entries[0].Value != "60" {
		t.Fatalf("after rename: %+v", got)
	}
}
