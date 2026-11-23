package engine

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

// heldResolver hands each run's context and link to the test and holds the run until it is cancelled.
type heldResolver struct {
	runs chan heldRun
}

type heldRun struct {
	ctx  context.Context
	link string
}

func newHeld() heldResolver { return heldResolver{runs: make(chan heldRun, 4)} }

func (r heldResolver) Resolve(ctx context.Context, link string) (*media.Media, error) {
	r.runs <- heldRun{ctx, link}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (heldResolver) Plan(*media.Media, media.Options) (*plan.Plan, error) {
	return nil, errors.New("unused")
}

func (r heldResolver) next(t *testing.T) heldRun {
	t.Helper()
	select {
	case run := <-r.runs:
		return run
	case <-time.After(10 * time.Second):
		t.Fatal("no run reached the resolver")
	}
	return heldRun{}
}

func TestRunCarriesTaskLimitAndSetLimitChangesItLive(t *testing.T) {
	held := newHeld()
	f := newFixture(t, nil, &Parts{Resolver: held})
	tk, err := f.e.Add(proto.AddParams{URL: "http://example.invalid/big.iso"})
	if err != nil {
		t.Fatal(err)
	}
	run := held.next(t)
	// a finite limit refuses any single wait above its burst at once, so no clock is involved
	over := limitBurst + 1
	if err := plan.Wait(run.ctx, nil, over); err != nil {
		t.Fatalf("an unlimited run was held back: %v", err)
	}
	got, err := f.e.SetLimit(tk.ID, 64)
	if err != nil || got.Limit != 64 {
		t.Fatalf("SetLimit = %+v, %v", got, err)
	}
	if err := plan.Wait(run.ctx, nil, over); err == nil || run.ctx.Err() != nil {
		t.Fatalf("the running download did not take its new limit: %v", err)
	}
	if _, err := f.e.SetLimit(tk.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := plan.Wait(run.ctx, nil, over); err != nil {
		t.Fatalf("lifting the limit left the run held back: %v", err)
	}
	if _, err := f.e.SetLimit(tk.ID, -1); err == nil {
		t.Fatal("a negative limit was taken")
	}
	if _, err := f.e.SetLimit("gone", 1); err == nil {
		t.Fatal("a limit for no download was taken")
	}

	// a limit set while paused reaches the next run, and survives a restart
	f.e.Pause(tk.ID, false)
	waitTask(t, f.e, tk.ID, inState(proto.StatePaused))
	if _, err := f.e.SetLimit(tk.ID, 8); err != nil {
		t.Fatal(err)
	}
	f.e.Close()
	f.start(t, &Parts{Resolver: held})
	if got, _ := f.e.Task(tk.ID); got.Limit != 8 {
		t.Fatalf("limit after a restart = %d", got.Limit)
	}
	f.e.Resume(tk.ID, false)
	run = held.next(t)
	if err := plan.Wait(run.ctx, nil, over); err == nil {
		t.Fatal("the next run ignored the stored limit")
	}
	// the shared limit still applies on top of the task's own
	shared := &countLimiter{}
	if err := plan.Wait(run.ctx, shared, 1); err != nil || shared.n != 1 {
		t.Fatalf("shared limiter took %d (%v)", shared.n, err)
	}
}

type countLimiter struct{ n int }

func (c *countLimiter) WaitN(_ context.Context, n int) error { c.n += n; return nil }

func TestRefreshRequeuesWithNewLinkAndKeepsPartial(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dl")
	os.MkdirAll(dir, 0o755)
	part := filepath.Join(dir, "a.bin.part")
	os.WriteFile(part, []byte("half"), 0o644)
	f := &fixture{dir: dir, state: filepath.Join(root, "state", "tasks.json"), conf: filepath.Join(root, "conf", "downloads.json"), ev: &events{}}
	b, _ := json.Marshal(map[string]any{"dir": dir})
	os.MkdirAll(filepath.Dir(f.conf), 0o700)
	os.WriteFile(f.conf, b, 0o600)
	b, _ = json.Marshal(stateFile{Version: stateFormat, Tasks: []record{
		{Task: proto.Task{ID: "failed", URL: "https://cdn.example/a.bin?sig=old", Kind: "file", State: proto.StateError, Dir: dir, Name: "a.bin", Path: filepath.Join(dir, "a.bin"),
			Done: 4, Size: 8, Finished: now(), Error: &proto.Error{Code: proto.CodeUnavailable}}, Known: true, FetchURL: "https://edge.example/a.bin?sig=old"},
		{Task: proto.Task{ID: "done", URL: "https://x.example/d.bin", Kind: "file", State: proto.StateDone, Dir: dir}, Known: true},
		{Task: proto.Task{ID: "tor", URL: "magnet:?xt=urn:btih:abc", Kind: "torrent", State: proto.StatePaused, Dir: dir}, Known: true},
	}})
	os.MkdirAll(filepath.Dir(f.state), 0o700)
	os.WriteFile(f.state, b, 0o600)
	held := newHeld()
	f.start(t, &Parts{Resolver: held})

	fresh := "https://cdn.example/a.bin?sig=new"
	got, err := f.e.Refresh("failed", fresh)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != fresh || got.Error != nil || got.Finished != "" {
		t.Fatalf("refreshed task = %+v", got)
	}
	run := held.next(t)
	if run.link != fresh {
		t.Fatalf("the run resolved %q, want the new link", run.link)
	}
	if _, err := os.Stat(part); err != nil {
		t.Fatalf("refresh lost the partial data: %v", err)
	}
	f.e.mu.Lock()
	rec := f.e.tasks["failed"].record
	f.e.mu.Unlock()
	if rec.FetchURL != "" || rec.Name != "a.bin" || rec.Done != 4 {
		t.Fatalf("record after refresh = %+v", rec)
	}

	var pe *proto.Error
	if _, err := f.e.Refresh("failed", "https://cdn.example/a.bin?sig=newer"); !errors.As(err, &pe) || pe.Code != proto.CodeInvalid {
		t.Fatalf("refreshing a running download: %v", err)
	}
	for _, c := range []struct{ id, link string }{
		{"done", "https://x.example/d2.bin"},
		{"tor", "https://x.example/t.bin"},
		{"failed", "ftp://nope"},
	} {
		if _, err := f.e.Refresh(c.id, c.link); !errors.As(err, &pe) || pe.Code != proto.CodeInvalid {
			t.Fatalf("refresh %s to %s: %v", c.id, c.link, err)
		}
	}
	if _, err := f.e.Refresh("gone", fresh); !errors.As(err, &pe) || pe.Code != proto.CodeNotFound {
		t.Fatalf("refresh of no download: %v", err)
	}
}

func serveBytes(data []byte, seen func(*http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			seen(r)
		}
		w.Header().Set("ETag", `"1"`)
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	}))
}

