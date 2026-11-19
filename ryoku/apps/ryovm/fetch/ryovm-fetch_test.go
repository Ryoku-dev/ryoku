package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestProbe(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantSize   int64
		wantRanged bool
	}{
		{
			name: "accept_ranges_bytes",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodHead {
					t.Errorf("expected HEAD, got %s", r.Method)
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				if ua := r.Header.Get("User-Agent"); ua != uaString {
					t.Errorf("expected User-Agent %q, got %q", uaString, ua)
					http.Error(w, "bad user agent", http.StatusBadRequest)
					return
				}
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Length", "4096")
				w.WriteHeader(http.StatusOK)
			},
			wantSize:   4096,
			wantRanged: true,
		},
		{
			name: "no_accept_ranges",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "2048")
				w.WriteHeader(http.StatusOK)
			},
			wantSize:   2048,
			wantRanged: false,
		},
		{
			name: "non_200_status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "not found", http.StatusNotFound)
			},
			wantSize:   0,
			wantRanged: false,
		},
		{
			name: "zero_content_length",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Length", "0")
				w.WriteHeader(http.StatusOK)
			},
			wantSize:   0,
			wantRanged: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(tc.handler)
			defer ts.Close()

			size, ranged := probe(context.Background(), ts.URL)
			if size != tc.wantSize {
				t.Errorf("probe size = %d, want %d", size, tc.wantSize)
			}
			if ranged != tc.wantRanged {
				t.Errorf("probe ranged = %v, want %v", ranged, tc.wantRanged)
			}
		})
	}

	t.Run("server_unreachable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		ts.Close()

		size, ranged := probe(context.Background(), ts.URL)
		if size != 0 || ranged {
			t.Errorf("probe unreachable = (%d, %v), want (0, false)", size, ranged)
		}
	})
}

func TestDownloadSingle(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		payload := make([]byte, 300<<10)
		for i := range payload {
			payload[i] = byte(i % 251)
		}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			if ua := r.Header.Get("User-Agent"); ua != uaString {
				t.Errorf("expected User-Agent %q, got %q", uaString, ua)
				http.Error(w, "bad user agent", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(payload); err != nil {
				t.Errorf("write payload: %v", err)
			}
		}))
		defer ts.Close()

		filePath := filepath.Join(t.TempDir(), "single.bin")
		out, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}

		var recv int64
		err = downloadSingle(context.Background(), ts.URL, out, &recv)
		closeErr := out.Close()

		if err != nil {
			t.Fatalf("downloadSingle returned unexpected error: %v", err)
		}
		if closeErr != nil {
			t.Fatalf("close file: %v", closeErr)
		}

		if gotRecv := atomic.LoadInt64(&recv); gotRecv != int64(len(payload)) {
			t.Errorf("recv = %d, want %d", gotRecv, len(payload))
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if !bytes.Equal(data, payload) {
			t.Errorf("downloaded content mismatch")
		}
	})

	t.Run("server_error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}))
		defer ts.Close()

		filePath := filepath.Join(t.TempDir(), "single_err.bin")
		out, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		defer out.Close()

		var recv int64
		err = downloadSingle(context.Background(), ts.URL, out, &recv)
		if err == nil {
			t.Fatal("expected error from downloadSingle, got nil")
		}
		wantMsg := "server replied 500 Internal Server Error"
		if err.Error() != wantMsg {
			t.Errorf("error = %q, want %q", err.Error(), wantMsg)
		}
	})
}

