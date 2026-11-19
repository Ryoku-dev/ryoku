package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// bucketDef refills one token every `every`, holding at most `burst`.
type bucketDef struct {
	every time.Duration
	burst float64
}

type engineDef struct {
	id   string
	name string
	// the result sources this engine feeds
	sources []string
	// every bucket must hold a token before a request goes out
	buckets []bucketDef
	// requests on the wire at once; 0 = no limit
	concurrency int
	// per request on the wire
	timeout time.Duration
}

const (
	defaultWireTimeout = 4 * time.Second
	backoffBase        = 15 * time.Minute
	backoffCap         = 6 * time.Hour
	// shorter waits are not worth a "waiting" line in the bar
	waitingNotice = 400 * time.Millisecond
)

// The budgets keep every engine well under what it allows anonymous clients
// (search-options section 7.2), so a busy afternoon never trips a gate.
func engineTable() []engineDef {
	gentle := []bucketDef{{every: 250 * time.Millisecond, burst: 4}}
	src := func(ids ...string) []string { return ids }
	return []engineDef{
		{id: "keenable", name: "Keenable", sources: src("web"), buckets: []bucketDef{{every: 500 * time.Millisecond, burst: 1}, {every: 6 * time.Second, burst: 30}}},
		{id: "ddg-lite", name: "DuckDuckGo", sources: src("web"), buckets: []bucketDef{{every: 4 * time.Second, burst: 2}, {every: time.Minute, burst: 10}}},
		{id: "bing-rss", name: "Bing", sources: src("web"), buckets: []bucketDef{{every: 5 * time.Second, burst: 1}}},
		{id: "ddg-ac", name: "DuckDuckGo suggestions", sources: src("suggest"), buckets: []bucketDef{{every: 150 * time.Millisecond, burst: 3}}},
		{id: "bing-suggest", name: "Bing suggestions", sources: src("suggest"), buckets: []bucketDef{{every: 150 * time.Millisecond, burst: 3}}},
		{id: "wikipedia", name: "Wikipedia", sources: src("wikipedia"), buckets: []bucketDef{{every: 300 * time.Millisecond, burst: 10}}, concurrency: 3},
		{id: "freedictionary", name: "FreeDictionaryAPI", sources: src("dictionary"), buckets: []bucketDef{{every: 3600 * time.Millisecond, burst: 20}}},
		{id: "wiktionary", name: "Wiktionary", sources: src("dictionary"), buckets: []bucketDef{{every: 300 * time.Millisecond, burst: 10}}, concurrency: 3},
		// anonymous search allows 10 a minute
		{id: "github", name: "GitHub", sources: src("github"), buckets: []bucketDef{{every: 6 * time.Second, burst: 3}}},
		// repository facts spend the hourly anonymous budget of 60
		{id: "github-api", name: "GitHub repositories", sources: src("github"), buckets: []bucketDef{{every: time.Minute, burst: 5}}},
		// 300 a day per address, shared by search and answers
		{id: "stackexchange", name: "Stack Exchange", sources: src("stackoverflow"), buckets: []bucketDef{{every: 288 * time.Second, burst: 10}}},
		{id: "mdn", name: "MDN Web Docs", sources: src("mdn"), buckets: gentle},
		{id: "npm", name: "npm", sources: src("npm"), buckets: gentle},
		{id: "crates", name: "crates.io", sources: src("crates"), buckets: []bucketDef{{every: time.Second, burst: 3}}},
		{id: "archwiki", name: "ArchWiki", sources: src("archwiki"), buckets: gentle, concurrency: 1},
		{id: "archlinux", name: "Arch Linux packages", sources: src("archpkg"), buckets: gentle},
		// the AUR allows 4000 RPC calls a day
		{id: "aur", name: "AUR", sources: src("aur"), buckets: []bucketDef{{every: 22 * time.Second, burst: 10}}},
		{id: "flathub", name: "Flathub", sources: src("flathub"), buckets: gentle},
		{id: "hackernews", name: "Hacker News", sources: src("hackernews"), buckets: gentle, timeout: 6 * time.Second},
		{id: "youtube", name: "YouTube", sources: src("youtube"), buckets: []bucketDef{{every: time.Second, burst: 3}}},
	}
}

// EngineInfo is the public health of one engine.
type EngineInfo struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
	State   string   `json:"state"`
	Until   string   `json:"until"`
}

type engineState struct {
	BlockedUntil time.Time `json:"blockedUntil,omitzero"`
	Strikes      int       `json:"strikes,omitempty"`
	LastOK       time.Time `json:"lastOk,omitzero"`
	LastRequest  time.Time `json:"lastRequest,omitzero"`
	Tokens       []float64 `json:"tokens,omitempty"`
	TokensAt     time.Time `json:"tokensAt,omitzero"`

	def   *engineDef
	slots chan struct{}
	timer *time.Timer
}