func TestChecksumVerifiesOrDeletes(t *testing.T) {
	data := randBytes(256 << 10)
	srv := serveBytes(data, nil)
	defer srv.Close()
	f := newFixture(t, nil, nil)
	sum := sha256.Sum256(data)

	ok, err := f.e.Add(proto.AddParams{URL: srv.URL + "/good.bin", Checksum: "SHA256:" + strings.ToUpper(hex.EncodeToString(sum[:]))})
	if err != nil {
		t.Fatal(err)
	}
	done := waitTask(t, f.e, ok.ID, inState(proto.StateDone, proto.StateError))
	if done.State != proto.StateDone || done.Verified != "sha256" {
		t.Fatalf("matching checksum: %+v", done)
	}

	wrong := md5.Sum([]byte("something else"))
	bad, err := f.e.Add(proto.AddParams{URL: srv.URL + "/bad.bin#md5=" + hex.EncodeToString(wrong[:])})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bad.URL, "#") {
		t.Fatalf("the checksum fragment stayed on the link: %s", bad.URL)
	}
	failed := waitTask(t, f.e, bad.ID, inState(proto.StateDone, proto.StateError))
	if failed.State != proto.StateError || failed.Error == nil || failed.Error.Code != proto.CodeChecksum || failed.Verified != "" {
		t.Fatalf("wrong checksum: %+v", failed)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "bad.bin")); !os.IsNotExist(err) {
		t.Fatalf("a file that failed its checksum stayed: %v", err)
	}
	if wire, _ := json.Marshal(failed); strings.Contains(string(wire), hex.EncodeToString(wrong[:])) {
		t.Fatal("the checksum went out on the wire")
	}

	var pe *proto.Error
	if _, err := f.e.Add(proto.AddParams{URL: srv.URL + "/x.bin", Checksum: "sha256:abc"}); !errors.As(err, &pe) || pe.Code != proto.CodeInvalid {
		t.Fatalf("a malformed checksum: %v", err)
	}
}

