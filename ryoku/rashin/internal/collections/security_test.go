package collections

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/safehttp"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPublicSourceClientRejectsLoopback(t *testing.T) {
	client := safehttp.NewClient(time.Second)
	defer client.CloseIdleConnections()
	for _, address := range []string{"http://127.0.0.1/private", "http://[::1]/private", "http://169.254.169.254/latest/meta-data/", "http://[::ffff:127.0.0.1]/private"} {
		response, err := client.Get(address)
		if response != nil {
			response.Body.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "Private source") {
			t.Fatalf("accepted %s: %v", address, err)
		}
	}
}

func TestPublicSourceRedirectCannotReachLoopback(t *testing.T) {
	client := safehttp.NewClient(time.Second)
	defer client.CloseIdleConnections()
	requests := 0
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"http://127.0.0.1/private"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	response, err := client.Get("https://8.8.8.8/public")
	if response != nil {
		response.Body.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "Private source") || requests != 1 {
		t.Fatalf("redirect escaped guard: requests=%d error=%v", requests, err)
	}
}

func TestCollectionCheckUsesPublicSourceGuard(t *testing.T) {
	s := &Store{Dir: filepath.Join(t.TempDir(), "collections")}
	item, err := s.Add(protocol.CollectionAddParams{Kind: "url", Text: "private", URL: "http://127.0.0.1/private"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Check(context.Background(), Default, item.ID); err == nil || !strings.Contains(err.Error(), "Private source") {
		t.Fatalf("unguarded check: %v", err)
	}
}

func TestSpecialSourcesNeverBlockCollections(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	s := &Store{Dir: filepath.Join(dir, "collections")}
	finished := make(chan error, 1)
	go func() { _, err := s.Add(protocol.CollectionAddParams{Kind: "file", Path: fifo}); finished <- err }()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("accepted FIFO")
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO blocked collection add")
	}
	item, err := s.Add(protocol.CollectionAddParams{Kind: "excerpt", Text: "saved", Source: fifo})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, err := s.Check(context.Background(), Default, item.ID); finished <- err }()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("accepted FIFO check")
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO blocked collection check")
	}
	regular := filepath.Join(dir, "regular")
	if err = os.WriteFile(regular, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err = os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Add(protocol.CollectionAddParams{Kind: "file", Path: link}); err == nil {
		t.Fatal("accepted symlink source")
	}
	if _, err = s.List(); err != nil {
		t.Fatal(err)
	}
}

func TestItemPagesFitWireAndGetPreservesFullText(t *testing.T) {
	text := strings.Repeat("x", 1<<20)
	items := make([]protocol.CollectionItem, 45)
	for i := range items {
		items[i] = protocol.CollectionItem{ID: fmt.Sprint(i), Collection: Default, Kind: "answer", Title: "Saved answer", Text: text}
	}
	cursor, seen := "", 0
	for {
		page, err := Page(items, protocol.CollectionItemsParams{Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(page)
		if err != nil || len(encoded) >= protocol.MaxLine {
			t.Fatalf("page exceeds wire limit: %d %v", len(encoded), err)
		}
		for _, item := range page.Items {
			if len(item.Text) > 2048 || !item.Truncated {
				t.Fatal("list contains full item text")
			}
		}
		seen += len(page.Items)
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}
	if seen != len(items) {
		t.Fatalf("only paged %d items", seen)
	}
	full, err := Page(items, protocol.CollectionItemsParams{ID: "16"})
	if err != nil || full.Item == nil || full.Item.Text != text {
		t.Fatalf("full item missing: %v", err)
	}
	if _, err = Page(items, protocol.CollectionItemsParams{Cursor: "-1"}); err == nil {
		t.Fatal("accepted invalid cursor")
	}
}
