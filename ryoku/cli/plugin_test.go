package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePlugin lays a plugin tree under a fresh temp dir: manifest.json plus the
// named entry files, each with placeholder content. It returns the dir.
func writePlugin(t *testing.T, manifestJSON string, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("// placeholder\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// a well-formed manifest with both entry points present.
const goodManifest = `{
  "id": "weathercard",
  "name": "Weather Card",
  "version": "1.2.0",
  "hosts": ["desktopWidget", "topbarGlyph"],
  "entryPoints": { "main": "service/Main.qml", "content": "content/Widget.qml" }
}`

func TestValidateManifestGood(t *testing.T) {
	dir := writePlugin(t, goodManifest, "service/Main.qml", "content/Widget.qml")
	m, err := validateManifest(dir, map[string]bool{"clock": true})
	if err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if m.ID != "weathercard" || m.Version != "1.2.0" {
		t.Fatalf("parsed %+v", m)
	}
}

func TestValidateManifestRejections(t *testing.T) {
	reserved := map[string]bool{"clock": true, "launcher": true}
	cases := []struct {
		name     string
		manifest string
		files    []string
		want     string
	}{
		{
			name: "reserved id",
			manifest: `{"id":"clock","name":"Impostor","version":"1.0.0",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "reserved",
		},
		{
			name: "missing entry point file",
			manifest: `{"id":"noentry","name":"No Entry","version":"1.0.0",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml"}, // content missing
			want:  "does not exist",
		},
		{
			name: "escaping entry point",
			manifest: `{"id":"escape","name":"Escape","version":"1.0.0",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"../outside.qml","content":"content/Widget.qml"}}`,
			files: []string{"content/Widget.qml"},
			want:  "relative path",
		},
		{
			name: "bad host",
			manifest: `{"id":"badhost","name":"Bad Host","version":"1.0.0",
				"hosts":["islandGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "unknown host",
		},
		{
			name: "no hosts",
			manifest: `{"id":"nohost","name":"No Host","version":"1.0.0",
				"hosts":[],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "no hosts",
		},
		{
			name: "bad id",
			manifest: `{"id":"Bad Id","name":"Bad","version":"1.0.0",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "lowercase",
		},
		{
			name: "missing version",
			manifest: `{"id":"nover","name":"No Ver",
				"hosts":["topbarGlyph"],"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`,
			files: []string{"service/Main.qml", "content/Widget.qml"},
			want:  "version",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := writePlugin(t, c.manifest, c.files...)
			_, err := validateManifest(dir, reserved)
			if err == nil {
				t.Fatalf("want error containing %q, got nil", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q, want substring %q", err, c.want)
			}
		})
	}
}

// TestValidateManifestSymlink rejects a plugin that ships a symlink anywhere in
// its tree.
func TestValidateManifestSymlink(t *testing.T) {
	dir := writePlugin(t, goodManifest, "service/Main.qml", "content/Widget.qml")
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "content", "sneaky.qml")); err != nil {
		t.Skipf("symlink not supported here: %v", err)
	}
	_, err := validateManifest(dir, map[string]bool{})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink must be rejected, got %v", err)
	}
}

// TestSafeRel pins the relative-path guard entry-point validation relies on.
func TestSafeRel(t *testing.T) {
	good := []string{"a.qml", "service/Main.qml", "./x/y.qml"}
	bad := []string{"", "/abs/path.qml", "../escape.qml", "a/../../b.qml"}
	for _, p := range good {
		if !safeRel(p) {
			t.Errorf("safeRel(%q) = false, want true", p)
		}
	}
	for _, p := range bad {
		if safeRel(p) {
			t.Errorf("safeRel(%q) = true, want false", p)
		}
	}
}

func TestReservedIDsUseInstalledShellCatalogue(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	if got := reservedIDs(); len(got) != 0 {
		t.Fatalf("reserved ids without shells index = %#v", got)
	}

	item := t.TempDir()
	catalogue := filepath.Join(item, "qml", "core", "widgets.json")
	if err := os.MkdirAll(filepath.Dir(catalogue), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogue, []byte(`[{"id":"clock"},{"id":"launcher"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	indexDir := filepath.Join(state, "ryoku", "store")
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	index, err := json.Marshal(map[string]any{"items": []map[string]any{{
		"id": "catalogue", "source": "store", "dir": item, "prepared": true,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(indexDir, "shells.json"), index, 0o644); err != nil {
		t.Fatal(err)
	}
	got := reservedIDs()
	if !got["clock"] || !got["launcher"] || got["totally-made-up"] {
		t.Fatalf("reserved ids = %#v", got)
	}
}

// TestPluginListRows merges a store receipt-owned install and a dev override with
// the plugins.json placement, marking each row's source.
func TestPluginListRows(t *testing.T) {
	data := t.TempDir()
	cfg := t.TempDir()
	state := t.TempDir()
	devRoot := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("RYOSTORE_PLUGINS_DIR", devRoot)
	t.Setenv("RYOKU_PLUGINS_DIR", "")

	// A store-installed plugin: its manifest in the install root plus a receipt.
	pdir := filepath.Join(data, "ryoku", "plugins", "weathercard")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "manifest.json"), []byte(goodManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	receiptDir := filepath.Join(state, "ryoku", "store", "plugins")
	if err := os.MkdirAll(receiptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(receiptDir, "weathercard.json"),
		[]byte(`{"category":"plugins","id":"weathercard","version":"1.2.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// A dev-override plugin under RYOSTORE_PLUGINS_DIR.
	ddir := filepath.Join(devRoot, "devcard")
	if err := os.MkdirAll(ddir, 0o755); err != nil {
		t.Fatal(err)
	}
	devManifest := `{"id":"devcard","name":"Dev Card","version":"9.9.9","hosts":["topbarGlyph"],
		"entryPoints":{"main":"service/Main.qml","content":"content/Widget.qml"}}`
	if err := os.WriteFile(filepath.Join(ddir, "manifest.json"), []byte(devManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	ryoku := filepath.Join(cfg, "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ryoku, "plugins.json"),
		[]byte(`{"weathercard":{"enabled":true,"host":"topbarGlyph"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rows := pluginListRows()
	byID := map[string]pluginRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	store, ok := byID["weathercard"]
	if !ok {
		t.Fatalf("store plugin missing from list: %+v", rows)
	}
	if store.Source != "store" || store.Version != "1.2.0" || !store.Enabled || store.Host != "topbarGlyph" {
		t.Fatalf("store row = %+v", store)
	}
	if store.Dir != pdir {
		t.Fatalf("store dir = %q, want %q", store.Dir, pdir)
	}
	dev, ok := byID["devcard"]
	if !ok {
		t.Fatalf("dev plugin missing from list: %+v", rows)
	}
	if dev.Source != "dev" || dev.Version != "9.9.9" || dev.Dir != ddir {
		t.Fatalf("dev row = %+v", dev)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d: %+v", len(rows), rows)
	}
}

// TestPluginListDevShadowsStore checks a dev override shadows a store install of
// the same id, listed once and marked dev, as discover.sh loads it.
func TestPluginListDevShadowsStore(t *testing.T) {
	data := t.TempDir()
	cfg := t.TempDir()
	state := t.TempDir()
	devRoot := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("RYOSTORE_PLUGINS_DIR", devRoot)
	t.Setenv("RYOKU_PLUGINS_DIR", "")

	receiptDir := filepath.Join(state, "ryoku", "store", "plugins")
	if err := os.MkdirAll(receiptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(receiptDir, "dup.json"),
		[]byte(`{"category":"plugins","id":"dup","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ddir := filepath.Join(devRoot, "dup")
	if err := os.MkdirAll(ddir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ddir, "manifest.json"),
		[]byte(`{"id":"dup","name":"Dup","version":"2.0.0","hosts":["topbarGlyph"],"entryPoints":{"main":"a","content":"b"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := pluginListRows()
	if len(rows) != 1 {
		t.Fatalf("want one row, got %+v", rows)
	}
	if rows[0].Source != "dev" || rows[0].Version != "2.0.0" {
		t.Fatalf("dev override must win: %+v", rows[0])
	}
}
