package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestShellCatalogMergesShippedAndCatalogueItems(t *testing.T) {
	f := newShellFixture(t)
	f.addShipped("orpheus", shippedTOML("orpheus"))
	python := testShellManifest()
	python.Settings = ShellSettings{IPC: "python", Call: "settings", Args: []string{"general"}}
	python.Provides.Dock = []ShellDockContribution{{ID: "python", Name: "Python", Entry: "qml/Scene.qml"}}
	f.addCatalogueItem("subshells", "python", python, defaultShellFiles(), nil)
	f.addCatalogueItem("barstyles", "sumi", testShellManifest(), defaultShellFiles(), nil)
	f.writeConfig("shell.json", map[string]any{"barStyle": "orpheus"})

	items, state, err := newShellItemProvider("subshells", nil).Load(context.Background(), false)
	if err != nil || state.Offline {
		t.Fatalf("Load = %v, offline %v", err, state.Offline)
	}
	byID := map[string]Item{}
	for _, item := range items {
		byID[item.ID] = item
	}
	if len(items) != 2 {
		t.Fatalf("subshells = %#v, want orpheus and python only", items)
	}
	orpheus := byID["orpheus"]
	if !orpheus.Installed || !orpheus.Active || orpheus.Metadata["core"] != true || orpheus.Settings == nil || orpheus.Settings.IPC != "orpheus" {
		t.Fatalf("shipped item = %#v", orpheus)
	}
	listed := byID["python"]
	if listed.Installed || listed.Active || listed.Provides == nil || len(listed.Provides.Dock) != 1 || listed.Settings == nil || listed.Settings.Args[0] != "general" {
		t.Fatalf("catalogue item = %#v", listed)
	}
	bars, _, err := newShellItemProvider("barstyles", nil).Load(context.Background(), false)
	if err != nil || len(bars) != 1 || bars[0].ID != "sumi" {
		t.Fatalf("barstyles = %#v, %v", bars, err)
	}
}

func TestShellRemoveDeletesEverythingAndFallsBackToOrpheus(t *testing.T) {
	f := newShellFixture(t)
	f.addShipped("orpheus", shippedTOML("orpheus"))
	shell := testShellManifest()
	shell.Modules = []string{"pykit"}
	files := append(defaultShellFiles(), shellTestFile{path: "qml/pykit/qmldir", body: "module pykit\n"})
	f.addCatalogueItem("subshells", "python", shell, files, nil)
	provider := newShellItemProvider("subshells", nil)
	if err := provider.Install(context.Background(), "python"); err != nil {
		t.Fatal(err)
	}
	f.writeConfig("shell.json", map[string]any{"barStyle": "python"})
	if _, err := runShellsSync(false); err != nil {
		t.Fatal(err)
	}
	if record := indexItem(t, f.readIndex(), "subshells", "python"); !record.Prepared {
		t.Fatalf("python not unpacked before removal: %#v", record)
	}

	if err := provider.Remove(context.Background(), "orpheus"); err == nil {
		t.Fatal("a shipped subshell was removable")
	}
	if err := provider.Remove(context.Background(), "python"); err != nil {
		t.Fatal(err)
	}
	destination, _, _ := productDestination("subshells", "python")
	for _, gone := range []string{
		filepath.Join(destination, shellBundleName),
		filepath.Join(shellUnpackedRoot(), "subshells", "python"),
		filepath.Join(shellModuleRoot(), "pykit"),
		receiptPath("subshells", "python"),
	} {
		if _, err := os.Lstat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s survived removal: %v", gone, err)
		}
	}
	if style := f.readConfig("shell.json")["barStyle"]; style != shellFallbackID {
		t.Fatalf("barStyle after removal = %v", style)
	}
	index := f.readIndex()
	for _, item := range index.Items {
		if item.ID == "python" {
			t.Fatalf("removed item still indexed: %#v", item)
		}
	}
	if index.Active != shellFallbackID {
		t.Fatalf("index active = %q", index.Active)
	}
}
