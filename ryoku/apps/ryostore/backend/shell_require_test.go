package main

import (
	"reflect"
	"testing"
)

func requirementKeys(requirements []shellRequirement) []string {
	keys := []string{}
	for _, requirement := range requirements {
		keys = append(keys, requirement.Selection+"="+requirement.Value)
	}
	return keys
}

func TestShellsRequireListsInstallsAndFallsBack(t *testing.T) {
	f := newShellFixture(t)
	f.addShipped("orpheus", shippedTOML("orpheus"))
	writeShellTestFile(t, f.shipped+"/modules/launcher/catalog.json", `{"version":1,"default":"hero","variants":[]}`, 0o644)
	f.addCatalogueItem("subshells", "python", testShellManifest(), defaultShellFiles(), nil)
	spotlight := testShellManifest()
	spotlight.Provides.Launcher = []ShellLauncherContribution{{ID: "spotlight", Name: "Spotlight", Entry: "qml/Scene.qml"}}
	f.addCatalogueItem("barstyles", "iris", spotlight, defaultShellFiles(), func(entry *ProductEntry) {
		entry.DownloadPaused = true
		entry.DownloadPauseReason = "Being reworked."
	})
	kit := testShellManifest()
	kit.Provides.DesktopWidgets = []ShellDesktopWidgetContribution{{
		ID: "kit", Widget: "qml/Scene.qml", Roster: "qml/roster.json",
		RosterRecords: []map[string]any{{"id": "clock", "prefix": "kitClock", "label": "Clock", "kind": "clock", "sizes": []any{}}},
	}}
	f.addCatalogueItem("barstyles", "kitbar", kit, defaultShellFiles(), func(entry *ProductEntry) {
		entry.DownloadPaused = true
	})
	f.writeConfig("shell.json", map[string]any{"barStyle": "python", "dock": map[string]any{"design": "gone"}})
	f.writeConfig("launcher.json", map[string]any{"variant": "spotlight"})
	f.writeConfig("widgets.json", map[string]any{"kitClockEnabled": true, "clockEnabled": true})

	listed, err := runShellsRequire(false, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []shellRequirement{
		{Selection: "barStyle", Value: "python", Category: "subshells", ID: "python", Available: true},
		{Selection: "dock.design", Value: "gone"},
		{Selection: "launcher.variant", Value: "spotlight", Category: "barstyles", ID: "iris"},
		{Selection: "desktopWidgets", Value: "kit", Category: "barstyles", ID: "kitbar"},
	}
	if !reflect.DeepEqual(listed.Missing, want) {
		t.Fatalf("missing = %#v\nwant %#v", listed.Missing, want)
	}
	if len(listed.Installed) != 0 || len(listed.FellBack) != 0 || productInstalled("subshells", "python") {
		t.Fatal("a plain listing changed something")
	}

	fixed, err := runShellsRequire(true, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := requirementKeys(fixed.Installed); !reflect.DeepEqual(got, []string{"barStyle=python"}) {
		t.Fatalf("installed = %v", got)
	}
	if productInstalled("barstyles", "iris") || productInstalled("barstyles", "kitbar") {
		t.Fatal("a paused item was installed")
	}
	if got := requirementKeys(fixed.FellBack); !reflect.DeepEqual(got, []string{"dock.design=gone", "launcher.variant=spotlight", "desktopWidgets=kit"}) {
		t.Fatalf("fell back = %v", got)
	}
	shell := f.readConfig("shell.json")
	if shell["barStyle"] != "python" || shell["dock"].(map[string]any)["design"] != defaultDockDesign {
		t.Fatalf("shell.json = %v", shell)
	}
	if launcher := f.readConfig("launcher.json"); launcher["variant"] != "hero" {
		t.Fatalf("launcher.json = %v", launcher)
	}
	widgets := f.readConfig("widgets.json")
	if widgets["kitClockEnabled"] != false || widgets["clockEnabled"] != true {
		t.Fatalf("widgets.json = %v", widgets)
	}
	if record := indexItem(t, f.readIndex(), "subshells", "python"); !record.InUse || !record.Prepared {
		t.Fatalf("installed selection is not prepared: %#v", record)
	}

	clean, err := runShellsRequire(false, false)
	if err != nil || len(clean.Missing) != 0 {
		t.Fatalf("after repair missing = %#v, %v", clean.Missing, err)
	}
}

func TestShellsRequireFallsBackBarStyleThatNothingOffers(t *testing.T) {
	f := newShellFixture(t)
	f.addShipped("orpheus", shippedTOML("orpheus"))
	f.writeConfig("shell.json", map[string]any{"barStyle": "vanished"})
	result, err := runShellsRequire(true, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := requirementKeys(result.FellBack); !reflect.DeepEqual(got, []string{"barStyle=vanished"}) {
		t.Fatalf("fell back = %v", got)
	}
	if style := f.readConfig("shell.json")["barStyle"]; style != shellFallbackID {
		t.Fatalf("barStyle = %v", style)
	}
}
