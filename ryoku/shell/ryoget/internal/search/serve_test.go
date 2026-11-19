package search

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type session struct {
	t     *testing.T
	in    *io.PipeWriter
	lines chan map[string]any
	done  chan error
	seen  []map[string]any
}

type pipeConn struct {
	io.Reader
	io.Writer
}

func startSession(t *testing.T, e *Service) *session {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{t: t, in: inW, lines: make(chan map[string]any, 1024), done: make(chan error, 1)}
	go func() {
		s.done <- e.Serve(ctx, pipeConn{inR, outW})
		outW.Close()
	}()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
		for sc.Scan() {
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Errorf("bad output line %q: %v", sc.Text(), err)
				continue
			}
			s.lines <- m
		}
		close(s.lines)
	}()
	t.Cleanup(func() { cancel(); inW.Close() })
	return s
}

func (s *session) send(line string) {
	s.t.Helper()
	if _, err := io.WriteString(s.in, line+"\n"); err != nil {
		s.t.Fatal(err)
	}
}

func (s *session) hello() map[string]any {
	s.t.Helper()
	s.send(`{"hello":{"client":"cli","version":2,"lang":"en-US"}}`)
	return s.until(func(m map[string]any) bool { return m["ready"] == true })[0]
}

func (s *session) next() map[string]any {
	s.t.Helper()
	select {
	case m, ok := <-s.lines:
		if !ok {
			s.t.Fatal("output closed")
		}
		s.seen = append(s.seen, m)
		return m
	case <-time.After(5 * time.Second):
		s.t.Fatalf("no output line in 5 s; seen so far: %v", s.seen)
	}
	return nil
}

// until reads lines up to and including the first one matching pred,
// returning only the match first and then everything before it.
func (s *session) until(pred func(map[string]any) bool) []map[string]any {
	s.t.Helper()
	var out []map[string]any
	for {
		m := s.next()
		if pred(m) {
			return append([]map[string]any{m}, out...)
		}
		out = append(out, m)
	}
}

