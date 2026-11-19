package localmodels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchResumesAndChecksChecksum(t *testing.T) {
	data := []byte(strings.Repeat("a", 8192) + strings.Repeat("b", 8192))
	sum := sha256.Sum256(data)
	const have = 8192
	var ranged bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranged = r.Header.Get("Range") == fmt.Sprintf("bytes=%d-", have)
		if !ranged {
			http.Error(w, "range missing", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", have, len(data)-1, len(data)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data[have:])
	}))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(file+partSuffix, data[:have], 0o600); err != nil {
		t.Fatal(err)
	}
	fetched, err := fetch(context.Background(), server.Client(), fetchRequest{URL: server.URL, Dest: file, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}, func(string, int64) {})
	if err != nil || !fetched || !ranged {
		t.Fatalf("resume: fetched=%v ranged=%v err=%v", fetched, ranged, err)
	}
	stored, err := os.ReadFile(file)
	if err != nil || string(stored) != string(data) {
		t.Fatalf("verified file differs: %v", err)
	}
	if _, err := os.Stat(file + partSuffix); !os.IsNotExist(err) {
		t.Fatalf("part file remained: %v", err)
	}
}

func TestFetchRestartsWhenServerIgnoresRange(t *testing.T) {
	data := []byte("the full immutable model")
	sum := sha256.Sum256(data)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(file+partSuffix, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fetch(context.Background(), server.Client(), fetchRequest{URL: server.URL, Dest: file, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}, func(string, int64) {}); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(file)
	if err != nil || string(stored) != string(data) {
		t.Fatalf("restart did not replace part: %s, %v", stored, err)
	}
}

func TestFetchCancellationKeepsPart(t *testing.T) {
	data := []byte(strings.Repeat("z", 2<<20))
	sum := sha256.Sum256(data)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	file := filepath.Join(t.TempDir(), "model.gguf")
	_, err := fetch(ctx, server.Client(), fetchRequest{URL: server.URL, Dest: file, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])}, func(_ string, n int64) {
		if n > 0 {
			cancel()
		}
	})
	if err == nil {
		t.Fatal("cancelled fetch succeeded")
	}
	if st, err := os.Stat(file + partSuffix); err != nil || st.Size() == 0 {
		t.Fatalf("cancelled bytes lost: %v", err)
	}
}
