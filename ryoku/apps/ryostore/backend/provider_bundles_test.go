package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestBundleProviderNormalization proves the bundle provider carries registry
// metadata, resolves relative art, reads inline components for the total item
// count, and browses without fetching per-bundle definitions; with no status
// source nothing is installed.
func TestBundleProviderNormalization(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	srv, _ := fixtureServer(t)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	prov := bundleProvider{
		cache:  newCache(),
		status: func(context.Context) map[string]map[string]bool { return nil },
		launch: func(string, []string) error { return nil },
	}
	if prov.Category().ID != "bundles" {
		t.Fatalf("category id = %q, want bundles", prov.Category().ID)
	}
	got, _, err := prov.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	d := itemsByID(got)["demo"]
	if d.TotalCount != 2 {
		t.Fatalf("total count = %d, want 2", d.TotalCount)
	}
	if d.Art != srv.URL+"/bundles/demo/assets/hero.png" {
		t.Fatalf("relative art not resolved: %q", d.Art)
	}
	if len(d.Screenshots) != 2 || d.Screenshots[0] != srv.URL+"/bundles/demo/assets/a.png" {
		t.Fatalf("relative screenshot not resolved: %+v", d.Screenshots)
	}
	if d.Screenshots[1] != "https://cdn.example/b.png" {
		t.Fatalf("absolute screenshot must pass through: %q", d.Screenshots[1])
	}
	if _, err := os.Stat(filepath.Join(extrasCacheDir(), "bundles", "demo", "bundle.json")); err == nil {
		t.Fatal("browse must not warm the per-bundle definition")
	}
	if d.InstalledCount != 0 || d.Installed {
		t.Fatalf("bundle with no status must be empty: %+v", d)
	}
}

// TestBundleProviderPartialAndFullCounts proves the actuator status join sets a
// first-class partial count and only marks a bundle installed when every item
// is present.
func TestBundleProviderPartialAndFullCounts(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv, _ := fixtureServer(t)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	partial := bundleProvider{
		cache: newCache(),
		status: func(context.Context) map[string]map[string]bool {
			return map[string]map[string]bool{"demo": {"cmatrix": true, "demo-cli": false}}
		},
		launch: func(string, []string) error { return nil },
	}
	got, _, err := partial.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("partial Load: %v", err)
	}
	d := itemsByID(got)["demo"]
	if d.InstalledCount != 1 || d.TotalCount != 2 {
		t.Fatalf("partial counts = %d/%d, want 1/2", d.InstalledCount, d.TotalCount)
	}
	if d.Installed {
		t.Fatal("a partial bundle must not be marked fully installed")
	}

	full := bundleProvider{
		cache: newCache(),
		status: func(context.Context) map[string]map[string]bool {
			return map[string]map[string]bool{"demo": {"cmatrix": true, "demo-cli": true}}
		},
		launch: func(string, []string) error { return nil },
	}
	got2, _, err := full.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("full Load: %v", err)
	}
	d2 := itemsByID(got2)["demo"]
	if d2.InstalledCount != 2 || !d2.Installed {
		t.Fatalf("fully present bundle not marked installed: %d/%d installed=%v", d2.InstalledCount, d2.TotalCount, d2.Installed)
	}
}