// finish closes stdin and returns every remaining line once Serve returns.
func (s *session) finish() []map[string]any {
	s.t.Helper()
	s.in.Close()
	var out []map[string]any
	for m := range s.lines {
		out = append(out, m)
	}
	select {
	case err := <-s.done:
		if err != nil {
			s.t.Fatalf("Serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		s.t.Fatal("Serve did not return after EOF")
	}
	return out
}

func num(m map[string]any, k string) int64 {
	f, _ := m[k].(float64)
	return int64(f)
}

func isFinal(id int64) func(map[string]any) bool {
	return func(m map[string]any) bool {
		_, src := m["source"]
		return num(m, "id") == id && m["state"] == "done" && !src
	}
}

func frames(lines []map[string]any, id int64, source string) []map[string]any {
	var out []map[string]any
	for _, m := range lines {
		if num(m, "id") == id && m["source"] == source {
			out = append(out, m)
		}
	}
	return out
}

func TestHelloReady(t *testing.T) {
	f := newFixtures(t)
	e := testService(t, f, "os-release-fedora")
	s := startSession(t, e)
	r := s.hello()
	if num(r, "version") != 2 {
		t.Errorf("version = %v", r["version"])
	}
	var ids, typing []string
	for _, raw := range r["sources"].([]any) {
		src := raw.(map[string]any)
		ids = append(ids, src["id"].(string))
		if src["typing"] == true {
			typing = append(typing, src["id"].(string))
		}
	}
	if strings.Join(ids, ",") != "dictionary,suggest,web,wikipedia,github,stackoverflow,mdn,npm,crates,archwiki,flathub,youtube,hackernews" {
		t.Errorf("sources = %v", ids)
	}
	if strings.Join(typing, ",") != "dictionary,suggest,wikipedia" {
		t.Errorf("typing sources = %v", typing)
	}
	webEngines := 0
	for _, raw := range r["engines"].([]any) {
		en := raw.(map[string]any)
		if en["state"] != "ok" || en["name"] == "" {
			t.Errorf("engine = %v", en)
		}
		for _, src := range en["sources"].([]any) {
			if src == "web" {
				webEngines++
			}
		}
	}
	if webEngines != 3 {
		t.Errorf("%d engines feed the web lane, want 3", webEngines)
	}
	if len(f.reqs) != 0 {
		t.Error("hello touched the network")
	}
	s.finish()
}

func TestQueryFramesAndFinalDone(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "")
	s := startSession(t, e)
	s.hello()
	s.send(`{"id":1,"q":"what does tbh mean in texting","scope":"all","mode":"suggest"}`)
	lines := s.until(isFinal(1))
	for _, src := range []string{"dictionary", "suggest", "wikipedia"} {
		fr := frames(lines, 1, src)
		if len(fr) < 2 || fr[0]["state"] != "loading" || fr[len(fr)-1]["state"] != "done" {
			t.Errorf("%s frames = %v", src, fr)
		}
	}
	def := frames(lines, 1, "dictionary")
	items := def[len(def)-1]["items"].([]any)
	card := items[0].(map[string]any)
	if card["kind"] != "definition" || card["meta"].(map[string]any)["attributionUrl"] == "" {
		t.Errorf("definition card = %v", card)
	}
	if n := len(frames(lines, 1, "web")); n != 0 {
		t.Errorf("web lane ran in suggest mode")
	}
	ua := f.last("duckduckgo.com/ac/").Header.Get("User-Agent")
	if ua != "ryoseek/test (+https://ryoku.dev) Go-http-client/2.0" {
		t.Errorf("User-Agent = %q", ua)
	}
	if al := f.last("duckduckgo.com/ac/").Header.Get("Accept-Language"); al != "en-US,en;q=0.9" {
		t.Errorf("Accept-Language = %q", al)
	}
	s.finish()
}

func TestTwoClientsShareOneRequest(t *testing.T) {
	f := newFixtures(t)
	hold := make(chan struct{})
	f.set("registry.npmjs.org/-/v1/search", route{file: "npm.json", wait: hold})
	e := testService(t, f, "")
	a, b := startSession(t, e), startSession(t, e)
	a.hello()
	b.hello()
	a.send(`{"id":1,"q":"wayland","scope":"npm","mode":"full"}`)
	waitFor(t, func() bool { return f.count("registry.npmjs.org/-/v1/search") == 1 })
	b.send(`{"id":7,"q":"wayland","scope":"npm","mode":"full"}`)
	time.Sleep(50 * time.Millisecond)
	close(hold)
	for _, s := range []*session{a, b} {
		lines := s.until(func(m map[string]any) bool { return m["state"] == "done" && m["source"] == "npm" })
		if n := len(lines[0]["items"].([]any)); n != 6 {
			t.Errorf("client got %d rows", n)
		}
	}
	if n := f.count("registry.npmjs.org/-/v1/search"); n != 1 {
		t.Errorf("%d requests for one query asked by two clients", n)
	}
	// and a third time from the cache, on disk too
	c := startSession(t, e)
	c.hello()
	c.send(`{"id":1,"q":"Wayland","scope":"npm","mode":"full"}`)
	c.until(isFinal(1))
	if n := f.count("registry.npmjs.org/-/v1/search"); n != 1 {
		t.Errorf("cached result asked again (%d requests)", n)
	}
	entries, _ := os.ReadDir(e.cache.dir)
	if len(entries) == 0 {
		t.Error("full result not written to the disk cache")
	}
}

func TestCancelStopsRowsButTheRequestFinishesIntoTheCache(t *testing.T) {
	f := newFixtures(t)
	hold := make(chan struct{})
	f.set("crates.io/api/v1/crates", route{file: "crates.json", wait: hold})
	e := testService(t, f, "")
	s := startSession(t, e)
	s.hello()
	s.send(`{"id":3,"q":"wayland","scope":"crates","mode":"full"}`)
	waitFor(t, func() bool { return f.count("crates.io/api/v1/crates") == 1 })
	s.send(`{"cancel":3}`)
	time.Sleep(30 * time.Millisecond)
	close(hold)
	waitFor(t, func() bool {
		_, ok := e.cache.get("src|crates|full|en-us|wayland", false)
		return ok
	})
	s.send(`{"id":4,"q":"wayland","scope":"crates","mode":"full"}`)
	lines := s.until(isFinal(4))
	for _, m := range lines {
		if num(m, "id") == 3 && m["state"] != "loading" {
			t.Errorf("cancelled query still wrote %v", m)
		}
	}
	if fr := frames(lines, 4, "crates"); len(fr[len(fr)-1]["items"].([]any)) != 6 {
		t.Errorf("cached rows = %v", fr)
	}
	if n := f.count("crates.io/api/v1/crates"); n != 1 {
		t.Errorf("%d requests; the cancelled one should have filled the cache", n)
	}
	s.finish()
}

func TestCancelBeforeTheWireSendsNothing(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "")
	budget(e, "npm", time.Hour, 0)
	s := startSession(t, e)
	s.hello()
	s.send(`{"id":1,"q":"wayland","scope":"npm","mode":"full"}`)
	w := s.until(func(m map[string]any) bool { return m["state"] == "waiting" })
	if w[0]["source"] != "npm" {
		t.Fatalf("waiting frame = %v", w[0])
	}
	s.send(`{"cancel":1}`)
	waitFor(t, e.idleFlights)
	if n := f.count("registry.npmjs.org/-/v1/search"); n != 0 {
		t.Errorf("abandoned query still sent %d requests", n)
	}
	s.finish()
}