type healthFile struct {
	Version int                     `json:"version"`
	Engines map[string]*engineState `json:"engines"`
}

// pausedError reports an engine that is blocked and will not be asked.
type pausedError struct {
	engine string
	until  time.Time
}

func (e *pausedError) Error() string {
	return e.engine + " paused until " + e.until.Format(time.RFC3339)
}

type governor struct {
	mu      sync.Mutex
	engines map[string]*engineState
	order   []string
	now     func() time.Time
	file    string
	dirty   bool
	flush   *time.Timer
	// called outside the lock whenever an engine is paused or resumes
	onChange func([]EngineInfo)
}

func newGovernor(file string, now func() time.Time) *governor {
	g := &governor{engines: map[string]*engineState{}, now: now, file: file}
	saved := healthFile{}
	if file != "" {
		if b, err := os.ReadFile(file); err == nil {
			_ = json.Unmarshal(b, &saved)
		}
	}
	for _, d := range engineTable() {
		d := d
		st := saved.Engines[d.id]
		if st == nil {
			st = &engineState{}
		}
		st.def = &d
		if len(st.Tokens) != len(d.buckets) {
			st.Tokens = make([]float64, len(d.buckets))
			for i, b := range d.buckets {
				st.Tokens[i] = b.burst
			}
			st.TokensAt = now()
		}
		if d.concurrency > 0 {
			st.slots = make(chan struct{}, d.concurrency)
		}
		g.engines[d.id] = st
		g.order = append(g.order, d.id)
		g.armResume(d.id, st)
	}
	return g
}

func (g *governor) def(id string) *engineDef {
	if st, ok := g.engines[id]; ok {
		return st.def
	}
	return nil
}

func (g *governor) infos() []EngineInfo {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.infosLocked()
}

func (g *governor) infosLocked() []EngineInfo {
	now := g.now()
	out := make([]EngineInfo, 0, len(g.order))
	for _, id := range g.order {
		st := g.engines[id]
		info := EngineInfo{ID: id, Name: st.def.name, Sources: st.def.sources, State: "ok"}
		if now.Before(st.BlockedUntil) {
			info.State = "paused"
			info.Until = st.BlockedUntil.UTC().Format(time.RFC3339)
		}
		out = append(out, info)
	}
	return out
}

// paused returns the time an engine resumes, or zero when it may be asked.
func (g *governor) paused(id string) time.Time {
	g.mu.Lock()
	defer g.mu.Unlock()
	st := g.engines[id]
	if st == nil || !g.now().Before(st.BlockedUntil) {
		return time.Time{}
	}
	return st.BlockedUntil
}

func (st *engineState) refill(now time.Time) {
	dt := now.Sub(st.TokensAt)
	if dt <= 0 {
		return
	}
	for i, b := range st.def.buckets {
		st.Tokens[i] = math.Min(b.burst, st.Tokens[i]+float64(dt)/float64(b.every))
	}
	st.TokensAt = now
}

func (st *engineState) wait() time.Duration {
	var w time.Duration
	for i, b := range st.def.buckets {
		if st.Tokens[i] < 1 {
			if d := time.Duration((1 - st.Tokens[i]) * float64(b.every)); d > w {
				w = d
			}
		}
	}
	return w
}

