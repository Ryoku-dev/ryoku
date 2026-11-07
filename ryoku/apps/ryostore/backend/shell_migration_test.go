package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// writeLegacyInstall lays out a pre-bundle install: loose files in the old
// store's layout under barstyles/<id>/ plus the receipt that owned them.
func writeLegacyInstall(t *testing.T, id string) string {
	t.Helper()
	destination, relative, err := productDestination("barstyles", id)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"Scene.qml": "import QtQuick\nItem {}\n", "LICENSE": "license\n"}
	receipt := Receipt{Category: "barstyles", ID: id, Version: "0.9.0", Destination: relative}
	for name, body := range files {
		writeShellTestFile(t, filepath.Join(destination, name), body, 0o644)
		receipt.Files = append(receipt.Files, ReceiptFile{
			Source: name, Destination: name, SHA256: testSHA([]byte(body)), Mode: "0644", Size: int64(len(body)),
		})
	}
	if err := writeReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	return destination
}

func writeLegacyViews(t *testing.T) {
	t.Helper()
	writeShellTestFile(t, filepath.Join(storeStateDir(), "barstyle-views", "x", "Scene.qml"), "x", 0o644)
	writeShellTestFile(t, filepath.Join(storeStateDir(), "barstyles.json"), "{}", 0o644)
}

func assertLegacyViewsGone(t *testing.T) {
	t.Helper()
	for _, path := range []string{filepath.Join(storeStateDir(), "barstyle-views"), filepath.Join(storeStateDir(), "barstyles.json")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("%s survived the migration: %v", path, err)
		}
	}
}

func TestLegacyLooseInstallIsReinstalledAsPackedSubshell(t *testing.T) {
	f := newShellFixture(t)
	// A paused item still migrates: the user already has it.
	f.addCatalogueItem("subshells", "caelestia", testShellManifest(), defaultShellFiles(), func(entry *ProductEntry) {
		entry.DownloadPaused = true
	})
	old := writeLegacyInstall(t, "caelestia")
	writeLegacyViews(t)
	f.writeConfig("shell.json", map[string]any{"barStyle": "caelestia"})

	if err := migrateLegacyShellProducts(context.Background(), f.cache()); err != nil {
		t.Fatal(err)
	}
	receipt, err := readReceipt("subshells", "caelestia")
	if err != nil || receipt.Shell == nil || receipt.Version != "1.0.0" {
		t.Fatalf("subshell receipt = %#v, %v", receipt, err)
	}
	destination, _, _ := productDestination("subshells", "caelestia")
	if err := verifyShellBundle(filepath.Join(destination, shellBundleName), receipt, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(old); !os.IsNotExist(err) {
		t.Fatalf("loose install survived: %v", err)
	}
	if _, err := os.Lstat(receiptPath("barstyles", "caelestia")); !os.IsNotExist(err) {
		t.Fatalf("legacy receipt survived: %v", err)
	}
	if style := f.readConfig("shell.json")["barStyle"]; style != "caelestia" {
		t.Fatalf("barStyle = %v, want the migrated id kept", style)
	}
	assertLegacyViewsGone(t)
}

func TestLegacyLooseBarStyleIsPackedInPlace(t *testing.T) {
	f := newShellFixture(t)
	f.addCatalogueItem("barstyles", "obi", testShellManifest(), defaultShellFiles(), nil)
	old := writeLegacyInstall(t, "obi")
	if err := migrateLegacyShellProducts(context.Background(), f.cache()); err != nil {
		t.Fatal(err)
	}
	receipt, err := readReceipt("barstyles", "obi")
	if err != nil || receipt.Shell == nil {
		t.Fatalf("barstyles receipt = %#v, %v", receipt, err)
	}
	listing, err := os.ReadDir(old)
	if err != nil || len(listing) != 1 || listing[0].Name() != shellBundleName {
		t.Fatalf("barstyles/obi holds %v, %v; want only the bundle", listing, err)
	}
}

func TestLegacyInstallTheCatalogueDroppedIsRemoved(t *testing.T) {
	f := newShellFixture(t)
	old := writeLegacyInstall(t, "gone")
	f.writeConfig("shell.json", map[string]any{"barStyle": "gone"})
	if err := migrateLegacyShellProducts(context.Background(), f.cache()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(receiptPath("barstyles", "gone")); !os.IsNotExist(err) {
		t.Fatalf("receipt survived: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(old, "Scene.qml")); !os.IsNotExist(err) {
		t.Fatalf("files survived: %v", err)
	}
	if style := f.readConfig("shell.json")["barStyle"]; style != shellFallbackID {
		t.Fatalf("barStyle = %v, want the fallback", style)
	}
}

func TestLegacyInstallIsUntouchedWhileTheCatalogueIsUnreachable(t *testing.T) {
	f := newShellFixture(t)
	old := writeLegacyInstall(t, "caelestia")
	t.Setenv("RYOSTORE_REVAMPED_BASE", filepath.Join(f.home, "missing-catalogue"))
	if err := migrateLegacyShellProducts(context.Background(), newRevampedCache()); err == nil {
		t.Fatal("migration claimed success without a catalogue")
	}
	if _, err := readReceipt("barstyles", "caelestia"); err != nil {
		t.Fatalf("legacy receipt lost: %v", err)
	}
	if matches, err := destinationMatchesReceipt(old, mustReadReceipt(t, "barstyles", "caelestia")); err != nil || !matches {
		t.Fatalf("legacy files changed: %v %v", matches, err)
	}
}

func TestModifiedLegacyInstallStaysInPlace(t *testing.T) {
	f := newShellFixture(t)
	f.addCatalogueItem("subshells", "tansu", testShellManifest(), defaultShellFiles(), nil)
	old := writeLegacyInstall(t, "tansu")
	writeShellTestFile(t, filepath.Join(old, "Scene.qml"), "edited by hand\n", 0o644)
	if err := migrateLegacyShellProducts(context.Background(), f.cache()); err == nil {
		t.Fatal("a modified install was reported as migrated")
	}
	if _, err := readReceipt("barstyles", "tansu"); err != nil {
		t.Fatalf("legacy receipt lost: %v", err)
	}
	if _, err := readReceipt("subshells", "tansu"); !os.IsNotExist(err) {
		t.Fatalf("subshell installed over a modified install: %v", err)
	}
}

func mustReadReceipt(t *testing.T, category, id string) Receipt {
	t.Helper()
	receipt, err := readReceipt(category, id)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}
