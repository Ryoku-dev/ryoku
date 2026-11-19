package hls

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoget/internal/plan"
)

type progress struct {
	mu      sync.Mutex
	total   int64
	added   int64
	resumed int64
	marks   map[int]float64
	count   int
}

func newProgress() *progress { return &progress{marks: map[int]float64{}} }

func (p *progress) SetTotal(n int64) { p.mu.Lock(); p.total = n; p.mu.Unlock() }
func (p *progress) Add(n int64)      { p.mu.Lock(); p.added += n; p.mu.Unlock() }
func (p *progress) Resumed(n int64)  { p.mu.Lock(); p.resumed += n; p.mu.Unlock() }
func (p *progress) Stage(string)     {}
func (p *progress) Mark(i, count int, fill float64) {
	p.mu.Lock()
	p.marks[i] = fill
	p.count = count
	p.mu.Unlock()
}

var key = []byte("0123456789abcdef")

func encrypt(t *testing.T, plain, iv []byte) []byte {
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	data := append(append([]byte(nil), plain...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, data)
	return out
}

func seqIV(n uint64) []byte {
	iv := make([]byte, 16)
	binary.BigEndian.PutUint64(iv[8:], n)
	return iv
}

type stream struct {
	srv      *httptest.Server
	plain    [][]byte
	hits     sync.Map
	flaky    atomic.Int32
	keyHits  atomic.Int32
	failFrom int
	failing  atomic.Bool
}

func segBody(i int) []byte {
	return bytes.Repeat([]byte(fmt.Sprintf("seg%03d|", i)), 300+i*7)
}

// newStream serves a master with two variants plus a separate audio
// rendition; the 1080p media playlist is AES-128 encrypted (explicit IV for
// the first half, sequence-number IV after), the 360p one has byte ranges.
func newStream(t *testing.T, n int) *stream {
	old := retryDelay
	retryDelay = time.Millisecond
	t.Cleanup(func() { retryDelay = old })
	s := &stream{failFrom: -1}
	for i := range n {
		s.plain = append(s.plain, segBody(i))
	}
	explicit := []byte("fedcba9876543210")
	enc := make([][]byte, n)
	for i := range n {
		iv := explicit
		if i >= n/2 {
			iv = seqIV(uint64(100 + i))
		}
		enc[i] = encrypt(t, s.plain[i], iv)
	}
	var joined []byte
	var offsets []int
	for i := range n {
		offsets = append(offsets, len(joined))
		joined = append(joined, s.plain[i]...)
	}
	h := http.NewServeMux()
	h.HandleFunc("/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `#EXTM3U
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="aud",NAME="English",DEFAULT=YES,AUTOSELECT=YES,URI="audio/en.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360,CODECS="avc1.4d401e,mp4a.40.2"
low/index.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=5000000,AVERAGE-BANDWIDTH=4500000,RESOLUTION=1920x1080,FRAME-RATE=29.970,CODECS="avc1.640028,mp4a.40.2",AUDIO="aud"
hi/index.m3u8?token=abc
#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=100000,URI="iframe.m3u8"
`)
	})
	h.HandleFunc("/hi/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "abc" {
			http.Error(w, "no token", http.StatusForbidden)
			return
		}
		var b strings.Builder
		b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:4\n#EXT-X-MEDIA-SEQUENCE:100\n#EXT-X-PLAYLIST-TYPE:VOD\n")
		b.WriteString(`#EXT-X-KEY:METHOD=AES-128,URI="/key.bin",IV=0x66656463626139383736353433323130` + "\n")
		for i := range n {
			if i == n/2 {
				b.WriteString(`#EXT-X-KEY:METHOD=AES-128,URI="/key.bin"` + "\n")
			}
			fmt.Fprintf(&b, "#EXTINF:4.0,\nseg%d.ts\n", i)
		}
		b.WriteString("#EXT-X-ENDLIST\n")
		w.Write([]byte(b.String()))
	})
	h.HandleFunc("/hi/", func(w http.ResponseWriter, r *http.Request) {
		var i int
		if _, err := fmt.Sscanf(filepath.Base(r.URL.Path), "seg%d.ts", &i); err != nil || i >= n {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Test") != "yes" {
			http.Error(w, "missing header", http.StatusForbidden)
			return
		}
		v, _ := s.hits.LoadOrStore(i, new(atomic.Int32))
		if v.(*atomic.Int32).Add(1) == 1 && i%3 == 1 {
			s.flaky.Add(1)
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		if s.failing.Load() && i >= s.failFrom {
			http.Error(w, "gone for now", http.StatusServiceUnavailable)
			return
		}
		w.Write(enc[i])
	})
	h.HandleFunc("/key.bin", func(w http.ResponseWriter, r *http.Request) {
		s.keyHits.Add(1)
		w.Write(key)
	})
	h.HandleFunc("/low/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		var b strings.Builder
		b.WriteString("#EXTM3U\n#EXT-X-VERSION:4\n#EXT-X-TARGETDURATION:4\n")
		for i := range n {
			fmt.Fprintf(&b, "#EXTINF:4.0,\n#EXT-X-BYTERANGE:%d@%d\nall.ts\n", len(s.plain[i]), offsets[i])
		}
		b.WriteString("#EXT-X-ENDLIST\n")
		w.Write([]byte(b.String()))
	})
	h.HandleFunc("/low/all.ts", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "all.ts", time.Time{}, bytes.NewReader(joined))
	})
	h.HandleFunc("/audio/en.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4.0,\na0.aac\n#EXT-X-ENDLIST\n")
	})
	h.HandleFunc("/audio/a0.aac", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("AUDIO")) })
	h.HandleFunc("/live.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4.0,\na.ts\n")
	})
	h.HandleFunc("/drm.m3u8", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://x\",KEYFORMAT=\"com.apple.streamingkeydelivery\"\n#EXTINF:4.0,\na.ts\n#EXT-X-ENDLIST\n")
	})
	s.srv = httptest.NewServer(h)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *stream) want(n int) []byte {
	var out []byte
	for i := range n {
		out = append(out, s.plain[i]...)
	}
	return out
}

