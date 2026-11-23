package httpdl

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoget/internal/plan"
	"ryoget/internal/proto"
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
// answers If-Range, and logs every request it sees.
type blob struct {
	mu      sync.Mutex
	data    []byte
	etag    string
	ranges  bool
	log     []string
	served  atomic.Int64
	ranged  atomic.Int32
	reqs    atomic.Int32
	active  atomic.Int32
	peak    atomic.Int32
	fail    func(n int32) int
	onReq   func(n int32, r *http.Request)
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
	// written on the first request of the server
	first *atomic.Int64
}

func (w countWriter) Write(p []byte) (int, error) {
	if w.b.trickle > 0 {
		time.Sleep(w.b.trickle)
	}
	n, err := w.ResponseWriter.Write(p)
	w.b.served.Add(int64(n))
	if w.first != nil {
		w.first.Add(int64(n))
	}
	return n, err
}

func (b *blob) entries() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.log...)
}

func (b *blob) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	n := b.reqs.Add(1)
	b.mu.Lock()
	b.log = append(b.log, r.Method+" "+r.Header.Get("Range"))
	b.mu.Unlock()
	if b.onReq != nil {
		b.onReq(n, r)
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
	if r.Header.Get("Range") != "" {
		rn := b.ranged.Add(1)
		if b.fail != nil {
			if code := b.fail(rn); code != 0 {
				http.Error(w, "nope", code)
				return
			}
		}
	}
	cw := countWriter{ResponseWriter: w, b: b}
	if !b.ranges {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method == http.MethodHead {
			return
		}
		w.WriteHeader(http.StatusOK)
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
	http.ServeContent(cw, r, "", time.Time{}, bytes.NewReader(data))
}

func (b *blob) set(data []byte, etag string) {
	b.mu.Lock()
	b.data, b.etag = data, etag
	b.mu.Unlock()
}

func testTuning() *tuning {
	t := defaultTuning()
	t.hold = 20 * time.Millisecond
	t.probeFirst = time.Hour
	t.saveEvery = 30 * time.Millisecond
	t.tick = 20 * time.Millisecond
	t.minReq, t.maxReq = 64<<10, 256<<10
	return &t
}

func newDL(t *testing.T, conns int, minSplit int64) *Downloader {
	return &Downloader{
		Conns:     func() int { return conns },
		MinSplit:  func() int64 { return minSplit },
		Backoff:   func(int) time.Duration { return time.Millisecond },
		HostsFile: filepath.Join(t.TempDir(), "hosts.json"),
		tune:      testTuning(),
	}
}

func fetchTo(d *Downloader, u, dst string, p plan.Progress) (string, error) {
	return d.FetchFile(context.Background(), plan.Part{URL: u}, dst, nil, p)
}

func sameFile(t *testing.T, path string, want []byte) {
	t.Helper()
	if got, _ := os.ReadFile(path); !bytes.Equal(got, want) {
		t.Fatal("content differs")
	}
}

func TestRangedSplitSeveralConnections(t *testing.T) {
	b := &blob{data: randBytes(4 << 20), etag: `"v1"`, ranges: true, trickle: time.Millisecond}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dir := t.TempDir()
	p := &prog{}
	final, err := fetchTo(newDL(t, 8, 128<<10), srv.URL+"/big.bin", filepath.Join(dir, "big.bin"), p)
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
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

// the first request is the probe and already carries the file's first bytes
func TestFirstRequestCarriesDataWithoutHeadOrOneByteProbe(t *testing.T) {
	var first atomic.Int64
	b := &blob{data: randBytes(1 << 20), etag: `"p"`, ranges: true, dispo: `attachment; filename="named.bin"`}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b.reqs.Load() == 0 {
			w = countWriter{ResponseWriter: w, b: &blob{}, first: &first}
		}
		b.ServeHTTP(w, r)
	}))
	defer srv.Close()
	dir := t.TempDir()
	var got *Info
	final, err := newDL(t, 4, 64<<10).FetchFile(context.Background(), plan.Part{URL: srv.URL + "/get?id=1"}, "", func(info *Info) (string, error) {
		if _, err := os.Stat(filepath.Join(dir, info.Name+".part")); !os.IsNotExist(err) {
			t.Error("bytes landed before the name was picked")
		}
		got = info
		return filepath.Join(dir, info.Name), nil
	}, &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if got == nil || got.Name != "named.bin" || got.Size != int64(len(b.data)) || !got.Ranges || got.ETag != `"p"` {
		t.Fatalf("namer saw %+v", got)
	}
	log := b.entries()
	if log[0] != "GET bytes=0-65535" {
		t.Fatalf("first request = %q", log[0])
	}
	for _, e := range log {
		if strings.HasPrefix(e, "HEAD") || e == "GET bytes=0-0" {
			t.Fatalf("request log has %q: %v", e, log)
		}
	}
	if first.Load() == 0 {
		t.Fatal("the first request carried no data")
	}
}

