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
	"sort"
	"strings"
	"testing"
)

// shellQMLCase is one shell QML category and the setting that wears it: the
// launcher variant in launcher.json, the dock design inside shell.json's dock,
// an overlay design inside shell.json's designs.
type shellQMLCase struct {
	product shellQMLProduct
	id      string // the store product the fixture serves
	shipped string // an id the shell ships, served as a shadow row
	// wear writes a config wearing id beside unrelated user settings; worn reads
	// what the config wears now; kept reports the unrelated settings survived.
	wear func(t *testing.T, id string)
	worn func(t *testing.T) string
	kept func(t *testing.T) bool
}

func shellQMLCases() []shellQMLCase {
	return []shellQMLCase{
		{
			product: launcherProducts, id: "tatami", shipped: "hero",
			wear: func(t *testing.T, id string) {
				writeTestConfig(t, defaultLauncherConfigPath(), map[string]any{"variant": id, "radius": 12})
			},
			worn: func(t *testing.T) string {
				value, _ := readTestConfig(t, defaultLauncherConfigPath())["variant"].(string)
				return value
			},
			kept: func(t *testing.T) bool {
				return readTestConfig(t, defaultLauncherConfigPath())["radius"] == float64(12)
			},
		},
		{
			product: dockProducts, id: "engawa", shipped: "ii",
			wear: func(t *testing.T, id string) {
				writeTestConfig(t, defaultShellConfigPath(), map[string]any{
					"barStyle": "orpheus",
					"dock":     map[string]any{"design": id, "edge": "left", "pinned": []any{"kitty"}},
				})
			},
			worn: func(t *testing.T) string {
				dock, _ := readTestConfig(t, defaultShellConfigPath())["dock"].(map[string]any)
				value, _ := dock["design"].(string)
				return value
			},
			kept: func(t *testing.T) bool {
				config := readTestConfig(t, defaultShellConfigPath())
				dock, _ := config["dock"].(map[string]any)
				return config["barStyle"] == "orpheus" && dock["edge"] == "left" &&
					fmt.Sprint(dock["pinned"]) == "[kitty]"
			},
		},
		overlayCase(osdProducts, "ito", "osd"),
		overlayCase(notificationProducts, "tegami", "notification"),
		overlayCase(sidebarProducts, "byobu", "sidebar"),
	}
}

// overlayCase wears one of shell.json's designs beside the other two and an
// unrelated dock, so a fallback that rewrites the wrong key shows up as a loss.
func overlayCase(product shellQMLProduct, id, key string) shellQMLCase {
	others := map[string]any{"osd": "ryoku", "notification": "ryoku", "sidebar": "ryoku"}
	return shellQMLCase{
		product: product, id: id, shipped: "ryoku",
		wear: func(t *testing.T, id string) {
			designs := map[string]any{}
			for k, v := range others {
				designs[k] = v
			}
			designs[key] = id
			writeTestConfig(t, defaultShellConfigPath(), map[string]any{
				"barStyle": "orpheus",
				"dock":     map[string]any{"design": "engawa"},
				"designs":  designs,
			})
		},
		worn: func(t *testing.T) string {
			designs, _ := readTestConfig(t, defaultShellConfigPath())["designs"].(map[string]any)
			value, _ := designs[key].(string)
			return value
		},
		kept: func(t *testing.T) bool {
			config := readTestConfig(t, defaultShellConfigPath())
			dock, _ := config["dock"].(map[string]any)
			designs, _ := config["designs"].(map[string]any)
			for k, v := range others {
				if k != key && designs[k] != v {
					return false
				}
			}
			return config["barStyle"] == "orpheus" && dock["design"] == "engawa"
		},
	}
}

type shellQMLFixture struct {
	provider shellQMLProvider
	entry    []byte
}

func productFileFor(name string, body []byte) ProductFile {
	return ProductFile{
		Source: name, Destination: name, Mode: "0644",
		Size: int64(len(body)), SHA256: fmt.Sprintf("%x", sha256.Sum256(body)), Install: true,
	}
}

