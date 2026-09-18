package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type barProviderFixture struct {
	cache  *Cache
	entry  ProductEntry
	scene  []byte
	server *httptest.Server
}

// barStyleByID finds a catalogue row without pinning the order of the shipped
// built-ins ahead of it.
func barStyleByID(items []Item, id string) *Item {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func newBarProviderFixture(t *testing.T) barProviderFixture {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	previousPatch := barStyleShellPatch
	barStyleShellPatch = func(string) error { return errors.New("shell unavailable in test") }
	t.Cleanup(func() { barStyleShellPatch = previousPatch })
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))

	scene := []byte("import QtQuick\nItem { property string marker: \"obi-v1\" }\n")
	sceneHash := sha256.Sum256(scene)
	manifest := ProductManifest{
		Schema: 1, ID: "obi", Category: "barstyles", Version: "1.0.0",
		Destination: "ryoku/barstyles/obi",
		Files: []ProductFile{{
			Source: "Scene.qml", Destination: "Scene.qml", Mode: "0644",
			Size: int64(len(scene)), SHA256: fmt.Sprintf("%x", sceneHash), Install: true,
		}},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifestRaw)
	entry := ProductEntry{
		ID: "obi", Name: "Obi", Version: "1.0.0", Path: "barstyles/obi",
		Author: "Ryoku Team", Summary: "Floating sash", Description: "A complete bar scene.",
		Tags: []string{"top"}, Accent: "#b86b5f", Surface: "#11100f",
		Preview: "assets/preview.webp", Screenshots: []string{}, Manifest: "manifest.json",
		ManifestSHA256: fmt.Sprintf("%x", manifestDigest),
	}
	registryRaw, err := json.Marshal(map[string]any{"schema": 1, "barstyles": []ProductEntry{entry}})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/barstyles/registry.json":
			_, _ = w.Write(registryRaw)
		case "/barstyles/obi/manifest.json":
			_, _ = w.Write(manifestRaw)
		case "/barstyles/obi/Scene.qml":
			_, _ = w.Write(scene)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return barProviderFixture{
		entry: entry, scene: scene, server: server,
		cache: &Cache{client: server.Client(), base: server.URL, dir: t.TempDir(), memo: map[string]memoEntry{}},
	}
}

