// Package thumbs keeps small remote images (search thumbnails, task covers) in
// a disk cache so QML can show them from a local path.
package thumbs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	MaxBytes = 2 << 20
	MaxAge   = 14 * 24 * time.Hour
	// pruning walks the whole directory, so it runs at most this often per dir
	pruneEvery = time.Hour
)

// UserAgent names the program to image hosts.
var UserAgent = "ryoget (Ryoku desktop; +https://ryoku.dev)"

var (
	ErrNotImage = errors.New("thumbs: not an image")
	ErrTooLarge = errors.New("thumbs: image too large")
	ErrBadURL   = errors.New("thumbs: unsupported url")
)

var exts = map[string]string{
	"image/jpeg":               ".jpg",
	"image/png":                ".png",
	"image/webp":               ".webp",
	"image/gif":                ".gif",
	"image/avif":               ".avif",
	"image/svg+xml":            ".svg",
	"image/bmp":                ".bmp",
	"image/x-icon":             ".ico",
	"image/vnd.microsoft.icon": ".ico",
}

var (
	mu       sync.Mutex
	inflight = map[string]*call{}
	pruned   = map[string]time.Time{}
)

type call struct {
	done chan struct{}
	path string
	err  error
}

// DefaultDir is the shared cache: $XDG_CACHE_HOME/ryoku/ryoget/thumbs.
func DefaultDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(base) {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "ryoku", "ryoget", "thumbs")
}

// Fetch returns a local file holding the image at rawURL, downloading it once.
// Concurrent calls for the same URL share one download.
func Fetch(ctx context.Context, client *http.Client, cacheDir, rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", ErrBadURL
	}
	if client == nil {
		client = http.DefaultClient
	}
	sum := sha256.Sum256([]byte(rawURL))
	base := filepath.Join(cacheDir, hex.EncodeToString(sum[:]))

	maybePrune(cacheDir)
	if p := cached(base); p != "" {
		return p, nil
	}

	mu.Lock()
	if c, ok := inflight[base]; ok {
		mu.Unlock()
		select {
		case <-c.done:
			return c.path, c.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	c := &call{done: make(chan struct{})}
	inflight[base] = c
	mu.Unlock()

	c.path, c.err = download(ctx, client, cacheDir, base, rawURL)
	mu.Lock()
	delete(inflight, base)
	mu.Unlock()
	close(c.done)
	return c.path, c.err
}

func cached(base string) string {
	matches, _ := filepath.Glob(base + ".*")
	for _, m := range matches {
		if strings.HasPrefix(filepath.Base(m), ".") {
			continue
		}
		now := time.Now()
		// reuse keeps a thumbnail alive; pruning counts from last use
		_ = os.Chtimes(m, now, now)
		return m
	}
	return ""
}

func download(ctx context.Context, client *http.Client, dir, base, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", ErrBadURL
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/*;q=0.8")
	// image CDNs (Wikimedia above all) refuse the anonymous Go user agent
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("thumbs: http %d", resp.StatusCode)
	}
	if resp.ContentLength > MaxBytes {
		return "", ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxBytes {
		return "", ErrTooLarge
	}
	ext := extFor(resp.Header.Get("Content-Type"), data)
	if ext == "" {
		return "", ErrNotImage
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".thumb-*")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	final := base + ext
	if err := os.Rename(tmp.Name(), final); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return final, nil
}

func extFor(header string, data []byte) string {
	mt, _, _ := mime.ParseMediaType(header)
	mt = strings.ToLower(mt)
	if mt == "" || mt == "application/octet-stream" || mt == "binary/octet-stream" {
		mt, _, _ = mime.ParseMediaType(http.DetectContentType(data))
	}
	if !strings.HasPrefix(mt, "image/") {
		return ""
	}
	if ext, ok := exts[mt]; ok {
		return ext
	}
	return ".img"
}

func maybePrune(dir string) {
	mu.Lock()
	if time.Since(pruned[dir]) < pruneEvery {
		mu.Unlock()
		return
	}
	pruned[dir] = time.Now()
	mu.Unlock()
	Prune(dir, MaxAge)
}

// Prune deletes cached files unused for longer than maxAge, and stale temp files.
func Prune(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		limit := cutoff
		if strings.HasPrefix(e.Name(), ".thumb-") {
			limit = time.Now().Add(-time.Hour)
		}
		if info.ModTime().Before(limit) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
