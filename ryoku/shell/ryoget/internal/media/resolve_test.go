package media

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoget/internal/plan"
)

type testService struct{ err error }

func (testService) ID() string            { return "test" }
func (testService) Name() string          { return "Test" }
func (testService) Match(u *url.URL) bool { return strings.HasPrefix(u.Path, "/svc/") }
func (s testService) Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*Media, error) {
	switch {
	case strings.HasSuffix(u.Path, "/ok"):
		return &Media{Kind: plan.KindMedia, Title: "native", Formats: []Format{{ID: "x"}}}, nil
	case strings.HasSuffix(u.Path, "/private"):
		return nil, fmt.Errorf("%w: private", ErrPrivate)
	case strings.HasSuffix(u.Path, "/settled"):
		return nil, Settled(fmt.Errorf("%w: tried", ErrUnavailable))
	case strings.HasSuffix(u.Path, "/ctx"):
		if !YTDLPAllowed(ctx) {
			return nil, Settled(errors.New("ctx flag missing"))
		}
		return &Media{Kind: plan.KindMedia, Title: "flag seen"}, nil
	}
	return nil, fmt.Errorf("%w: native broke", ErrUnsupported)
}

var fallbackCalls atomic.Int32

func init() {
	Register(testService{})
}

func withFakeFallback(t *testing.T) {
	old := Fallback()
	fallbackCalls.Store(0)
	RegisterFallback(func(ctx context.Context, u *url.URL, env *plan.Env) (*Media, error) {
		fallbackCalls.Add(1)
		if strings.Contains(u.Path, "nothing") {
			return nil, fmt.Errorf("%w: yt-dlp knows nothing", ErrUnsupported)
		}
		return &Media{Kind: plan.KindMedia, Title: "via yt-dlp", Formats: []Format{{ID: "y"}}}, nil
	})
	t.Cleanup(func() { RegisterFallback(old) })
}

func testSite(t *testing.T) *httptest.Server {
	h := http.NewServeMux()
	h.HandleFunc("/files/report.pdf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="Q3 report.pdf"`)
		http.ServeContent(w, r, "", timeZero(), strings.NewReader(strings.Repeat("x", 5000)))
	})
	h.HandleFunc("/dl", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("no ranges here"))
	})
	h.HandleFunc("/blob", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("\x89PNG\r\n\x1a\n...."))
	})
	h.HandleFunc("/live/master.m3u8", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		fmt.Fprint(w, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=900000,RESOLUTION=1280x720,CODECS=\"avc1.4d401f,mp4a.40.2\"\nv720.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=400000,RESOLUTION=640x360\nv360.m3u8\n")
	})
	h.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Fallback title</title>