func TestHeadersReachServerAndAreForgotten(t *testing.T) {
	data := randBytes(64 << 10)
	var mu sync.Mutex
	var reqs []http.Header
	srv := serveBytes(data, func(r *http.Request) {
		mu.Lock()
		reqs = append(reqs, r.Header.Clone())
		mu.Unlock()
	})
	defer srv.Close()
	f := newFixture(t, nil, nil)
	tk, err := f.e.Add(proto.AddParams{URL: srv.URL + "/gated.bin", Headers: map[string]string{
		"referer":    "https://site.example/page",
		"Cookie":     "session=s3cret",
		"X-Extra":    "dropped",
		"User-Agent": "bad\r\nInjected: 1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	done := waitTask(t, f.e, tk.ID, inState(proto.StateDone, proto.StateError))
	if done.State != proto.StateDone {
		t.Fatalf("download with headers: %+v", done.Error)
	}
	mu.Lock()
	if len(reqs) == 0 {
		t.Fatal("no request reached the server")
	}
	for _, h := range reqs {
		if h.Get("Referer") != "https://site.example/page" || h.Get("Cookie") != "session=s3cret" || h.Get("X-Extra") != "" || h.Get("Injected") != "" {
			t.Fatalf("request headers = %v", h)
		}
	}
	mu.Unlock()
	f.e.Close()
	b, _ := os.ReadFile(f.state)
	if strings.Contains(string(b), "s3cret") || strings.Contains(string(b), `"headers"`) {
		t.Fatalf("tasks.json kept the headers: %s", b)
	}
}

func TestExistingNamesTheDownloadForALink(t *testing.T) {
	data := randBytes(16 << 10)
	srv := serveBytes(data, nil)
	defer srv.Close()
	f := newFixture(t, nil, nil)
	link := srv.URL + "/have.bin"
	tk, _ := f.e.Add(proto.AddParams{URL: link})
	done := waitTask(t, f.e, tk.ID, inState(proto.StateDone))
	held, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/later.bin", Paused: true})

	ex := f.e.Existing(link + "#sha256=" + strings.Repeat("0", 64))
	if ex == nil || ex.ID != tk.ID || ex.State != proto.StateDone || ex.Path != done.Path {
		t.Fatalf("existing for a finished link = %+v", ex)
	}
	if ex := f.e.Existing(srv.URL + "/later.bin"); ex == nil || ex.ID != held.ID || ex.State != proto.StatePaused || ex.Path != "" {
		t.Fatalf("existing for a paused link = %+v", ex)
	}
	os.Remove(done.Path)
	if ex := f.e.Existing(link); ex == nil || ex.Path != "" {
		t.Fatalf("existing named a deleted file: %+v", ex)
	}
	if ex := f.e.Existing(srv.URL + "/new.bin"); ex != nil {
		t.Fatalf("a new link has an existing download: %+v", ex)
	}
}

func TestSortByTypeFilesByExtension(t *testing.T) {
	f := newFixture(t, map[string]any{"sortByType": true}, nil)
	add := func(p proto.AddParams) proto.Task {
		t.Helper()
		p.Paused = true
		tk, err := f.e.Add(p)
		if err != nil {
			t.Fatal(err)
		}
		return tk
	}
	if tk := add(proto.AddParams{URL: "https://x.example/files/Pack.ZIP?dl=1"}); tk.Dir != filepath.Join(f.dir, typeFolder(typeArchives)) {
		t.Fatalf(".zip landed in %s", tk.Dir)
	}
	if tk := add(proto.AddParams{URL: "https://x.example/files/thing.qqq"}); tk.Dir != f.dir {
		t.Fatalf("an unknown extension landed in %s", tk.Dir)
	}
	if tk := add(proto.AddParams{URL: "https://x.example/watch?v=1", Mode: "audio"}); tk.Dir != filepath.Join(f.dir, typeFolder(typeMusic)) {
		t.Fatalf("an audio download landed in %s", tk.Dir)
	}
	own := filepath.Join(t.TempDir(), "mine")
	if tk := add(proto.AddParams{URL: "https://x.example/a.zip", Dir: own}); tk.Dir != own {
		t.Fatalf("an explicit folder was overridden: %s", tk.Dir)
	}
	if tk := add(proto.AddParams{URL: "magnet:?xt=urn:btih:abc"}); tk.Dir != f.dir {
		t.Fatalf("a torrent landed in %s", tk.Dir)
	}

	// resolving a page into media refiles it, torrents go back to the base folder
	f.e.mu.Lock()
	defer f.e.mu.Unlock()
	page := &task{record: record{Task: proto.Task{ID: "p", URL: "https://x.example/watch?v=2", Kind: "media", Mode: "auto", Dir: f.dir, Name: "clip.mp4"}, Sort: f.dir}}
	f.e.placeByKind(page)
	if want := filepath.Join(f.dir, typeFolder(typeVideo)); page.Dir != want || page.Path != filepath.Join(want, "clip.mp4") {
		t.Fatalf("media refiled to %s (%s)", page.Dir, page.Path)
	}
	page.Kind = "torrent"
	f.e.placeByKind(page)
	if page.Dir != f.dir {
		t.Fatalf("a torrent refiled to %s", page.Dir)
	}
	off := &task{record: record{Task: proto.Task{ID: "o", URL: "https://x.example/a.zip", Kind: "media", Dir: "/keep"}}}
	f.e.placeByKind(off)
	if off.Dir != "/keep" {
		t.Fatalf("an unsorted task moved to %s", off.Dir)
	}
}

func TestSortByTypeIsOffByDefault(t *testing.T) {
	f := newFixture(t, nil, nil)
	tk, err := f.e.Add(proto.AddParams{URL: "https://x.example/a.zip", Paused: true})
	if err != nil {
		t.Fatal(err)
	}
	if tk.Dir != f.dir {
		t.Fatalf("a .zip left the download folder by default: %s", tk.Dir)
	}
}
