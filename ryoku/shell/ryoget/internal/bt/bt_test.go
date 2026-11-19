package bt

import (
	"bytes"
	"context"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"
)

const pieceLen = 16 << 10

type payload struct {
	seedDir string
	path    string
	raw     []byte
	mi      *metainfo.MetaInfo
	info    metainfo.Info
	data    map[string][]byte
}

// makePayload writes files of the given sizes (in pieces) under
// <tmp>/seed/payload and a .torrent for them.
func makePayload(t *testing.T, pieces map[string]int) payload {
	t.Helper()
	tmp := t.TempDir()
	p := payload{seedDir: filepath.Join(tmp, "seed"), data: map[string][]byte{}}
	root := filepath.Join(p.seedDir, "payload")
	rng := rand.New(rand.NewPCG(uint64(len(pieces)), 7))
	for name, n := range pieces {
		b := make([]byte, n*pieceLen)
		for i := range b {
			b[i] = byte(rng.Uint32())
		}
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, b, 0o644); err != nil {
			t.Fatal(err)
		}
		p.data[name] = b
	}
	p.info = metainfo.Info{PieceLength: pieceLen}
	if err := p.info.BuildFromFilePath(root); err != nil {
		t.Fatal(err)
	}
	ib, err := bencode.Marshal(p.info)
	if err != nil {
		t.Fatal(err)
	}
	p.mi = &metainfo.MetaInfo{InfoBytes: ib}
	var buf bytes.Buffer
	if err := p.mi.Write(&buf); err != nil {
		t.Fatal(err)
	}
	p.raw = buf.Bytes()
	p.path = filepath.Join(tmp, "payload.torrent")
	if err := os.WriteFile(p.path, p.raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func (p payload) index(t *testing.T, name string) int {
	t.Helper()
	for i, f := range infoFiles(&p.info) {
		if f.Path == name {
			return i
		}
	}
	t.Fatalf("no file %q", name)
	return -1
}

func (p payload) magnet() string {
	ih := p.mi.HashInfoBytes()
	return p.mi.Magnet(&ih, &p.info).String()
}

func newClient(t *testing.T, cfg Config, peers ...*Client) *Client {
	t.Helper()
	if cfg.StateDir == "" {
		cfg.StateDir = t.TempDir()
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	link(c, peers...)
	return c
}

func link(c *Client, peers ...*Client) {
	c.addPeers = func(tor *torrent.Torrent) {
		for _, p := range peers {
			tor.AddClientPeer(p.cl)
		}
	}
}

func wait(t *testing.T, ch <-chan struct{}, d time.Duration, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(d):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func eventually(t *testing.T, d time.Duration, what string, ok func() bool) {
	t.Helper()
	end := time.Now().Add(d)
	for !ok() {
		if time.Now().After(end) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func seed(t *testing.T, p payload) (*Client, *Torrent) {
	t.Helper()
	c := newClient(t, Config{Seed: SeedPolicy{On: true}})
	tt, err := c.Add(context.Background(), p.path, p.seedDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, tt.Completed(), 10*time.Second, "seeder check")
	return c, tt
}

func sameFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from the seeded data", path)
	}
}

func absent(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s should not exist (err %v)", p, err)
		}
	}
}

func completePieces(tt *Torrent) (n int) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	if tt.tor == nil {
		return 0
	}
	for _, r := range tt.tor.PieceStateRuns() {
		if r.Complete {
			n += r.Length
		}
	}
	return
}