<meta property="og:title" content="A &amp; B clip">
<meta property="og:image" content="/thumb.jpg">
<meta property="og:video:type" content="video/mp4">
<meta content="https://cdn.example/clip.mp4" property="og:video:secure_url">
</head></html>`)
	})
	h.HandleFunc("/embed-only", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Just words</title><meta property="og:video" content="https://player.example/e/1"><meta property="og:video:type" content="text/html"></head></html>`)
	})
	h.HandleFunc("/nothing", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><head><title> Plain page </title></head><body>hi</body></html>`)
	})
	h.HandleFunc("/gone", http.NotFound)
	h.HandleFunc("/svc/", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "should not be fetched", 500) })
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveDirect(t *testing.T) {
	srv := testSite(t)
	env := &plan.Env{HTTP: srv.Client()}
	m, err := Resolve(context.Background(), srv.URL+"/files/report.pdf", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != plan.KindFile || m.Name != "Q3 report.pdf" || m.Size != 5000 || !m.Resumable || m.Mime != "application/pdf" {
		t.Fatalf("file = %+v", m)
	}
	m, err = Resolve(context.Background(), srv.URL+"/dl", env, false)
	if err != nil || m.Resumable || m.Size != 14 || m.Name != "dl" {
		t.Fatalf("plain = %+v, %v", m, err)
	}
	m, err = Resolve(context.Background(), srv.URL+"/blob", env, false)
	if err != nil || m.Name != "blob.png" {
		t.Fatalf("blob = %+v, %v", m, err)
	}
	if _, err := Resolve(context.Background(), srv.URL+"/gone", env, true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("404 err = %v", err)
	}
}

func TestResolveHLSAndOpenGraph(t *testing.T) {
	srv := testSite(t)
	env := &plan.Env{HTTP: srv.Client()}
	m, err := Resolve(context.Background(), srv.URL+"/live/master.m3u8", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != plan.KindMedia || len(m.Formats) != 2 || m.Formats[0].Height != 720 ||
		m.Formats[0].Parts[0].Kind != plan.PartHLS || m.Formats[0].Parts[0].URL != srv.URL+"/live/v720.m3u8" {
		t.Fatalf("hls = %+v", m)
	}
	m, err = Resolve(context.Background(), srv.URL+"/post#frag", env, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "A & B clip" || m.Thumb != srv.URL+"/thumb.jpg" || len(m.Formats) != 1 ||
		m.Formats[0].Parts[0].URL != "https://cdn.example/clip.mp4" || m.Formats[0].Parts[0].Headers.Get("Referer") != srv.URL+"/post" {
		t.Fatalf("og = %+v", m)
	}
}

func TestResolvePageAndFallback(t *testing.T) {
	srv := testSite(t)
	env := &plan.Env{HTTP: srv.Client()}
	withFakeFallback(t)

	m, err := Resolve(context.Background(), srv.URL+"/nothing", env, false)
	if !errors.Is(err, ErrUnsupported) || m == nil || m.Kind != plan.KindPage || m.Title != "Plain page" {
		t.Fatalf("page = %+v, %v", m, err)
	}
	if fallbackCalls.Load() != 0 {
		t.Fatal("fallback ran while not allowed")
	}
	m, err = Resolve(context.Background(), srv.URL+"/embed-only", env, true)
	if err != nil || m.Title != "via yt-dlp" || fallbackCalls.Load() != 1 {
		t.Fatalf("embed fallback = %+v, %v", m, err)
	}
	m, err = Resolve(context.Background(), srv.URL+"/nothing", env, true)
	if !errors.Is(err, ErrUnsupported) || m.Kind != plan.KindPage {
		t.Fatalf("fallback miss = %+v, %v", m, err)
	}
}

func TestResolveServiceChain(t *testing.T) {
	srv := testSite(t)
	env := &plan.Env{HTTP: srv.Client()}
	withFakeFallback(t)
	cases := []struct {
		path      string
		allow     bool
		title     string
		err       error
		fallbacks int32
	}{
		{"/svc/ok", true, "native", nil, 0},
		{"/svc/broken", true, "via yt-dlp", nil, 1},
		{"/svc/broken", false, "", ErrUnsupported, 0},
		{"/svc/private", true, "", ErrPrivate, 0},
		{"/svc/settled", true, "", ErrUnavailable, 0},
		{"/svc/ctx", true, "flag seen", nil, 0},
	}
	for _, c := range cases {
		fallbackCalls.Store(0)
		m, err := Resolve(context.Background(), srv.URL+c.path, env, c.allow)
		if c.err != nil {
			if !errors.Is(err, c.err) {
				t.Errorf("%s: err = %v; want %v", c.path, err, c.err)
			}
		} else if err != nil || m.Title != c.title || m.URL != srv.URL+c.path {
			t.Errorf("%s: %+v, %v", c.path, m, err)
		}
		if fallbackCalls.Load() != c.fallbacks {
			t.Errorf("%s: %d fallbacks", c.path, fallbackCalls.Load())
		}
	}
}

func TestResolveRefusalsAndTorrents(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://host/x", "javascript:alert(1)", "", "   ", "not a link at all"} {
		if _, err := Resolve(context.Background(), raw, nil, true); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: err = %v", raw, err)
		}
	}
	m, err := Resolve(context.Background(), "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=Big+Buck+Bunny", nil, false)
	if err != nil || m.Kind != plan.KindTorrent || m.Title != "Big Buck Bunny" || m.Source.ID != "torrent" {
		t.Fatalf("magnet = %+v, %v", m, err)
	}
	if _, err := Resolve(context.Background(), "magnet:?dn=nohash", nil, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("hashless magnet err = %v", err)
	}
	m, err = Resolve(context.Background(), "https://releases.test/files/distro.iso.torrent?x=1", nil, false)
	if err != nil || m.Kind != plan.KindTorrent || m.Name != "distro.iso.torrent" {
		t.Fatalf("torrent url = %+v, %v", m, err)
	}
	local := filepath.Join(t.TempDir(), "local.torrent")
	os.WriteFile(local, []byte("d4:infod4:name1:xee"), 0o644)
	m, err = Resolve(context.Background(), local, nil, false)
	if err != nil || m.Kind != plan.KindTorrent || m.URL != local {
		t.Fatalf("local torrent = %+v, %v", m, err)
	}
}

func timeZero() (t time.Time) { return }

// pickyHost answers only one kind of user agent: browsers get their
// connection dropped (like some speed-test mirrors) or a 403.
func pickyHost(t *testing.T, refuse string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.UserAgent(), "Firefox") {
			if refuse == "drop" {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					conn.Close()
				}
				return
			}
			http.Error(w, "no browsers", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeContent(w, r, "", timeZero(), strings.NewReader(strings.Repeat("z", 4096)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveRetriesWithPlainUA(t *testing.T) {
	for _, refuse := range []string{"drop", "403"} {
		srv := pickyHost(t, refuse)
		m, err := Resolve(context.Background(), srv.URL+"/100MB.bin", &plan.Env{HTTP: srv.Client()}, false)
		if err != nil {
			t.Fatalf("%s: %v", refuse, err)
		}
		if m.Kind != plan.KindFile || m.Size != 4096 || !m.Resumable || m.Name != "100MB.bin" || m.Headers["User-Agent"] != plainUA {
			t.Fatalf("%s: %+v", refuse, m)
		}
		pl, err := PlanFor(m, Options{})
		if err != nil || pl.Parts[0].Headers.Get("User-Agent") != plainUA {
			t.Fatalf("%s: plan lost the user agent: %+v, %v", refuse, pl, err)
		}
	}
}

func TestResolveKeepsBrowserUA(t *testing.T) {
	srv := testSite(t)
	m, err := Resolve(context.Background(), srv.URL+"/files/report.pdf", &plan.Env{HTTP: srv.Client()}, false)
	if err != nil || m.Headers["User-Agent"] != browserUA {
		t.Fatalf("file = %+v, %v", m, err)
	}
	m, err = Resolve(context.Background(), srv.URL+"/post", &plan.Env{HTTP: srv.Client()}, false)
	if err != nil || m.Formats[0].Parts[0].Headers.Get("User-Agent") != browserUA {
		t.Fatalf("og = %+v, %v", m, err)
	}
}

func TestResolveBothAgentsFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
	}))
	defer srv.Close()
	if _, err := Resolve(context.Background(), srv.URL+"/x.bin", &plan.Env{HTTP: srv.Client()}, false); !errors.Is(err, ErrNetwork) {
		t.Fatalf("err = %v", err)
	}
	gate := pickyHost(t, "403")
	gate.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusForbidden) })
	if _, err := Resolve(context.Background(), gate.URL+"/x.bin", &plan.Env{HTTP: gate.Client()}, false); !errors.Is(err, ErrPrivate) {
		t.Fatalf("403 err = %v", err)
	}
}