// TestBundleProviderInstallLaunches proves Install delegates to the floating
// terminal launcher with the bundle id rather than mutating anything itself.
func TestBundleProviderInstallLaunches(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	got := ""
	prov := bundleProvider{
		cache:  newCache(),
		status: func(context.Context) map[string]map[string]bool { return nil },
		launch: func(id string, _ []string) error { got = id; return nil },
	}
	if err := prov.Install(context.Background(), "demo"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got != "demo" {
		t.Fatalf("launcher id = %q, want demo", got)
	}
}

// TestBundleProviderInstallComponents proves the manual selection reaches the
// launcher as the --only list, and that a whole-bundle Install passes none.
func TestBundleProviderInstallComponents(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var gotID string
	var gotOnly []string
	prov := bundleProvider{
		cache:  newCache(),
		status: func(context.Context) map[string]map[string]bool { return nil },
		launch: func(id string, only []string) error { gotID = id; gotOnly = only; return nil },
	}
	if err := prov.InstallComponents(context.Background(), "demo", []string{"steam", "lutris"}); err != nil {
		t.Fatalf("InstallComponents: %v", err)
	}
	if gotID != "demo" || strings.Join(gotOnly, ",") != "steam,lutris" {
		t.Fatalf("launcher got id=%q only=%v, want demo [steam lutris]", gotID, gotOnly)
	}
	gotOnly = []string{"stale"}
	if err := prov.Install(context.Background(), "demo"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if gotOnly != nil {
		t.Fatalf("whole-bundle Install must pass no components, got %v", gotOnly)
	}
}

// TestBundleProviderBrowseFetchesRegistryOnly proves browsing is a single
// registry request: no per-bundle definition and no installer is fetched to
// render the catalogue.
func TestBundleProviderBrowseFetchesRegistryOnly(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var mu sync.Mutex
	var paths []string
	const reg = `{"version":1,"bundles":[{"id":"demo","name":"Demo","description":"d","path":"bundles/demo","preview":"assets/hero.png","components":[` +
		`{"type":"package","name":"cmatrix","detect":"cmatrix","tier":"core","interactive":false,"summary":"rain"},` +
		`{"type":"script","name":"demo-cli","detect":"demo","tier":"optional","interactive":true,"summary":"cli"}]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/bundles/registry.json") {
			_, _ = w.Write([]byte(reg))
			return
		}
		http.Error(w, "browse fetched more than the registry", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	prov := bundleProvider{
		cache:  newCache(),
		status: func(context.Context) map[string]map[string]bool { return nil },
		launch: func(string, []string) error { return nil },
	}
	got, _, err := prov.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d := itemsByID(got)["demo"]; d.TotalCount != 2 {
		t.Fatalf("total count = %d, want 2", d.TotalCount)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 1 || !strings.HasSuffix(paths[0], "/bundles/registry.json") {
		t.Fatalf("browse made unexpected requests: %v", paths)
	}
}

// TestEnsureInstaller proves a script installer is cached and served from disk
// when the source is offline, and a never-cached one errors clearly.
func TestEnsureInstaller(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	srv, down := fixtureServer(t)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	p, err := ensureInstaller("demo-cli")
	if err != nil {
		t.Fatalf("ensureInstaller: %v", err)
	}
	if want := filepath.Join(extrasCacheDir(), "installers", "demo-cli.sh"); p != want {
		t.Fatalf("path = %q, want %q", p, want)
	}
	*down = true
	if _, err := ensureInstaller("demo-cli"); err != nil {
		t.Fatalf("offline ensureInstaller: %v", err)
	}
	if _, err := ensureInstaller("missing"); err == nil {
		t.Fatal("expected an error for an uncached, unreachable installer")
	}
}

// TestEnsureInstallerResolvesBundlePath proves an installer published under
// its bundle's directory (the layout bundle.json declares) is found there,
// while a legacy root-only installer still resolves through the fallback.
func TestEnsureInstallerResolvesBundlePath(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	const reg = `{"version":1,"bundles":[{"id":"zedish","name":"Zedish","description":"d","path":"bundles/zedish","components":[
		{"type":"script","name":"zed-theme","detect":"zed-theme","tier":"core","interactive":false,"summary":"theme"},
		{"type":"script","name":"legacy-cli","detect":"legacy-cli","tier":"core","interactive":false,"summary":"cli"}]}]}`
	const script = "#!/bin/sh\necho themed\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bundles/registry.json":
			_, _ = w.Write([]byte(reg))
		case "/bundles/zedish/installers/zed-theme.sh":
			_, _ = w.Write([]byte(script))
		case "/installers/legacy-cli.sh":
			_, _ = w.Write([]byte(script))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	p, err := ensureInstaller("zed-theme")
	if err != nil {
		t.Fatalf("ensureInstaller: %v", err)
	}
	if want := filepath.Join(extrasCacheDir(), "bundles", "zedish", "installers", "zed-theme.sh"); p != want {
		t.Fatalf("path = %q, want %q", p, want)
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != script {
		t.Fatalf("cached installer = %q, %v; want the bundle copy", b, err)
	}

	p, err = ensureInstaller("legacy-cli")
	if err != nil {
		t.Fatalf("legacy ensureInstaller: %v", err)
	}
	if want := filepath.Join(extrasCacheDir(), "installers", "legacy-cli.sh"); p != want {
		t.Fatalf("legacy path = %q, want %q", p, want)
	}
}

// TestEnsureNautilusPack proves a pack's scripts install executable under their
// subdir, a tracking manifest records them, and removal clears the subdir.
func TestEnsureNautilusPack(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv, _ := fixtureServer(t)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	if _, err := ensureNautilusPack("video-reformat"); err != nil {
		t.Fatalf("ensureNautilusPack: %v", err)
	}
	script := filepath.Join(data, "nautilus", "scripts", "Ryoku Creator", "Reformat 9x16")
	fi, err := os.Stat(script)
	if err != nil {
		t.Fatalf("script not installed: %v", err)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("script not executable: %v", fi.Mode())
	}
	if _, err := os.Stat(filepath.Join(data, "ryoku", "nautilus", "video-reformat", "manifest.json")); err != nil {
		t.Errorf("tracking manifest missing: %v", err)
	}
	if err := removeNautilusPack("video-reformat"); err != nil {
		t.Fatalf("removeNautilusPack: %v", err)
	}
	if _, err := os.Stat(script); !os.IsNotExist(err) {
		t.Errorf("script survived removal: %v", err)
	}
}

// TestAssetFetchBustsCDN proves the raw source fetch used by the asset install
// primitives defeats the GitHub raw (Fastly) CDN: a unique query per request
// plus a no-cache header, so a refresh never gets a stale hit.
func TestAssetFetchBustsCDN(t *testing.T) {
	got := make(chan string, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cc := r.Header.Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("missing no-cache header, got %q", cc)
		}
		got <- r.URL.RawQuery
		w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	c := newCache()
	ctx := context.Background()
	if _, err := c.get(ctx, "plugins/registry.json"); err != nil {
		t.Fatalf("get: %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := c.get(ctx, "plugins/registry.json"); err != nil {
		t.Fatalf("get: %v", err)
	}
	q1, q2 := <-got, <-got
	if q1 == "" {
		t.Fatal("first fetch sent no cache-busting query")
	}
	if q1 == q2 {
		t.Fatalf("two fetches reused query %q; a CDN could serve a stale hit", q1)
	}
}

func TestBundleProviderRejectsMalformedDefinition(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bundles/registry.json":
			w.Write([]byte(`{"bundles":[{"id":"broken","path":"collections/broken"}]}`))
		case "/collections/broken/bundle.json":
			w.Write([]byte(`{"items":`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	prov := bundleProvider{
		cache:  newCache(),
		status: func(context.Context) map[string]map[string]bool { return nil },
		launch: func(string, []string) error { return nil },
	}
	if _, _, err := prov.Load(context.Background(), false); err == nil {
		t.Fatal("malformed required bundle definition was accepted")
	}
}

func TestBundleProviderRejectsMissingDefinition(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bundles/registry.json" {
			w.Write([]byte(`{"bundles":[{"id":"missing","path":"collections/missing"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	prov := bundleProvider{
		cache:  newCache(),
		status: func(context.Context) map[string]map[string]bool { return nil },
		launch: func(string, []string) error { return nil },
	}
	if _, _, err := prov.Load(context.Background(), false); err == nil {
		t.Fatal("missing required bundle definition was accepted")
	}
}

type nautilusFixture struct {
	subdir  string
	scripts []string
	bodies  map[string]string
}

func serveNautilusFixture(t *testing.T, fixture *nautilusFixture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nautilus/registry.json":
			w.Write([]byte(`{"packs":[{"id":"pack","name":"Pack","path":"nautilus/pack"}]}`))
		case "/nautilus/pack/manifest.json":
			raw, err := json.Marshal(map[string]any{"subdir": fixture.subdir, "scripts": fixture.scripts})
			if err != nil {
				t.Error(err)
				http.Error(w, "marshal", http.StatusInternalServerError)
				return
			}
			w.Write(raw)
		default:
			const prefix = "/nautilus/pack/scripts/"
			if !strings.HasPrefix(r.URL.Path, prefix) {
				http.NotFound(w, r)
				return
			}
			name := strings.TrimPrefix(r.URL.Path, prefix)
			body, ok := fixture.bodies[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestEnsureNautilusPackRejectsEscapesAndSymlinks(t *testing.T) {
	t.Run("traversing subdir", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("XDG_DATA_HOME", data)
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		fixture := &nautilusFixture{subdir: "../../outside", scripts: []string{"safe"}, bodies: map[string]string{"safe": "payload"}}
		srv := serveNautilusFixture(t, fixture)
		t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

		if _, err := ensureNautilusPack("pack"); err == nil {
			t.Fatal("traversing Nautilus subdir accepted")
		}
		if _, err := os.Lstat(filepath.Join(data, "outside")); !os.IsNotExist(err) {
			t.Fatalf("pack escaped scripts root: %v", err)
		}
	})

	t.Run("traversing script", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("XDG_DATA_HOME", data)
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		fixture := &nautilusFixture{subdir: "Pack", scripts: []string{"../escape"}, bodies: map[string]string{"../escape": "payload"}}
		srv := serveNautilusFixture(t, fixture)
		t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

		if _, err := ensureNautilusPack("pack"); err == nil {
			t.Fatal("traversing Nautilus script accepted")
		}
		if _, err := os.Lstat(filepath.Join(nautilusScriptsDir(), "escape")); !os.IsNotExist(err) {
			t.Fatalf("script escaped pack root: %v", err)
		}
	})

	t.Run("symlink destination", func(t *testing.T) {
		data := t.TempDir()
		t.Setenv("XDG_DATA_HOME", data)
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
		fixture := &nautilusFixture{subdir: "Pack", scripts: []string{"safe"}, bodies: map[string]string{"safe": "payload"}}
		srv := serveNautilusFixture(t, fixture)
		t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

		external := t.TempDir()
		root := filepath.Join(nautilusScriptsDir(), "Pack")
		if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, root); err != nil {
			t.Fatal(err)
		}
		if _, err := ensureNautilusPack("pack"); err == nil {
			t.Fatal("symlinked Nautilus destination accepted")
		}
		if _, err := os.Lstat(filepath.Join(external, "safe")); !os.IsNotExist(err) {
			t.Fatalf("pack wrote through destination symlink: %v", err)
		}
	})
}

func TestEnsureNautilusPackPublishesWholeReplacement(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	fixture := &nautilusFixture{subdir: "Pack", scripts: []string{"old"}, bodies: map[string]string{"old": "known-good"}}
	srv := serveNautilusFixture(t, fixture)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	if _, err := ensureNautilusPack("pack"); err != nil {
		t.Fatalf("initial install: %v", err)
	}
	root := filepath.Join(nautilusScriptsDir(), "Pack")

	fixture.scripts = []string{"old", "missing"}
	fixture.bodies["old"] = "changed"
	if _, err := ensureNautilusPack("pack"); err == nil {
		t.Fatal("incomplete replacement succeeded")
	}
	old, err := os.ReadFile(filepath.Join(root, "old"))
	if err != nil || string(old) != "known-good" {
		t.Fatalf("failed replacement changed known-good pack: data=%q err=%v", old, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "missing")); !os.IsNotExist(err) {
		t.Fatalf("failed replacement exposed partial script: %v", err)
	}

	fixture.scripts = []string{"new"}
	fixture.bodies = map[string]string{"new": "replacement"}
	if _, err := ensureNautilusPack("pack"); err != nil {
		t.Fatalf("complete replacement: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "old")); !os.IsNotExist(err) {
		t.Fatalf("renamed stale script survived replacement: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "new")); err != nil || string(b) != "replacement" {
		t.Fatalf("replacement script missing: data=%q err=%v", b, err)
	}
}

func TestRemoveNautilusPackRejectsEscapingTrackingRecord(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	track := nautilusTrackDir("pack")
	if err := os.MkdirAll(track, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(track, "manifest.json"), []byte(`{"subdir":"../../outside"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(data, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(outside, "keep")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeNautilusPack("pack"); err == nil {
		t.Fatal("escaping tracking record accepted")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("escaping removal deleted outside data: %v", err)
	}
}

func TestEnsureNautilusPackRejectsSubdirChange(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	fixture := &nautilusFixture{subdir: "Old", scripts: []string{"script"}, bodies: map[string]string{"script": "old"}}
	srv := serveNautilusFixture(t, fixture)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	if _, err := ensureNautilusPack("pack"); err != nil {
		t.Fatalf("initial install: %v", err)
	}
	fixture.subdir = "New"
	fixture.bodies["script"] = "new"
	if _, err := ensureNautilusPack("pack"); err == nil {
		t.Fatal("Nautilus subdir change was accepted")
	}
	if b, err := os.ReadFile(filepath.Join(nautilusScriptsDir(), "Old", "script")); err != nil || string(b) != "old" {
		t.Fatalf("old subdir was not preserved: data=%q err=%v", b, err)
	}
	if _, err := os.Lstat(filepath.Join(nautilusScriptsDir(), "New")); !os.IsNotExist(err) {
		t.Fatalf("new subdir was published: %v", err)
	}
}

func TestComponentBoundariesRejectDotID(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, err := ensureInstaller("."); err == nil {
		t.Fatal("dot installer name accepted")
	}
	if _, err := ensureNautilusPack("."); err == nil {
		t.Fatal("dot Nautilus id accepted")
	}
}

func TestBundleProviderRejectsDotID(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bundles/registry.json" {
			w.Write([]byte(`{"bundles":[{"id":".","path":"bundles/dot"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)
	prov := bundleProvider{
		cache:  newCache(),
		status: func(context.Context) map[string]map[string]bool { return nil },
		launch: func(string, []string) error { return nil },
	}
	if _, _, err := prov.Load(context.Background(), false); err == nil {
		t.Fatal("dot bundle id accepted")
	}
}

func TestEnsureNautilusPackRejectsJournalNamespaceCollision(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	fixture := &nautilusFixture{subdir: "Pack", scripts: []string{"script"}, bodies: map[string]string{"script": "new"}}
	srv := serveNautilusFixture(t, fixture)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	root := filepath.Join(nautilusScriptsDir(), "Pack")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	collision := backupTreePath(root)
	if err := os.MkdirAll(collision, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(collision, "owned-by-another-pack")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureNautilusPack("pack"); err == nil {
		t.Fatal("journal namespace collision was accepted")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("colliding live tree was deleted: %v", err)
	}
}

func TestEnsureNautilusPackRecoversBeforeNetworkFetch(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	fixture := &nautilusFixture{subdir: "Pack", scripts: []string{"script"}, bodies: map[string]string{"script": "known-good"}}
	srv := serveNautilusFixture(t, fixture)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)
	if _, err := ensureNautilusPack("pack"); err != nil {
		t.Fatalf("initial install: %v", err)
	}

	root := filepath.Join(nautilusScriptsDir(), "Pack")
	backup := backupTreePath(root)
	if err := os.Rename(root, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalTreePath(root), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	srv.Close()
	if _, err := ensureNautilusPack("pack"); err == nil {
		t.Fatal("offline retry unexpectedly succeeded")
	}
	if b, err := os.ReadFile(filepath.Join(root, "script")); err != nil || string(b) != "known-good" {
		t.Fatalf("known-good pack stayed hidden in journal: data=%q err=%v", b, err)
	}
}

func TestRemoveNautilusPackWaitsForPublicationLock(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	track := nautilusTrackDir("pack")
	root := filepath.Join(nautilusScriptsDir(), "Pack")
	if err := os.MkdirAll(track, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(track, "manifest.json"), []byte(`{"subdir":"Pack"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	unlock, err := lockTree(root)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- removeNautilusPack("pack") }()
	select {
	case err := <-done:
		t.Fatalf("Nautilus removal raced publication lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Nautilus removal did not resume after publication lock")
	}
}

// TestBundleComponentGroupReachesUI proves the optional `group` label survives
// the whole path the store detail view depends on: registry JSON, into
// bundleComponent, out through metadata.items. The map is built field by field,
// so a component field that is parsed but not copied here renders nothing; the
// second half pins that an ungrouped component omits the key entirely rather
// than emitting an empty one, which is what keeps ungrouped bundles unlabelled.
func TestBundleComponentGroupReachesUI(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv, _ := fixtureServer(t)
	t.Setenv("RYOKU_EXTRAS_BASE", srv.URL)

	prov := bundleProvider{
		cache:  newCache(),
		status: func(context.Context) map[string]map[string]bool { return nil },
		launch: func(string, []string) error { return nil },
	}
	got, _, err := prov.Load(context.Background(), false)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	raw, ok := itemsByID(got)["demo"].Metadata["items"].([]map[string]any)
	if !ok {
		t.Fatalf("metadata items missing or wrong shape: %#v", itemsByID(got)["demo"].Metadata["items"])
	}
	byName := make(map[string]map[string]any, len(raw))
	for _, c := range raw {
		byName[c["name"].(string)] = c
	}
	if g := byName["cmatrix"]["group"]; g != "Tools" {
		t.Fatalf("grouped component lost its label: group = %#v, want \"Tools\"", g)
	}
	if _, present := byName["demo-cli"]["group"]; present {
		t.Fatalf("ungrouped component must omit group, got %#v", byName["demo-cli"])
	}
}