func TestTableSplitsAlignedAndBehindTheRequest(t *testing.T) {
	tab := newTable(10<<20, 1<<20, true)
	a := tab.claim(tab.segs[0], 1, 0)
	b, ok := tab.acquire(2, false, time.Now())
	if !ok || b.cursor != 5<<20 || a.seg.end != 5<<20 {
		t.Fatalf("midpoint split: %+v, victim end %d", b, a.seg.end)
	}
	// the victim's request reaches past the midpoint: split at its end
	tab.setReqEnd(a.seg, 1, 4<<20-100)
	tab.advance(a.seg, 1, 0, 1<<20)
	tab.advance(b.seg, 2, 5<<20, 2<<20)
	c, ok := tab.acquire(3, false, time.Now())
	if !ok || c.cursor != 4<<20 || a.seg.end != 4<<20 {
		t.Fatalf("split behind the request: %+v, victim end %d", c, a.seg.end)
	}
	tab2 := newTable(3<<20+12345, 1<<20, true)
	tab2.claim(tab2.segs[0], 1, 0)
	d, _ := tab2.acquire(2, false, time.Now())
	if d.cursor%splitAlign != 0 {
		t.Fatalf("split at %d is not aligned", d.cursor)
	}
}

func TestNoRangeServerIsOneStream(t *testing.T) {
	b := &blob{data: randBytes(300 << 10), ranges: false}
	srv := httptest.NewServer(b)
	defer srv.Close()
	p := &prog{}
	final, err := fetchTo(newDL(t, 8, 16<<10), srv.URL+"/plain.dat", filepath.Join(t.TempDir(), "plain.dat"), p)
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if n := b.reqs.Load(); n != 1 {
		t.Fatalf("%d requests to a server without ranges, want 1", n)
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

// stopHalfway runs a download that stalls after about half the file and stops it.
func stopHalfway(t *testing.T, u, dst string, size int, fetch func(*Downloader, context.Context, plan.Progress) error) {
	t.Helper()
	d := newDL(t, 4, 64<<10)
	lim := &budget{}
	lim.left.Store(int64(size / 2))
	d.Limit = lim
	ctx, cancel := context.WithCancel(context.Background())
	p := &prog{}
	errc := make(chan error, 1)
	go func() { errc <- fetch(d, ctx, p) }()
	waitFor(t, func() bool { return p.done.Load() >= int64(size/3) })
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("stop error = %v", err)
	}
}

func fileStop(u, dst string) func(*Downloader, context.Context, plan.Progress) error {
	return func(d *Downloader, ctx context.Context, p plan.Progress) error {
		_, err := d.FetchFile(ctx, plan.Part{URL: u}, dst, nil, p)
		return err
	}
}

func TestStopThenResumeFromSidecar(t *testing.T) {
	b := &blob{data: randBytes(2 << 20), etag: `"r1"`, ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "f.bin")
	u := srv.URL + "/f.bin"
	stopHalfway(t, u, dst, len(b.data), fileStop(u, dst))
	if _, err := os.Stat(dst + ".part" + SidecarExt); err != nil {
		t.Fatal("sidecar missing after stop:", err)
	}

	b.served.Store(0)
	p2 := &prog{}
	final, err := fetchTo(newDL(t, 4, 64<<10), u, dst, p2)
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if p2.resumed.Load() < int64(len(b.data)/3) {
		t.Fatalf("resumed only %d bytes", p2.resumed.Load())
	}
	if served := b.served.Load(); served > int64(len(b.data))*3/4 {
		t.Fatalf("served %d of %d bytes on resume", served, len(b.data))
	}
}

// a refreshed link to the same file (same size and ETag) keeps the partial
func TestResumeUnderChangedURLKeepsPartial(t *testing.T) {
	b := &blob{data: randBytes(2 << 20), etag: `"same"`, ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "f.bin")
	old := srv.URL + "/f.bin?sig=old"
	stopHalfway(t, old, dst, len(b.data), fileStop(old, dst))
	b.served.Store(0)
	p2 := &prog{}
	final, err := fetchTo(newDL(t, 4, 64<<10), srv.URL+"/f.bin?sig=new", dst, p2)
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if p2.resumed.Load() < int64(len(b.data)/3) {
		t.Fatalf("resumed only %d bytes under the new URL", p2.resumed.Load())
	}
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
	dst := filepath.Join(t.TempDir(), "f.bin")
	u := srv.URL + "/f.bin"
	stopHalfway(t, u, dst, len(b.data), fileStop(u, dst))
	next := randBytes(1<<20 + 512)
	b.set(next, `"b"`)
	p := &prog{}
	final, err := fetchTo(newDL(t, 4, 64<<10), u, dst, p)
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, next)
	if p.resumed.Load() != 0 {
		t.Fatal("stale bytes were counted as resumed")
	}
}

