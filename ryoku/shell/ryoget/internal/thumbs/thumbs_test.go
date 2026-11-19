package thumbs

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryoget-thumbs-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", filepath.Join(root, "home"))
	os.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

var png, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

func server(t *testing.T, hits *int32) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		switch r.URL.Path {
		case "/a.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(png)
		case "/sniff":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(png)
		case "/page":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html></html>"))
		case "/huge":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(bytes.Repeat([]byte{0xff}, MaxBytes+10))
		case "/slow":
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchCachesImages(t *testing.T) {
	var hits int32
	srv := server(t, &hits)
	dir := t.TempDir()
	ctx := context.Background()
	p, err := Fetch(ctx, srv.Client(), dir, srv.URL+"/a.png")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(p) != dir || !strings.HasSuffix(p, ".png") || len(filepath.Base(p)) != 64+4 {
		t.Errorf("path = %q, want <sha256>.png in the cache dir", p)
	}
	if b, _ := os.ReadFile(p); !bytes.Equal(b, png) {
		t.Error("cached bytes differ")
	}
	p2, err := Fetch(ctx, srv.Client(), dir, srv.URL+"/a.png")
	if err != nil || p2 != p || atomic.LoadInt32(&hits) != 1 {
		t.Errorf("second fetch: %q %v hits=%d; want the cached file without a request", p2, err, hits)
	}
}

func TestFetchSniffsGenericTypes(t *testing.T) {
	var hits int32
	srv := server(t, &hits)
	p, err := Fetch(context.Background(), srv.Client(), t.TempDir(), srv.URL+"/sniff")
	if err != nil || !strings.HasSuffix(p, ".png") {
		t.Fatalf("%q %v", p, err)
	}
}

func TestFetchRejects(t *testing.T) {
	var hits int32
	srv := server(t, &hits)
	dir := t.TempDir()
	ctx := context.Background()
	if _, err := Fetch(ctx, srv.Client(), dir, srv.URL+"/page"); !errors.Is(err, ErrNotImage) {
		t.Errorf("html: %v", err)
	}
	if _, err := Fetch(ctx, srv.Client(), dir, srv.URL+"/huge"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("huge: %v", err)
	}
	if _, err := Fetch(ctx, srv.Client(), dir, srv.URL+"/missing"); err == nil {
		t.Error("404 accepted")
	}
	if _, err := Fetch(ctx, srv.Client(), dir, "file:///etc/passwd"); !errors.Is(err, ErrBadURL) {
		t.Errorf("file url: %v", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if _, err := Fetch(tctx, srv.Client(), dir, srv.URL+"/slow"); err == nil {
		t.Error("slow fetch ignored its context")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("rejected downloads left files: %v", entries)
	}
}

func TestPruneRemovesStaleFiles(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.png")
	fresh := filepath.Join(dir, "fresh.png")
	tmp := filepath.Join(dir, ".thumb-123")
	for _, p := range []string{old, fresh, tmp} {
		os.WriteFile(p, png, 0o600)
	}
	past := time.Now().Add(-15 * 24 * time.Hour)
	os.Chtimes(old, past, past)
	hour := time.Now().Add(-2 * time.Hour)
	os.Chtimes(tmp, hour, hour)
	Prune(dir, MaxAge)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("15-day-old thumbnail kept")
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("abandoned temp file kept")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("fresh thumbnail removed")
	}
}

func TestDefaultDirFollowsXDG(t *testing.T) {
	if got, want := DefaultDir(), filepath.Join(os.Getenv("XDG_CACHE_HOME"), "ryoku", "ryoget", "thumbs"); got != want {
		t.Errorf("DefaultDir = %q, want %q", got, want)
	}
}