func TestSelectedFilesOnly(t *testing.T) {
	p := makePayload(t, map[string]int{"a.bin": 3, "b.bin": 2, "c/d.bin": 1})
	sc, st := seed(t, p)
	if s := st.Stats(); !s.Complete || !s.Seeding || s.Done != s.Size {
		t.Fatalf("seeder stats %+v", s)
	}
	lc := newClient(t, Config{}, sc)
	dir := t.TempDir()
	a, b, d := p.index(t, "a.bin"), p.index(t, "b.bin"), p.index(t, "c/d.bin")
	lt, err := lc.Add(context.Background(), p.path, dir, []int{a, d})
	if err != nil {
		t.Fatal(err)
	}
	wait(t, lt.Finished(), 20*time.Second, "leecher finish")
	root := filepath.Join(dir, "payload")
	sameFile(t, filepath.Join(root, "a.bin"), p.data["a.bin"])
	sameFile(t, filepath.Join(root, "c", "d.bin"), p.data["c/d.bin"])
	absent(t, filepath.Join(root, "b.bin"), filepath.Join(root, "b.bin.part"))

	s := lt.Stats()
	want := int64(len(p.data["a.bin"]) + len(p.data["c/d.bin"]))
	if s.Size != want || s.Done != want || !s.Complete || s.Seeding {
		t.Fatalf("leecher stats %+v, want size %d", s, want)
	}
	files := lt.Files()
	if files[b].Selected || files[b].Done != 0 || !files[a].Selected || files[a].Done != files[a].Size {
		t.Fatalf("files %+v", files)
	}
	n := p.info.NumPieces()
	var bPieces [2]int
	off := 0
	for i, f := range p.info.UpvertedFiles() {
		if i == b {
			bPieces = [2]int{off / pieceLen, (off + int(f.Length)) / pieceLen}
		}
		off += int(f.Length)
	}
	for k, v := range s.Pieces {
		piece := k * n / len(s.Pieces)
		inB := piece >= bPieces[0] && piece < bPieces[1]
		if inB && v != 0 || !inB && v != 255 {
			t.Fatalf("bucket %d (piece %d) = %d", k, piece, v)
		}
	}
}

func TestPauseResume(t *testing.T) {
	p := makePayload(t, map[string]int{"big.bin": 40})
	sc, _ := seed(t, p)
	lc := newClient(t, Config{Down: rate.NewLimiter(256<<10, 64<<10)}, sc)
	dir := t.TempDir()
	lt, err := lc.Add(context.Background(), p.path, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, 10*time.Second, "first bytes", func() bool { return lt.Stats().Done > 0 })
	lt.Pause()
	if !lt.Paused() {
		t.Fatal("not paused")
	}
	before := lt.Stats()
	time.Sleep(500 * time.Millisecond)
	after := lt.Stats()
	if after.Done != before.Done || after.Down != 0 || after.Conns != 0 || after.Done >= after.Size {
		t.Fatalf("paused torrent moved or finished: before %+v after %+v", before, after)
	}
	lt.Resume()
	wait(t, lt.Finished(), 30*time.Second, "finish after resume")
	sameFile(t, filepath.Join(dir, "payload", "big.bin"), p.data["big.bin"])
}

func TestResumeFromPersistedCompletion(t *testing.T) {
	p := makePayload(t, map[string]int{"big.bin": 40})
	sc, _ := seed(t, p)
	state, dir := t.TempDir(), t.TempDir()
	c1, err := New(Config{StateDir: state, Down: rate.NewLimiter(256<<10, 64<<10)})
	if err != nil {
		t.Fatal(err)
	}
	link(c1, sc)
	t1, err := c1.Add(context.Background(), p.path, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, 15*time.Second, "some pieces", func() bool { return completePieces(t1) >= 8 })
	had := completePieces(t1)
	if had == p.info.NumPieces() {
		t.Fatal("finished before the restart; slow the limiter")
	}
	if err := c1.Close(); err != nil {
		t.Fatal(err)
	}

	c2 := newClient(t, Config{StateDir: state})
	t2, err := c2.Add(context.Background(), p.path, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, t2.Metadata(), 5*time.Second, "metadata from kept state")
	time.Sleep(300 * time.Millisecond)
	if got := completePieces(t2); got < had {
		t.Fatalf("restart kept %d of %d complete pieces", got, had)
	}
	t2.mu.Lock()
	st := t2.tor.Stats()
	t2.mu.Unlock()
	if hashed := st.BytesHashed.Int64(); hashed != 0 {
		t.Fatalf("restart rehashed %d bytes", hashed)
	}
	if d := t2.Stats().Done; d < int64(had)*pieceLen {
		t.Fatalf("done %d after restart, want at least %d", d, had*pieceLen)
	}
	link(c2, sc)
	t2.Pause()
	t2.Resume()
	wait(t, t2.Finished(), 30*time.Second, "finish after restart")
	sameFile(t, filepath.Join(dir, "payload", "big.bin"), p.data["big.bin"])
}