func TestVariants(t *testing.T) {
	s := newStream(t, 4)
	vs, err := Variants(context.Background(), nil, s.srv.URL+"/master.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 3 {
		t.Fatalf("variants = %+v", vs)
	}
	hi := vs[0]
	if hi.Height != 1080 || hi.Bandwidth != 4500000 || hi.VideoCodec != "avc1.640028" || hi.AudioURL != s.srv.URL+"/audio/en.m3u8" ||
		hi.URL != s.srv.URL+"/hi/index.m3u8?token=abc" || hi.FPS < 29.9 {
		t.Fatalf("hi = %+v", hi)
	}
	if vs[1].Height != 360 || vs[1].AudioURL != "" || !vs[2].AudioOnly {
		t.Fatalf("order = %+v", vs)
	}
	media, err := Variants(context.Background(), nil, s.srv.URL+"/low/index.m3u8", nil)
	if err != nil || len(media) != 1 || media[0].URL != s.srv.URL+"/low/index.m3u8" {
		t.Fatalf("media playlist variants = %+v, %v", media, err)
	}
}

func TestFetchMasterEncrypted(t *testing.T) {
	const n = 20
	s := newStream(t, n)
	dst := filepath.Join(t.TempDir(), "out.ts")
	p := newProgress()
	part := plan.Part{Kind: plan.PartHLS, URL: s.srv.URL + "/master.m3u8", Headers: http.Header{"X-Test": {"yes"}}}
	if err := Fetch(context.Background(), &plan.Env{}, part, dst, p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, s.want(n)) {
		t.Fatalf("output differs: %d bytes, want %d", len(got), len(s.want(n)))
	}
	if s.flaky.Load() == 0 {
		t.Fatal("no flaky segment exercised")
	}
	if p.count != n || len(p.marks) != n || p.total != int64(len(got)) {
		t.Fatalf("progress count %d marks %d total %d", p.count, len(p.marks), p.total)
	}
	if s.keyHits.Load() != 1 {
		t.Fatalf("key fetched %d times", s.keyHits.Load())
	}
	if _, err := os.Stat(dst + stateSuffix); !os.IsNotExist(err) {
		t.Fatal("state sidecar left behind")
	}
}

