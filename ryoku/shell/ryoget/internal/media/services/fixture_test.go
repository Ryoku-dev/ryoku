package services

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ryoget/internal/media"
	"ryoget/internal/plan"
)

// route answers one request the service makes; match is on method, host and
// path, plus a substring of the query or body when set.
type route struct {
	method string
	url    string // host + path, no scheme or query
	query  string
	body   string
	status int
	file   string // under testdata/
	text   string
	header map[string]string
}

type fixtureTransport struct {
	t      *testing.T
	routes []route
	mu     sync.Mutex
	seen   []string
}

func (f *fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}
	key := req.URL.Host + req.URL.Path
	f.mu.Lock()
	f.seen = append(f.seen, req.Method+" "+key)
	f.mu.Unlock()
	for _, r := range f.routes {
		method := r.method
		if method == "" {
			method = http.MethodGet
		}
		if method != req.Method || r.url != key {
			continue
		}
		if r.query != "" && !strings.Contains(req.URL.RawQuery, r.query) {
			continue
		}
		if r.body != "" && !bytes.Contains(reqBody, []byte(r.body)) {
			continue
		}
		data := []byte(r.text)
		if r.file != "" {
			b, err := os.ReadFile(filepath.Join("testdata", r.file))
			if err != nil {
				f.t.Fatalf("fixture %s: %v", r.file, err)
			}
			data = b
		}
		status := r.status
		if status == 0 {
			status = http.StatusOK
		}
		h := http.Header{}
		for k, v := range r.header {
			h.Add(k, v)
		}
		return &http.Response{
			StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)),
			Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
			Header: h, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Request: req,
		}, nil
	}
	return nil, fmt.Errorf("no fixture for %s %s?%s", req.Method, key, req.URL.RawQuery)
}

func fixtureEnv(t *testing.T, routes ...route) *plan.Env {
	t.Helper()
	return &plan.Env{HTTP: &http.Client{Transport: &fixtureTransport{t: t, routes: routes}}}
}

func resolveFixture(t *testing.T, raw string, routes ...route) (*media.Media, error) {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	s := media.Lookup(u)
	if s == nil {
		t.Fatalf("no service matches %s", raw)
	}
	return s.Resolve(t.Context(), u, fixtureEnv(t, routes...))
}

func findFormat(m *media.Media, id string) *media.Format {
	for i := range m.Formats {
		if m.Formats[i].ID == id {
			return &m.Formats[i]
		}
	}
	return nil
}

// checkMedia asserts the shape every service result must have.
func checkMedia(t *testing.T, m *media.Media, source string) {
	t.Helper()
	if m == nil {
		t.Fatal("nil media")
	}
	if m.Source.ID != source || m.Source.Name == "" {
		t.Fatalf("source = %+v, want id %q", m.Source, source)
	}
	if m.Title == "" {
		t.Fatal("empty title")
	}
	switch m.Kind {
	case plan.KindFile:
		if m.URL == "" {
			t.Fatal("file media without URL")
		}
		return
	case plan.KindMedia:
	default:
		t.Fatalf("kind = %q", m.Kind)
	}
	if len(m.Formats) == 0 && len(m.Items) == 0 {
		t.Fatal("no formats and no items")
	}
	ids := map[string]bool{}
	for _, f := range m.Formats {
		if f.ID == "" || ids[f.ID] {
			t.Fatalf("format id %q empty or repeated", f.ID)
		}
		ids[f.ID] = true
		if f.Kind != "video" && f.Kind != "audio" {
			t.Fatalf("format %s kind %q", f.ID, f.Kind)
		}
		if !f.HasVideo && !f.HasAudio {
			t.Fatalf("format %s carries neither video nor audio", f.ID)
		}
		if len(f.Parts) == 0 {
			t.Fatalf("format %s has no parts", f.ID)
		}
		for _, p := range f.Parts {
			if p.URL == "" || (p.Kind != plan.PartHTTP && p.Kind != plan.PartHLS) {
				t.Fatalf("format %s part %+v", f.ID, p)
			}
		}
	}
	if len(m.Formats) > 0 && !ids[m.Default] {
		t.Fatalf("default %q is not a format", m.Default)
	}
	if _, err := media.PlanFor(m, media.Options{}); len(m.Formats) > 0 && err != nil {
		t.Fatalf("planner rejects the result: %v", err)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
