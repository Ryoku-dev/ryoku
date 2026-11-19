package httpdl

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoget/internal/plan"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryoget-httpdl-test-")
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

type prog struct {
	total, done, resumed atomic.Int64
	conns, maxConns      atomic.Int32
	mu                   sync.Mutex
	marks                [64]float64
}

func (p *prog) SetTotal(n int64) { p.total.Store(n) }
func (p *prog) Add(n int64)      { p.done.Add(n) }
func (p *prog) Resumed(n int64)  { p.resumed.Add(n); p.done.Add(n) }
func (p *prog) Stage(string)     {}
func (p *prog) Mark(i, count int, fill float64) {
	p.mu.Lock()
	if count == 64 {
		p.marks[i] = fill
	}
	p.mu.Unlock()
}
func (p *prog) Conns(n int) {
	p.conns.Store(int32(n))
	for {
		m := p.maxConns.Load()
		if int32(n) <= m || p.maxConns.CompareAndSwap(m, int32(n)) {
			return
		}
	}
}

// blob serves content with ranges through http.ServeContent, which also
// answers If-Range; fail lets a test inject errors per ranged request.
type blob struct {
	mu      sync.Mutex
	data    []byte
	etag    string
	ranges  bool
	served  atomic.Int64
	ranged  atomic.Int32
	reqs    atomic.Int32
	active  atomic.Int32
	peak    atomic.Int32
	fail    func(n int32) int
	onReq   func(n int32)
	trickle time.Duration
	dispo   string
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

type countWriter struct {
	http.ResponseWriter
	b *blob
}

func (w countWriter) Write(p []byte) (int, error) {
	if w.b.trickle > 0 {
		time.Sleep(w.b.trickle)
	}
	n, err := w.ResponseWriter.Write(p)
	w.b.served.Add(int64(n))
	return n, err
}

func (b *blob) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	n := b.reqs.Add(1)
	if b.onReq != nil {
		b.onReq(n)
	}
	cur := b.active.Add(1)
	defer b.active.Add(-1)
	for {
		p := b.peak.Load()
		if cur <= p || b.peak.CompareAndSwap(p, cur) {
			break
		}
	}
	b.mu.Lock()
	data, etag := b.data, b.etag
	b.mu.Unlock()
	if b.dispo != "" {
		w.Header().Set("Content-Disposition", b.dispo)
	}
	if r.Header.Get("Range") != "" && r.Header.Get("Range") != "bytes=0-0" {
		rn := b.ranged.Add(1)
		if b.fail != nil {
			if code := b.fail(rn); code != 0 {
				http.Error(w, "nope", code)
				return
			}
		}
	}
	if !b.ranges {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method == http.MethodHead {
			return
		}
		w.WriteHeader(http.StatusOK)
		cw := countWriter{w, b}
		for off := 0; off < len(data); off += 16 << 10 {
			if _, err := cw.Write(data[off:min(off+16<<10, len(data))]); err != nil {
				return
			}
		}
		return
	}
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(countWriter{w, b}, r, "", time.Time{}, bytes.NewReader(data))
}

func (b *blob) set(data []byte, etag string) {
	b.mu.Lock()
	b.data, b.etag = data, etag
	b.mu.Unlock()
}

func newDL(conns int, minSplit int64) *Downloader {
	return &Downloader{
		Conns:    func() int { return conns },
		MinSplit: func() int64 { return minSplit },
		Backoff:  func(int) time.Duration { return time.Millisecond },
	}
}

func TestRangedSplitSeveralConnections(t *testing.T) {
	b := &blob{data: randBytes(4 << 20), etag: `"v1"`, ranges: true, trickle: time.Millisecond}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dir := t.TempDir()
	p := &prog{}
	final, err := newDL(8, 128<<10).FetchFile(context.Background(), plan.Part{URL: srv.URL + "/big.bin"}, filepath.Join(dir, "big.bin"), p)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(final)
	if !bytes.Equal(got, b.data) {
		t.Fatal("content differs")
	}
	if b.ranged.Load() < 8 {
		t.Fatalf("ranged requests = %d, want >= 8", b.ranged.Load())
	}
	if b.peak.Load() < 4 {
		t.Fatalf("peak parallel requests = %d", b.peak.Load())
	}
	if p.done.Load() != int64(len(b.data)) || p.total.Load() != int64(len(b.data)) {
		t.Fatalf("progress %d/%d", p.done.Load(), p.total.Load())
	}
	for i, m := range p.marks {
		if m != 1 {
			t.Fatalf("bucket %d = %v", i, m)
		}
	}
	if _, err := os.Stat(final + ".part" + SidecarExt); !os.IsNotExist(err) {
		t.Fatal("sidecar left behind")
	}
}