func TestFetchNamedVariantByteRange(t *testing.T) {
	const n = 6
	s := newStream(t, n)
	dst := filepath.Join(t.TempDir(), "low.ts")
	part := plan.Part{Kind: plan.PartHLS, URL: s.srv.URL + "/master.m3u8", Args: []string{"height=480"}}
	if err := Fetch(context.Background(), nil, part, dst, newProgress()); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, s.want(n)) {
		t.Fatalf("byte range output differs")
	}
	adst := filepath.Join(t.TempDir(), "audio.ts")
	if err := Fetch(context.Background(), nil, plan.Part{URL: s.srv.URL + "/master.m3u8", Args: []string{"audio"}}, adst, newProgress()); err != nil {
		t.Fatal(err)
	}
	if a, _ := os.ReadFile(adst); string(a) != "AUDIO" {
		t.Fatalf("audio rendition = %q", a)
	}
}

func TestFetchResumes(t *testing.T) {
	const n = 24
	s := newStream(t, n)
	dst := filepath.Join(t.TempDir(), "out.ts")
	part := plan.Part{URL: s.srv.URL + "/hi/index.m3u8?token=abc", Headers: http.Header{"X-Test": {"yes"}}}
	s.failFrom = 17
	s.failing.Store(true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := Fetch(ctx, nil, part, dst, newProgress())
	if err == nil {
		t.Fatal("fetch passed a failing segment")
	}
	st := readState(dst)
	if st.Done != 16 || st.Count != n {
		t.Fatalf("state after failure = %+v", st)
	}
	s.failing.Store(false)
	p := newProgress()
	if err := Fetch(context.Background(), nil, part, dst, p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if !bytes.Equal(got, s.want(n)) {
		t.Fatalf("resumed output differs: %d vs %d", len(got), len(s.want(n)))
	}
	if p.resumed != st.Offset || p.added != int64(len(got))-st.Offset {
		t.Fatalf("resumed %d added %d, offset %d total %d", p.resumed, p.added, st.Offset, len(got))
	}
}

func TestFetchRefusesLiveAndDRM(t *testing.T) {
	s := newStream(t, 2)
	if err := Fetch(context.Background(), nil, plan.Part{URL: s.srv.URL + "/live.m3u8"}, filepath.Join(t.TempDir(), "x"), newProgress()); !errors.Is(err, ErrLive) {
		t.Fatalf("live err = %v", err)
	}
	if err := Fetch(context.Background(), nil, plan.Part{URL: s.srv.URL + "/drm.m3u8"}, filepath.Join(t.TempDir(), "x"), newProgress()); !errors.Is(err, ErrDRM) {
		t.Fatalf("drm err = %v", err)
	}
}

type countLimiter struct{ n atomic.Int64 }

func (c *countLimiter) WaitN(ctx context.Context, n int) error {
	if n > 16<<10 {
		return errors.New("burst exceeded")
	}
	c.n.Add(int64(n))
	return ctx.Err()
}

func TestFetchUsesLimiter(t *testing.T) {
	s := newStream(t, 3)
	lim := &countLimiter{}
	part := plan.Part{URL: s.srv.URL + "/low/index.m3u8"}
	if err := Fetch(context.Background(), &plan.Env{Limit: lim}, part, filepath.Join(t.TempDir(), "x"), newProgress()); err != nil {
		t.Fatal(err)
	}
	if lim.n.Load() == 0 {
		t.Fatal("limiter never consulted")
	}
}

func TestDecryptRejectsBadPadding(t *testing.T) {
	data := encrypt(t, []byte("hello"), seqIV(1))
	if _, err := decrypt(data, []byte("ffffffffffffffff"), seqIV(1)); err == nil {
		t.Fatal("wrong key decrypted")
	}
	if out, err := decrypt(data, key, seqIV(1)); err != nil || string(out) != "hello" {
		t.Fatalf("decrypt = %q, %v", out, err)
	}
}