func TestSeedingStopsAtRatio(t *testing.T) {
	p := makePayload(t, map[string]int{"r.bin": 8})
	size := int64(8 * pieceLen)
	sc, _ := seed(t, p)
	state, dir := t.TempDir(), t.TempDir()
	policy := SeedPolicy{On: true, Ratio: 1}
	bc, err := New(Config{StateDir: state, Seed: policy})
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	link(bc, sc)
	bt, err := bc.Add(context.Background(), p.path, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, bt.Completed(), 20*time.Second, "middle download")
	if err := sc.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-bt.Finished():
		t.Fatal("stopped seeding before reaching the ratio")
	default:
	}
	if s := bt.Stats(); !s.Seeding {
		t.Fatalf("not seeding: %+v", s)
	}

	cc := newClient(t, Config{}, bc)
	ct, err := cc.Add(context.Background(), p.path, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, ct.Finished(), 20*time.Second, "last download")
	wait(t, bt.Finished(), 10*time.Second, "seeding to stop at ratio 1")
	s := bt.Stats()
	if s.Uploaded < size || s.Ratio < 1 || s.Seeding || s.Conns != 0 {
		t.Fatalf("stats after seeding %+v", s)
	}
	if err := bc.Close(); err != nil {
		t.Fatal(err)
	}

	again := newClient(t, Config{StateDir: state, Seed: policy})
	at, err := again.Add(context.Background(), p.path, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if up := at.Stats().Uploaded; up < size {
		t.Fatalf("uploaded %d not kept across restart", up)
	}
	wait(t, at.Finished(), 5*time.Second, "kept ratio to end seeding at once")
}

func TestMagnetResolveThenAdd(t *testing.T) {
	p := makePayload(t, map[string]int{"one.bin": 2, "two.bin": 3})
	sc, _ := seed(t, p)
	lc := newClient(t, Config{}, sc)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	name, files, err := lc.Resolve(ctx, p.magnet())
	if err != nil {
		t.Fatal(err)
	}
	if name != "payload" || len(files) != 2 {
		t.Fatalf("resolved %q %+v", name, files)
	}
	two := p.index(t, "two.bin")
	dir := t.TempDir()
	lt, err := lc.Add(ctx, p.magnet(), dir, []int{two})
	if err != nil {
		t.Fatal(err)
	}
	wait(t, lt.Metadata(), 2*time.Second, "kept metadata")
	wait(t, lt.Finished(), 20*time.Second, "magnet download")
	sameFile(t, filepath.Join(dir, "payload", "two.bin"), p.data["two.bin"])
	absent(t, filepath.Join(dir, "payload", "one.bin"), filepath.Join(dir, "payload", "one.bin.part"))
	if lt.Name() != "payload" || lt.Path() != filepath.Join(dir, "payload") {
		t.Fatalf("name %q path %q", lt.Name(), lt.Path())
	}
}

