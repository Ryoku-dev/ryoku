package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"ryoget/internal/media"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryoget-engine-test-")
	if err != nil {
		panic(err)
	}
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "XDG_DOWNLOAD_DIR"} {
		os.Setenv(k, filepath.Join(root, strings.ToLower(k)))
	}
	os.Setenv("HOME", root)
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

// gated serves files whose bodies wait for the gate; probes pass at once.
type gated struct {
	mu    sync.Mutex
	files map[string][]byte
	gate  chan struct{}
	gets  atomic.Int32
}

func newGated() *gated { return &gated{files: map[string][]byte{}, gate: make(chan struct{})} }

func (g *gated) open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	select {
	case <-g.gate:
	default:
		close(g.gate)
	}
}

func (g *gated) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	data, ok := g.files[r.URL.Path]
	gate := g.gate
	g.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet && r.Header.Get("Range") != "bytes=0-0" {
		g.gets.Add(1)
		select {
		case <-gate:
		case <-r.Context().Done():
			return
		}
	}
	w.Header().Set("ETag", `"`+r.URL.Path+`"`)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

type events struct {
	mu   sync.Mutex
	list []struct {
		name string
		data any
	}
}

func (ev *events) on(name string, data any) {
	ev.mu.Lock()
	ev.list = append(ev.list, struct {
		name string
		data any
	}{name, data})
	ev.mu.Unlock()
}

func (ev *events) count(name string) int {
	ev.mu.Lock()
	defer ev.mu.Unlock()
	n := 0
	for _, e := range ev.list {
		if e.name == name {
			n++
		}
	}
	return n
}

type fixture struct {
	e     *Engine
	dir   string
	state string
	conf  string
	ev    *events
}

func newFixture(t *testing.T, settings map[string]any, parts *Parts) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{dir: filepath.Join(root, "dl"), state: filepath.Join(root, "state", "tasks.json"), conf: filepath.Join(root, "conf", "downloads.json"), ev: &events{}}
	if settings == nil {
		settings = map[string]any{}
	}
	settings["dir"] = f.dir
	b, _ := json.Marshal(settings)
	os.MkdirAll(filepath.Dir(f.conf), 0o700)
	os.WriteFile(f.conf, b, 0o600)
	f.start(t, parts)
	return f
}

func (f *fixture) start(t *testing.T, parts *Parts) {
	t.Helper()
	f.e = New(Options{StateFile: f.state, ConfigFile: f.conf, OnEvent: f.ev.on})
	if parts != nil {
		f.e.Use(*parts)
	}
	if err := f.e.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.e.Close)
}