// without ETag or Last-Modified, the bytes before the frontier are the proof
func TestValidatorlessResumeRestartsWhenTheFileChanged(t *testing.T) {
	b := &blob{data: randBytes(2 << 20), ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "f.bin")
	u := srv.URL + "/f.bin"
	stopHalfway(t, u, dst, len(b.data), fileStop(u, dst))
	next := randBytes(len(b.data))
	b.set(next, "")
	p := &prog{}
	final, err := fetchTo(newDL(t, 4, 64<<10), u, dst, p)
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, next)
	if p.resumed.Load() != 0 || p.done.Load() != int64(len(next)) {
		t.Fatalf("resumed %d, done %d after a change", p.resumed.Load(), p.done.Load())
	}
}

// a media part re-resolved after a pause comes back under a freshly signed
// URL without validators, the overlap check passes and the bytes are kept
func TestFetchResumesUnderNewSignedURL(t *testing.T) {
	b := &blob{data: randBytes(2 << 20), ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	dst := filepath.Join(t.TempDir(), "part0.webm")
	stopHalfway(t, "", dst, len(b.data), func(d *Downloader, ctx context.Context, p plan.Progress) error {
		return d.Fetch(ctx, plan.Part{URL: srv.URL + "/v?sig=old"}, dst, p)
	})
	p2 := &prog{}
	if err := newDL(t, 4, 64<<10).Fetch(context.Background(), plan.Part{URL: srv.URL + "/v?sig=new"}, dst, p2); err != nil {
		t.Fatal(err)
	}
	sameFile(t, dst, b.data)
	if p2.resumed.Load() < int64(len(b.data)/3) {
		t.Fatalf("resumed only %d bytes under the new URL", p2.resumed.Load())
	}
}

func TestBrokenSidecarRestartsCleanly(t *testing.T) {
	const size = 512 << 10
	for _, tc := range []struct {
		name     string
		segs     []span
		partSize int64
	}{
		{"gap", []span{{Start: 0, End: 100 << 10, Done: 100 << 10}, {Start: 200 << 10, End: size, Done: 0}}, size},
		{"overlap", []span{{Start: 0, End: 300 << 10, Done: 300 << 10}, {Start: 200 << 10, End: size, Done: 0}}, size},
		{"short part", []span{{Start: 0, End: 256 << 10, Done: 256 << 10}, {Start: 256 << 10, End: size, Done: 0}}, size - 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &blob{data: randBytes(size), etag: `"s"`, ranges: true}
			srv := httptest.NewServer(b)
			defer srv.Close()
			dst := filepath.Join(t.TempDir(), "f.bin")
			if err := os.WriteFile(dst+".part", randBytes(int(tc.partSize)), 0o644); err != nil {
				t.Fatal(err)
			}
			sc, _ := json.Marshal(sidecar{URL: srv.URL + "/f.bin", ETag: `"s"`, Size: size, Segments: tc.segs})
			if err := os.WriteFile(dst+".part"+SidecarExt, sc, 0o644); err != nil {
				t.Fatal(err)
			}
			p := &prog{}
			final, err := fetchTo(newDL(t, 4, 64<<10), srv.URL+"/f.bin", dst, p)
			if err != nil {
				t.Fatal(err)
			}
			sameFile(t, final, b.data)
			if p.resumed.Load() != 0 {
				t.Fatalf("trusted %d bytes from a broken sidecar", p.resumed.Load())
			}
			if _, err := os.Stat(dst + ".part" + SidecarExt); !os.IsNotExist(err) {
				t.Fatal("sidecar left behind")
			}
		})
	}
}

func TestIfRangeMismatchMidDownloadRestarts(t *testing.T) {
	b := &blob{data: randBytes(1 << 20), etag: `"a"`, ranges: true}
	next := randBytes(1 << 20)
	// the first answer sees "a", every later one "b", so the If-Range "a" GETs come back 200
	b.onReq = func(n int32, _ *http.Request) {
		if n == 2 {
			b.set(next, `"b"`)
		}
	}
	srv := httptest.NewServer(b)
	defer srv.Close()
	p := &prog{}
	final, err := fetchTo(newDL(t, 4, 64<<10), srv.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), p)
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, next)
	if p.done.Load() != int64(len(next)) {
		t.Fatalf("progress %d after restart, want %d", p.done.Load(), len(next))
	}
}

