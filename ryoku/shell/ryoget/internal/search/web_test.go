package search

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestKeenableParse(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "")
	items, err := searchKeenable(context.Background(), e, newQuery("what does tbh mean in texting", "en-US"))
	if err != nil || len(items) != 10 {
		t.Fatalf("err=%v items=%d", err, len(items))
	}
	if items[0].URL != "https://dictionary.cambridge.org/us/dictionary/english/tbh" || items[0].Site != "dictionary.cambridge.org" {
		t.Errorf("first = %+v", items[0])
	}
	for _, it := range items {
		if strings.Contains(it.Snippet, "**") || strings.Contains(it.Snippet, "](") {
			t.Errorf("markdown left in snippet: %q", it.Snippet)
		}
	}
	req := f.last("api.keenable.ai/v1/search/public")
	if req.Method != "POST" || req.Header.Get("X-Keenable-Title") != "Ryoku" {
		t.Errorf("request = %s %v", req.Method, req.Header)
	}
}

func TestDDGLiteParse(t *testing.T) {
	body, err := os.ReadFile("testdata/ddglite.html")
	if err != nil {
		t.Fatal(err)
	}
	items, err := parseDDGLite(body)
	if err != nil || len(items) < 5 {
		t.Fatalf("err=%v items=%d", err, len(items))
	}
	for _, it := range items {
		if !webURL(it.URL) || strings.Contains(it.URL, "duckduckgo.com") {
			t.Errorf("redirect not unwrapped: %q", it.URL)
		}
		if it.Title == "" {
			t.Errorf("no title: %+v", it)
		}
	}
	withSnippet := 0
	for _, it := range items {
		if it.Snippet != "" {
			withSnippet++
		}
	}
	if withSnippet < len(items)/2 {
		t.Errorf("snippet rows not paired: %d of %d", withSnippet, len(items))
	}
}

func TestDDGLiteChallengePauses(t *testing.T) {
	f := newFixtures(t)
	f.set("lite.duckduckgo.com/lite/", route{status: 202, file: "ddglite_anomaly.html"})
	e := testService(t, f, "")
	_, err := searchDDGLite(context.Background(), e, newQuery("x", "en-US"))
	if p, ok := isPaused(err); !ok || p.engine != "ddg-lite" {
		t.Fatalf("err = %v, want ddg-lite paused", err)
	}
	// the challenge page also counts as a block when served with 200
	f2 := newFixtures(t)
	f2.set("lite.duckduckgo.com/lite/", route{file: "ddglite_anomaly.html"})
	e2 := testService(t, f2, "")
	if _, err := searchDDGLite(context.Background(), e2, newQuery("x", "en-US")); err == nil {
		t.Fatal("anomaly page read as results")
	}
}

func TestBingRSSParse(t *testing.T) {
	body, err := os.ReadFile("testdata/bingrss.xml")
	if err != nil {
		t.Fatal(err)
	}
	items, err := parseBingRSS(body)
	if err != nil || len(items) != 10 {
		t.Fatalf("err=%v items=%d", err, len(items))
	}
	if items[0].URL != "https://www.wikihow.com/What-Does-TBH-Mean" || items[0].Snippet == "" || items[0].Date != "" {
		t.Errorf("first = %+v", items[0])
	}
}

