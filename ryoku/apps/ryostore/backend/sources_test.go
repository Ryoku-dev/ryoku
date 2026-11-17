package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeSourceURL(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"https://store.example.org/ryoku/", "https://store.example.org/ryoku", true},
		{"https://store.example.org", "https://store.example.org", true},
		{"https://store.example.org/a/../b", "https://store.example.org/b", true},
		{"http://127.0.0.1:8000", "http://127.0.0.1:8000", true},
		{"http://localhost:8000/x/", "http://localhost:8000/x", true},
		{"http://[::1]:9/", "http://[::1]:9", true},
		{"file:///srv/store/", "file:///srv/store", true},
		{"/srv/store", "file:///srv/store", true},
		{"http://store.example.org", "", false},
		{"http://192.168.1.4/", "", false},
		{"ftp://store.example.org", "", false},
		{"https://user:pw@store.example.org", "", false},
		{"https://store.example.org/?a=1", "", false},
		{"https://store.example.org/#x", "", false},
		{"https:///nohost", "", false},
		{"file://host/srv", "", false},
		{"file:///", "", false},
		{"relative/path", "", false},
		{"", "", false},
		{"https://store.example.org/a b", "", false},
		{"https://" + strings.Repeat("a", 600) + ".org", "", false},
	}
	for _, c := range cases {
		got, err := normalizeSourceURL(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("normalizeSourceURL(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("normalizeSourceURL(%q) = %q, want an error", c.in, got)
		}
	}
}