func TestMagnetAddBeforeMetadata(t *testing.T) {
	p := makePayload(t, map[string]int{"m.bin": 4})
	sc, _ := seed(t, p)
	lc := newClient(t, Config{}, sc)
	dir := t.TempDir()
	lt, err := lc.Add(context.Background(), p.magnet(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, lt.Metadata(), 15*time.Second, "metadata from the swarm")
	wait(t, lt.Finished(), 20*time.Second, "magnet download")
	sameFile(t, filepath.Join(dir, "payload", "m.bin"), p.data["m.bin"])
	if _, err := os.Stat(lc.metainfoPath(p.mi.HashInfoBytes())); err != nil {
		t.Fatalf("metainfo not kept: %v", err)
	}
}

func TestTorrentURLFetchedOnce(t *testing.T) {
	p := makePayload(t, map[string]int{"u.bin": 2})
	sc, _ := seed(t, p)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(p.raw)
	}))
	src := srv.URL + "/payload.torrent"
	state, dir := t.TempDir(), t.TempDir()
	c1, err := New(Config{StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	link(c1, sc)
	t1, err := c1.Add(context.Background(), src, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, t1.Finished(), 20*time.Second, "url download")
	c1.Close()
	srv.Close()

	c2 := newClient(t, Config{StateDir: state})
	t2, err := c2.Add(context.Background(), src, dir, nil)
	if err != nil {
		t.Fatalf("re-add after restart: %v", err)
	}
	wait(t, t2.Completed(), 5*time.Second, "kept torrent complete")
	if hits.Load() != 1 {
		t.Fatalf("fetched the .torrent %d times", hits.Load())
	}
}

func TestDropDeletesData(t *testing.T) {
	p := makePayload(t, map[string]int{"x/y.bin": 2, "z.bin": 1})
	sc, _ := seed(t, p)
	lc := newClient(t, Config{}, sc)
	dir := t.TempDir()
	lt, err := lc.Add(context.Background(), p.path, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, lt.Finished(), 20*time.Second, "download")
	ih := p.mi.HashInfoBytes()
	if err := lt.Drop(true); err != nil {
		t.Fatal(err)
	}
	absent(t, filepath.Join(dir, "payload"), lc.metainfoPath(ih), lc.sidecarPath(ih))
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("download folder itself removed: %v", err)
	}
	eventually(t, 5*time.Second, "completion purge", func() bool {
		lc.mu.Lock()
		defer lc.mu.Unlock()
		return len(lc.purging) == 0
	})
	c, _ := lc.pc.Get(metainfo.PieceKey{InfoHash: ih, Index: 0})
	if c.Ok {
		t.Fatalf("piece completion kept after drop: %+v", c)
	}
}

func TestAddRejectsGarbage(t *testing.T) {
	c := newClient(t, Config{})
	bad := filepath.Join(t.TempDir(), "bad.torrent")
	os.WriteFile(bad, []byte("not bencode"), 0o644)
	for _, src := range []string{bad, "magnet:?xt=urn:nothing", filepath.Join(t.TempDir(), "missing.torrent")} {
		if _, err := c.Add(context.Background(), src, t.TempDir(), nil); err == nil {
			t.Fatalf("Add(%q) accepted", src)
		}
	}
	if _, err := c.Add(context.Background(), bad, t.TempDir(), []int{}); err == nil {
		t.Fatal("empty selection accepted")
	}
}

func run(n int, complete, partial bool) torrent.PieceStateRun {
	return torrent.PieceStateRun{
		PieceState: torrent.PieceState{Completion: storage.Completion{Complete: complete, Ok: true}, Partial: partial},
		Length:     n,
	}
}

func TestBuckets(t *testing.T) {
	half := func(int) float64 { return 0.5 }
	for _, tc := range []struct {
		name string
		runs torrent.PieceStateRuns
		want func(b int) byte
	}{
		{"none", nil, func(int) byte { return 0 }},
		{"few pieces", torrent.PieceStateRuns{run(5, true, false), run(5, false, false)},
			func(b int) byte { return map[bool]byte{true: 255}[b*10/64 < 5] }},
		{"many pieces", torrent.PieceStateRuns{run(500, true, false), run(500, false, false)},
			func(b int) byte { return map[bool]byte{true: 255}[b < 32] }},
		{"partial", torrent.PieceStateRuns{run(2, false, true)}, func(int) byte { return 128 }},
		{"mixed bucket", torrent.PieceStateRuns{run(1, true, false), run(1, false, false), run(126, true, false)},
			func(b int) byte { return map[bool]byte{true: 128, false: 255}[b == 0] }},
	} {
		got := buckets(tc.runs, half)
		for b, v := range got {
			if v != tc.want(b) {
				t.Fatalf("%s: bucket %d = %d, want %d", tc.name, b, v, tc.want(b))
			}
		}
	}
}