func Test5xxAnd425Retried(t *testing.T) {
	b := &blob{data: randBytes(512 << 10), etag: `"x"`, ranges: true}
	b.fail = func(n int32) int {
		switch n {
		case 1:
			return http.StatusTooEarly
		case 2, 3:
			return http.StatusServiceUnavailable
		}
		return 0
	}
	srv := httptest.NewServer(b)
	defer srv.Close()
	final, err := fetchTo(newDL(t, 1, 1<<20), srv.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
}

func TestRetryAfterIsHonoured(t *testing.T) {
	b := &blob{data: randBytes(256 << 10), etag: `"x"`, ranges: true}
	var refusedAt, nextAt atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch b.reqs.Load() {
		case 1:
			refusedAt.Store(time.Now().UnixNano())
			b.reqs.Add(1)
			w.Header().Set("Retry-After", "1")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		case 2:
			nextAt.Store(time.Now().UnixNano())
		}
		b.ServeHTTP(w, r)
	}))
	defer srv.Close()
	// the first request is fine, the second is told to wait a second
	final, err := fetchTo(newDL(t, 1, 1<<20), srv.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if gap := time.Duration(nextAt.Load() - refusedAt.Load()); refusedAt.Load() == 0 || gap < 900*time.Millisecond {
		t.Fatalf("retried %v after a Retry-After of 1 s", gap)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, c := range []struct {
		v    string
		want time.Duration
	}{
		{"", 0},
		{"7", 7 * time.Second},
		{"-1", 0},
		{"86400", maxRetryAfter},
		{now.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second},
		{now.Add(time.Hour).Format(http.TimeFormat), maxRetryAfter},
		{now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"soon", 0},
	} {
		if got := retryAfter(c.v, now); got != c.want {
			t.Errorf("retryAfter(%q) = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestBackoffGrowsWithJitterAndCaps(t *testing.T) {
	if d := backoffDelay(0, 0); d != 250*time.Millisecond {
		t.Fatalf("first low = %v", d)
	}
	if d := backoffDelay(3, 0.999999); d < 3900*time.Millisecond || d >= 4*time.Second {
		t.Fatalf("fourth high = %v", d)
	}
	if d := backoffDelay(40, 0); d != backoffCap/2 {
		t.Fatalf("capped low = %v", d)
	}
}

func TestGivesUpWhenNothingProgresses(t *testing.T) {
	b := &blob{data: randBytes(64 << 10), etag: `"x"`, ranges: true}
	b.fail = func(int32) int { return http.StatusBadGateway }
	srv := httptest.NewServer(b)
	defer srv.Close()
	d := newDL(t, 1, 1<<20)
	_, err := fetchTo(d, srv.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), &prog{})
	var se *StatusError
	if !errors.As(err, &se) || se.Code != http.StatusBadGateway {
		t.Fatalf("err = %v", err)
	}
	if got := b.ranged.Load(); got != int32(d.tune.maxRetries+1) {
		t.Fatalf("tries = %d, want %d", got, d.tune.maxRetries+1)
	}
}

func Test404IsNotRetried(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	_, err := fetchTo(newDL(t, 1, 1<<20), srv.URL+"/nope", filepath.Join(t.TempDir(), "x"), &prog{})
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 404 {
		t.Fatalf("err = %v", err)
	}
}

// rangeStart is where a request's Range begins, -1 without one.
func rangeStart(r *http.Request) int64 {
	v, ok := strings.CutPrefix(r.Header.Get("Range"), "bytes=")
	if !ok {
		return -1
	}
	a, _, _ := strings.Cut(v, "-")
	n, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return -1
	}
	return n
}

// drop answers the range's headers and closes the connection before the body
func drop(w http.ResponseWriter, r *http.Request, size int) {
	start := int(rangeStart(r))
	w.Header().Set("Content-Range", "bytes "+strconv.Itoa(start)+"-"+strconv.Itoa(size-1)+"/"+strconv.Itoa(size))
	w.Header().Set("Content-Length", strconv.Itoa(size-start))
	w.WriteHeader(http.StatusPartialContent)
	_ = http.NewResponseController(w).Flush()
	panic(http.ErrAbortHandler)
}

// cutWriter lets n body bytes through and then drops the connection
type cutWriter struct {
	http.ResponseWriter
	n int
}

func (w *cutWriter) Write(p []byte) (int, error) {
	if len(p) > w.n {
		_, _ = w.ResponseWriter.Write(p[:w.n])
		w.n = 0
		_ = http.NewResponseController(w.ResponseWriter).Flush()
		panic(http.ErrAbortHandler)
	}
	w.n -= len(p)
	return w.ResponseWriter.Write(p)
}

func TestOneConnectionGivingUpDoesNotFailTheDownload(t *testing.T) {
	const size = 512 << 10
	b := &blob{data: randBytes(size), etag: `"g"`, ranges: true}
	d := newDL(t, 1, 1<<20)
	d.tune.maxRetries = 3
	// the range after the first request breaks one more time than a connection's budget
	var drops atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rangeStart(r) == d.tune.minReq && drops.Add(1) <= int32(d.tune.maxRetries+1) {
			drop(w, r, size)
		}
		b.ServeHTTP(w, r)
	}))
	defer srv.Close()
	final, err := fetchTo(d, srv.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if drops.Load() <= int32(d.tune.maxRetries+1) {
		t.Fatal("no connection gave up, the test proves nothing")
	}

	// the same range broken for good: the replacement round makes no progress and the download fails
	var forever atomic.Int32
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rangeStart(r) == d.tune.minReq {
			forever.Add(1)
			drop(w, r, size)
		}
		b.ServeHTTP(w, r)
	}))
	defer dead.Close()
	if _, err := fetchTo(d, dead.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), &prog{}); err == nil {
		t.Fatal("a range nobody can fetch did not fail the download")
	}
	if n := forever.Load(); n != int32(2*(d.tune.maxRetries+1)) {
		t.Fatalf("%d tries, want two rounds of %d", n, d.tune.maxRetries+1)
	}
}