func TestWorkStealingSplitsLargestSegment(t *testing.T) {
	j := &job{d: newDL(2, 10), info: &Info{Size: 100, Ranges: true}, minSplit: 10}
	j.fresh()
	if len(j.segs) != 2 {
		t.Fatalf("segments = %d", len(j.segs))
	}
	a, b := j.next(), j.next()
	b.Done = b.End - b.Start
	b.owned = false
	s := j.next()
	if s == nil || s.Start != 25 || a.End != 25 || s.End != 50 {
		t.Fatalf("steal: got %+v, a=%+v", s, a)
	}
}

func TestNoRangeFallback(t *testing.T) {
	b := &blob{data: randBytes(300 << 10), ranges: false}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dir := t.TempDir()
	p := &prog{}
	final, err := newDL(8, 16<<10).FetchFile(context.Background(), plan.Part{URL: srv.URL + "/plain.dat"}, filepath.Join(dir, "plain.dat"), p)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(final)
	if !bytes.Equal(got, b.data) {
		t.Fatal("content differs")
	}
	if p.maxConns.Load() > 1 {
		t.Fatalf("used %d connections on a server without ranges", p.maxConns.Load())
	}
}

type budget struct{ left atomic.Int64 }

func (l *budget) WaitN(ctx context.Context, n int) error {
	if l.left.Add(-int64(n)) < 0 {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func TestStopThenResumeFromSidecar(t *testing.T) {
	b := &blob{data: randBytes(2 << 20), etag: `"r1"`, ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dir := t.TempDir()
	dst := filepath.Join(dir, "f.bin")
	part := plan.Part{URL: srv.URL + "/f.bin"}

	d := newDL(4, 64<<10)
	lim := &budget{}
	lim.left.Store(int64(len(b.data) / 2))
	d.Limit = lim
	ctx, cancel := context.WithCancel(context.Background())
	p1 := &prog{}
	errc := make(chan error, 1)
	go func() { _, err := d.FetchFile(ctx, part, dst, p1); errc <- err }()
	waitFor(t, func() bool { return p1.done.Load() >= int64(len(b.data)/3) })
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("stop error = %v", err)
	}
	if _, err := os.Stat(dst + ".part" + SidecarExt); err != nil {
		t.Fatal("sidecar missing after stop:", err)
	}

	b.served.Store(0)
	p2 := &prog{}
	final, err := newDL(4, 64<<10).FetchFile(context.Background(), part, dst, p2)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(final)
	if !bytes.Equal(got, b.data) {
		t.Fatal("resumed content differs")
	}
	if p2.resumed.Load() < int64(len(b.data)/3) {
		t.Fatalf("resumed only %d bytes", p2.resumed.Load())
	}
	// the probe's single byte aside, only the missing half travels again
	if served := b.served.Load(); served > int64(len(b.data))*3/4 {
		t.Fatalf("served %d of %d bytes on resume", served, len(b.data))
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestChangedOnServerRestarts(t *testing.T) {
	b := &blob{data: randBytes(1 << 20), etag: `"a"`, ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dir := t.TempDir()
	dst := filepath.Join(dir, "f.bin")
	part := plan.Part{URL: srv.URL + "/f.bin"}

	// a stopped download whose file then changes: the probe sees a new ETag
	d := newDL(4, 64<<10)
	lim := &budget{}
	lim.left.Store(int64(len(b.data) / 2))
	d.Limit = lim
	ctx, cancel := context.WithCancel(context.Background())
	p1 := &prog{}
	errc := make(chan error, 1)
	go func() { _, err := d.FetchFile(ctx, part, dst, p1); errc <- err }()
	waitFor(t, func() bool { return p1.done.Load() >= int64(len(b.data)/3) })
	cancel()
	<-errc
	next := randBytes(1<<20 + 512)
	b.set(next, `"b"`)
	final, err := newDL(4, 64<<10).FetchFile(context.Background(), part, dst, &prog{})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(final); !bytes.Equal(got, next) {
		t.Fatal("stale bytes survived a changed file")
	}
}

func TestIfRangeMismatchMidDownloadRestarts(t *testing.T) {
	b := &blob{data: randBytes(1 << 20), etag: `"a"`, ranges: true}
	next := randBytes(1 << 20)
	// HEAD and the one-byte probe see "a"; every later request sees "b",
	// so the If-Range "a" ranged GETs come back 200 with the whole new file
	b.onReq = func(n int32) {
		if n == 3 {
			b.set(next, `"b"`)
		}
	}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dir := t.TempDir()
	p := &prog{}
	final, err := newDL(4, 64<<10).FetchFile(context.Background(), plan.Part{URL: srv.URL + "/f.bin"}, filepath.Join(dir, "f.bin"), p)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(final); !bytes.Equal(got, next) {
		t.Fatal("If-Range mismatch did not restart with the new file")
	}
	if p.done.Load() != int64(len(next)) {
		t.Fatalf("progress %d after restart, want %d", p.done.Load(), len(next))
	}
}

func Test5xxRetry(t *testing.T) {
	b := &blob{data: randBytes(512 << 10), etag: `"x"`, ranges: true}
	b.fail = func(n int32) int {
		if n <= 3 {
			return http.StatusServiceUnavailable
		}
		return 0
	}
	srv := httptest.NewServer(b)
	defer srv.Close()
	final, err := newDL(1, 1<<20).FetchFile(context.Background(), plan.Part{URL: srv.URL + "/f.bin"}, filepath.Join(t.TempDir(), "f.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(final); !bytes.Equal(got, b.data) {
		t.Fatal("content differs after retries")
	}
}

func Test5xxGivesUpAfterFiveTries(t *testing.T) {
	b := &blob{data: randBytes(64 << 10), etag: `"x"`, ranges: true}
	b.fail = func(int32) int { return http.StatusBadGateway }
	srv := httptest.NewServer(b)
	defer srv.Close()
	_, err := newDL(1, 1<<20).FetchFile(context.Background(), plan.Part{URL: srv.URL + "/f.bin"}, filepath.Join(t.TempDir(), "f.bin"), &prog{})
	var se *StatusError
	if !errors.As(err, &se) || se.Code != http.StatusBadGateway {
		t.Fatalf("err = %v", err)
	}
	if got := b.ranged.Load(); got != maxTries {
		t.Fatalf("tries = %d, want %d", got, maxTries)
	}
}

func Test404IsNotRetried(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	_, err := newDL(1, 1<<20).FetchFile(context.Background(), plan.Part{URL: srv.URL + "/nope"}, filepath.Join(t.TempDir(), "x"), &prog{})
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 404 {
		t.Fatalf("err = %v", err)
	}
}

func TestFileName(t *testing.T) {
	cases := []struct{ cd, final, orig, ct, want string }{
		{`attachment; filename="report.pdf"`, "https://x.test/dl?id=1", "", "", "report.pdf"},
		{`attachment; filename="fallback.txt"; filename*=UTF-8''na%C3%AFve%20file.txt`, "https://x.test/a", "", "", "naïve file.txt"},
		{`attachment; filename=plain name.zip`, "https://x.test/a", "", "", "plain name.zip"},
		{`attachment; filename="../../etc/passwd"`, "https://x.test/a", "", "", "_.._etc_passwd"},
		{"", "https://cdn.test/files/archive.tar.gz?sig=1", "", "", "archive.tar.gz"},
		{"", "https://cdn.test/abc123", "https://site.test/get/movie.mkv", "", "movie.mkv"},
		{"", "https://cdn.test/abc%20def", "", "image/png", "abc def.png"},
		{"", "https://host.test/", "https://host.test/", "text/html", "host.test.html"},
	}
	for _, c := range cases {
		orig := c.orig
		if orig == "" {
			orig = c.final
		}
		if got := FileName(c.cd, c.final, orig, c.ct); got != c.want {
			t.Errorf("FileName(%q, %q) = %q, want %q", c.cd, c.final, got, c.want)
		}
	}
}

func TestContentDispositionNamesProbe(t *testing.T) {
	b := &blob{data: []byte("hello"), etag: `"e"`, ranges: true, dispo: `attachment; filename*=UTF-8''r%C3%A9sum%C3%A9.pdf`}
	srv := httptest.NewServer(b)
	defer srv.Close()
	info, err := Probe(context.Background(), nil, srv.URL+"/download?id=7", nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "résumé.pdf" || info.Size != 5 || !info.Ranges || info.ETag != `"e"` {
		t.Fatalf("probe = %+v", info)
	}
}

func TestCollisionNaming(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(dir, "a (2).txt.part"), nil, 0o644)
	if got := Unique(dir, "a.txt", nil); got != "a (3).txt" {
		t.Fatalf("Unique = %q", got)
	}
	if got := Unique(dir, "b.txt", func(n string) bool { return n == "b.txt" }); got != "b (2).txt" {
		t.Fatalf("Unique taken = %q", got)
	}
	if got := Numbered("x.tar.gz", 2); got != "x (2).tar.gz" {
		t.Fatalf("Numbered = %q", got)
	}
	if got := Numbered("README", 4); got != "README (4)" {
		t.Fatalf("Numbered = %q", got)
	}

	b := &blob{data: []byte("new bytes"), etag: `"n"`, ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	final, err := newDL(2, 1<<20).FetchFile(context.Background(), plan.Part{URL: srv.URL + "/a.txt"}, filepath.Join(dir, "a.txt"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(final) != "a (2).txt" {
		t.Fatalf("final = %q", final)
	}
	if old, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(old) != "old" {
		t.Fatal("existing file was replaced")
	}
}

// a server that serves only two ranged connections at a time and answers the
// rest with 429: the download narrows to what it is allowed and still finishes
func TestConnectionLimitedServerNarrows(t *testing.T) {
	b := &blob{data: randBytes(1 << 20), etag: `"l"`, ranges: true, trickle: time.Millisecond}
	slots := make(chan struct{}, 2)
	var refused atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rg := r.Header.Get("Range"); rg != "" && rg != "bytes=0-0" {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			default:
				refused.Add(1)
				http.Error(w, "slow down", http.StatusTooManyRequests)
				return
			}
		}
		b.ServeHTTP(w, r)
	}))
	defer srv.Close()
	final, err := newDL(8, 64<<10).FetchFile(context.Background(), plan.Part{URL: srv.URL + "/f.bin"}, filepath.Join(t.TempDir(), "f.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(final); !bytes.Equal(got, b.data) {
		t.Fatal("content differs")
	}
	if refused.Load() == 0 {
		t.Fatal("the limit never bit; the test proves nothing")
	}
}

// a media part re-resolved after a pause comes back under a freshly signed
// URL; Fetch resumes the same bytes instead of starting over
func TestFetchResumesUnderNewSignedURL(t *testing.T) {
	b := &blob{data: randBytes(2 << 20), ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "part0.webm")

	d := newDL(4, 64<<10)
	lim := &budget{}
	lim.left.Store(int64(len(b.data) / 2))
	d.Limit = lim
	ctx, cancel := context.WithCancel(context.Background())
	p1 := &prog{}
	errc := make(chan error, 1)
	go func() { errc <- d.Fetch(ctx, plan.Part{URL: srv.URL + "/v?sig=old"}, dst, p1) }()
	waitFor(t, func() bool { return p1.done.Load() >= int64(len(b.data)/3) })
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("stop error = %v", err)
	}

	p2 := &prog{}
	if err := newDL(4, 64<<10).Fetch(context.Background(), plan.Part{URL: srv.URL + "/v?sig=new"}, dst, p2); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); !bytes.Equal(got, b.data) {
		t.Fatal("content differs")
	}
	if p2.resumed.Load() < int64(len(b.data)/3) {
		t.Fatalf("resumed only %d bytes under the new URL", p2.resumed.Load())
	}
}
