package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func installTestShell(t *testing.T, f *shellFixture, category, id string, shell ShellManifest) {
	t.Helper()
	files := defaultShellFiles()
	for _, module := range shell.Modules {
		files = append(files, shellTestFile{path: "qml/" + module + "/qmldir", body: "module " + module + "\n"})
	}
	entry := f.addCatalogueItem(category, id, shell, files, nil)
	if err := installProduct(context.Background(), f.cache(), category, entry); err != nil {
		t.Fatal(err)
	}
}

func TestShippedSubshellsAreDiscoveredFromItemToml(t *testing.T) {
	f := newShellFixture(t)
	f.addShipped("orpheus", shippedTOML("orpheus"))
	f.addShipped("kairos", shippedTOML("kairos")+`
[[provides.launcher]]
id = "kairos"
name = "Kairos"
description = "Clock island search."
entry = "qml/launcher/Main.qml"
preview = "qml/launcher/Preview.qml"
`)
	f.addShipped("broken", shippedTOML("broken")+"\nsurprise = true\n")
	f.addShipped("quiet", strings.Replace(shippedTOML("quiet"), "official = true", "official = true\nlisted = false", 1))

	items, problems, err := discoverShippedShells()
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if !reflect.DeepEqual(ids, []string{"kairos", "orpheus", "quiet"}) {
		t.Fatalf("shipped ids = %v", ids)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "broken") {
		t.Fatalf("problems = %v, want the broken item.toml reported", problems)
	}
	kairos := items[0]
	if kairos.Source != "shipped" || kairos.Dir != "subshells/kairos" || kairos.Shell.Settings.IPC != "kairos" {
		t.Fatalf("kairos = %#v", kairos)
	}
	if len(kairos.Shell.Provides.Launcher) != 1 || kairos.Shell.Provides.Launcher[0].Preview != "qml/launcher/Preview.qml" {
		t.Fatalf("kairos launcher = %#v", kairos.Shell.Provides.Launcher)
	}
}