func TestDroppedConnectionsMidTransferStillComplete(t *testing.T) {
	b := &blob{data: randBytes(2 << 20), etag: `"d"`, ranges: true}
	var cut atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// every other answer dies 40 KiB in
		if cut.Add(1)%2 == 0 {
			w = &cutWriter{ResponseWriter: w, n: 40 << 10}
		}
		b.ServeHTTP(w, r)
	}))
	defer srv.Close()
	final, err := fetchTo(newDL(t, 4, 64<<10), srv.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
}

func TestNoSpaceFailsBeforeWriting(t *testing.T) {
	b := &blob{data: randBytes(1 << 20), etag: `"n"`, ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	d := newDL(t, 4, 64<<10)
	d.tune.free = func(string) (int64, error) { return 1000, nil }
	dst := filepath.Join(t.TempDir(), "f.bin")
	_, err := fetchTo(d, srv.URL+"/f.bin", dst, &prog{})
	var pe *proto.Error
	if !errors.As(err, &pe) || pe.Code != proto.CodeNoSpace {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(pe.Message, "1.0 MiB") {
		t.Fatalf("message %q does not name the size", pe.Message)
	}
	if _, err := os.Stat(dst + ".part"); !os.IsNotExist(err) {
		t.Fatal("a .part was created without room for it")
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
	if log := b.entries(); len(log) != 1 || log[0] != "GET bytes=0-0" {
		t.Fatalf("probe sent %v", log)
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
	final, err := fetchTo(newDL(t, 2, 1<<20), srv.URL+"/a.txt", filepath.Join(dir, "a.txt"), &prog{})
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
// rest with 429: the download narrows, finishes, and remembers the host's limit
func TestConnectionLimitedServerNarrowsAndRemembers(t *testing.T) {
	b := &blob{data: randBytes(2 << 20), etag: `"l"`, ranges: true, trickle: time.Millisecond}
	slots := make(chan struct{}, 2)
	var refused atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			refused.Add(1)
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		b.ServeHTTP(w, r)
	}))
	defer srv.Close()
	d := newDL(t, 8, 64<<10)
	final, err := fetchTo(d, srv.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if refused.Load() == 0 {
		t.Fatal("the limit never bit, the test proves nothing")
	}
	u, _ := url.Parse(srv.URL)
	learned := learnedLimit(d.HostsFile, u.Host)
	if learned < 1 || learned > 2 {
		t.Fatalf("learned limit %d, want 1 or 2", learned)
	}

	// the next download starts at the learned limit
	b.peak.Store(0)
	final, err = fetchTo(d, srv.URL+"/f.bin", filepath.Join(t.TempDir(), "g.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if b.peak.Load() > int32(learned) {
		t.Fatalf("second run peaked at %d connections with a learned limit of %d", b.peak.Load(), learned)
	}
}

func TestHostLimitsExpireAndChangeOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "hosts.json")
	recordLimit(path, "Example.test:443", 3)
	if got := learnedLimit(path, "example.test:443"); got != 3 {
		t.Fatalf("read back %d", got)
	}
	fi, _ := os.Stat(path)
	time.Sleep(20 * time.Millisecond)
	recordLimit(path, "example.test:443", 3)
	if fi2, _ := os.Stat(path); !fi2.ModTime().Equal(fi.ModTime()) {
		t.Fatal("an unchanged limit rewrote the file")
	}
	old, _ := json.Marshal(map[string]hostLimit{"old.test": {Conns: 2, At: time.Now().Add(-25 * time.Hour)}})
	os.WriteFile(path, old, 0o644)
	if got := learnedLimit(path, "old.test"); got != 0 {
		t.Fatalf("a day-old limit still counts: %d", got)
	}
}

// the redirect's target serves the ranged requests until it expires, then the
// original link is asked again and hands out a fresh target
func TestRedirectTargetReusedThenOriginalAfter403(t *testing.T) {
	b := &blob{data: randBytes(1 << 20), etag: `"r"`, ranges: true}
	var orig, gen1 atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orig":
			n := orig.Add(1)
			http.Redirect(w, r, "/final?gen="+strconv.Itoa(int(n)), http.StatusFound)
		case "/final":
			if r.URL.Query().Get("gen") == "1" && gen1.Add(1) > 4 {
				http.Error(w, "expired", http.StatusForbidden)
				return
			}
			b.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	final, err := fetchTo(newDL(t, 1, 64<<10), srv.URL+"/orig", filepath.Join(t.TempDir(), "f.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if gen1.Load() < 4 {
		t.Fatalf("the redirect target saw %d requests, want the ranged ones", gen1.Load())
	}
	if n := orig.Load(); n != 2 {
		t.Fatalf("the original link was asked %d times, want the first and one after the 403", n)
	}
}

// a last range whose connection stalls is raced from its frontier and wins
func TestStalledLastRangeIsRaced(t *testing.T) {
	const size = 2 << 20
	b := &blob{data: randBytes(size), etag: `"t"`, ranges: true, trickle: 10 * time.Millisecond}
	// the first request covers the whole file, so the split piece is the first to start past 0
	var stalledAt, racedFrom atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if start := rangeStart(r); start > 0 {
			if stalledAt.CompareAndSwap(0, start) {
				w = &stallWriter{ResponseWriter: w, r: r, n: 8 << 10}
			} else if start > stalledAt.Load() {
				racedFrom.Store(start)
			}
		}
		b.ServeHTTP(w, r)
	}))
	defer srv.Close()
	// half the file is splittable once, the stalled half no more
	d := newDL(t, 2, 512<<10)
	d.tune.minReq = size
	began := time.Now()
	final, err := fetchTo(d, srv.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if stalledAt.Load() == 0 || racedFrom.Load() == 0 {
		t.Fatal("no range stalled or none was raced")
	}
	if took := time.Since(began); took > 15*time.Second {
		t.Fatalf("took %v, the stall was waited out", took)
	}
}

// stallWriter sends n bytes and then holds the connection open with nothing more
type stallWriter struct {
	http.ResponseWriter
	r *http.Request
	n int
}

func (w *stallWriter) Write(p []byte) (int, error) {
	if len(p) <= w.n {
		w.n -= len(p)
		return w.ResponseWriter.Write(p)
	}
	k, _ := w.ResponseWriter.Write(p[:w.n])
	w.n = 0
	_ = http.NewResponseController(w.ResponseWriter).Flush()
	<-w.r.Context().Done()
	return k, w.r.Context().Err()
}

// ramp-up: four connections first, the rest once those deliver bytes
func TestRampUpOpensFourFirst(t *testing.T) {
	b := &blob{data: randBytes(8 << 20), etag: `"u"`, ranges: true, trickle: time.Millisecond}
	gate := make(chan struct{})
	var inflight, peakGated atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inflight.Add(1)
		defer inflight.Add(-1)
		select {
		case <-gate:
		default:
			for {
				p := peakGated.Load()
				if cur <= p || peakGated.CompareAndSwap(p, cur) {
					break
				}
			}
			// the first connection streams, the others wait without a byte
			if rangeStart(r) != 0 {
				select {
				case <-gate:
				case <-r.Context().Done():
					return
				}
			}
		}
		b.ServeHTTP(w, r)
	}))
	defer srv.Close()
	errc := make(chan error, 1)
	go func() {
		_, err := fetchTo(newDL(t, 8, 64<<10), srv.URL+"/f.bin", filepath.Join(t.TempDir(), "f.bin"), &prog{})
		errc <- err
	}()
	waitFor(t, func() bool { return inflight.Load() >= 4 })
	time.Sleep(300 * time.Millisecond)
	if p := peakGated.Load(); p != 4 {
		t.Fatalf("%d connections before any but the first delivered, want 4", p)
	}
	b.peak.Store(0)
	close(gate)
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if p := b.peak.Load(); p <= 4 {
		t.Fatalf("never ramped past 4 connections (peak %d)", p)
	}
}

