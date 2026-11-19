package search

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestBucketHoldsRequestsAndSaysWaiting(t *testing.T) {
	f := newFixtures(t).standard()
	e := testService(t, f, "")
	budget(e, "npm", 600*time.Millisecond, 1)
	ctx := context.Background()
	if _, _, err := searchNpm(ctx, e, newQuery("a", "en-US")); err != nil {
		t.Fatal(err)
	}
	waited := false
	start := time.Now()
	release, err := e.gov.acquire(ctx, "npm", func() { waited = true })
	if err != nil {
		t.Fatal(err)
	}
	release()
	if d := time.Since(start); d < 450*time.Millisecond || !waited {
		t.Errorf("second request went out after %v (waited=%v); want held about 600 ms and told", d, waited)
	}

	// a caller that gives up while held never sends anything
	cctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := e.gov.acquire(cctx, "npm", nil); err == nil {
		t.Error("acquire ignored its context")
	}
}

func TestBackoffGrowsAndCaps(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)}
	g := newGovernor("", c.now)
	want := []time.Duration{15 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour, 6 * time.Hour, 6 * time.Hour, 6 * time.Hour}
	for i, w := range want {
		until := g.strike("ddg-lite", 0)
		if got := until.Sub(c.now()); got != w {
			t.Errorf("strike %d: paused %v, want %v", i+1, got, w)
		}
		c.add(7 * time.Hour)
	}
	g.ok("ddg-lite")
	if got := g.strike("ddg-lite", 0).Sub(c.now()); got != 15*time.Minute {
		t.Errorf("after a success the backoff restarts at 15 min, got %v", got)
	}
	c.add(time.Hour)
	if got := g.strike("ddg-lite", 90*time.Second).Sub(c.now()); got != 90*time.Second {
		t.Errorf("Retry-After ignored: %v", got)
	}
}

func TestClassify(t *testing.T) {
	now := time.Now()
	h := func(kv ...string) http.Header {
		out := http.Header{}
		for i := 0; i+1 < len(kv); i += 2 {
			out.Set(kv[i], kv[i+1])
		}
		return out
	}
	cases := []struct {
		name    string
		engine  string
		resp    response
		blocked bool
		retry   time.Duration
	}{
		{"ddg 202", "ddg-lite", response{status: 202, header: h("Content-Type", "text/html")}, true, 0},
		{"anomaly 200", "ddg-lite", response{status: 200, header: h("Content-Type", "text/html"), body: []byte(`<div class="anomaly-modal">`)}, true, 0},
		{"anubis", "archwiki", response{status: 200, header: h("Content-Type", "text/html"), body: []byte(`<title>Making sure you're not a bot!</title>`)}, true, 0},
		{"captcha title", "mdn", response{status: 200, header: h("Content-Type", "text/html"), body: []byte(`<title>Captcha</title>`)}, true, 0},
		{"js interstitial", "keenable", response{status: 200, header: h("Content-Type", "text/html"), body: []byte(`please click here if you are not redirected`)}, true, 0},
		{"cloudflare 503", "npm", response{status: 503, header: h("Content-Type", "text/html")}, true, 0},
		{"keenable 429", "keenable", response{status: 429, header: h("Content-Type", "application/json", "Retry-After", "30"), body: []byte(`{"error":{"code":"rate_limited"}}`)}, true, 30 * time.Second},
		{"github reset", "github", response{status: 403, header: h("X-RateLimit-Remaining", "0", "X-RateLimit-Reset", "0")}, true, 0},
		{"json answer", "npm", response{status: 200, header: h("Content-Type", "application/json"), body: []byte(`{"anomaly-modal":1}`)}, false, 0},
		{"not found", "wiktionary", response{status: 404, header: h("Content-Type", "application/json")}, false, 0},
		{"202 elsewhere", "npm", response{status: 202, header: h("Content-Type", "application/json")}, false, 0},
	}
	for _, c := range cases {
		blocked, retry := classify(c.engine, &c.resp, now)
		if blocked != c.blocked || retry != c.retry {
			t.Errorf("%s: blocked=%v retry=%v, want %v %v", c.name, blocked, retry, c.blocked, c.retry)
		}
	}
	if got := retryAfter(h("Retry-After", now.Add(2*time.Minute).UTC().Format(http.TimeFormat)), now); got < 110*time.Second || got > 2*time.Minute {
		t.Errorf("Retry-After as a date = %v", got)
	}
}

func TestPauseSurvivesRestartAndBlocksRequests(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)}
	state := filepath.Join(t.TempDir(), "health.json")
	f := newFixtures(t).standard()
	f.set("lite.duckduckgo.com/lite/", route{status: 202, file: "ddglite_anomaly.html"})
	e := testServiceWith(t, f, serviceOpts{stateFile: state, now: c.now})
	if _, err := searchDDGLite(context.Background(), e, newQuery("x", "en-US")); err == nil {
		t.Fatal("block not reported")
	}
	var events [][]EngineInfo
	e.gov.onChange = func(infos []EngineInfo) { events = append(events, infos) }
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}

	again := testServiceWith(t, f, serviceOpts{stateFile: state, now: c.now})
	until := again.gov.paused("ddg-lite")
	if want := c.now().Add(15 * time.Minute); !until.Equal(want) {
		t.Fatalf("restarted daemon forgot the pause: until=%v want %v", until, want)
	}
	for _, info := range again.gov.infos() {
		if info.ID == "ddg-lite" && (info.State != "paused" || info.Until == "") {
			t.Errorf("engine info = %+v", info)
		}
	}
	before := f.count("lite.duckduckgo.com/lite/")
	_, err := searchDDGLite(context.Background(), again, newQuery("y", "en-US"))
	if _, ok := isPaused(err); !ok || f.count("lite.duckduckgo.com/lite/") != before {
		t.Fatalf("paused engine was asked: err=%v", err)
	}
	c.add(16 * time.Minute)
	if _, err := searchDDGLite(context.Background(), again, newQuery("y", "en-US")); err == nil {
		t.Fatal("still blocked by the fixture, expected another strike")
	}
	if d := again.gov.paused("ddg-lite").Sub(c.now()); d != 30*time.Minute {
		t.Errorf("second strike after restart paused %v, want 30 min", d)
	}
}

func TestBucketLevelsSurviveRestart(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)}
	state := filepath.Join(t.TempDir(), "health.json")
	g := newGovernor(state, c.now)
	for range 2 {
		release, err := g.acquire(context.Background(), "ddg-lite", nil)
		if err != nil {
			t.Fatal(err)
		}
		release()
	}
	if err := g.close(); err != nil {
		t.Fatal(err)
	}
	g2 := newGovernor(state, c.now)
	g2.mu.Lock()
	st := g2.engines["ddg-lite"]
	st.refill(c.now())
	w := st.wait()
	g2.mu.Unlock()
	if w <= 0 {
		t.Error("a restart refilled the DuckDuckGo burst")
	}
}