func TestPublicTrackerList(t *testing.T) {
	var hits atomic.Int32
	fail := atomic.Bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if fail.Load() {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		w.Write([]byte("udp://a.example:1337/announce\n\nhttps://b.example/announce\nnot a tracker\nudp://a.example:1337/announce\n"))
	}))
	defer srv.Close()
	old := publicTrackersURL
	publicTrackersURL = srv.URL
	defer func() { publicTrackersURL = old }()

	dir := t.TempDir()
	got := make(chan []string, 4)
	tl := newTrackerList(dir, srv.Client())
	tl.with(func(l []string) { got <- l })
	want := []string{"udp://a.example:1337/announce", "https://b.example/announce"}
	select {
	case l := <-got:
		if !slices.Equal(l, want) {
			t.Fatalf("list %q", l)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no list")
	}
	tl.with(func(l []string) { got <- l })
	if l := <-got; len(l) != 2 || hits.Load() != 1 {
		t.Fatalf("fresh list refetched: %d hits", hits.Load())
	}
	tl.close()

	old2 := time.Now().Add(-48 * time.Hour)
	os.Chtimes(filepath.Join(dir, trackersFile), old2, old2)
	fail.Store(true)
	tl = newTrackerList(dir, srv.Client())
	tl.with(func(l []string) { got <- l })
	select {
	case l := <-got:
		if len(l) != 2 {
			t.Fatalf("stale list lost on failure: %q", l)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no stale list after failure")
	}
	tl.with(func(l []string) { got <- l })
	<-got
	if hits.Load() != 2 {
		t.Fatalf("failed refresh retried at once: %d hits", hits.Load())
	}
	tl.close()
	b, _ := os.ReadFile(filepath.Join(dir, trackersFile))
	if !strings.Contains(string(b), "https://b.example/announce") {
		t.Fatalf("cache file %q", b)
	}
}

func TestForgetWithoutClient(t *testing.T) {
	p := makePayload(t, map[string]int{"f.bin": 3})
	sc, _ := seed(t, p)
	state, dir := t.TempDir(), t.TempDir()
	c1, err := New(Config{StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	link(c1, sc)
	t1, err := c1.Add(context.Background(), p.path, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, t1.Finished(), 20*time.Second, "download")
	ih := p.mi.HashInfoBytes()
	metaPath, sidePath := c1.metainfoPath(ih), c1.sidecarPath(ih)
	c1.Close()

	if err := Forget(state, p.path); err != nil {
		t.Fatal(err)
	}
	absent(t, metaPath, sidePath)
	sameFile(t, filepath.Join(dir, "payload", "f.bin"), p.data["f.bin"])
	c2 := newClient(t, Config{StateDir: state})
	if got, _ := c2.pc.Get(metainfo.PieceKey{InfoHash: ih, Index: 0}); got.Ok {
		t.Fatalf("piece completion kept: %+v", got)
	}
	if err := Forget(state, p.magnet()); err != nil {
		t.Fatalf("forgetting an unknown torrent: %v", err)
	}
}

func TestClientForgetsTorrentItDoesNotHold(t *testing.T) {
	p := makePayload(t, map[string]int{"g.bin": 3})
	q := makePayload(t, map[string]int{"h.bin": 2, "i.bin": 1})
	sp, _ := seed(t, p)
	sq, _ := seed(t, q)
	state, dir := t.TempDir(), t.TempDir()
	c1, err := New(Config{StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	link(c1, sp)
	t1, err := c1.Add(context.Background(), p.path, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, t1.Finished(), 20*time.Second, "download")
	c1.Close()

	c2 := newClient(t, Config{StateDir: state}, sq)
	held, err := c2.Add(context.Background(), q.path, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	wait(t, held.Completed(), 20*time.Second, "held download")
	if err := c2.Forget(q.path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c2.metainfoPath(q.mi.HashInfoBytes())); err != nil {
		t.Fatalf("forgot a torrent the client holds: %v", err)
	}

	ih := p.mi.HashInfoBytes()
	if err := c2.Forget(p.path); err != nil {
		t.Fatal(err)
	}
	absent(t, c2.metainfoPath(ih), c2.sidecarPath(ih))
	sameFile(t, filepath.Join(dir, "payload", "g.bin"), p.data["g.bin"])
	eventually(t, 5*time.Second, "completion purge", func() bool {
		c2.mu.Lock()
		defer c2.mu.Unlock()
		return len(c2.purging) == 0
	})
	if got, _ := c2.pc.Get(metainfo.PieceKey{InfoHash: ih, Index: 0}); got.Ok {
		t.Fatalf("piece completion kept: %+v", got)
	}
	c2.mu.Lock()
	_, known := c2.sources[p.path]
	c2.mu.Unlock()
	if known {
		t.Fatal("source still mapped after forget")
	}
}