func TestPartitions(t *testing.T) {
	if !partitions([]span{{Start: 50, End: 100, Done: 10}, {Start: 0, End: 50, Done: 50}}, 100) {
		t.Fatal("a valid partition was rejected")
	}
	for _, s := range [][]span{
		{{Start: 0, End: 50}},
		{{Start: 0, End: 60}, {Start: 50, End: 100}},
		{{Start: 0, End: 50, Done: 51}, {Start: 50, End: 100}},
		{{Start: 0, End: 0}, {Start: 0, End: 100}},
	} {
		if partitions(s, 100) {
			t.Fatalf("accepted %+v", s)
		}
	}
}

// serveRange answers ranges from data with an ETag and ignores If-Range, like some servers do.
func serveRange(w http.ResponseWriter, r *http.Request, data []byte, etag string) {
	size := len(data)
	w.Header().Set("ETag", etag)
	v, ranged := strings.CutPrefix(r.Header.Get("Range"), "bytes=")
	if !ranged {
		w.Header().Set("Content-Length", strconv.Itoa(size))
		_, _ = w.Write(data)
		return
	}
	a, b, _ := strings.Cut(v, "-")
	s, _ := strconv.Atoi(a)
	e := size - 1
	if n, err := strconv.Atoi(b); err == nil && n < size {
		e = n
	}
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", s, e, size))
	w.Header().Set("Content-Length", strconv.Itoa(e-s+1))
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(data[s : e+1])
}

