package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// shellFixture is an isolated home with a local revamped catalogue and a
// shipped shell root, so shell items install and sync without the network.
type shellFixture struct {
	t         *testing.T
	home      string
	catalogue string
	shipped   string
	entries   map[string][]ProductEntry
}

type shellTestFile struct {
	path string
	body string
	exec bool
}

func newShellFixture(t *testing.T) *shellFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	fixture := &shellFixture{
		t: t, home: home,
		catalogue: filepath.Join(home, "catalogue"),
		shipped:   filepath.Join(home, "shell"),
		entries:   map[string][]ProductEntry{},
	}
	t.Setenv("RYOSTORE_SHIPPED_ROOT", fixture.shipped)
	t.Setenv("RYOSTORE_REVAMPED_BASE", fixture.catalogue)
	stubPackageManager(t, "pacman")
	fixture.writeRegistry("subshells")
	fixture.writeRegistry("barstyles")
	return fixture
}

func (f *shellFixture) cache() *Cache {
	return newRevampedCache()
}

func testShellManifest() ShellManifest {
	return ShellManifest{
		Entry: "qml/Scene.qml", FrameHost: "each", Owns: []string{}, Modules: []string{},
		Settings: ShellSettings{Args: []string{}},
		Provides: ShellProvides{
			Dock: []ShellDockContribution{}, Launcher: []ShellLauncherContribution{},
			DesktopWidgets: []ShellDesktopWidgetContribution{},
		},
	}
}

func defaultShellFiles() []shellTestFile {
	return []shellTestFile{
		{path: "LICENSE", body: "license\n"},
		{path: "qml/Scene.qml", body: "import QtQuick\nItem {}\n"},
	}
}

// addCatalogueItem publishes one item to the local catalogue and returns its
// registry entry. edit may change the entry before the registry is written.
func (f *shellFixture) addCatalogueItem(category, id string, shell ShellManifest, files []shellTestFile, edit func(*ProductEntry)) ProductEntry {
	f.t.Helper()
	root := filepath.Join(f.catalogue, category, id)
	productFiles := make([]ProductFile, 0, len(files))
	for _, file := range files {
		mode := os.FileMode(0o644)
		modeText := "0644"
		if file.exec {
			mode, modeText = 0o755, "0755"
		}
		writeShellTestFile(f.t, filepath.Join(root, filepath.FromSlash(file.path)), file.body, mode)
		productFiles = append(productFiles, ProductFile{
			Source: file.path, Destination: file.path, SHA256: testSHA([]byte(file.body)),
			Mode: modeText, Size: int64(len(file.body)), Install: true,
		})
	}
	sort.Slice(productFiles, func(i, j int) bool { return productFiles[i].Source < productFiles[j].Source })
	manifest := ProductManifest{
		Schema: 1, ID: id, Category: category, Version: "1.0.0",
		Destination: "ryoku/" + category + "/" + id, Files: productFiles, Shell: &shell,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		f.t.Fatal(err)
	}
	writeShellTestFile(f.t, filepath.Join(root, "manifest.json"), string(raw), 0o644)
	entry := ProductEntry{
		ID: id, Name: "Item " + id, Version: "1.0.0", Path: category + "/" + id,
		Author: "Ryoku Team", Summary: "summary of " + id, Description: "description of " + id,
		Tags: []string{}, Screenshots: []string{}, Accent: "#7aa2f7", Surface: "#101010",
		Preview: "preview.webp", Manifest: "manifest.json", ManifestSHA256: testSHA(raw),
		Upstream: "https://example.invalid/" + id,
	}
	if summary := shellProvidesSummary(shell.Provides); len(summary.Dock)+len(summary.Launcher)+len(summary.DesktopWidgets) > 0 {
		entry.Provides = &summary
	}
	if edit != nil {
		edit(&entry)
	}
	kept := []ProductEntry{}
	for _, existing := range f.entries[category] {
		if existing.ID != id {
			kept = append(kept, existing)
		}
	}
	f.entries[category] = append(kept, entry)
	f.writeRegistry(category)
	return entry
}

func (f *shellFixture) dropCatalogueItem(category, id string) {
	kept := []ProductEntry{}
	for _, existing := range f.entries[category] {
		if existing.ID != id {
			kept = append(kept, existing)
		}
	}
	f.entries[category] = kept
	f.writeRegistry(category)
}

func (f *shellFixture) writeRegistry(category string) {
	f.t.Helper()
	entries := f.entries[category]
	if entries == nil {
		entries = []ProductEntry{}
	}
	raw, err := json.Marshal(map[string]any{"schema": 1, category: entries})
	if err != nil {
		f.t.Fatal(err)
	}
	writeShellTestFile(f.t, filepath.Join(f.catalogue, category, "registry.json"), string(raw), 0o644)
}

// addShipped writes one shipped subshell under the shell config root.
func (f *shellFixture) addShipped(id, itemTOML string) {
	f.t.Helper()
	root := filepath.Join(f.shipped, "subshells", id)
	writeShellTestFile(f.t, filepath.Join(root, "item.toml"), itemTOML, 0o644)
	writeShellTestFile(f.t, filepath.Join(root, "qml", "Scene.qml"), "import QtQuick\nItem {}\n", 0o644)
}

func shippedTOML(id string) string {
	return fmt.Sprintf(`id = %q
name = %q
version = "1.0.0"
summary = "shipped %s"
description = "shipped subshell %s"
license = "GPL-3.0-or-later"
authors = ["Ryoku Team"]
tags = ["shell"]
official = true

[shell]
entry = "qml/Scene.qml"
frameHost = "each"
owns = []
modules = []

[shell.settings]
ipc = %q
call = "settings"
args = []
hubEditor = ""
`, id, id, id, id, id)
}

func (f *shellFixture) writeConfig(name string, state map[string]any) {
	f.t.Helper()
	writeTestConfig(f.t, filepath.Join(configHome(), "ryoku", name), state)
}

func (f *shellFixture) readConfig(name string) map[string]any {
	f.t.Helper()
	state, err := readConfigState(filepath.Join(configHome(), "ryoku", name))
	if err != nil {
		f.t.Fatal(err)
	}
	return state
}

func (f *shellFixture) readIndex() shellsIndex {
	f.t.Helper()
	raw, err := os.ReadFile(shellsIndexPath())
	if err != nil {
		f.t.Fatal(err)
	}
	var index shellsIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		f.t.Fatal(err)
	}
	return index
}

func indexItem(t *testing.T, index shellsIndex, category, id string) shellIndexItem {
	t.Helper()
	for _, item := range index.Items {
		if item.Category == category && item.ID == id {
			return item
		}
	}
	t.Fatalf("shells.json has no %s/%s: %#v", category, id, index.Items)
	return shellIndexItem{}
}

func writeShellTestFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func testSHA(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// writeRawBundle writes an arbitrary tar.gz, for bundles the store would
// never produce, and returns its receipt row.
func writeRawBundle(t *testing.T, path string, headers []*tar.Header, bodies []string) ReceiptFile {
	t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gz)
	for index, header := range headers {
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if bodies[index] != "" {
			if _, err := archive.Write([]byte(bodies[index])); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	writeShellTestFile(t, path, buffer.String(), 0o644)
	return ReceiptFile{
		Source: shellBundleName, Destination: shellBundleName,
		SHA256: testSHA(buffer.Bytes()), Mode: "0644", Size: int64(buffer.Len()),
	}
}
