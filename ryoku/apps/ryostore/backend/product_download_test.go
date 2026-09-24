package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestProductFilesUseTheDownloadClient pins the fix for lockscreen installs
// failing with "files[5] content/bg.mp4: context deadline exceeded
// (Client.Timeout...)": a multi-megabyte product asset must download on the
// long-bounded client, not the 12s catalogue probe client. The catalogue
// client here is bound to die instantly; if the fetch routes through it, the
// test times out and fails.
func TestProductFilesUseTheDownloadClient(t *testing.T) {
	payload := make([]byte, 4<<20) // 4 MiB, like a wallpaper video
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])

	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		// A deliberate trickle: slow enough that a 10ms client timeout dies
		// mid-body, exactly the "while reading body" shape from the report.
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for off := 0; off < len(payload); off += 64 << 10 {
			end := off + 64<<10
			if end > len(payload) {
				end = len(payload)
			}
			if _, err := w.Write(payload[off:end]); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(2 * time.Millisecond)
		}
	}))
	t.Cleanup(server.Close)

	cache := &Cache{
		client:   &http.Client{Timeout: 10 * time.Millisecond},
		download: &http.Client{Timeout: productDownloadTimeout},
		base:     server.URL,
		dir:      t.TempDir(),
		memo:     map[string]memoEntry{},
	}

	data, err := fetchProductFile(context.Background(), cache, "lockscreens/demo/content/bg.mp4", int64(len(payload)), hash)
	if err != nil {
		t.Fatalf("fetchProductFile through the download client: %v", err)
	}
	if len(data) != len(payload) {
		t.Fatalf("fetched %d bytes, want %d", len(data), len(payload))
	}
	if hits != 1 {
		t.Fatalf("server saw %d requests, want 1", hits)
	}
}

// TestNewCacheCarriesBothClients guards the wiring itself: a production cache
// must own a distinct, longer-bounded download client, or product installs
// silently fall back to the probe budget.
func TestNewCacheCarriesBothClients(t *testing.T) {
	t.Setenv("RYOSTORE_BASE", "")
	t.Setenv("RYOKU_EXTRAS_BASE", "")
	cache := newCache()
	if cache.download == nil {
		t.Fatal("newCache has no download client")
	}
	if cache.download == cache.client {
		t.Fatal("product downloads share the catalogue client")
	}
	if cache.download.Timeout <= cache.client.Timeout {
		t.Fatalf("download timeout %v must exceed catalogue timeout %v",
			cache.download.Timeout, cache.client.Timeout)
	}
}