func TestBarProviderUsesRegistryReceiptsAndDerivedIndex(t *testing.T) {
	fixture := newBarProviderFixture(t)
	config := filepath.Join(configHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("{\"barStyle\":\"obi\",\"keep\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	provider := barProvider{cache: fixture.cache, shellConfig: config}

	items, _, err := provider.Load(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 || items[0].ID != "sumi" || !items[0].Installed || !items[0].Active {
		t.Fatalf("initial items = %+v", items)
	}
	if obi := barStyleByID(items, "obi"); obi == nil || obi.Installed || obi.Active {
		t.Fatalf("initial external item = %+v", obi)
	}

	if err := provider.Install(context.Background(), "obi"); err != nil {
		t.Fatal(err)
	}
	dst, _, err := productDestination("barstyles", "obi")
	if err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(filepath.Join(dst, "Scene.qml")); err != nil || string(body) != string(fixture.scene) {
		t.Fatalf("installed scene = %q, err=%v", body, err)
	}
	var rows []barStyleIndexRow
	if raw, err := os.ReadFile(barStyleIndexPath()); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	wantRow := barStyleIndexRow{ID: "obi", Version: "1.0.0", Scene: "Scene.qml"}
	receipt, err := readReceipt("barstyles", "obi")
	if err != nil {
		t.Fatal(err)
	}
	wantRow.View = barStyleView(wantRow, receipt)
	if len(rows) != 1 || rows[0] != wantRow {
		t.Fatalf("installed index = %+v", rows)
	}
	viewPath := filepath.Join(storeStateDir(), filepath.FromSlash(rows[0].View))
	if body, err := os.ReadFile(filepath.Join(viewPath, "Scene.qml")); err != nil || string(body) != string(fixture.scene) {
		t.Fatalf("versioned view scene = %q, err=%v", body, err)
	}

	items, _, err = provider.Load(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if obi := barStyleByID(items, "obi"); obi == nil || !obi.Installed || !obi.Active {
		t.Fatalf("installed item = %+v", obi)
	}

	if err := provider.Remove(context.Background(), "obi"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("removed destination still exists: %v", err)
	}
	if raw, err := os.ReadFile(barStyleIndexPath()); err != nil {
		t.Fatal(err)
	} else if string(raw) != "[]\n" {
		t.Fatalf("removed index = %q", raw)
	}
	var shell map[string]any
	if raw, err := os.ReadFile(config); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(raw, &shell); err != nil {
		t.Fatal(err)
	}
	if shell["barStyle"] != "sumi" || shell["keep"] != true {
		t.Fatalf("fallback config = %#v", shell)
	}
}

func TestBarProviderKeepsReceiptOwnedStyleUsableOffline(t *testing.T) {
	fixture := newBarProviderFixture(t)
	config := filepath.Join(configHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("{\"barStyle\":\"obi\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	provider := barProvider{cache: fixture.cache, shellConfig: config}
	if err := provider.Install(context.Background(), "obi"); err != nil {
		t.Fatal(err)
	}

	fixture.server.Close()
	provider.cache = &Cache{
		client: fixture.server.Client(),
		base:   fixture.server.URL,
		dir:    t.TempDir(),
		memo:   map[string]memoEntry{},
	}
	items, state, err := provider.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("offline load: %v", err)
	}
	if !state.Offline {
		t.Fatal("cold offline load was not marked offline")
	}
	if obi := barStyleByID(items, "obi"); len(items) != 5 || obi == nil || !obi.Installed || !obi.Active {
		t.Fatalf("offline items = %+v", items)
	}
	if err := provider.Remove(context.Background(), "obi"); err != nil {
		t.Fatalf("offline remove: %v", err)
	}
	if raw, err := os.ReadFile(barStyleIndexPath()); err != nil || string(raw) != "[]\n" {
		t.Fatalf("offline removal index = %q, err=%v", raw, err)
	}
}

func TestBarProviderReadyRecoveryPublishesIndexAndFallback(t *testing.T) {
	fixture := newBarProviderFixture(t)
	config := filepath.Join(configHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("{\"barStyle\":\"obi\",\"keep\":true}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	provider := barProvider{cache: fixture.cache, shellConfig: config}
	if err := provider.Install(context.Background(), "obi"); err != nil {
		t.Fatal(err)
	}

	previousCheckpoint := productTransactionCheckpoint
	productTransactionCheckpoint = func(phase string) error {
		if phase == "ready" {
			return errProductTransactionInterrupted
		}
		return nil
	}
	if err := provider.Remove(context.Background(), "obi"); !errors.Is(err, errProductTransactionInterrupted) {
		t.Fatalf("ready interruption = %v", err)
	}
	productTransactionCheckpoint = previousCheckpoint
	t.Cleanup(func() { productTransactionCheckpoint = previousCheckpoint })

	if err := recoverStoreTransactions(); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(barStyleIndexPath()); err != nil || string(raw) != "[]\n" {
		t.Fatalf("recovered index = %q, err=%v", raw, err)
	}
	var shell map[string]any
	if raw, err := os.ReadFile(config); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(raw, &shell); err != nil {
		t.Fatal(err)
	}
	if shell["barStyle"] != "sumi" || shell["keep"] != true {
		t.Fatalf("recovered fallback = %#v", shell)
	}
}

func TestBarProviderLoadRecoversInterruptedTransaction(t *testing.T) {
	fixture := newBarProviderFixture(t)
	provider := barProvider{cache: fixture.cache, shellConfig: defaultShellConfigPath()}
	previousCheckpoint := productTransactionCheckpoint
	productTransactionCheckpoint = func(phase string) error {
		if phase == "install-published" {
			return errProductTransactionInterrupted
		}
		return nil
	}
	if err := provider.Install(context.Background(), "obi"); !errors.Is(err, errProductTransactionInterrupted) {
		t.Fatalf("interrupted install = %v", err)
	}
	productTransactionCheckpoint = previousCheckpoint
	defer func() { productTransactionCheckpoint = previousCheckpoint }()

	items, _, err := provider.Load(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if obi := barStyleByID(items, "obi"); len(items) != 5 || obi == nil || obi.Installed {
		t.Fatalf("catalog after recovery = %+v", items)
	}
	destination, _, err := productDestination("barstyles", "obi")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("interrupted destination remains: %v", err)
	}
	if raw, err := os.ReadFile(barStyleIndexPath()); err != nil || string(raw) != "[]\n" {
		t.Fatalf("recovered index = %q, err=%v", raw, err)
	}
}

func TestBarProviderRefusesBuiltinRemoval(t *testing.T) {
	if err := (barProvider{}).Remove(context.Background(), "sumi"); err == nil {
		t.Fatal("built-in Sumi was removable")
	}
}

func TestBarProviderDerivedStateRollsBackWithProduct(t *testing.T) {
	fixture := newBarProviderFixture(t)
	provider := barProvider{cache: fixture.cache, shellConfig: defaultShellConfigPath()}
	previousCheckpoint := productTransactionCheckpoint
	productTransactionCheckpoint = func(phase string) error {
		if phase == "derived-state" {
			return errors.New("injected derived-state failure")
		}
		return nil
	}
	defer func() { productTransactionCheckpoint = previousCheckpoint }()

	if err := provider.Install(context.Background(), "obi"); err == nil {
		t.Fatal("install accepted derived-state failure")
	}
	if _, err := readReceipt("barstyles", "obi"); !os.IsNotExist(err) {
		t.Fatalf("rolled-back receipt remains: %v", err)
	}
	if raw, err := os.ReadFile(barStyleIndexPath()); err != nil || string(raw) != "[]\n" {
		t.Fatalf("rolled-back index = %q, err=%v", raw, err)
	}
}

func TestBarProviderReadyRecoveryPreservesNewerSelection(t *testing.T) {
	fixture := newBarProviderFixture(t)
	provider := barProvider{cache: fixture.cache, shellConfig: defaultShellConfigPath()}
	ctx := context.Background()
	if err := provider.Install(ctx, "obi"); err != nil {
		t.Fatal(err)
	}
	if err := setBarStyleSelection(provider.shellConfig, "obi"); err != nil {
		t.Fatal(err)
	}

	previousCheckpoint := productTransactionCheckpoint
	productTransactionCheckpoint = func(phase string) error {
		if phase == "ready" {
			return errProductTransactionInterrupted
		}
		return nil
	}
	if err := provider.Remove(ctx, "obi"); !errors.Is(err, errProductTransactionInterrupted) {
		t.Fatalf("interrupted removal error = %v", err)
	}
	productTransactionCheckpoint = previousCheckpoint
	defer func() { productTransactionCheckpoint = previousCheckpoint }()

	if err := atomicWrite(barStyleIndexPath(), []byte("[{\"id\":\"obi\"}]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeBarStyleSelection(provider.shellConfig, "nacre"); err != nil {
		t.Fatal(err)
	}
	if err := recoverStoreTransactions(); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(barStyleIndexPath()); err != nil || string(raw) != "[]\n" {
		t.Fatalf("recovered index = %q, err=%v", raw, err)
	}
	if active := readBarStyleSelection(provider.shellConfig); active != "nacre" {
		t.Fatalf("recovered active style = %q", active)
	}
	var shell map[string]any
	if raw, err := os.ReadFile(provider.shellConfig); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(raw, &shell); err != nil {
		t.Fatal(err)
	}
	if _, present := shell[barStyleTransactionKey]; present {
		t.Fatalf("recovered settings retain transaction marker: %#v", shell)
	}
}

func TestBarProviderRemovalRollbackRestoresOwnedSelection(t *testing.T) {
	fixture := newBarProviderFixture(t)
	provider := barProvider{cache: fixture.cache, shellConfig: defaultShellConfigPath()}
	ctx := context.Background()
	if err := provider.Install(ctx, "obi"); err != nil {
		t.Fatal(err)
	}
	if err := setBarStyleSelection(provider.shellConfig, "obi"); err != nil {
		t.Fatal(err)
	}
	previousCheckpoint := productTransactionCheckpoint
	productTransactionCheckpoint = func(phase string) error {
		if phase == "derived-state" {
			return errors.New("injected derived-state failure")
		}
		return nil
	}
	defer func() { productTransactionCheckpoint = previousCheckpoint }()

	if err := provider.Remove(ctx, "obi"); err == nil {
		t.Fatal("removal accepted derived-state failure")
	}
	if _, err := readReceipt("barstyles", "obi"); err != nil {
		t.Fatalf("rolled-back receipt: %v", err)
	}
	if active := readBarStyleSelection(provider.shellConfig); active != "obi" {
		t.Fatalf("rolled-back active style = %q", active)
	}
	var shell map[string]any
	if raw, err := os.ReadFile(provider.shellConfig); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(raw, &shell); err != nil {
		t.Fatal(err)
	}
	if _, present := shell[barStyleTransactionKey]; present {
		t.Fatalf("rolled-back settings retain transaction marker: %#v", shell)
	}
}

func TestBarStyleViewSnapshotIsImmutable(t *testing.T) {
	fixture := newBarProviderFixture(t)
	provider := barProvider{cache: fixture.cache, shellConfig: defaultShellConfigPath()}
	if err := provider.Install(context.Background(), "obi"); err != nil {
		t.Fatal(err)
	}
	var rows []barStyleIndexRow
	if raw, err := os.ReadFile(barStyleIndexPath()); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("installed index = %+v", rows)
	}
	viewScene := filepath.Join(storeStateDir(), filepath.FromSlash(rows[0].View), "Scene.qml")
	destination, _, err := productDestination("barstyles", "obi")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "Scene.qml"), []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(viewScene); err != nil || string(raw) != string(fixture.scene) {
		t.Fatalf("immutable view changed with destination: %q, err=%v", raw, err)
	}
}

func TestBarStyleViewRebuildsCorruptSnapshot(t *testing.T) {
	fixture := newBarProviderFixture(t)
	provider := barProvider{cache: fixture.cache, shellConfig: defaultShellConfigPath()}
	if err := provider.Install(context.Background(), "obi"); err != nil {
		t.Fatal(err)
	}
	var rows []barStyleIndexRow
	if raw, err := os.ReadFile(barStyleIndexPath()); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	viewScene := filepath.Join(storeStateDir(), filepath.FromSlash(rows[0].View), "Scene.qml")
	if err := os.WriteFile(viewScene, make([]byte, len(fixture.scene)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := provider.Load(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(viewScene); err != nil || string(raw) != string(fixture.scene) {
		t.Fatalf("corrupt view was not rebuilt: %q, err=%v", raw, err)
	}
}