func TestParseStoreIndex(t *testing.T) {
	good := `{"schema":1,"name":"Acme store","description":"Plugins by Acme.","homepage":"https://acme.example","categories":["plugins","barstyles"]}`
	index, err := parseStoreIndex([]byte(good))
	if err != nil || index.Name != "Acme store" || len(index.Categories) != 2 {
		t.Fatalf("good index: %+v, %v", index, err)
	}
	if _, err := parseStoreIndex([]byte(`{"schema":1,"name":"Acme","categories":["docks"]}`)); err != nil {
		t.Fatalf("minimal index: %v", err)
	}
	bad := map[string]string{
		"unknown field":      `{"schema":1,"name":"A","categories":["plugins"],"official":true}`,
		"schema 2":           `{"schema":2,"name":"A","categories":["plugins"]}`,
		"no name":            `{"schema":1,"categories":["plugins"]}`,
		"padded name":        `{"schema":1,"name":" A","categories":["plugins"]}`,
		"control in name":    "{\"schema\":1,\"name\":\"A\\u0007\",\"categories\":[\"plugins\"]}",
		"long name":          `{"schema":1,"name":"` + strings.Repeat("n", 49) + `","categories":["plugins"]}`,
		"long description":   `{"schema":1,"name":"A","description":"` + strings.Repeat("d", 281) + `","categories":["plugins"]}`,
		"http homepage":      `{"schema":1,"name":"A","homepage":"http://acme.example","categories":["plugins"]}`,
		"no categories":      `{"schema":1,"name":"A","categories":[]}`,
		"duplicate category": `{"schema":1,"name":"A","categories":["plugins","plugins"]}`,
		"profiles":           `{"schema":1,"name":"A","categories":["profiles"]}`,
		"bundles":            `{"schema":1,"name":"A","categories":["bundles"]}`,
		"rices":              `{"schema":1,"name":"A","categories":["rices"]}`,
		"colorschemes":       `{"schema":1,"name":"A","categories":["colorschemes"]}`,
		"nautilus":           `{"schema":1,"name":"A","categories":["nautilus"]}`,
		"installers":         `{"schema":1,"name":"A","categories":["installers"]}`,
		"lockscreens":        `{"schema":1,"name":"A","categories":["lockscreens"]}`,
		"vesktop themes":     `{"schema":1,"name":"A","categories":["vesktop-themes"]}`,
		"two values":         `{"schema":1,"name":"A","categories":["plugins"]}{}`,
		"not an object":      `[]`,
	}
	for name, raw := range bad {
		if _, err := parseStoreIndex([]byte(raw)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSourcesConfigValidation(t *testing.T) {
	setTransactionXDG(t)
	valid := storeSource{ID: "acme", Name: "Acme", URL: "https://acme.example/store", Enabled: true, Added: "2026-11-16T10:00:00Z"}
	if err := validateSourcesConfig(sourcesConfig{Schema: 1, Sources: []storeSource{valid}}); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	mutate := func(f func(*storeSource)) []storeSource {
		s := valid
		f(&s)
		return []storeSource{s}
	}
	bad := map[string]sourcesConfig{
		"schema":         {Schema: 2},
		"reserved id":    {Schema: 1, Sources: mutate(func(s *storeSource) { s.ID = "ryostore" })},
		"bad id":         {Schema: 1, Sources: mutate(func(s *storeSource) { s.ID = "Acme" })},
		"empty name":     {Schema: 1, Sources: mutate(func(s *storeSource) { s.Name = "" })},
		"plain http":     {Schema: 1, Sources: mutate(func(s *storeSource) { s.URL = "http://acme.example" })},
		"not normalized": {Schema: 1, Sources: mutate(func(s *storeSource) { s.URL = "https://acme.example/store/" })},
		"bad added":      {Schema: 1, Sources: mutate(func(s *storeSource) { s.Added = "yesterday" })},
		"duplicate id":   {Schema: 1, Sources: []storeSource{valid, func() storeSource { s := valid; s.URL = "https://b.example"; return s }()}},
		"duplicate url":  {Schema: 1, Sources: []storeSource{valid, func() storeSource { s := valid; s.ID = "acme-2"; return s }()}},
	}
	for name, config := range bad {
		if err := validateSourcesConfig(config); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// a hand edit with an unknown field is refused, not silently dropped
	if err := os.MkdirAll(filepath.Dir(sourcesConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcesConfigPath(), []byte(`{"schema":1,"sources":[{"id":"acme","name":"Acme","url":"https://acme.example/store","enabled":true,"added":"2026-11-16T10:00:00Z","official":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSourcesConfig(); err == nil {
		t.Fatal("an unknown source field was accepted")
	}
	if err := os.Remove(sourcesConfigPath()); err != nil {
		t.Fatal(err)
	}
	config, err := readSourcesConfig()
	if err != nil || config.Schema != 1 || len(config.Sources) != 0 {
		t.Fatalf("missing config: %+v, %v", config, err)
	}
}

func TestSourceIDFor(t *testing.T) {
	taken := map[string]bool{"acme-store": true}
	cases := map[string]string{
		"Acme Store":            "acme-store-2",
		"Ryostore":              "ryostore-2",
		"  ***  ":               "store",
		"Café Noir!":            "caf-noir",
		strings.Repeat("x", 60): strings.Repeat("x", 29),
	}
	for name, want := range cases {
		if got := sourceIDFor(name, taken); got != want || !validUserSourceID(got) {
			t.Errorf("sourceIDFor(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestUserSourcesServeOnlyHashedCategories(t *testing.T) {
	for _, category := range userSourceCategories {
		if err := validateProductIdentity("acme", category, "demo"); err != nil {
			t.Errorf("%s refused for a user store: %v", category, err)
		}
	}
	for _, category := range []string{"profiles", "bundles", "rices", "lockscreens", "fastfetch", "ryotunes-skins", vesktopThemesCategory, "decors", "launcher-images", "fastfetch-emblems"} {
		if err := validateProductIdentity("acme", category, "demo"); err == nil {
			t.Errorf("%s allowed for a user store", category)
		}
		if err := validateProductIdentity("", category, "demo"); err != nil {
			t.Errorf("%s refused for the official stores: %v", category, err)
		}
	}
	for _, source := range []string{"ryostore", "ryostore-revamped", "Acme", "../x", strings.Repeat("a", 33)} {
		if err := validateProductIdentity(source, "plugins", "demo"); err == nil {
			t.Errorf("source %q accepted", source)
		}
	}
}

func TestUserSourceStateIsNamespaced(t *testing.T) {
	setTransactionXDG(t)
	official, officialRel, err := sourceProductDestination("", "plugins", "demo")
	if err != nil {
		t.Fatal(err)
	}
	user, userRel, err := sourceProductDestination("acme", "plugins", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if officialRel != "ryoku/plugins/demo" || userRel != "ryoku/sources/acme/plugins/demo" {
		t.Fatalf("destinations %q %q", officialRel, userRel)
	}
	if official == user || sourceReceiptPath("", "plugins", "demo") == sourceReceiptPath("acme", "plugins", "demo") ||
		productJournalPath("", "plugins", "demo") == productJournalPath("acme", "plugins", "demo") {
		t.Fatal("a user store shares state paths with the official stores")
	}
	if want := filepath.Join(storeStateDir(), "sources", "acme", "plugins", "demo.json"); sourceReceiptPath("acme", "plugins", "demo") != want {
		t.Fatalf("user receipt path %s", sourceReceiptPath("acme", "plugins", "demo"))
	}
}

// TestReceiptCannotChangeSource proves a receipt is read only as the source
// whose folder holds it, so a copied file cannot move an install between stores.
func TestReceiptCannotChangeSource(t *testing.T) {
	setTransactionXDG(t)
	receipt := Receipt{
		Source: "acme", Category: "plugins", ID: "demo", Version: "1.0.0",
		Destination: "ryoku/sources/acme/plugins/demo",
		Files:       []ReceiptFile{{Source: "manifest.json", Destination: "manifest.json", SHA256: strings.Repeat("a", 64), Mode: "0644", Size: 1}},
	}
	if err := writeReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := readSourceReceipt("acme", "plugins", "demo"); err != nil {
		t.Fatalf("own receipt: %v", err)
	}
	raw, err := os.ReadFile(sourceReceiptPath("acme", "plugins", "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(receiptPath("plugins", "demo")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath("plugins", "demo"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readReceipt("plugins", "demo"); err == nil {
		t.Fatal("a user store receipt read as official")
	}
	official := receipt
	official.Source, official.Destination = "", "ryoku/plugins/demo"
	if err := validateReceipt("plugins", "demo", official); err != nil {
		t.Fatal(err)
	}
	wrongDestination := receipt
	wrongDestination.Destination = "ryoku/plugins/demo"
	if err := validateReceipt("plugins", "demo", wrongDestination); err == nil {
		t.Fatal("a user store receipt claimed the official folder")
	}
}

func configureSource(t *testing.T, id, name, address string) storeSource {
	t.Helper()
	source := storeSource{ID: id, Name: name, URL: address, Enabled: true, Added: "2026-11-16T10:00:00Z"}
	config, err := readSourcesConfig()
	if err != nil {
		t.Fatal(err)
	}
	config.Sources = append(config.Sources, source)
	if err := writeSourcesConfig(config); err != nil {
		t.Fatal(err)
	}
	return source
}

// TestCollisionRule proves one copy of a category/id is installed at a time:
// a user store cannot replace, adopt or shadow the official install, the
// official store cannot replace a user store's install, and each store's
// install lands in its own namespace.
func TestCollisionRule(t *testing.T) {
	setTransactionXDG(t)
	ctx := context.Background()
	official := newTransactionFixture(t, "1.0.0", []byte("official\n"), []byte("official\n"))
	user := newTransactionFixture(t, "2.0.0", []byte("community\n"), []byte("community\n"), func(e *ProductEntry) {
		e.Official = true // a user store's own claim; never trusted
	})
	user.cache.source = configureSource(t, "acme", "Acme", user.cache.base).ID

	if err := installProduct(ctx, official.cache, "plugins", official.entry); err != nil {
		t.Fatalf("official install: %v", err)
	}
	err := installProduct(ctx, user.cache, "plugins", user.entry)
	if err == nil || !strings.Contains(err.Error(), "already installed from Ryostore") {
		t.Fatalf("user install over the official one: %v", err)
	}
	if user.requests.hit("/plugins/demo/content/Plugin.qml") {
		t.Fatal("the refused install downloaded its payload")
	}
	if got := readFile(t, filepath.Join(installedPluginPath(), "content", "Plugin.qml")); got != "official\n" {
		t.Fatalf("official payload changed: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(dataHome(), "ryoku", "sources")); !os.IsNotExist(err) {
		t.Fatalf("the refused install left a namespace behind: %v", err)
	}

	if err := removeProduct(ctx, "plugins", "demo"); err != nil {
		t.Fatal(err)
	}
	if err := installProduct(ctx, user.cache, "plugins", user.entry); err != nil {
		t.Fatalf("user install: %v", err)
	}
	userDir, _, _ := sourceProductDestination("acme", "plugins", "demo")
	if got := readFile(t, filepath.Join(userDir, "content", "Plugin.qml")); got != "community\n" {
		t.Fatalf("user payload: %q", got)
	}
	if _, err := os.Lstat(installedPluginPath()); !os.IsNotExist(err) {
		t.Fatalf("the user install wrote the official folder: %v", err)
	}
	receipt, err := readSourceReceipt("acme", "plugins", "demo")
	if err != nil || receipt.Source != "acme" || receipt.Destination != "ryoku/sources/acme/plugins/demo" {
		t.Fatalf("user receipt %+v, %v", receipt, err)
	}
	if record, ok := readSourceRecord("acme"); !ok || record.Name != "Acme" {
		t.Fatalf("source record %+v %v", record, ok)
	}
	if revision := readRevisionForTest(t); revision.Source != "acme" || revision.Operation != "install" {
		t.Fatalf("revision %+v", revision)
	}
	rows := readPluginIndexForTest(t)
	if len(rows) != 1 || rows[0].ID != "demo" || rows[0].Version != "2.0.0" {
		t.Fatalf("plugin index %+v", rows)
	}

	err = installProduct(ctx, official.cache, "plugins", official.entry)
	if err == nil || !strings.Contains(err.Error(), "already installed from Acme") {
		t.Fatalf("official install over the user one: %v", err)
	}
	if owners := productOwners("plugins", "demo"); len(owners) != 1 || owners[0] != "acme" {
		t.Fatalf("owners %v", owners)
	}

	// the catalogue lists both, each with its own state
	items, _, err := pluginProvider{cache: user.cache}.Load(ctx, true)
	if err != nil || len(items) != 1 || !items[0].Installed || items[0].Metadata["official"] != nil || items[0].Metadata["community"] != true {
		t.Fatalf("user listing %+v, %v", items, err)
	}
	items, _, err = pluginProvider{cache: official.cache}.Load(ctx, true)
	if err != nil || len(items) != 1 || items[0].Installed {
		t.Fatalf("official listing %+v, %v", items, err)
	}

	if err := removeUserSourceProduct(ctx, "acme", "plugins", "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(sourceReceiptPath("acme", "plugins", "demo")); !os.IsNotExist(err) {
		t.Fatalf("user receipt survived removal: %v", err)
	}
	if rows := readPluginIndexForTest(t); len(rows) != 0 {
		t.Fatalf("plugin index after removal %+v", rows)
	}
}

// TestUserSourceJournalRecovers proves an interrupted user store install is
// rolled back from its own journal, never touching the official namespace.
func TestUserSourceJournalRecovers(t *testing.T) {
	setTransactionXDG(t)
	ctx := context.Background()
	user := newTransactionFixture(t, "1.0.0", []byte("x\n"), []byte("x\n"))
	user.cache.source = configureSource(t, "acme", "Acme", user.cache.base).ID
	productTransactionCheckpoint = func(phase string) error {
		if phase == "install-receipt" {
			return errProductTransactionInterrupted
		}
		return nil
	}
	t.Cleanup(func() { productTransactionCheckpoint = func(string) error { return nil } })
	if err := installProduct(ctx, user.cache, "plugins", user.entry); !errors.Is(err, errProductTransactionInterrupted) {
		t.Fatalf("interrupted install: %v", err)
	}
	if _, err := os.Lstat(productJournalPath("acme", "plugins", "demo")); err != nil {
		t.Fatalf("no user journal: %v", err)
	}
	productTransactionCheckpoint = func(string) error { return nil }
	if err := recoverStoreTransactions(); err != nil {
		t.Fatal(err)
	}
	userDir, _, _ := sourceProductDestination("acme", "plugins", "demo")
	for _, path := range []string{userDir, sourceReceiptPath("acme", "plugins", "demo"), productJournalPath("acme", "plugins", "demo")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("%s survived recovery: %v", path, err)
		}
	}
}

// writeFileStore lays out a minimal user store: index, plugins registry, one
// plugin with its hashed manifest.
func writeFileStore(t *testing.T, root string, categories []string) {
	t.Helper()
	write := func(rel string, data []byte) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	index, _ := json.Marshal(map[string]any{"schema": 1, "name": "Acme store", "categories": categories})
	write("ryostore.json", index)
	runtime := []byte(`{"id":"clock","name":"Clock","version":"1.0.0","hosts":["desktopWidget"]}`)
	write("plugins/clock/manifest.json", runtime)
	preview := []byte("png")
	write("plugins/clock/assets/preview.png", preview)
	manifest, _ := json.Marshal(ProductManifest{Schema: 1, ID: "clock", Category: "plugins", Version: "1.0.0", Destination: "ryoku/plugins/clock",
		Files: []ProductFile{
			{Source: "assets/preview.png", Destination: "assets/preview.png", SHA256: fmt.Sprintf("%x", sha256.Sum256(preview)), Mode: "0644", Size: int64(len(preview)), Install: false},
			{Source: "manifest.json", Destination: "manifest.json", SHA256: fmt.Sprintf("%x", sha256.Sum256(runtime)), Mode: "0644", Size: int64(len(runtime)), Install: true},
		}})
	write("plugins/clock/product-manifest.json", manifest)
	registry, _ := json.Marshal(map[string]any{"schema": 1, "plugins": []ProductEntry{{
		ID: "clock", Name: "Clock", Version: "1.0.0", Path: "plugins/clock", Author: "Acme", Summary: "A clock.",
		Description: "A desktop clock.", Tags: []string{"time"}, Screenshots: []string{}, Accent: "#cdc4ba", Surface: "#101010",
		Preview: "assets/preview.png", Manifest: "product-manifest.json", ManifestSHA256: fmt.Sprintf("%x", sha256.Sum256(manifest)),
		Upstream: "https://acme.example",
	}}})
	write("plugins/registry.json", registry)
}

func TestAddSourceValidatesBeforeRecording(t *testing.T) {
	setTransactionXDG(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	good := t.TempDir()
	writeFileStore(t, good, []string{"plugins"})

	entry, err := addSource(ctx, "file://"+good+"/", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != "acme-store" || entry.Name != "Acme store" || entry.URL != "file://"+good || entry.Status != "ok" ||
		entry.Items == nil || *entry.Items != 1 || entry.Official || !entry.Enabled || entry.Added != "2026-11-16T12:00:00Z" {
		t.Fatalf("added %+v", entry)
	}
	if _, err := addSource(ctx, good, "Again", now); err == nil || !strings.Contains(err.Error(), "already added") {
		t.Fatalf("duplicate add: %v", err)
	}

	forbidden := t.TempDir()
	writeFileStore(t, forbidden, []string{"plugins", "profiles"})
	if _, err := addSource(ctx, forbidden, "", now); err == nil || !strings.Contains(err.Error(), "profiles") {
		t.Fatalf("store serving profiles: %v", err)
	}
	broken := t.TempDir()
	writeFileStore(t, broken, []string{"plugins"})
	if err := os.WriteFile(filepath.Join(broken, "plugins", "registry.json"), []byte(`{"schema":1,"plugins":[{"id":"x"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := addSource(ctx, broken, "", now); err == nil {
		t.Fatal("a store with an invalid registry was added")
	}
	if _, err := addSource(ctx, t.TempDir(), "", now); err == nil {
		t.Fatal("a folder with no index was added")
	}
	if _, err := addSource(ctx, "file://"+good+"x", "Bad\x00", now); err == nil {
		t.Fatal("a control character in --name was accepted")
	}
	config, err := readSourcesConfig()
	if err != nil || len(config.Sources) != 1 {
		t.Fatalf("config after refusals %+v, %v", config, err)
	}

	if _, err := removeSource("ryostore"); err == nil {
		t.Fatal("an official source was removed")
	}
	if _, err := setSourceEnabled("ryostore-revamped", false); err == nil {
		t.Fatal("an official source was turned off")
	}
	disabled, err := setSourceEnabled("acme-store", false)
	if err != nil || disabled.Enabled {
		t.Fatalf("disable %+v, %v", disabled, err)
	}
	if described := describeUserSource(ctx, disabled); described.Status != "disabled" {
		t.Fatalf("disabled source %+v", described)
	}
	if installed, err := removeSource("acme-store"); err != nil || installed != 0 {
		t.Fatalf("remove: %d, %v", installed, err)
	}
	if _, err := removeSource("acme-store"); err == nil {
		t.Fatal("removing an absent source succeeded")
	}
}

// TestUserSourceEndToEnd adds a file:// store, lists its item in the
// catalogue with its source, installs it with --source, keeps it listed and
// removable after the store is removed, and reports the removed store.
func TestUserSourceEndToEnd(t *testing.T) {
	setTransactionXDG(t)
	ctx := context.Background()
	root := t.TempDir()
	writeFileStore(t, root, []string{"plugins"})
	entry, err := addSource(ctx, root, "Acme", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	official := newTransactionFixture(t, "1.0.0", []byte("x\n"), []byte("x\n"))
	provs := []Provider{pluginProvider{cache: official.cache}}

	if err := runInstall(provs, []string{"plugins", "clock", "--source", entry.ID}); err != nil {
		t.Fatal(err)
	}
	cat := BuildCatalog(ctx, catalogProviders(ctx, provs, true), true)
	var clock, demo *Item
	for i := range cat.Items {
		switch cat.Items[i].ID {
		case "clock":
			clock = &cat.Items[i]
		case "demo":
			demo = &cat.Items[i]
		}
	}
	if clock == nil || clock.Source != "acme" || clock.SourceName != "Acme" || clock.Official || !clock.Installed {
		t.Fatalf("user item %+v", clock)
	}
	if demo == nil || demo.Source != "ryostore" || !demo.Official {
		t.Fatalf("official item %+v", demo)
	}
	if cat.Categories[0].InstalledCount != 1 || cat.Categories[0].Count != 2 {
		t.Fatalf("category %+v", cat.Categories[0])
	}

	if _, err := removeSource(entry.ID); err != nil {
		t.Fatal(err)
	}
	cat = BuildCatalog(ctx, catalogProviders(ctx, provs, true), true)
	found := false
	for _, item := range cat.Items {
		if item.ID == "clock" {
			found = item.Installed && item.Source == "acme" && item.SourceName == "Acme" && item.Metadata["sourceUnavailable"] == true
		}
	}
	if !found {
		t.Fatalf("an install from a removed store left the catalogue: %+v", cat.Items)
	}
	listed := listSources(ctx)
	last := listed[len(listed)-1]
	if last.ID != "acme" || last.Status != "removed" || last.Installed != 1 || !strings.Contains(last.Note, "no updates") {
		t.Fatalf("removed source entry %+v", last)
	}
	// removal finds the owning store from its receipt
	if err := runRemove(provs, []string{"plugins", "clock"}); err != nil {
		t.Fatal(err)
	}
	if owners := productOwners("plugins", "clock"); len(owners) != 0 {
		t.Fatalf("owners after removal %v", owners)
	}
	for _, dir := range []string{userSourceStateDir("acme"), filepath.Join(dataHome(), "ryoku", "sources", "acme")} {
		if _, err := os.Lstat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s survived the last install of a removed store: %v", dir, err)
		}
	}
}

func TestSourcedProviderMarksInstalledElsewhere(t *testing.T) {
	setTransactionXDG(t)
	official := fakeProvider{category: Category{ID: "plugins"}, items: []Item{{ID: "clock", Category: "plugins", Installed: true}, {ID: "solo", Category: "plugins"}}}
	user := fakeProvider{category: Category{ID: "plugins"}, items: []Item{{ID: "clock", Category: "plugins"}}}
	wrapped := sourcedProvider{Provider: official, parts: []sourcePart{{source: storeSource{ID: "acme", Name: "Acme"}, provider: user}}}
	cat := BuildCatalog(context.Background(), []Provider{wrapped}, false)
	if len(cat.Items) != 3 {
		t.Fatalf("items %+v", cat.Items)
	}
	community := cat.Items[2]
	if community.Source != "acme" || community.Official || community.InstalledFrom != "Ryostore" {
		t.Fatalf("user copy %+v", community)
	}
	if cat.Items[0].Source != "ryostore" || !cat.Items[0].Official || cat.Items[0].InstalledFrom != "" || cat.Items[1].InstalledFrom != "" {
		t.Fatalf("official items %+v", cat.Items[:2])
	}
	// a failing user store costs only its own items
	broken := sourcedProvider{Provider: official, parts: []sourcePart{{source: storeSource{ID: "acme", Name: "Acme"}, provider: fakeProvider{category: Category{ID: "plugins"}, err: errors.New("down")}}}}
	cat = BuildCatalog(context.Background(), []Provider{broken}, false)
	if len(cat.Items) != 2 || cat.Categories[0].Error != "" {
		t.Fatalf("broken user store %+v", cat)
	}
}

func TestSnapshotKeyedPerSourceSet(t *testing.T) {
	setTransactionXDG(t)
	base := snapshotPath()
	if filepath.Base(base) != "catalog.json" {
		t.Fatalf("no sources: %s", base)
	}
	configureSource(t, "acme", "Acme", "https://acme.example")
	keyed := snapshotPath()
	if keyed == base || !strings.HasPrefix(filepath.Base(keyed), "catalog-") {
		t.Fatalf("one source: %s", keyed)
	}
	configureSource(t, "beta", "Beta", "https://beta.example")
	if snapshotPath() == keyed {
		t.Fatal("two sources share one snapshot with one")
	}
	if _, err := setSourceEnabled("beta", false); err != nil {
		t.Fatal(err)
	}
	if snapshotPath() != keyed {
		t.Fatal("a turned-off source changed the snapshot key")
	}
	if err := os.MkdirAll(filepath.Dir(keyed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyed, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := setSourceEnabled("beta", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(keyed); !os.IsNotExist(err) {
		t.Fatal("changing the source set kept an old snapshot")
	}
}

func TestSourcesCommandJSON(t *testing.T) {
	setTransactionXDG(t)
	t.Setenv("RYOSTORE_BASE", "file://"+t.TempDir())
	t.Setenv("RYOSTORE_REVAMPED_BASE", "file://"+t.TempDir())
	root := t.TempDir()
	writeFileStore(t, root, []string{"plugins"})
	var out bytes.Buffer
	if err := runSources(&out, []string{"add", "file://" + root, "--name", "Acme"}); err != nil {
		t.Fatal(err)
	}
	var added sourceEntry
	if err := json.Unmarshal(out.Bytes(), &added); err != nil || added.ID != "acme" {
		t.Fatalf("add printed %s: %v", out.String(), err)
	}
	out.Reset()
	if err := runSources(&out, []string{"list", "--json"}); err != nil {
		t.Fatal(err)
	}
	var list struct {
		Schema  int              `json:"schema"`
		Sources []map[string]any `json:"sources"`
	}
	if err := json.Unmarshal(out.Bytes(), &list); err != nil || list.Schema != 1 || len(list.Sources) != 3 {
		t.Fatalf("list printed %s: %v", out.String(), err)
	}
	if list.Sources[0]["id"] != "ryostore" || list.Sources[0]["official"] != true ||
		list.Sources[1]["id"] != "ryostore-revamped" || list.Sources[2]["id"] != "acme" || list.Sources[2]["official"] != false ||
		list.Sources[2]["status"] != "ok" || list.Sources[2]["items"] != float64(1) {
		t.Fatalf("list %+v", list.Sources)
	}
	for _, bad := range [][]string{{}, {"list", "--yaml"}, {"add"}, {"remove"}, {"enable", "a", "b"}, {"frobnicate"}} {
		if err := runSources(&out, bad); err == nil {
			t.Errorf("sources %v accepted", bad)
		}
	}
}