func TestFuseRanksSharedLinksAndPrefersPlainSnippets(t *testing.T) {
	keen := []Item{
		{Title: "A", URL: "https://a.example/x", Snippet: "keenable a"},
		{Title: "B", URL: "https://www.b.example/y/", Snippet: "keenable b"},
		{Title: "K", URL: "https://k.example/", Snippet: "only keenable"},
	}
	ddg := []Item{
		{Title: "B", URL: "http://b.example/y", Snippet: "ddg b"},
		{Title: "D", URL: "https://d.example/", Snippet: "only ddg"},
	}
	out := fuse([]laneResult{{engine: "keenable", items: keen}, {engine: "ddg-lite", items: ddg}})
	if len(out) != 4 {
		t.Fatalf("dedupe failed: %d items", len(out))
	}
	if out[0].Title != "B" {
		t.Errorf("a link both engines found must lead, got %q", out[0].Title)
	}
	if out[0].Snippet != "ddg b" {
		t.Errorf("snippet = %q, want DuckDuckGo's plain one", out[0].Snippet)
	}
	if eng := out[0].Meta["engines"].([]string); len(eng) != 2 || eng[0] != "keenable" || eng[1] != "ddg-lite" {
		t.Errorf("engines = %v", eng)
	}
	if out[0].thumbURL != "https://b.example/favicon.ico" || !out[0].thumbPage {
		t.Errorf("favicon = %q page=%v", out[0].thumbURL, out[0].thumbPage)
	}
	for _, it := range out {
		if strings.Contains(it.thumbURL, "duckduckgo.com") {
			t.Errorf("icon proxied through DuckDuckGo: %s", it.thumbURL)
		}
	}
}

// The two engines' answers to the same query, recorded seconds apart; they
// share the NVIDIA forum thread.
func TestWebLaneMergesBothEngines(t *testing.T) {
	f := newFixtures(t).standard()
	f.set("api.keenable.ai/v1/search/public", route{file: "keenable_nvidia.json"})
	f.set("lite.duckduckgo.com/lite/", route{file: "ddglite_nvidia.html"})
	e := testService(t, f, "")
	d := collect(e, "nvidia wayland flicker", "web")["web"]
	if len(d.Items) < 15 {
		t.Fatalf("web lane gave %d rows, want both lists merged", len(d.Items))
	}
	first := d.Items[0]
	if first.URL != "https://forums.developer.nvidia.com/t/opengl-xwayland-insane-flickering/283087" {
		t.Errorf("the link both engines found should lead, got %q", first.URL)
	}
	if eng := first.Meta["engines"].([]string); len(eng) != 2 {
		t.Errorf("engines = %v", eng)
	}
	seen := map[string]bool{}
	for _, it := range d.Items {
		if seen[normURL(it.URL)] {
			t.Errorf("duplicate row %s", it.URL)
		}
		seen[normURL(it.URL)] = true
	}
	if f.count("www.bing.com/search") != 0 {
		t.Error("Bing asked although both engines answered")
	}
}

func TestWebLaneFallsBackToBing(t *testing.T) {
	f := newFixtures(t).standard()
	f.set("api.keenable.ai/v1/search/public", route{status: 429, body: `{"error":{"code":"rate_limited"}}`, header: map[string]string{"Retry-After": "120"}})
	f.set("lite.duckduckgo.com/lite/", route{status: 202, file: "ddglite_anomaly.html"})
	e := testService(t, f, "")
	d := collect(e, "what does tbh mean in texting", "web")["web"]
	if len(d.Items) == 0 || f.count("www.bing.com/search") != 1 {
		t.Fatalf("Bing fallback: %d rows, %d Bing requests", len(d.Items), f.count("www.bing.com/search"))
	}
	if eng := d.Items[0].Meta["engines"].([]string); eng[0] != "bing-rss" {
		t.Errorf("engines = %v", eng)
	}
	if d := e.gov.paused("keenable").Sub(e.now()); d < 110e9 || d > 130e9 {
		t.Errorf("Keenable paused %v, want its Retry-After of 120 s", d)
	}
}

func TestWebLanePausedWhenEveryEngineIs(t *testing.T) {
	f := newFixtures(t)
	e := testService(t, f, "")
	for _, id := range []string{"keenable", "ddg-lite", "bing-rss"} {
		e.gov.strike(id, 0)
	}
	var lines []any
	e.query(context.Background(), 1, "tbh", "web", "full", "en-US", func(v any) { lines = append(lines, v) })
	var paused *stateMsg
	for _, l := range lines {
		if m, ok := l.(stateMsg); ok && m.State == "paused" {
			paused = &m
		}
	}
	if paused == nil || paused.Until == "" {
		t.Fatalf("lines = %+v, want a paused frame with until", lines)
	}
	if len(f.reqs) != 0 {
		t.Errorf("%d requests sent to paused engines", len(f.reqs))
	}
}
