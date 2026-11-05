package doctor

import (
	"encoding/json"
	"testing"
)

func TestAppAliasRepairPlan(t *testing.T) {
	catalog := []appRoleCatalog{{
		Role: "browser",
		Candidates: []appCandidateCatalog{{
			Cmd:       "zen-browser",
			Installed: true,
			Aliases: []appAliasStatus{
				{Cmd: "zen", Installed: false},
				{Cmd: "zen-browser", Installed: true},
			},
		}},
	}}
	updated, changes, err := appAliasRepairPlan([]byte(`{
		"desktop": {"apps": {"browser": "zen", "terminal": "my-terminal"}},
		"other": {"kept": true}
	}`), catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0] != "browser: zen -> zen-browser" {
		t.Fatalf("changes = %#v", changes)
	}
	var root map[string]any
	if err := json.Unmarshal(updated, &root); err != nil {
		t.Fatal(err)
	}
	desktop := root["desktop"].(map[string]any)
	apps := desktop["apps"].(map[string]any)
	if got := apps["browser"]; got != "zen-browser" {
		t.Fatalf("browser = %v, want zen-browser", got)
	}
	if got := apps["terminal"]; got != "my-terminal" {
		t.Fatalf("custom terminal changed to %v", got)
	}
	if kept := root["other"].(map[string]any)["kept"]; kept != true {
		t.Fatalf("unrelated setting changed to %v", kept)
	}
}

func TestAppAliasRepairPlanKeepsInstalledAlias(t *testing.T) {
	catalog := []appRoleCatalog{{
		Role: "browser",
		Candidates: []appCandidateCatalog{{
			Cmd:       "zen",
			Installed: true,
			Aliases: []appAliasStatus{
				{Cmd: "zen", Installed: true},
				{Cmd: "zen-browser", Installed: true},
			},
		}},
	}}
	raw := []byte(`{"desktop":{"apps":{"browser":"zen-browser"}}}`)
	updated, changes, err := appAliasRepairPlan(raw, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("changes = %#v, want none", changes)
	}
	if string(updated) != string(raw) {
		t.Fatalf("unchanged store was rewritten: %s", updated)
	}
}