func TestNewerQuerySupersedesOlder(t *testing.T) {
	f := newFixtures(t).standard()
	hold := make(chan struct{})
	// released before the service closes, so Close does not wait on it
	defer close(hold)
	f.set("hn.algolia.com/api/v1/search", route{wait: hold})
	e := testService(t, f, "")
	s := startSession(t, e)
	s.hello()
	s.send(`{"id":1,"q":"slow","scope":"hackernews","mode":"full"}`)
	s.until(func(m map[string]any) bool { return m["state"] == "loading" })
	s.send(`{"id":2,"q":"array map","scope":"mdn","mode":"full"}`)
	lines := s.until(isFinal(2))
	lines = append(lines, s.finish()...)
	for _, m := range lines {
		if num(m, "id") == 1 {
			t.Errorf("superseded query still wrote %v", m)
		}
	}
}

func TestEngineEventsReachClients(t *testing.T) {
	e := testService(t, newFixtures(t), "")
	s := startSession(t, e)
	s.hello()
	e.gov.strike("keenable", 0)
	m := s.until(func(m map[string]any) bool { return m["engines"] != nil })[0]
	for _, raw := range m["engines"].([]any) {
		en := raw.(map[string]any)
		if en["id"] == "keenable" && (en["state"] != "paused" || en["until"] == "") {
			t.Errorf("keenable = %v", en)
		}
	}
	s.finish()
}