func TestShellsSyncUnpacksWhatIsInUseAndPrunesOnlyOnRequest(t *testing.T) {
	f := newShellFixture(t)
	f.addShipped("orpheus", shippedTOML("orpheus"))
	demo := testShellManifest()
	demo.Owns = []string{"windowClose", "osd"}
	demo.OwnsSetting = map[string]string{"osd": "demo.ownOsd"}
	demo.Modules = []string{"demokit"}
	installTestShell(t, f, "barstyles", "demo", demo)
	installTestShell(t, f, "barstyles", "idle", testShellManifest())
	f.writeConfig("shell.json", map[string]any{"barStyle": "demo"})

	result, err := runShellsSync(false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.InUse, []string{"barstyles/demo"}) {
		t.Fatalf("in use = %v", result.InUse)
	}
	receipt, _ := readReceipt("barstyles", "demo")
	unpacked := shellUnpackedDir("barstyles", "demo", shellReceiptDigest(receipt))
	if raw, err := os.ReadFile(filepath.Join(unpacked, "qml", "Scene.qml")); err != nil || !strings.Contains(string(raw), "Item") {
		t.Fatalf("unpacked scene = %q, %v", raw, err)
	}
	link := filepath.Join(shellModuleRoot(), "demokit")
	if target, err := os.Readlink(link); err != nil || target != filepath.Join(unpacked, "qml", "demokit") {
		t.Fatalf("module link = %q, %v", target, err)
	}

	index := f.readIndex()
	if index.Active != "demo" || index.Fallback != "orpheus" || index.ModuleRoot != shellModuleRoot() {
		t.Fatalf("index head = %#v", index)
	}
	if !reflect.DeepEqual(index.ActiveOwns, []string{"windowClose", "osd"}) {
		t.Fatalf("activeOwns = %v", index.ActiveOwns)
	}
	shipped := indexItem(t, index, "subshells", "orpheus")
	if shipped.Dir != "subshells/orpheus" || shipped.Entry != "subshells/orpheus/qml/Scene.qml" || !shipped.Prepared || shipped.InUse {
		t.Fatalf("shipped record = %#v", shipped)
	}
	if shipped.Settings.IPC != "orpheus" || shipped.Settings.Args == nil || shipped.OwnsSetting == nil {
		t.Fatalf("shipped settings = %#v", shipped)
	}
	active := indexItem(t, index, "barstyles", "demo")
	if !active.InUse || !active.Prepared || active.Dir != unpacked || active.Entry != filepath.Join(unpacked, "qml", "Scene.qml") {
		t.Fatalf("active record = %#v", active)
	}
	if active.OwnsSetting["osd"] != "demo.ownOsd" {
		t.Fatalf("ownsSetting = %v", active.OwnsSetting)
	}
	packed := indexItem(t, index, "barstyles", "idle")
	if packed.InUse || packed.Prepared || packed.Dir != "" || packed.Entry != "" {
		t.Fatalf("packed record = %#v", packed)
	}

	if again, err := runShellsSync(false); err != nil || again.Changed {
		t.Fatalf("idle sync changed = %v, %v", again.Changed, err)
	}

	f.writeConfig("shell.json", map[string]any{"barStyle": "orpheus"})
	if _, err := runShellsSync(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unpacked); err != nil {
		t.Fatalf("sync without prune deleted the unpacked item: %v", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("sync without prune deleted the module link: %v", err)
	}
	index = f.readIndex()
	if len(index.ActiveOwns) != 0 {
		t.Fatalf("activeOwns after switch = %v", index.ActiveOwns)
	}
	if stale := indexItem(t, index, "barstyles", "demo"); stale.InUse || !stale.Prepared {
		t.Fatalf("still unpacked item = %#v", stale)
	}

	pruned, err := runShellsSync(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned.Pruned) != 2 {
		t.Fatalf("pruned = %v, want the unpacked folder and the link", pruned.Pruned)
	}
	if _, err := os.Stat(unpacked); !os.IsNotExist(err) {
		t.Fatalf("prune kept the unpacked item: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("prune kept the module link: %v", err)
	}
	if record := indexItem(t, f.readIndex(), "barstyles", "demo"); record.Prepared || record.Entry != "" {
		t.Fatalf("pruned record = %#v", record)
	}
}

func TestShellsSyncCountsContributionsAsInUse(t *testing.T) {
	f := newShellFixture(t)
	f.addShipped("orpheus", shippedTOML("orpheus"))
	docks := testShellManifest()
	docks.Provides.Dock = []ShellDockContribution{{ID: "shima", Name: "Shima", Entry: "qml/Scene.qml"}}
	installTestShell(t, f, "barstyles", "docky", docks)
	widgets := testShellManifest()
	widgets.Provides.DesktopWidgets = []ShellDesktopWidgetContribution{{
		ID: "kit", Widget: "qml/Scene.qml", Roster: "qml/roster.json",
		RosterRecords: []map[string]any{{"id": "clock", "prefix": "kitClock", "label": "Clock", "kind": "clock", "sizes": []any{}}},
	}}
	installTestShell(t, f, "subshells", "widgety", widgets)
	f.writeConfig("shell.json", map[string]any{"barStyle": "orpheus", "dock": map[string]any{"design": "shima"}})
	f.writeConfig("widgets.json", map[string]any{"outputs": map[string]any{"DP-1": map[string]any{"kitClockEnabled": true}}})

	result, err := runShellsSync(false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"barstyles/docky", "subshells/orpheus", "subshells/widgety"}
	if !reflect.DeepEqual(result.InUse, want) {
		t.Fatalf("in use = %v, want %v", result.InUse, want)
	}
	record := indexItem(t, f.readIndex(), "subshells", "widgety")
	if len(record.Provides.DesktopWidgets) != 1 || len(record.Provides.DesktopWidgets[0].Roster) != 1 ||
		!strings.HasSuffix(record.Provides.DesktopWidgets[0].Widget, "/qml/Scene.qml") {
		t.Fatalf("widget contribution = %#v", record.Provides.DesktopWidgets)
	}
}