func writeSidecar(t *testing.T, dst string, sc sidecar) {
	t.Helper()
	b, _ := json.Marshal(sc)
	if err := os.WriteFile(dst+".part"+SidecarExt, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCrossHostRedirectTargetGetsNoCredentials(t *testing.T) {
	b := &blob{data: randBytes(1 << 20), etag: `"r"`, ranges: true}
	var withCreds, total atomic.Int32
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		total.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			withCreds.Add(1)
		}
		b.ServeHTTP(w, r)
	}))
	defer cdn.Close()
	// another host name for the same loopback server
	cdnURL := strings.Replace(cdn.URL, "127.0.0.1", "localhost", 1)
	var siteCreds atomic.Int32
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" && r.Header.Get("Cookie") != "" {
			siteCreds.Add(1)
		}
		http.Redirect(w, r, cdnURL+"/blob?sig=1", http.StatusFound)
	}))
	defer site.Close()
	h := http.Header{}
	h.Set("Authorization", "Bearer secret")
	h.Set("Cookie", "session=secret")
	final, err := newDL(t, 4, 64<<10).FetchFile(context.Background(), plan.Part{URL: site.URL + "/dl", Headers: h}, filepath.Join(t.TempDir(), "f.bin"), nil, &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
	if siteCreds.Load() == 0 {
		t.Fatal("the original host never got the credentials")
	}
	if total.Load() < 2 {
		t.Fatalf("the redirect target saw %d requests, want the ranged ones too", total.Load())
	}
	if n := withCreds.Load(); n > 0 {
		t.Fatalf("%d of %d requests to another host carried Authorization or Cookie", n, total.Load())
	}
}

// a server that ignores If-Range but shows a new ETag must not splice two files together
func TestRangeAnswerWithChangedETagRestarts(t *testing.T) {
	const size = 1 << 20
	old, cur := randBytes(size), randBytes(size)
	dst := filepath.Join(t.TempDir(), "f.bin")
	part := make([]byte, size)
	copy(part, old[:size/2])
	if err := os.WriteFile(dst+".part", part, 0o644); err != nil {
		t.Fatal(err)
	}
	writeSidecar(t, dst, sidecar{URL: "x", ETag: `"old"`, Size: size, Segments: []span{{Start: 0, End: size, Done: size / 2}}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveRange(w, r, cur, `"new"`)
	}))
	defer srv.Close()
	p := &prog{}
	final, err := fetchTo(newDL(t, 4, 64<<10), srv.URL+"/f.bin", dst, p)
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, cur)
	if p.resumed.Load() != 0 {
		t.Fatalf("kept %d bytes of the old file", p.resumed.Load())
	}
}

func TestValidatorlessResumeOnServerThatDroppedRanges(t *testing.T) {
	const size = 1 << 20
	data := randBytes(size)
	dst := filepath.Join(t.TempDir(), "f.bin")
	part := make([]byte, size)
	copy(part, data[:size/2])
	if err := os.WriteFile(dst+".part", part, 0o644); err != nil {
		t.Fatal(err)
	}
	writeSidecar(t, dst, sidecar{URL: "x", Size: size, Segments: []span{{Start: 0, End: size, Done: size / 2}}})
	b := &blob{data: data, ranges: false}
	srv := httptest.NewServer(b)
	defer srv.Close()
	final, err := fetchTo(newDL(t, 4, 64<<10), srv.URL+"/f.bin", dst, &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, data)
	if _, err := os.Stat(dst + ".part" + SidecarExt); !os.IsNotExist(err) {
		t.Fatal("sidecar left behind")
	}
}