func TestFetchRange(t *testing.T) {
	t.Run("valid_range_offset", func(t *testing.T) {
		const start = int64(20)
		const end = int64(29)
		rangeChunk := []byte("ABCDEFGHIJ")

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
				http.Error(w, "bad method", http.StatusMethodNotAllowed)
				return
			}
			if ua := r.Header.Get("User-Agent"); ua != uaString {
				t.Errorf("expected User-Agent %q, got %q", uaString, ua)
				http.Error(w, "bad user agent", http.StatusBadRequest)
				return
			}
			wantRange := fmt.Sprintf("bytes=%d-%d", start, end)
			if gotRange := r.Header.Get("Range"); gotRange != wantRange {
				t.Errorf("Range header = %q, want %q", gotRange, wantRange)
				http.Error(w, "bad range", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/50", start, end))
			w.WriteHeader(http.StatusPartialContent)
			if _, err := w.Write(rangeChunk); err != nil {
				t.Errorf("write range chunk: %v", err)
			}
		}))
		defer ts.Close()

		filePath := filepath.Join(t.TempDir(), "range.bin")
		initial := bytes.Repeat([]byte{'.'}, 50)
		if err := os.WriteFile(filePath, initial, 0644); err != nil {
			t.Fatalf("write initial file: %v", err)
		}

		out, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("open file: %v", err)
		}

		var recv int64
		err = fetchRange(context.Background(), ts.URL, out, start, end, &recv)
		closeErr := out.Close()

		if err != nil {
			t.Fatalf("fetchRange unexpected error: %v", err)
		}
		if closeErr != nil {
			t.Fatalf("close file: %v", closeErr)
		}

		if gotRecv := atomic.LoadInt64(&recv); gotRecv != int64(len(rangeChunk)) {
			t.Errorf("recv = %d, want %d", gotRecv, len(rangeChunk))
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}

		expected := append([]byte{}, initial...)
		copy(expected[start:end+1], rangeChunk)
		if !bytes.Equal(data, expected) {
			t.Errorf("file content mismatch at offset: got %q, want %q", data, expected)
		}
	})

	t.Run("server_replies_200_not_206", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("not partial")); err != nil {
				t.Errorf("write response: %v", err)
			}
		}))
		defer ts.Close()

		filePath := filepath.Join(t.TempDir(), "range_200.bin")
		out, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		defer out.Close()

		var recv int64
		err = fetchRange(context.Background(), ts.URL, out, 0, 9, &recv)
		if err == nil {
			t.Fatal("expected error when server returns 200, got nil")
		}
		wantMsg := "range request rejected (200 OK)"
		if err.Error() != wantMsg {
			t.Errorf("error = %q, want %q", err.Error(), wantMsg)
		}
	})
}

func TestDownloadParallel(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		size := int64(2*(1<<20) + 11)
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = byte(i % 251)
		}

		modtime := time.Now()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ua := r.Header.Get("User-Agent"); ua != uaString {
				t.Errorf("unexpected User-Agent %q", ua)
				http.Error(w, "bad user agent", http.StatusBadRequest)
				return
			}
			http.ServeContent(w, r, "payload.bin", modtime, bytes.NewReader(payload))
		}))
		defer ts.Close()

		filePath := filepath.Join(t.TempDir(), "parallel.bin")
		out, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}

		var recv int64
		err = downloadParallel(context.Background(), ts.URL, out, size, &recv)
		closeErr := out.Close()

		if err != nil {
			t.Fatalf("downloadParallel unexpected error: %v", err)
		}
		if closeErr != nil {
			t.Fatalf("close file: %v", closeErr)
		}

		if gotRecv := atomic.LoadInt64(&recv); gotRecv != size {
			t.Errorf("recv = %d, want %d", gotRecv, size)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if !bytes.Equal(data, payload) {
			t.Errorf("downloaded content mismatch")
		}
	})

	t.Run("server_error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "server error", http.StatusInternalServerError)
		}))
		defer ts.Close()

		filePath := filepath.Join(t.TempDir(), "parallel_err.bin")
		out, err := os.Create(filePath)
		if err != nil {
			t.Fatalf("create temp file: %v", err)
		}
		defer out.Close()

		var recv int64
		size := int64(1024)
		err = downloadParallel(context.Background(), ts.URL, out, size, &recv)
		if err == nil {
			t.Fatal("expected error from downloadParallel on server error, got nil")
		}
	})
}
