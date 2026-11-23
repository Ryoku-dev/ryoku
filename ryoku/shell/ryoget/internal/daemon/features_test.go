package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ryoget/internal/engine"
	"ryoget/internal/media"
	"ryoget/internal/notify"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

func fileServer() *httptest.Server {
	data := bytes.Repeat([]byte("ryoget"), 4096)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"1"`)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
}

func TestLimitAndRefreshVerbs(t *testing.T) {
	srv := fileServer()
	defer srv.Close()
	r := startDaemon(t, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := dial(t, r)

	var task proto.Task
	if err := c.Call(ctx, "add", proto.AddParams{URL: srv.URL + "/a.bin", Paused: true}, &task); err != nil {
		t.Fatal(err)
	}
	var pe *proto.Error
	for _, bad := range []struct {
		params any
		code   string
	}{
		{map[string]any{"id": task.ID}, proto.CodeInvalid},
		{map[string]any{"id": task.ID, "kib": -5}, proto.CodeInvalid},
		{map[string]any{"id": task.ID, "kib": "fast"}, proto.CodeInvalid},
		{map[string]any{"id": "nope", "kib": 1}, proto.CodeNotFound},
	} {
		if err := c.Call(ctx, "limit", bad.params, nil); !errors.As(err, &pe) || pe.Code != bad.code {
			t.Fatalf("limit %v: %v, want %s", bad.params, err, bad.code)
		}
	}
	var limited proto.Task
	if err := c.Call(ctx, "limit", map[string]any{"id": task.ID, "kib": 128}, &limited); err != nil || limited.Limit != 128 {
		t.Fatalf("limit = %+v, %v", limited, err)
	}

	for _, bad := range []struct {
		params any
		code   string
	}{
		{map[string]any{"id": task.ID, "url": "not a link"}, proto.CodeInvalid},
		{map[string]any{"id": "nope", "url": srv.URL + "/b.bin"}, proto.CodeNotFound},
	} {
		if err := c.Call(ctx, "refresh", bad.params, nil); !errors.As(err, &pe) || pe.Code != bad.code {
			t.Fatalf("refresh %v: %v, want %s", bad.params, err, bad.code)
		}
	}
	var refreshed proto.Task
	if err := c.Call(ctx, "refresh", map[string]any{"id": task.ID, "url": srv.URL + "/b.bin"}, &refreshed); err != nil {
		t.Fatal(err)
	}
	if refreshed.URL != srv.URL+"/b.bin" || refreshed.State == proto.StatePaused || refreshed.Limit != 128 {
		t.Fatalf("refresh = %+v", refreshed)
	}
}

func TestResolveReportsExistingDownload(t *testing.T) {
	srv := fileServer()
	defer srv.Close()
	r := startDaemon(t, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := dial(t, r)
	var task proto.Task
	if err := c.Call(ctx, "add", proto.AddParams{URL: srv.URL + "/have.bin", Paused: true}, &task); err != nil {
		t.Fatal(err)
	}
	var res proto.Resolved
	if err := c.Call(ctx, "resolve", map[string]any{"url": srv.URL + "/have.bin"}, &res); err != nil {
		t.Fatal(err)
	}
	if res.Existing == nil || res.Existing.ID != task.ID || res.Existing.State != proto.StatePaused {
		t.Fatalf("resolve existing = %+v", res.Existing)
	}
	var fresh json.RawMessage
	if err := c.Call(ctx, "resolve", map[string]any{"url": srv.URL + "/new.bin"}, &fresh); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(fresh, []byte(`"existing"`)) {
		t.Fatalf("a new link came back with an existing download: %s", fresh)
	}
}

// heldResolver reports each run and holds it until cancelled.
type heldResolver struct{ runs chan string }

func (h heldResolver) Resolve(ctx context.Context, link string) (*media.Media, error) {
	h.runs <- link
	<-ctx.Done()
	return nil, ctx.Err()
}

func (heldResolver) Plan(*media.Media, media.Options) (*plan.Plan, error) {
	return nil, errors.New("unused")
}

func TestFailureNoticeOffersRetryAndTheActionRetries(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "tasks.json")
	conf := filepath.Join(root, "downloads.json")
	b, _ := json.Marshal(map[string]any{"dir": filepath.Join(root, "dl")})
	os.WriteFile(conf, b, 0o600)
	b, _ = json.Marshal(map[string]any{"version": 1, "tasks": []proto.Task{{
		ID: "failed", URL: "https://x.example/a.bin", Kind: "file", State: proto.StateError, Title: "a.bin", Dir: filepath.Join(root, "dl"),
		Error: &proto.Error{Code: proto.CodeNetwork, Message: "The connection dropped."},
	}}})
	os.WriteFile(state, b, 0o600)

	held := heldResolver{runs: make(chan string, 1)}
	eng := engine.New(engine.Options{StateFile: state, ConfigFile: conf})
	eng.Use(engine.Parts{Resolver: held})
	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(eng.Close)
	s := &Server{eng: eng, log: log.New(io.Discard, "", 0), poke: make(chan struct{}, 1), conns: map[*conn]struct{}{}}

	failed, _ := eng.Task("failed")
	n := failNotice(failed)
	if n.Retry != "failed" || !slices.Contains(notify.Actions(n), notify.ActRetry) {
		t.Fatalf("failure notice = %+v offers %v", n, notify.Actions(n))
	}
	s.onAction(n, notify.ActRetry)
	select {
	case link := <-held.runs:
		if link != "https://x.example/a.bin" {
			t.Fatalf("retry ran %s", link)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("pressing Try again did not run the download")
	}
	if tk, _ := eng.Task("failed"); tk.State != proto.StateResolving || tk.Error != nil {
		t.Fatalf("after Try again: %+v", tk)
	}
	select {
	case <-s.poke:
	default:
		t.Fatal("the idle loop was not told the daemon has work again")
	}
}