// acquire holds the caller until the engine's budget allows a request. A
// paused engine is never asked: the caller gets *pausedError at once.
func (g *governor) acquire(ctx context.Context, id string, waiting func()) (release func(), err error) {
	st := g.engines[id]
	if st == nil {
		return func() {}, nil
	}
	if st.slots != nil {
		if len(st.slots) == cap(st.slots) && waiting != nil {
			waiting()
		}
		select {
		case st.slots <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	release = func() {
		if st.slots != nil {
			<-st.slots
		}
	}
	notified := false
	for {
		g.mu.Lock()
		now := g.now()
		if now.Before(st.BlockedUntil) {
			until := st.BlockedUntil
			g.mu.Unlock()
			release()
			return nil, &pausedError{engine: id, until: until}
		}
		st.refill(now)
		w := st.wait()
		if w <= 0 {
			for i := range st.Tokens {
				st.Tokens[i]--
			}
			st.LastRequest = now
			g.markDirtyLocked()
			g.mu.Unlock()
			return release, nil
		}
		g.mu.Unlock()
		if w >= waitingNotice && !notified && waiting != nil {
			notified = true
			waiting()
		}
		t := time.NewTimer(w)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			release()
			return nil, ctx.Err()
		}
	}
}

func (g *governor) ok(id string) {
	g.mu.Lock()
	st := g.engines[id]
	if st == nil {
		g.mu.Unlock()
		return
	}
	st.Strikes = 0
	st.LastOK = g.now()
	g.markDirtyLocked()
	g.mu.Unlock()
}

// strike pauses an engine after a block: Retry-After when the engine sent
// one, else 15 min doubling per strike up to 6 h.
func (g *governor) strike(id string, retry time.Duration) time.Time {
	g.mu.Lock()
	st := g.engines[id]
	if st == nil {
		g.mu.Unlock()
		return time.Time{}
	}
	st.Strikes++
	d := retry
	if d <= 0 {
		d = backoffBase << (st.Strikes - 1)
		if st.Strikes > 6 || d > backoffCap {
			d = backoffCap
		}
	}
	return g.pauseLocked(id, st, d)
}

// hold pauses an engine without a strike, for budgets the engine announced
// itself (quota resets, backoff fields).
func (g *governor) hold(id string, d time.Duration) time.Time {
	g.mu.Lock()
	st := g.engines[id]
	if st == nil {
		g.mu.Unlock()
		return time.Time{}
	}
	return g.pauseLocked(id, st, d)
}

func (g *governor) pauseLocked(id string, st *engineState, d time.Duration) time.Time {
	until := g.now().Add(d)
	changed := false
	if until.After(st.BlockedUntil) {
		changed = !g.now().Before(st.BlockedUntil)
		st.BlockedUntil = until
	}
	g.markDirtyLocked()
	g.armResume(id, st)
	until = st.BlockedUntil
	var infos []EngineInfo
	if changed && g.onChange != nil {
		infos = g.infosLocked()
	}
	cb := g.onChange
	g.mu.Unlock()
	if infos != nil {
		cb(infos)
	}
	return until
}

// armResume announces the engine again once its pause runs out.
func (g *governor) armResume(id string, st *engineState) {
	if st.timer != nil {
		st.timer.Stop()
	}
	d := st.BlockedUntil.Sub(g.now())
	if d <= 0 {
		return
	}
	st.timer = time.AfterFunc(d+10*time.Millisecond, func() {
		g.mu.Lock()
		cb := g.onChange
		var infos []EngineInfo
		if cb != nil && !g.now().Before(st.BlockedUntil) {
			infos = g.infosLocked()
		}
		g.mu.Unlock()
		if infos != nil {
			cb(infos)
		}
	})
}

func (g *governor) markDirtyLocked() {
	if g.file == "" {
		return
	}
	g.dirty = true
	if g.flush == nil {
		g.flush = time.AfterFunc(500*time.Millisecond, func() { _ = g.save() })
	}
}

func (g *governor) save() error {
	g.mu.Lock()
	g.flush = nil
	if !g.dirty || g.file == "" {
		g.mu.Unlock()
		return nil
	}
	g.dirty = false
	b, err := json.MarshalIndent(healthFile{Version: 1, Engines: g.engines}, "", "  ")
	g.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(g.file), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(g.file), ".health-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), g.file)
}

func (g *governor) close() error {
	g.mu.Lock()
	if g.flush != nil {
		g.flush.Stop()
		g.flush = nil
	}
	for _, st := range g.engines {
		if st.timer != nil {
			st.timer.Stop()
		}
	}
	g.mu.Unlock()
	return g.save()
}

var blockMarkers = [][]byte{
	[]byte("anomaly-modal"), []byte("challenge-form"),
	[]byte("<title>captcha"), []byte("captcha-delivery.com"), []byte("g-recaptcha"), []byte("h-captcha"),
	[]byte("anubis_challenge"), []byte("making sure you're not a bot"), []byte("within.website/x/cmd/anubis"),
	[]byte("<title>just a moment"), []byte("cf-chl-"), []byte("enable javascript and cookies to continue"),
	[]byte("please click here if you are not redirected"),
}

// classify tells a block (the engine refuses us) from an answer. A block is
// never read as "no results".
func classify(engine string, resp *response, now time.Time) (blocked bool, retry time.Duration) {
	retry = retryAfter(resp.header, now)
	if resp.header.Get("X-RateLimit-Remaining") == "0" {
		if reset, err := strconv.ParseInt(resp.header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			if d := time.Unix(reset, 0).Sub(now); d > retry {
				retry = d
			}
		}
	}
	switch resp.status {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return true, retry
	case http.StatusAccepted:
		if strings.HasPrefix(engine, "ddg") {
			return true, retry
		}
	}
	ct := strings.ToLower(resp.header.Get("Content-Type"))
	if strings.Contains(ct, "html") || ct == "" {
		head := resp.body
		if len(head) > 64<<10 {
			head = head[:64<<10]
		}
		head = bytes.ToLower(head)
		for _, m := range blockMarkers {
			if bytes.Contains(head, m) {
				return true, retry
			}
		}
	}
	return false, 0
}

func retryAfter(h http.Header, now time.Time) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

func isPaused(err error) (*pausedError, bool) {
	var p *pausedError
	ok := errors.As(err, &p)
	return p, ok
}

// earliest picks the first resume time among paused engines.
func earliest(errs ...error) *pausedError {
	var out []*pausedError
	for _, err := range errs {
		if p, ok := isPaused(err); ok {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(a, b int) bool { return out[a].until.Before(out[b].until) })
	return out[0]
}
