package search

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryoget-search-test-")
	if err != nil {
		panic(err)
	}
	home := filepath.Join(root, "home")
	for k, v := range map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
		"XDG_RUNTIME_DIR": filepath.Join(root, "runtime"),
		"HTTP_PROXY":      "",
		"HTTPS_PROXY":     "",
	} {
		os.Setenv(k, v)
	}
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

// a 1x1 PNG served for every image request
var pixel, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

type route struct {
	status int
	file   string
	body   string
	header map[string]string
	// hold the request until the caller gives up
	hang bool
	// hold the request until this channel closes
	wait chan struct{}
}

type fixtures struct {
	t      *testing.T
	mu     sync.Mutex
	routes map[string]route
	hits   map[string]int
	reqs   []*http.Request
}

func newFixtures(t *testing.T) *fixtures {
	return &fixtures{t: t, routes: map[string]route{}, hits: map[string]int{}}
}

// set registers host+path (no query) → response.
func (f *fixtures) set(hostPath string, r route) {
	f.mu.Lock()
	f.routes[hostPath] = r
	f.mu.Unlock()
}

func (f *fixtures) count(hostPath string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[hostPath]
}

func (f *fixtures) last(hostPath string) *http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.reqs) - 1; i >= 0; i-- {
		if f.reqs[i].URL.Host+f.reqs[i].URL.Path == hostPath {
			return f.reqs[i]
		}
	}
	return nil
}

func (f *fixtures) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.URL.Host + req.URL.Path
	f.mu.Lock()
	r, ok := f.routes[key]
	f.hits[key]++
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	if r.hang {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	if r.wait != nil {
		select {
		case <-r.wait:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	resp := &http.Response{StatusCode: 200, Header: http.Header{}, Request: req}
	switch {
	case ok:
		if r.status != 0 {
			resp.StatusCode = r.status
		}
		body := []byte(r.body)
		if r.file != "" {
			b, err := os.ReadFile(filepath.Join("testdata", r.file))
			if err != nil {
				f.t.Errorf("fixture %s: %v", r.file, err)
			}
			body = b
			switch {
			case strings.HasSuffix(r.file, ".html"):
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
			case strings.HasSuffix(r.file, ".xml"):
				resp.Header.Set("Content-Type", "text/xml; charset=utf-8")
			default:
				resp.Header.Set("Content-Type", "application/json")
			}
		}
		for k, v := range r.header {
			resp.Header.Set(k, v)
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
	case strings.HasPrefix(req.Header.Get("Accept"), "image/"):
		resp.Header.Set("Content-Type", "image/png")
		resp.Body = io.NopCloser(bytes.NewReader(pixel))
		resp.ContentLength = int64(len(pixel))
	default:
		resp.StatusCode = 404
		resp.Body = io.NopCloser(strings.NewReader(`{"message":"not found"}`))
	}
	return resp, nil
}

// standard answers every source from its recorded fixture.
func (f *fixtures) standard() *fixtures {
	f.set("api.keenable.ai/v1/search/public", route{file: "keenable.json"})
	f.set("lite.duckduckgo.com/lite/", route{file: "ddglite.html"})
	f.set("www.bing.com/search", route{file: "bingrss.xml"})
	f.set("duckduckgo.com/ac/", route{file: "ddgac.json"})
	f.set("api.bing.com/osjson.aspx", route{file: "bingos.json"})
	f.set("en.wikipedia.org/w/rest.php/v1/search/title", route{file: "wikititle.json"})
	f.set("freedictionaryapi.com/api/v1/entries/en/tbh", route{file: "fdict.json"})
	f.set("en.wiktionary.org/api/rest_v1/page/definition/tbh", route{file: "wikt.json"})
	f.set("en.wikipedia.org/w/rest.php/v1/search/page", route{file: "wikipedia.json"})
	f.set("api.github.com/search/repositories", route{file: "github.json"})
	f.set("api.stackexchange.com/2.3/search/excerpts", route{file: "stackoverflow.json"})
	f.set("developer.mozilla.org/api/v1/search", route{file: "mdn.json"})
	f.set("registry.npmjs.org/-/v1/search", route{file: "npm.json"})
	f.set("crates.io/api/v1/crates", route{file: "crates.json"})
	f.set("wiki.archlinux.org/api.php", route{file: "archwiki.json"})
	f.set("archlinux.org/packages/search/json/", route{file: "archpkg.json"})
	f.set("flathub.org/api/v2/search", route{file: "flathub.json"})
	f.set("hn.algolia.com/api/v1/search", route{file: "hn.json"})
	return f
}

type serviceOpts struct {
	osRelease string
	stateFile string
	cacheDir  string
	now       func() time.Time
	// keep the real engine budgets instead of unlimited ones
	realBudgets bool
}

func testService(t *testing.T, f *fixtures, osRelease string) *Service {
	t.Helper()
	return testServiceWith(t, f, serviceOpts{osRelease: osRelease})
}

func testServiceWith(t *testing.T, f *fixtures, o serviceOpts) *Service {
	t.Helper()
	if o.osRelease == "" {
		o.osRelease = "os-release-arch"
	}
	if o.cacheDir == "" {
		o.cacheDir = t.TempDir()
	}
	e, err := New(Options{
		Client: &http.Client{Transport: f}, CacheDir: o.cacheDir, StateFile: o.stateFile,
		Version: "test", Now: o.now, OSRelease: filepath.Join("testdata", o.osRelease),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !o.realBudgets {
		unlimited(e)
	}
	youtubeSearch = fakeYouTube
	t.Cleanup(func() { e.Close() })
	return e
}

// unlimited lifts every budget so tests never wait on a bucket.
func unlimited(e *Service) {
	e.gov.mu.Lock()
	defer e.gov.mu.Unlock()
	for _, st := range e.gov.engines {
		d := *st.def
		d.buckets = []bucketDef{{every: time.Millisecond, burst: 1000}}
		st.def = &d
		st.Tokens = []float64{1000}
	}
}

// budget gives one engine a single bucket for a test.
func budget(e *Service, engine string, every time.Duration, burst float64) {
	e.gov.mu.Lock()
	defer e.gov.mu.Unlock()
	st := e.gov.engines[engine]
	d := *st.def
	d.buckets = []bucketDef{{every: every, burst: burst}}
	st.def = &d
	st.Tokens = []float64{burst}
	st.TokensAt = e.gov.now()
}