func TestDetailsOverTheSocket(t *testing.T) {
	f := newFixtures(t)
	f.set("en.wikipedia.org/api/rest_v1/page/summary/Wayland_(protocol)", route{file: "wikipedia_summary.json"})
	f.set("api.github.com/repos/hyprwm/Hyprland", route{file: "github_repo.json"})
	f.set("api.stackexchange.com/2.3/questions/70869849/answers", route{file: "stackoverflow_answers.json"})
	f.set("example.org/post", route{header: map[string]string{"Content-Type": "text/html; charset=utf-8"}, body: `<!doctype html><html><head>
<title>Fallback title</title>
<meta property="og:title" content="Fixing &amp; tuning flicker">
<meta name="description" content="How to stop NVIDIA flicker on Wayland.">
<meta property="og:image" content="/cover.png">
<meta property="og:site_name" content="Example Blog">
</head><body><p>body</p></body></html>`})
	e := testService(t, f, "")
	s := startSession(t, e)
	s.hello()
	detail := func(id int, src, key string) map[string]any {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"id": id, "detail": map[string]string{"source": src, "key": key}})
		s.send(string(b))
		m := s.until(func(m map[string]any) bool { return num(m, "id") == int64(id) && m["detail"] != nil })[0]
		d := m["detail"].(map[string]any)
		if d["source"] != src || d["key"] != key {
			t.Fatalf("detail does not echo source/key: %v", d)
		}
		return d
	}
	w := detail(2, "wikipedia", "en:Wayland_(protocol)")
	if w["title"] != "Wayland (protocol)" || w["thumb"] == "" {
		t.Errorf("wikipedia detail = %v", w)
	}
	g := detail(3, "github", "hyprwm/Hyprland")
	if len(g["facts"].([]any)) < 3 || g["error"] != nil {
		t.Errorf("github detail = %v", g)
	}
	so := detail(4, "stackoverflow", "70869849")
	if !strings.Contains(so["text"].(string), "\n") {
		t.Errorf("stackoverflow detail = %v", so)
	}
	og := detail(5, "web", "https://example.org/post")
	if og["title"] != "Fixing & tuning flicker" || og["site"] != "Example Blog" || og["thumb"] == "" {
		t.Errorf("opengraph detail = %v", og)
	}
	if bad := detail(6, "github", "../../etc"); bad["error"] == nil {
		t.Errorf("bad key must report an error: %v", bad)
	}
	s.finish()
}

func TestEOFLetsRunningWorkAnswer(t *testing.T) {
	e := testService(t, newFixtures(t).standard(), "")
	s := startSession(t, e)
	s.send(`{"id":1,"q":"wayland","scope":"code","mode":"full"}`)
	lines := s.finish()
	if len(lines) == 0 || !isFinal(1)(lines[len(lines)-1]) {
		t.Fatalf("closing stdin must still deliver the query, got %d lines", len(lines))
	}
	if !e.Idle() {
		t.Error("service not idle after its only client left")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in 3 s")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (e *Service) idleFlights() bool {
	e.fmu.Lock()
	defer e.fmu.Unlock()
	return len(e.flights) == 0
}

// A restarted daemon reads full results back from disk; those rows must rank
// and pass the All gate exactly like fresh ones.
func TestDiskCachedResultsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	f := newFixtures(t).standard()
	first := testServiceWith(t, f, serviceOpts{cacheDir: dir})
	before := collect(first, "wayland compositor", "all")
	if len(before["github"].Items) == 0 {
		t.Fatal("fixture gave no GitHub rows")
	}
	first.Close()

	offline := newFixtures(t)
	again := testServiceWith(t, offline, serviceOpts{cacheDir: dir})
	after := collect(again, "wayland compositor", "all")
	for src, d := range before {
		got := after[src]
		if len(got.Items) != len(d.Items) || got.Rank != d.Rank {
			t.Errorf("%s after restart: %d rows rank %d, before %d rows rank %d", src, len(got.Items), got.Rank, len(d.Items), d.Rank)
		}
	}
	if n := offline.count("api.github.com/search/repositories"); n != 0 {
		t.Errorf("restarted daemon asked GitHub again (%d)", n)
	}
}

func TestDetailIgnoresFrontPageBoilerplate(t *testing.T) {
	u, _ := url.Parse("https://www.reddit.com/r/linux/comments/1beuq38/nvidia_wayland_is_still_an_absolute_dumpster_fire/")
	d := parseOpenGraph([]byte(`<html><head><title>Reddit - The heart of the internet</title>
<meta property="og:title" content="Reddit - The heart of the internet">
<meta name="description" content="Reddit is where millions of people gather for conversations.">
<meta property="og:image" content="https://www.redditstatic.com/icon.png"></head></html>`), u)
	if d.Title != "" || d.Text != "" || d.thumbURL != "" {
		t.Errorf("boilerplate kept: %+v", d)
	}
	u2, _ := url.Parse("https://mort.coffee/home/wayland-input-latency/")
	d2 := parseOpenGraph([]byte(`<html><head><title>Hard numbers in the Wayland vs X11 input latency discussion</title></head></html>`), u2)
	if d2.Title == "" {
		t.Error("a real page title was dropped")
	}
}