func waitTask(t *testing.T, e *Engine, id string, ok func(proto.Task) bool) proto.Task {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		tk, err := e.Task(id)
		if err == nil && ok(tk) {
			return tk
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s stuck: %+v (err %v)", id, tk, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func inState(states ...string) func(proto.Task) bool {
	return func(tk proto.Task) bool {
		for _, s := range states {
			if tk.State == s {
				return true
			}
		}
		return false
	}
}

func TestSettingsDefaultsClampAndPatch(t *testing.T) {
	d := DefaultSettings()
	if d.MaxActive != 3 || d.Connections != 8 || d.MinSplitMiB != 1 || d.VideoQuality != "best" || d.AudioFormat != "best" ||
		d.SeedRatio != 1 || d.SeedMinutes != 30 || !d.Seed || !d.DHT || d.Port != 0 || !d.Notify || !d.ResumeOnStart || !d.YtdlpFallback {
		t.Fatalf("defaults = %+v", d)
	}
	if d.Dir != filepath.Join(os.Getenv("XDG_DOWNLOAD_DIR")) {
		t.Fatalf("dir = %q", d.Dir)
	}
	s, err := ApplyPatch(d, json.RawMessage(`{"maxActive":99,"connections":0,"videoQuality":"8k","port":80,"dir":"relative"}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.MaxActive != 16 || s.Connections != 1 || s.VideoQuality != "best" || s.Port != 1024 || s.Dir != d.Dir {
		t.Fatalf("clamped = %+v", s)
	}
	if _, err := ApplyPatch(d, json.RawMessage(`{"nope":1}`)); err == nil {
		t.Fatal("unknown key accepted")
	}
}

func TestDownloadAndPersist(t *testing.T) {
	g := newGated()
	data := randBytes(3 << 20)
	g.files["/a.bin"] = data
	g.open()
	srv := httptest.NewServer(g)
	defer srv.Close()
	f := newFixture(t, nil, nil)
	tk, err := f.e.Add(proto.AddParams{URL: srv.URL + "/a.bin"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitTask(t, f.e, tk.ID, inState(proto.StateDone))
	if got, _ := os.ReadFile(done.Path); !bytes.Equal(got, data) {
		t.Fatal("content differs")
	}
	if done.Name != "a.bin" || done.Size != int64(len(data)) || done.Done != done.Size || done.Finished == "" {
		t.Fatalf("done task = %+v", done)
	}
	if p, _ := base64.StdEncoding.DecodeString(done.Pieces); len(p) != 64 || p[0] != 255 || p[63] != 255 {
		t.Fatalf("pieces = %v", p)
	}
	if f.ev.count("task") == 0 {
		t.Fatal("no task events")
	}
	// the tick that finds nothing running sends the closing stats
	waitFor(t, func() bool { return f.ev.count("stats") > 0 })
	f.e.Close()
	b, err := os.ReadFile(f.state)
	if err != nil || !strings.Contains(string(b), tk.ID) {
		t.Fatalf("tasks.json = %s (%v)", b, err)
	}
}

func TestMaxActiveQueues(t *testing.T) {
	g := newGated()
	srv := httptest.NewServer(g)
	defer srv.Close()
	defer g.open()
	for _, n := range []string{"/1", "/2", "/3"} {
		g.files[n] = randBytes(64 << 10)
	}
	f := newFixture(t, map[string]any{"maxActive": 1}, nil)
	var ids []string
	for _, n := range []string{"/1", "/2", "/3"} {
		tk, err := f.e.Add(proto.AddParams{URL: srv.URL + n})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, tk.ID)
	}
	waitTask(t, f.e, ids[0], inState(proto.StateActive))
	for _, id := range ids[1:] {
		if tk, _ := f.e.Task(id); tk.State != proto.StateQueued {
			t.Fatalf("%s state %s, want queued", id, tk.State)
		}
	}
	if st := f.e.Stats(); st.Active != 1 || st.Queued != 2 || len(st.History) != 60 {
		t.Fatalf("stats = %+v", st)
	}
	if err := f.e.Move(ids[2], 0); err != nil {
		t.Fatal(err)
	}
	if tasks := f.e.Tasks(); tasks[1].ID != ids[2] || tasks[2].ID != ids[1] {
		t.Fatalf("move did not reorder: %s %s", tasks[1].ID, tasks[2].ID)
	}
	if _, err := f.e.SetSettings(json.RawMessage(`{"maxActive":3}`)); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		waitTask(t, f.e, id, inState(proto.StateActive))
	}
	g.open()
	for _, id := range ids {
		waitTask(t, f.e, id, inState(proto.StateDone))
	}
	if f.ev.count("settings") != 1 {
		t.Fatal("no settings event")
	}
}

func TestPauseResumeCancelRemove(t *testing.T) {
	g := newGated()
	srv := httptest.NewServer(g)
	defer srv.Close()
	defer g.open()
	data := randBytes(512 << 10)
	g.files["/p.bin"] = data
	g.files["/c.bin"] = data
	g.files["/r.bin"] = data
	f := newFixture(t, nil, nil)

	p, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/p.bin"})
	waitTask(t, f.e, p.ID, inState(proto.StateActive))
	if err := f.e.Pause(p.ID, false); err != nil {
		t.Fatal(err)
	}
	paused := waitTask(t, f.e, p.ID, inState(proto.StatePaused))
	if _, err := os.Stat(filepath.Join(f.dir, "p.bin.part")); err != nil {
		t.Fatal("pause dropped the partial file")
	}

	c, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/c.bin"})
	waitTask(t, f.e, c.ID, inState(proto.StateActive))
	if err := f.e.Cancel(c.ID); err != nil {
		t.Fatal(err)
	}
	waitTask(t, f.e, c.ID, inState(proto.StateStopped))
	if _, err := os.Stat(filepath.Join(f.dir, "c.bin.part")); !os.IsNotExist(err) {
		t.Fatal("cancel kept the partial file")
	}

	r, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/r.bin"})
	waitTask(t, f.e, r.ID, inState(proto.StateActive))
	if err := f.e.Remove(r.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.e.Task(r.ID); err == nil {
		t.Fatal("removed task still listed")
	}
	if f.ev.count("removed") != 1 {
		t.Fatal("no removed event")
	}

	g.open()
	if err := f.e.Resume(paused.ID, false); err != nil {
		t.Fatal(err)
	}
	done := waitTask(t, f.e, p.ID, inState(proto.StateDone))
	if got, _ := os.ReadFile(done.Path); !bytes.Equal(got, data) {
		t.Fatal("resumed content differs")
	}
	if _, err := f.e.Retry(c.ID); err != nil {
		t.Fatal(err)
	}
	waitTask(t, f.e, c.ID, inState(proto.StateDone))
	waitFor(t, func() bool {
		_, err := os.Stat(filepath.Join(f.dir, "r.bin.part"))
		return os.IsNotExist(err)
	})
	if err := f.e.Clear(nil); err != nil {
		t.Fatal(err)
	}
	if n := len(f.e.Tasks()); n != 0 {
		t.Fatalf("%d tasks left after clear", n)
	}
	if _, err := os.Stat(done.Path); err != nil {
		t.Fatal("clear deleted a finished file")
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRestartResumesActiveTask(t *testing.T) {
	g := newGated()
	srv := httptest.NewServer(g)
	defer srv.Close()
	defer g.open()
	data := randBytes(256 << 10)
	g.files["/x.bin"] = data
	f := newFixture(t, nil, nil)
	tk, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/x.bin"})
	waitTask(t, f.e, tk.ID, inState(proto.StateActive))
	f.e.Close()
	var sf stateFile
	b, _ := os.ReadFile(f.state)
	if err := json.Unmarshal(b, &sf); err != nil || len(sf.Tasks) != 1 || sf.Tasks[0].State != proto.StateActive {
		t.Fatalf("saved state = %s", b)
	}
	g.open()
	f.start(t, nil)
	waitTask(t, f.e, tk.ID, inState(proto.StateDone))
}

func TestNoResumeOnStartPauses(t *testing.T) {
	g := newGated()
	srv := httptest.NewServer(g)
	defer srv.Close()
	defer g.open()
	g.files["/y.bin"] = randBytes(64 << 10)
	f := newFixture(t, map[string]any{"resumeOnStart": false}, nil)
	tk, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/y.bin"})
	waitTask(t, f.e, tk.ID, inState(proto.StateActive))
	f.e.Close()
	f.start(t, nil)
	if got, _ := f.e.Task(tk.ID); got.State != proto.StatePaused {
		t.Fatalf("state %s, want paused", got.State)
	}
	if f.e.Busy() {
		t.Fatal("busy with only paused tasks")
	}
}

func TestErrorCodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/private") {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	f := newFixture(t, nil, nil)
	a, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/missing"})
	b, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/private"})
	if got := waitTask(t, f.e, a.ID, inState(proto.StateError)); got.Error.Code != proto.CodeUnavailable {
		t.Fatalf("404 code = %s", got.Error.Code)
	}
	if got := waitTask(t, f.e, b.ID, inState(proto.StateError)); got.Error.Code != proto.CodePrivate {
		t.Fatalf("403 code = %s", got.Error.Code)
	}
	if _, err := f.e.Add(proto.AddParams{URL: "ftp://x/y"}); err == nil {
		t.Fatal("ftp accepted")
	}
}

type fakeResolver struct{ base string }

func (r fakeResolver) Resolve(_ context.Context, link string) (*media.Media, error) {
	return &media.Media{Kind: plan.KindMedia, URL: link, Title: "Clip", Source: media.Source{ID: "fake", Name: "Fake"},
		Formats: []media.Format{{ID: "720", Label: "720p"}}}, nil
}

func (r fakeResolver) Plan(m *media.Media, o media.Options) (*plan.Plan, error) {
	return &plan.Plan{Name: "Clip", Container: "mkv", Post: plan.PostMerge, Parts: []plan.Part{
		{Kind: plan.PartHTTP, URL: r.base + "/v", Ext: "webm", Role: "video"},
		{Kind: plan.PartHLS, URL: "hls://a", Ext: "m4a", Role: "audio"},
	}}, nil
}

type fakeHLS struct{}

func (fakeHLS) Fetch(_ context.Context, part plan.Part, dst string, p plan.Progress) error {
	p.SetTotal(10)
	for i := range 10 {
		p.Add(1)
		p.Mark(i, 10, 1)
	}
	return os.WriteFile(dst, []byte("AUDIOAUDIO"), 0o644)
}

type catFinisher struct{}

func (catFinisher) Finish(_ context.Context, pl *plan.Plan, inputs []string, out string) error {
	var all []byte
	for _, in := range inputs {
		b, err := os.ReadFile(in)
		if err != nil {
			return err
		}
		all = append(all, b...)
	}
	return os.WriteFile(out, all, 0o644)
}

func TestMediaPlanMergesAndPublishes(t *testing.T) {
	g := newGated()
	g.files["/v"] = []byte("VIDEO")
	g.open()
	srv := httptest.NewServer(g)
	defer srv.Close()
	f := newFixture(t, nil, &Parts{
		Resolver: fakeResolver{base: srv.URL},
		Fetchers: map[plan.PartKind]Fetcher{plan.PartHLS: fakeHLS{}},
		Finisher: catFinisher{},
	})
	os.MkdirAll(f.dir, 0o755)
	os.WriteFile(filepath.Join(f.dir, "Clip.mkv"), []byte("taken"), 0o644)
	tk, err := f.e.Add(proto.AddParams{URL: "https://fake.test/watch?v=1", Format: "720"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitTask(t, f.e, tk.ID, inState(proto.StateDone, proto.StateError))
	if done.State != proto.StateDone {
		t.Fatalf("media task failed: %+v", done.Error)
	}
	if filepath.Base(done.Path) != "Clip (2).mkv" || done.Kind != "media" || done.Title != "Clip" || done.Format.Label != "720p" {
		t.Fatalf("task = %+v", done)
	}
	if got, _ := os.ReadFile(done.Path); string(got) != "VIDEOAUDIOAUDIO" {
		t.Fatalf("merged = %q", got)
	}
	if _, err := os.Stat(filepath.Join(f.dir, tempPrefix+tk.ID)); !os.IsNotExist(err) {
		t.Fatal("temp dir left behind")
	}
}

func TestProgressPiecesSplitAcrossParts(t *testing.T) {
	p := newProgress()
	pp := p.setParts([]int64{100, -1})
	pp[0].SetTotal(100)
	pp[0].Add(100)
	for i := range 64 {
		pp[0].Mark(i, 64, 1)
	}
	pp[1].Mark(0, 4, 1)
	s := p.sample()
	if s.total != -1 || s.done != 100 {
		t.Fatalf("sample = %+v", s)
	}
	for i := range 32 {
		if s.pieces[i] != 255 {
			t.Fatalf("part 0 bucket %d = %d", i, s.pieces[i])
		}
	}
	if s.pieces[32] != 255 || s.pieces[39] != 255 || s.pieces[40] != 0 || s.pieces[63] != 0 {
		t.Fatalf("part 1 buckets = %v", s.pieces[32:])
	}
}

type fakeTorrents struct {
	news, closes atomic.Int32
	live         atomic.Int32
	added        chan *fakeTorrent
	mu           sync.Mutex
	forgot       []string
}

type fakeClient struct{ f *fakeTorrents }

func (c fakeClient) Add(_ context.Context, src, dir string, sel []int) (Torrent, error) {
	t := &fakeTorrent{meta: make(chan struct{}), done: make(chan struct{}), fin: make(chan struct{}), dir: dir}
	c.f.added <- t
	return t, nil
}
func (c fakeClient) Resolve(context.Context, string) (string, []proto.File, error) {
	return "ubuntu.iso", []proto.File{{Path: "ubuntu.iso", Size: 10}}, nil
}
func (fakeClient) SetSeed(bool, float64, int) {}
func (c fakeClient) Forget(src string) error {
	c.f.mu.Lock()
	c.f.forgot = append(c.f.forgot, src)
	c.f.mu.Unlock()
	return nil
}
func (c fakeClient) Close() error {
	c.f.closes.Add(1)
	c.f.live.Add(-1)
	return nil
}

type fakeTorrent struct {
	meta, done, fin chan struct{}
	dir             string
	dropped         atomic.Bool
}

func (t *fakeTorrent) Metadata() <-chan struct{} { return t.meta }
func (t *fakeTorrent) Name() string              { return "ubuntu.iso" }
func (t *fakeTorrent) Path() string              { return filepath.Join(t.dir, "ubuntu.iso") }
func (t *fakeTorrent) Files() []proto.File {
	return []proto.File{{Path: "ubuntu.iso", Size: 10, Selected: true}}
}
func (t *fakeTorrent) Select([]int) error         { return nil }
func (t *fakeTorrent) Pause()                     {}
func (t *fakeTorrent) Resume()                    {}
func (t *fakeTorrent) Drop(bool) error            { t.dropped.Store(true); return nil }
func (t *fakeTorrent) Completed() <-chan struct{} { return t.done }
func (t *fakeTorrent) Finished() <-chan struct{}  { return t.fin }
func (t *fakeTorrent) Err() error                 { return nil }
func (t *fakeTorrent) Stats() TorrentStats {
	return TorrentStats{Size: 10, Done: 10, Complete: true, Peers: 2, Seeds: 1, Conns: 3}
}

func TestTorrentClientLivesOnlyWhileNeeded(t *testing.T) {
	ft := &fakeTorrents{added: make(chan *fakeTorrent, 4)}
	f := newFixture(t, map[string]any{"seed": true}, &Parts{Torrents: func(proto.Settings, *rate.Limiter, *rate.Limiter) (Torrents, error) {
		ft.news.Add(1)
		ft.live.Add(1)
		return fakeClient{ft}, nil
	}})
	if ft.news.Load() != 0 {
		t.Fatal("torrent client built at start")
	}
	r, err := f.e.Resolve(context.Background(), "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=ubuntu")
	if err != nil || r.Kind != "torrent" || len(r.Files) != 1 || r.Size != 10 {
		t.Fatalf("resolve = %+v, %v", r, err)
	}
	if ft.live.Load() != 0 {
		t.Fatal("client kept after a paste-card resolve")
	}
	tk, err := f.e.Add(proto.AddParams{URL: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=ubuntu"})
	if err != nil || tk.Kind != "torrent" || tk.Title != "ubuntu" {
		t.Fatalf("add = %+v, %v", tk, err)
	}
	tor := <-ft.added
	close(tor.meta)
	close(tor.done)
	got := waitTask(t, f.e, tk.ID, inState(proto.StateSeeding))
	if got.Name != "ubuntu.iso" || len(got.Files) != 1 {
		t.Fatalf("seeding task = %+v", got)
	}
	if ft.live.Load() != 1 {
		t.Fatal("client gone while seeding")
	}
	close(tor.fin)
	waitTask(t, f.e, tk.ID, inState(proto.StateDone))
	waitFor(t, func() bool { return ft.live.Load() == 0 })
	if !tor.dropped.Load() {
		t.Fatal("finished torrent not released")
	}
}

// A paused torrent cancelled while another torrent keeps the client open has
// its kept state cleared through that client, and directly when none is open.
func TestCancelPausedTorrentForgetsState(t *testing.T) {
	ft := &fakeTorrents{added: make(chan *fakeTorrent, 4)}
	var direct []string
	f := newFixture(t, map[string]any{"seed": true}, &Parts{
		Torrents: func(proto.Settings, *rate.Limiter, *rate.Limiter) (Torrents, error) {
			ft.news.Add(1)
			ft.live.Add(1)
			return fakeClient{ft}, nil
		},
		Forget: func(src string) error { direct = append(direct, src); return nil },
	})
	const a = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=a"
	const b = "magnet:?xt=urn:btih:89abcdef0123456789abcdef0123456789abcdef&dn=b"
	ta, err := f.e.Add(proto.AddParams{URL: a})
	if err != nil {
		t.Fatal(err)
	}
	close((<-ft.added).meta)
	waitTask(t, f.e, ta.ID, func(tk proto.Task) bool { return tk.Name != "" })
	if err := f.e.Pause(ta.ID, false); err != nil {
		t.Fatal(err)
	}
	waitTask(t, f.e, ta.ID, inState(proto.StatePaused))
	waitFor(t, func() bool { return ft.live.Load() == 0 })

	tb, err := f.e.Add(proto.AddParams{URL: b})
	if err != nil {
		t.Fatal(err)
	}
	torB := <-ft.added
	close(torB.meta)
	close(torB.done)
	waitTask(t, f.e, tb.ID, inState(proto.StateSeeding))
	if err := f.e.Cancel(ta.ID); err != nil {
		t.Fatal(err)
	}
	ft.mu.Lock()
	forgot := slices.Clone(ft.forgot)
	ft.mu.Unlock()
	if !slices.Equal(forgot, []string{a}) || len(direct) != 0 {
		t.Fatalf("with a client open: through client %q, direct %q", forgot, direct)
	}

	close(torB.fin)
	waitTask(t, f.e, tb.ID, inState(proto.StateDone))
	waitFor(t, func() bool { return ft.live.Load() == 0 })
	if err := f.e.Remove(tb.ID, false); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(direct, []string{b}) {
		t.Fatalf("with no client: direct %q", direct)
	}
}

// setResolver serves a post whose three items are pages of their own; each
// item page resolves to a direct file on the test server.
type setResolver struct{ base string }

func (r setResolver) Resolve(_ context.Context, link string) (*media.Media, error) {
	if link == "https://post.test/set" || link == "https://post.test/list" {
		items := make([]media.Item, 3)
		for i := range items {
			items[i] = media.Item{Title: fmt.Sprintf("Track %d", i+1), Kind: "audio", URL: fmt.Sprintf("https://post.test/track/%d", i+1)}
		}
		m := &media.Media{Kind: plan.KindMedia, URL: link, Title: "Set", Source: media.Source{ID: "sc", Name: "SoundCloud"}, Items: items}
		if link == "https://post.test/list" {
			// a playlist resolves to its first video, which is also its first item
			m.URL = items[0].URL
		}
		return m, nil
	}
	n := strings.TrimPrefix(link, "https://post.test/track/")
	return &media.Media{Kind: plan.KindFile, URL: r.base + "/t" + n, Name: "track" + n + ".mp3", Title: "Track " + n}, nil
}

func (setResolver) Plan(m *media.Media, o media.Options) (*plan.Plan, error) {
	return media.PlanFor(m, o)
}

func TestPageItemsExpandIntoTasks(t *testing.T) {
	g := newGated()
	for i := 1; i <= 3; i++ {
		g.files[fmt.Sprintf("/t%d", i)] = []byte(fmt.Sprintf("audio %d", i))
	}
	g.open()
	srv := httptest.NewServer(g)
	defer srv.Close()
	for _, c := range []struct {
		name  string
		url   string
		files []int
		want  []int
	}{
		{"whole post", "https://post.test/set", nil, []int{1, 2, 3}},
		{"by indices", "https://post.test/set", []int{0, 2}, []int{1, 3}},
		{"playlist", "https://post.test/list", nil, []int{1, 2, 3}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, nil, &Parts{Resolver: setResolver{base: srv.URL}})
			first, err := f.e.Add(proto.AddParams{URL: c.url, Mode: "audio", Files: c.files})
			if err != nil {
				t.Fatal(err)
			}
			waitFor(t, func() bool {
				tasks := f.e.Tasks()
				if len(tasks) != len(c.want) {
					return false
				}
				for _, tk := range tasks {
					if tk.State != proto.StateDone {
						return false
					}
				}
				return true
			})
			tasks := f.e.Tasks()
			if tasks[0].ID != first.ID {
				t.Fatalf("first task %s, add returned %s", tasks[0].ID, first.ID)
			}
			for i, tk := range tasks {
				n := c.want[i]
				if tk.URL != fmt.Sprintf("https://post.test/track/%d", n) || tk.Title != fmt.Sprintf("Track %d", n) || tk.Mode != "audio" || tk.Dir != f.dir {
					t.Fatalf("task %d = %+v", i, tk)
				}
				if got, _ := os.ReadFile(tk.Path); string(got) != fmt.Sprintf("audio %d", n) {
					t.Fatalf("task %d file %q = %q", i, tk.Path, got)
				}
			}
		})
	}
}

func TestMapErrDropsSentinelPrefix(t *testing.T) {
	err := fmt.Errorf("%w: %s", media.ErrUnsupported, "This YouTube link is not a video or a playlist.")
	if pe := mapErr(err); pe.Code != proto.CodeUnsupported || pe.Message != "This YouTube link is not a video or a playlist." {
		t.Fatalf("mapErr = %+v", pe)
	}
	if pe := mapErr(media.ErrUnavailable); pe.Code != proto.CodeUnavailable || pe.Message != "unavailable" {
		t.Fatalf("bare sentinel = %+v", pe)
	}
}

// A home list hides what it has already shown, so the stamp must keep the first
// look, let a client backdate it, survive a restart, and clear when the task runs again.
func TestSeenStampsSettledAndPausedTasks(t *testing.T) {
	g := newGated()
	g.files["/done.bin"] = randBytes(32 << 10)
	g.files["/held.bin"] = randBytes(32 << 10)
	srv := httptest.NewServer(g)
	defer srv.Close()
	defer g.open()
	f := newFixture(t, nil, nil)
	held, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/held.bin"})
	bad, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/missing"})
	failed := waitTask(t, f.e, bad.ID, inState(proto.StateError))
	if failed.Finished == "" || failed.Seen != "" {
		t.Fatalf("a failure is stamped when it stops and starts unseen: finished %q seen %q", failed.Finished, failed.Seen)
	}
	waitTask(t, f.e, held.ID, inState(proto.StateActive))
	first := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	f.e.Seen([]string{held.ID, bad.ID, "gone"}, first)
	if tk, _ := f.e.Task(held.ID); tk.Seen != "" {
		t.Fatalf("a running task was stamped %q", tk.Seen)
	}
	if err := f.e.Pause(held.ID, false); err != nil {
		t.Fatal(err)
	}
	waitTask(t, f.e, held.ID, inState(proto.StatePaused))
	f.e.Seen([]string{held.ID}, first)
	f.e.Seen([]string{held.ID}, first.Add(time.Hour))
	f.e.Seen([]string{bad.ID}, first.Add(-time.Hour))
	seen := func(id string) string { tk, _ := f.e.Task(id); return tk.Seen }
	if seen(held.ID) != "2026-11-22T10:00:00Z" || seen(bad.ID) != "2026-11-22T09:00:00Z" {
		t.Fatalf("seen = %q and %q, want the first look and the backdated one", seen(held.ID), seen(bad.ID))
	}
	f.e.Close()
	f.start(t, nil)
	if seen(bad.ID) != "2026-11-22T09:00:00Z" {
		t.Fatalf("seen after a restart = %q", seen(bad.ID))
	}
	if err := f.e.Resume(held.ID, false); err != nil {
		t.Fatal(err)
	}
	if tk, _ := f.e.Task(held.ID); tk.Seen != "" || tk.Finished != "" {
		t.Fatalf("a resumed task kept seen %q finished %q", tk.Seen, tk.Finished)
	}
}

func TestCancelStampsFinished(t *testing.T) {
	g := newGated()
	g.files["/c.bin"] = randBytes(32 << 10)
	srv := httptest.NewServer(g)
	defer srv.Close()
	defer g.open()
	f := newFixture(t, map[string]any{"maxActive": 1}, nil)
	first, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/c.bin"})
	queued, _ := f.e.Add(proto.AddParams{URL: srv.URL + "/c.bin"})
	waitTask(t, f.e, first.ID, inState(proto.StateActive))
	for _, id := range []string{first.ID, queued.ID} {
		if err := f.e.Cancel(id); err != nil {
			t.Fatal(err)
		}
		if tk := waitTask(t, f.e, id, inState(proto.StateStopped)); tk.Finished == "" {
			t.Fatalf("%s stopped without a stamp", id)
		}
	}
}

func seedFixture(t *testing.T, settings map[string]any, tasks []record) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{dir: filepath.Join(root, "dl"), state: filepath.Join(root, "state", "tasks.json"), conf: filepath.Join(root, "conf", "downloads.json"), ev: &events{}}
	settings["dir"] = f.dir
	b, _ := json.Marshal(settings)
	os.MkdirAll(filepath.Dir(f.conf), 0o700)
	os.WriteFile(f.conf, b, 0o600)
	for i := range tasks {
		if tasks[i].Dir == "" {
			tasks[i].Dir = f.dir
		}
	}
	b, _ = json.Marshal(stateFile{Version: stateFormat, Tasks: tasks})
	os.MkdirAll(filepath.Dir(f.state), 0o700)
	os.WriteFile(f.state, b, 0o600)
	os.MkdirAll(f.dir, 0o755)
	f.start(t, nil)
	return f
}

// Retention forgets what settled before the window and never what is unfinished: a
// finished file stays on disk, a failed download's partial data goes with its entry.
func TestRetentionForgetsOldSettledTasks(t *testing.T) {
	days := func(n int) string {
		return time.Now().Add(-time.Duration(n) * 24 * time.Hour).UTC().Format(time.RFC3339)
	}
	f := seedFixture(t, map[string]any{}, []record{
		{Task: proto.Task{ID: "old-done", URL: "http://x/old.bin", Kind: "file", State: proto.StateDone, Name: "old.bin", Finished: days(10)}, Known: true},
		{Task: proto.Task{ID: "old-failed", URL: "http://x/failed.bin", Kind: "file", State: proto.StateError, Name: "failed.bin", Finished: days(10), Error: &proto.Error{Code: proto.CodeNetwork}}, Known: true},
		{Task: proto.Task{ID: "old-stopped", URL: "http://x/stopped.bin", Kind: "file", State: proto.StateStopped, Started: days(10)}},
		{Task: proto.Task{ID: "recent-done", URL: "http://x/recent.bin", Kind: "file", State: proto.StateDone, Finished: days(2)}},
		{Task: proto.Task{ID: "old-paused", URL: "http://x/paused.bin", Kind: "file", State: proto.StatePaused, Created: days(30)}},
	})
	ids := func() []string {
		var out []string
		for _, tk := range f.e.Tasks() {
			out = append(out, tk.ID)
		}
		return out
	}
	if got := ids(); !slices.Equal(got, []string{"recent-done", "old-paused"}) {
		t.Fatalf("after a start with a week kept: %v", got)
	}
	if f.ev.count("removed") != 3 {
		t.Fatalf("removed events = %d, want 3", f.ev.count("removed"))
	}
	if _, err := f.e.SetSettings(json.RawMessage(`{"keepDays":0}`)); err != nil {
		t.Fatal(err)
	}
	if got := ids(); len(got) != 2 {
		t.Fatalf("keeping everything dropped %v", got)
	}
	if _, err := f.e.SetSettings(json.RawMessage(`{"keepDays":1}`)); err != nil {
		t.Fatal(err)
	}
	if got := ids(); !slices.Equal(got, []string{"old-paused"}) {
		t.Fatalf("after keeping a day: %v", got)
	}
}

func TestRetentionKeepsFilesAndDropsPartials(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dl")
	os.MkdirAll(dir, 0o755)
	done := filepath.Join(dir, "old.bin")
	partial := filepath.Join(dir, "failed.bin.part")
	os.WriteFile(done, []byte("landed"), 0o644)
	os.WriteFile(partial, []byte("half"), 0o644)
	old := time.Now().Add(-40 * 24 * time.Hour).UTC().Format(time.RFC3339)
	seedFixture(t, map[string]any{"keepDays": 30}, []record{
		{Task: proto.Task{ID: "old-done", URL: "http://x/old.bin", Kind: "file", State: proto.StateDone, Dir: dir, Name: "old.bin", Path: done, Finished: old}, Known: true},
		{Task: proto.Task{ID: "old-failed", URL: "http://x/failed.bin", Kind: "file", State: proto.StateError, Dir: dir, Name: "failed.bin", Finished: old}, Known: true},
	})
	if _, err := os.Stat(done); err != nil {
		t.Fatalf("retention deleted a finished file: %v", err)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatalf("retention kept a failed download's partial data: %v", err)
	}
}
