package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestWarmAssetsCachesRemoteRewritesAndIsIdempotent(t *testing.T) {
	t.Setenv("RYOKU_EXTRAS_BASE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	var mu sync.Mutex
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		_, _ = w.Write([]byte("IMG:" + r.URL.Path))
	}))
	defer srv.Close()

	preview := srv.URL + "/rices/a/assets/preview.webp"
	shot := srv.URL + "/rices/a/assets/shot.png"
	items := []Item{
		{ID: "a", Category: "rices", Art: preview, Screenshots: []string{shot}},
		{ID: "b", Category: "rices", Art: "file:///already/on/disk.png"},
	}
	warmAssets(context.Background(), srv.Client(), items)

	if !strings.HasPrefix(items[0].Art, "file://") {
		t.Fatalf("remote preview not cached to a local path: %q", items[0].Art)
	}
	if !strings.HasPrefix(items[0].Screenshots[0], "file://") {
		t.Fatalf("remote screenshot not cached to a local path: %q", items[0].Screenshots[0])
	}
	if items[1].Art != "file:///already/on/disk.png" {
		t.Fatalf("an already-local asset must pass through untouched: %q", items[1].Art)
	}

	cached := strings.TrimPrefix(items[0].Art, "file://")
	body, err := os.ReadFile(cached)
	if err != nil || string(body) != "IMG:/rices/a/assets/preview.webp" {
		t.Fatalf("cache content = %q err=%v", body, err)
	}
	if !strings.HasSuffix(cached, ".webp") {
		t.Fatalf("cached path must keep the extension for QML format detection: %q", cached)
	}

	// A second warm of the same URL reuses the cache: no new network hit.
	before := hits["/rices/a/assets/preview.webp"]
	again := []Item{{ID: "a", Category: "rices", Art: preview}}
	warmAssets(context.Background(), srv.Client(), again)
	if hits["/rices/a/assets/preview.webp"] != before {
		t.Fatalf("re-warm refetched a cached asset: %d -> %d", before, hits["/rices/a/assets/preview.webp"])
	}
	if again[0].Art != items[0].Art {
		t.Fatalf("re-warm resolved to a different path: %q vs %q", again[0].Art, items[0].Art)
	}
}

func TestWarmAssetsLeavesLargeMarketplaceRemote(t *testing.T) {
	t.Setenv("RYOKU_EXTRAS_BASE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		_, _ = w.Write([]byte("asset"))
	}))
	defer srv.Close()

	preview := srv.URL + "/omarchy-plugins/a/preview.webp"
	shot := srv.URL + "/omarchy-plugins/a/shot.png"
	if err := os.MkdirAll(assetCacheDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachedAssetPath(preview), []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}

	items := []Item{{
		ID:          "a",
		Category:    "omarchy-plugins",
		Art:         preview,
		ArtRaw:      preview,
		Screenshots: []string{shot},
	}}
	rewriteCachedAssets(items)
	warmAssets(context.Background(), srv.Client(), items)

	if items[0].Art != preview || items[0].ArtRaw != preview || items[0].Screenshots[0] != shot {
		t.Fatalf("large marketplace assets must stay remote for lazy delegates: %#v", items[0])
	}
	mu.Lock()
	gotHits := hits
	mu.Unlock()
	if gotHits != 0 {
		t.Fatalf("large marketplace warm made %d network requests, want 0", gotHits)
	}
}

func TestWarmAssetsLeavesRemoteURLOnFailure(t *testing.T) {
	t.Setenv("RYOKU_EXTRAS_BASE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	missing := srv.URL + "/rices/a/assets/preview.webp"
	items := []Item{{ID: "a", Category: "rices", Art: missing}}
	warmAssets(context.Background(), srv.Client(), items)
	if items[0].Art != missing {
		t.Fatalf("a failed download must leave the remote URL as a fallback, got %q", items[0].Art)
	}
}