// newShellQMLFixture serves a registry holding one product of the case's
// category, plus a copy of its row under each id in shadows.
func newShellQMLFixture(t *testing.T, c shellQMLCase, shadows ...string) shellQMLFixture {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))

	category := c.product.category.ID
	served := map[string][]byte{c.product.entry: []byte("import QtQuick\nItem { readonly property bool shown: false }\n")}
	if c.product.preview != "" {
		served[c.product.preview] = []byte("import QtQuick\nItem { implicitWidth: 720; implicitHeight: 250 }\n")
	}
	files := make([]ProductFile, 0, len(served))
	for name, body := range served {
		files = append(files, productFileFor(name, body))
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Source < files[j].Source })
	manifest := ProductManifest{
		Schema: 1, ID: c.id, Category: category, Version: "1.0.0",
		Destination: "ryoku/" + category + "/" + c.id, Files: files,
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	entry := ProductEntry{
		ID: c.id, Name: c.id, Version: "1.0.0", Path: category + "/" + c.id,
		Author: "Ryoku Team", Summary: "Fixture", Description: "A fixture product.",
		Tags: []string{"fixture"}, Accent: "#c8b98f", Surface: "#0f0e0c",
		Preview: "assets/preview.png", Screenshots: []string{}, Manifest: "manifest.json",
		ManifestSHA256: fmt.Sprintf("%x", sha256.Sum256(manifestRaw)),
	}
	rows := []ProductEntry{entry}
	for _, id := range shadows {
		shadow := entry
		shadow.ID, shadow.Path = id, category+"/"+id
		rows = append(rows, shadow)
	}
	registryRaw, err := json.Marshal(map[string]any{"schema": 1, category: rows})
	if err != nil {
		t.Fatal(err)
	}
	prefix := "/" + category + "/" + c.id + "/"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, inProduct := strings.CutPrefix(r.URL.Path, prefix)
		switch {
		case r.URL.Path == "/"+category+"/registry.json":
			_, _ = w.Write(registryRaw)
		case inProduct && name == "manifest.json":
			_, _ = w.Write(manifestRaw)
		case inProduct && served[name] != nil:
			_, _ = w.Write(served[name])
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	cache := &Cache{client: server.Client(), base: server.URL, dir: t.TempDir(), memo: map[string]memoEntry{}}
	return shellQMLFixture{provider: newShellQMLProvider(c.product, cache), entry: served[c.product.entry]}
}

func writeTestConfig(t *testing.T, path string, state map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeConfigState(path, state); err != nil {
		t.Fatal(err)
	}
}

func readTestConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	state, err := readConfigState(path)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func readShellQMLIndex(t *testing.T, product shellQMLProduct) []shellQMLIndexRow {
	t.Helper()
	raw, err := os.ReadFile(product.indexPath())
	if err != nil {
		t.Fatal(err)
	}
	var rows []shellQMLIndexRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func shellQMLMarked(t *testing.T, c shellQMLCase) bool {
	t.Helper()
	selection, _ := selectionFor(c.product.category.ID)
	_, marked := readTestConfig(t, selection.path())[selection.marker]
	return marked
}

func itemByID(items []Item, id string) *Item {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func TestShellQMLInstallIsRunnableAndRemovalFallsBackToTheDefault(t *testing.T) {
	for _, c := range shellQMLCases() {
		t.Run(c.product.category.ID, func(t *testing.T) {
			fixture := newShellQMLFixture(t, c)
			ctx := context.Background()
			c.wear(t, c.id)

			items, state, err := fixture.provider.Load(ctx, false)
			if err != nil {
				t.Fatal(err)
			}
			if product := itemByID(items, c.id); len(items) != 1 || state.Offline || product == nil || product.Installed || product.Active {
				t.Fatalf("catalogue before install = %+v (offline %v)", items, state.Offline)
			}

			if err := fixture.provider.Install(ctx, c.id); err != nil {
				t.Fatal(err)
			}
			rows := readShellQMLIndex(t, c.product)
			if len(rows) != 1 || rows[0].ID != c.id || rows[0].Version != "1.0.0" || rows[0].Preview != c.product.preview {
				t.Fatalf("index after install = %+v", rows)
			}
			view := filepath.Join(storeStateDir(), filepath.FromSlash(rows[0].View))
			if body, err := os.ReadFile(filepath.Join(view, c.product.entry)); err != nil || string(body) != string(fixture.entry) {
				t.Fatalf("view %s = %q, err=%v", c.product.entry, body, err)
			}

			items, _, err = fixture.provider.Load(ctx, false)
			if err != nil {
				t.Fatal(err)
			}
			product := itemByID(items, c.id)
			if product == nil || !product.Installed || !product.Active || product.Metadata["view"] != rows[0].View ||
				product.Metadata["entry"] != c.product.entry {
				t.Fatalf("installed item = %+v", product)
			}

			if err := fixture.provider.Remove(ctx, c.id); err != nil {
				t.Fatal(err)
			}
			if rows := readShellQMLIndex(t, c.product); len(rows) != 0 {
				t.Fatalf("index after removal = %+v", rows)
			}
			if _, err := os.Stat(filepath.Join(storeStateDir(), c.product.views.root, c.id)); !os.IsNotExist(err) {
				t.Fatalf("removed product kept its view: %v", err)
			}
			selection, _ := selectionFor(c.product.category.ID)
			if got := c.worn(t); got != selection.fallback || !c.kept(t) {
				t.Fatalf("after removing the worn product the config wears %q (kept=%v)", got, c.kept(t))
			}
			if shellQMLMarked(t, c) {
				t.Fatal("committed removal left its marker")
			}
		})
	}
}

// Removing a product nobody wears must not touch the user's pick.
func TestShellQMLRemovalKeepsAnotherSelection(t *testing.T) {
	for _, c := range shellQMLCases() {
		t.Run(c.product.category.ID, func(t *testing.T) {
			fixture := newShellQMLFixture(t, c)
			ctx := context.Background()
			if err := fixture.provider.Install(ctx, c.id); err != nil {
				t.Fatal(err)
			}
			c.wear(t, c.shipped)
			if err := fixture.provider.Remove(ctx, c.id); err != nil {
				t.Fatal(err)
			}
			if got := c.worn(t); got != c.shipped || !c.kept(t) {
				t.Fatalf("config wears %q (kept=%v), want %q", got, c.kept(t), c.shipped)
			}
		})
	}
}

func TestShellQMLRemovalRollbackRestoresTheSelection(t *testing.T) {
	for _, c := range shellQMLCases() {
		t.Run(c.product.category.ID, func(t *testing.T) {
			fixture := newShellQMLFixture(t, c)
			ctx := context.Background()
			if err := fixture.provider.Install(ctx, c.id); err != nil {
				t.Fatal(err)
			}
			c.wear(t, c.id)
			previous := productTransactionCheckpoint
			productTransactionCheckpoint = func(phase string) error {
				if phase == "derived-state" {
					return errors.New("injected derived-state failure")
				}
				return nil
			}
			t.Cleanup(func() { productTransactionCheckpoint = previous })

			if err := fixture.provider.Remove(ctx, c.id); err == nil {
				t.Fatal("removal accepted a derived-state failure")
			}
			if _, err := readReceipt(c.product.category.ID, c.id); err != nil {
				t.Fatalf("rolled-back receipt: %v", err)
			}
			if got := c.worn(t); got != c.id || !c.kept(t) {
				t.Fatalf("rolled-back config wears %q (kept=%v)", got, c.kept(t))
			}
			if shellQMLMarked(t, c) {
				t.Fatal("rolled-back settings retain the marker")
			}
			if rows := readShellQMLIndex(t, c.product); len(rows) != 1 || rows[0].ID != c.id {
				t.Fatalf("rolled-back index = %+v", rows)
			}
		})
	}
}

func TestShellQMLIdsTheShellShipsAreRefused(t *testing.T) {
	for _, c := range shellQMLCases() {
		t.Run(c.product.category.ID, func(t *testing.T) {
			fixture := newShellQMLFixture(t, c, c.shipped)
			ctx := context.Background()
			items, _, err := fixture.provider.Load(ctx, false)
			if err != nil {
				t.Fatal(err)
			}
			if itemByID(items, c.shipped) != nil {
				t.Fatalf("a store row shadows the shipped %s: %+v", c.shipped, items)
			}
			if err := fixture.provider.Install(ctx, c.shipped); err == nil {
				t.Fatalf("the shipped %s was installable from the store", c.shipped)
			}
			if err := fixture.provider.Remove(ctx, c.shipped); err == nil {
				t.Fatalf("the shipped %s was removable", c.shipped)
			}
		})
	}
}

// A source without the category's registry yet shows an empty shelf rather
// than an error or an offline store.
func TestShellQMLCatalogueWithoutRegistryIsEmpty(t *testing.T) {
	for _, c := range shellQMLCases() {
		t.Run(c.product.category.ID, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
			server := httptest.NewServer(http.NotFoundHandler())
			t.Cleanup(server.Close)
			cache := &Cache{client: server.Client(), base: server.URL, dir: t.TempDir(), memo: map[string]memoEntry{}}
			items, state, err := newShellQMLProvider(c.product, cache).Load(context.Background(), false)
			if err != nil || len(items) != 0 || state.Offline {
				t.Fatalf("items=%+v offline=%v err=%v", items, state.Offline, err)
			}
		})
	}
}

// The store refuses the shell's own launcher ids, so the list must follow the
// shell's catalog whenever a launcher ships or retires.
func TestBuiltinLauncherIDsMatchTheShellCatalog(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "shell", "quickshell", "shell", "modules", "launcher", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Variants []struct {
			ID string `json:"id"`
		} `json:"variants"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	var shipped, refused []string
	for _, variant := range catalog.Variants {
		shipped = append(shipped, variant.ID)
	}
	for id := range launcherProducts.builtins {
		refused = append(refused, id)
	}
	sort.Strings(shipped)
	sort.Strings(refused)
	if fmt.Sprint(shipped) != fmt.Sprint(refused) {
		t.Fatalf("shell ships %v, the store refuses %v", shipped, refused)
	}
}