// a racer the host turns away lowers the limit instead of being sent again at once
func TestRefusedRacerLowersTheLimit(t *testing.T) {
	const size = 2 << 20
	b := &blob{data: randBytes(size), etag: `"q"`, ranges: true, trickle: 10 * time.Millisecond}
	var stalledAt atomic.Int64
	var refused atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if start := rangeStart(r); start > 0 {
			if !stalledAt.CompareAndSwap(0, start) {
				refused.Add(1)
				http.Error(w, "one connection only", http.StatusTooManyRequests)
				return
			}
			w = &stallWriter{ResponseWriter: w, r: r, n: 8 << 10}
		}
		b.ServeHTTP(w, r)
	}))
	defer srv.Close()
	d := newDL(t, 2, 512<<10)
	d.tune.minReq = size
	d.tune.raceSecs, d.tune.raceWarmup = 0.01, 50*time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := d.FetchFile(ctx, plan.Part{URL: srv.URL + "/f.bin"}, filepath.Join(t.TempDir(), "f.bin"), nil, &prog{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the stalled download to run until the deadline", err)
	}
	if n := refused.Load(); n != 1 {
		t.Fatalf("%d racers refused, want 1 and then none", n)
	}
	u, _ := url.Parse(srv.URL)
	if got := learnedLimit(d.HostsFile, u.Host); got != 1 {
		t.Fatalf("learned limit %d, want 1", got)
	}
}

// fallocate reserved the whole .part already, a resume needs no more room
func TestResumeNeedsRoomOnlyForUnreservedBlocks(t *testing.T) {
	const size = 1 << 20
	data := randBytes(size)
	dir := t.TempDir()
	dst := filepath.Join(dir, "f.bin")
	f, err := os.OpenFile(dst+".part", os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := allocate(f, size); err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteAt(data[:size/2], 0)
	f.Close()
	writeSidecar(t, dst, sidecar{URL: "x", ETag: `"same"`, Size: size, Segments: []span{{Start: 0, End: size, Done: size / 2}}})
	b := &blob{data: data, etag: `"same"`, ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	d := newDL(t, 4, 64<<10)
	// a volume with room for 1.2 files, counting the blocks the directory holds
	d.tune.free = func(string) (int64, error) {
		var used int64
		es, _ := os.ReadDir(dir)
		for _, e := range es {
			used += allocated(filepath.Join(dir, e.Name()))
		}
		return size*6/5 - used, nil
	}
	p := &prog{}
	final, err := fetchTo(d, srv.URL+"/f.bin", dst, p)
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, data)
	if p.resumed.Load() != size/2 {
		t.Fatalf("resumed %d bytes, want %d", p.resumed.Load(), size/2)
	}
}

// the full-size .part of a file that changed is gone before the new one asks for room
func TestRestartAfterChangeFreesTheStalePart(t *testing.T) {
	const size = 1 << 20
	dir := t.TempDir()
	dst := filepath.Join(dir, "f.bin")
	if err := os.WriteFile(dst+".part", randBytes(size), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSidecar(t, dst, sidecar{URL: "x", ETag: `"old"`, Size: size, Segments: []span{{Start: 0, End: size, Done: size / 2}}})
	b := &blob{data: randBytes(size), etag: `"new"`, ranges: true}
	srv := httptest.NewServer(b)
	defer srv.Close()
	d := newDL(t, 4, 64<<10)
	// a volume with room for 1.5 files, less what lies in the directory
	d.tune.free = func(string) (int64, error) {
		var used int64
		es, _ := os.ReadDir(dir)
		for _, e := range es {
			if fi, err := e.Info(); err == nil {
				used += fi.Size()
			}
		}
		return size*3/2 - used, nil
	}
	final, err := fetchTo(d, srv.URL+"/f.bin", dst, &prog{})
	if err != nil {
		t.Fatal(err)
	}
	sameFile(t, final, b.data)
}

// a stream without ranges that always dies at the same place restarts from zero each time,
// which is no progress, so it gives up instead of looping forever
func TestStreamWithoutRangesThatAlwaysDropsGivesUp(t *testing.T) {
	b := &blob{data: randBytes(300 << 10), ranges: false}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.ServeHTTP(&cutWriter{ResponseWriter: w, n: 100 << 10}, r)
	}))
	defer srv.Close()
	d := newDL(t, 1, 1<<20)
	d.tune.maxRetries = 3
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := d.FetchFile(ctx, plan.Part{URL: srv.URL + "/f.bin"}, filepath.Join(t.TempDir(), "f.bin"), nil, &prog{})
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the download to give up", err)
	}
	if n := b.reqs.Load(); n > int32(4*(d.tune.maxRetries+1)) {
		t.Fatalf("%d requests before giving up", n)
	}
}
