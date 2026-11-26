// Package search is Ryoku's web search: keyless sources queried under one
// rate governor, ranked and merged here, and served to clients (the bar,
// the CLI) by the ryoseek daemon over JSON lines.
package search

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ryoget/internal/thumbs"
)

// Item is one result row, as clients render it.
type Item struct {
	Key     string         `json:"key"`
	Source  string         `json:"source"`
	Kind    string         `json:"kind"`
	Title   string         `json:"title"`
	URL     string         `json:"url"`
	Snippet string         `json:"snippet"`
	Site    string         `json:"site"`
	Date    string         `json:"date"`
	Thumb   string         `json:"thumb"`
	Meta    map[string]any `json:"meta"`

	thumbURL string
	// the thumbnail lives on an arbitrary host (a favicon), so it goes
	// through the guarded page client
	thumbPage bool
	pop       float64
	score     float64
	tcov      float64
	rel       float64
}

const (
	KindPage       = "page"
	KindArticle    = "article"
	KindRepo       = "repo"
	KindQuestion   = "question"
	KindVideo      = "video"
	KindPackage    = "package"
	KindThread     = "thread"
	KindDoc        = "doc"
	KindSuggestion = "suggestion"
	KindDefinition = "definition"
)

// SourceInfo is what the ready line advertises; its order is the section
// order in the All scope.
type SourceInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Scope  string `json:"scope"`
	Glyph  string `json:"glyph"`
	Typing bool   `json:"typing"`
}

var Scopes = []string{"web", "knowledge", "code", "linux", "video", "discuss"}

type mode int

const (
	modeFull mode = 1 << iota
	modeSuggest
	modeBoth = modeFull | modeSuggest
)

type source struct {
	SourceInfo
	// how many rows the source shows in the All scope
	limit int
	// relevance an item needs to show in the All scope of a full search
	gate gate
	// section rank whenever the source has rows; 0 = from relevance
	lead  int
	modes mode
	// fetches through its own per-engine caches (the merged web lane)
	direct   bool
	archOnly bool
	// nil = always; else the source only runs for queries it can answer
	applies func(q Query) bool
	search  func(ctx context.Context, e *Service, q Query) ([]Item, bool, error)
	detail  func(ctx context.Context, e *Service, key, locale string) (Detail, error)
}

func registry() []*source {
	return []*source{
		dictionarySource, suggestSource, webSource, wikipediaSource,
		githubSource, stackoverflowSource, mdnSource, npmSource, cratesSource,
		archwikiSource, archpkgSource, aurSource, flathubSource,
		youtubeSource, hackernewsSource,
	}
}

const thumbTimeout = 2 * time.Second

type Options struct {
	// engine health and budgets, kept across daemon restarts
	StateFile string
	// results/ and thumbs/ live here
	CacheDir string
	Version  string
	Client   *http.Client
	Now      func() time.Time
	// file read to decide whether Arch-only sources apply
	OSRelease string
}

type Service struct {
	client     *http.Client
	pageClient *http.Client
	// the caller's client as given: innertube has to present a YouTube
	// client's own user agent, so it must not get ours
	rawClient *http.Client
	thumbDir  string
	sources   []*source
	byID      map[string]*source
	now       func() time.Time

	gov   *governor
	cache *store

	base    context.Context
	stop    context.CancelFunc
	fmu     sync.Mutex
	flights map[string]*flight
	running sync.WaitGroup

	cmu     sync.Mutex
	clients map[*client]bool

	thumbTimeout time.Duration
}

// UserAgent is the identity every request carries.
func UserAgent(version string) string {
	if version == "" {
		version = "dev"
	}
	return "ryoseek/" + version + " (+https://ryoku.dev) Go-http-client/2.0"
}

func New(o Options) (*Service, error) {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Client == nil {
		o.Client = newClient()
	}
	pageClient := newPageClient()
	if o.Client.Transport != nil {
		// a caller-supplied transport (tests, proxies) serves pages too
		pageClient = &http.Client{Transport: o.Client.Transport, CheckRedirect: checkRedirect}
	}
	ua := UserAgent(o.Version)
	e := &Service{
		client:       withIdentity(o.Client, ua),
		rawClient:    o.Client,
		pageClient:   withIdentity(pageClient, ua),
		now:          o.Now,
		byID:         map[string]*source{},
		flights:      map[string]*flight{},
		clients:      map[*client]bool{},
		thumbTimeout: thumbTimeout,
	}
	resultDir := ""
	if o.CacheDir != "" {
		resultDir = filepath.Join(o.CacheDir, "results")
		e.thumbDir = filepath.Join(o.CacheDir, "thumbs")
	}
	e.cache = newStore(resultDir, o.Now)
	e.gov = newGovernor(o.StateFile, o.Now)
	e.gov.onChange = e.broadcastEngines
	e.base, e.stop = context.WithCancel(context.Background())
	if o.OSRelease == "" {
		o.OSRelease = "/etc/os-release"
	}
	arch := archBased(o.OSRelease)
	for _, s := range registry() {
		if s.archOnly && !arch {
			continue
		}
		e.sources = append(e.sources, s)
		e.byID[s.ID] = s
	}
	return e, nil
}

// Idle reports that no client is connected and nothing is being fetched.
func (e *Service) Idle() bool {
	e.cmu.Lock()
	clients := len(e.clients)
	e.cmu.Unlock()
	e.fmu.Lock()
	flights := len(e.flights)
	e.fmu.Unlock()
	return clients == 0 && flights == 0
}

// Close lets requests already sent finish into the cache (up to 10 s), then
// writes the engine state.
func (e *Service) Close() error {
	done := make(chan struct{})
	go func() {
		e.running.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
	e.stop()
	return e.gov.close()
}

func (e *Service) Sources() []SourceInfo {
	out := make([]SourceInfo, len(e.sources))
	for i, s := range e.sources {
		out[i] = s.SourceInfo
		out[i].Typing = s.runsIn()&modeSuggest != 0
	}
	return out
}

// pick resolves a scope argument (all, a scope name, or one source id) and
// keeps the sources that run in the mode. Anything narrower than All gets
// each source's full list.
func (e *Service) pick(scope string, m mode) ([]*source, bool) {
	var out []*source
	full := true
	switch s, ok := e.byID[scope]; {
	case scope == "" || scope == "all":
		full = false
		out = e.sources
	case ok:
		out = []*source{s}
	default:
		for _, s := range e.sources {
			if s.Scope == scope {
				out = append(out, s)
			}
		}
	}
	kept := out[:0:0]
	for _, s := range out {
		if s.runsIn()&m != 0 {
			kept = append(kept, s)
		}
	}
	return kept, full
}

// runsIn reads an unset mode as full search only.
func (s *source) runsIn() mode {
	if s.modes == 0 {
		return modeFull
	}
	return s.modes
}

func archBased(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok || (k != "ID" && k != "ID_LIKE") {
			continue
		}
		for _, id := range strings.Fields(strings.Trim(v, `"'`)) {
			if id == "arch" {
				return true
			}
		}
	}
	return false
}

var localePattern = regexp.MustCompile(`^[a-z]{2,3}(-[a-z0-9]{2,8})?$`)

// normLocale accepts "pt_BR", "pt-BR.UTF-8", "de"; anything else is en-US.
// The result keeps the region upper case ("pt-BR").
func normLocale(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "_", "-")
	if !localePattern.MatchString(s) {
		return "en-US"
	}
	if p, r, ok := strings.Cut(s, "-"); ok {
		if len(r) == 2 {
			r = strings.ToUpper(r)
		}
		return p + "-" + r
	}
	return s
}

// normLang is the lower-case form the source tables key on ("pt-br").
func normLang(s string) string { return strings.ToLower(normLocale(s)) }

func primaryLang(lang string) string {
	p, _, _ := strings.Cut(lang, "-")
	return p
}

type stateMsg struct {
	ID     int64  `json:"id"`
	Source string `json:"source"`
	State  string `json:"state"`
	Until  string `json:"until,omitempty"`
}

type doneMsg struct {
	ID     int64  `json:"id"`
	Source string `json:"source"`
	State  string `json:"state"`
	Items  []Item `json:"items"`
	More   bool   `json:"more"`
	// section order in All: higher first, ties keep the ready-line order
	Rank int `json:"rank"`
}

type errorMsg struct {
	ID     int64  `json:"id"`
	Source string `json:"source"`
	State  string `json:"state"`
	Error  string `json:"error"`
}

type finalMsg struct {
	ID    int64  `json:"id"`
	State string `json:"state"`
}

type claims struct {
	mu   sync.Mutex
	seen map[string]bool
}

// claim lets the first source to show a link keep it; later sections skip it.
func (c *claims) claim(raw string) bool {
	k := normURL(raw)
	if k == "" {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen[k] {
		return false
	}
	c.seen[k] = true
	return true
}

func (e *Service) query(ctx context.Context, id int64, text, scope, modeName, locale string, emit func(any)) {
	m := modeFull
	if modeName == "suggest" {
		m = modeSuggest
	}
	q := newQuery(text, locale)
	q.suggest = m == modeSuggest
	srcs, full := e.pick(scope, m)
	runs := srcs[:0:0]
	for _, s := range srcs {
		if s.applies == nil || s.applies(q) {
			runs = append(runs, s)
		}
	}
	srcs = runs
	if q.Text == "" || len(srcs) == 0 {
		emit(finalMsg{ID: id, State: "done"})
		return
	}
	for _, s := range srcs {
		emit(stateMsg{ID: id, Source: s.ID, State: "loading"})
	}
	shared := &claims{seen: map[string]bool{}}
	var wg sync.WaitGroup
	for _, s := range srcs {
		wg.Add(1)
		go func(s *source) {
			defer wg.Done()
			e.runSource(ctx, id, s, q, full, shared, emit)
		}(s)
	}
	wg.Wait()
	emit(finalMsg{ID: id, State: "done"})
}

func (e *Service) runSource(ctx context.Context, id int64, s *source, q Query, full bool, shared *claims, emit func(any)) {
	var once sync.Once
	waiting := func() {
		once.Do(func() { emit(stateMsg{ID: id, Source: s.ID, State: "waiting"}) })
	}
	items, more, err := e.results(ctx, s, q, waiting)
	if ctx.Err() != nil {
		return
	}
	if p, ok := isPaused(err); ok {
		emit(stateMsg{ID: id, Source: s.ID, State: "paused", Until: p.until.UTC().Format(time.RFC3339)})
		return
	}
	if err != nil {
		emit(errorMsg{ID: id, Source: s.ID, State: "error", Error: errorText(err)})
		return
	}
	limit := len(items)
	if !full && s.limit < limit {
		limit = s.limit
	}
	gated := !full && !q.suggest
	out := make([]Item, 0, limit)
	i := 0
	for ; i < len(items) && len(out) < limit; i++ {
		if gated && !s.gate.pass(items[i]) {
			// a dropped row still exists in the source's own view
			more = true
			continue
		}
		if shared.claim(items[i].URL) {
			out = append(out, items[i])
		}
	}
	if i < len(items) {
		more = true
	}
	r := sectionRank(out)
	if s.lead > 0 && len(out) > 0 {
		r = s.lead
	}
	e.fillThumbs(ctx, out)
	emit(doneMsg{ID: id, Source: s.ID, State: "done", Items: out, More: more, Rank: r})
}

// results returns a source's ranked list. The cache keeps the list as the
// source gave it and the ranking runs on every read: relevance scores are
// not stored, so a list read back from disk is ranked like a fresh one.
func (e *Service) results(ctx context.Context, s *source, q Query, waiting func()) ([]Item, bool, error) {
	label := func(items []Item) []Item {
		for i := range items {
			items[i].Source = s.ID
			if items[i].Key == "" {
				items[i].Key = items[i].URL
			}
		}
		return items
	}
	if s.direct {
		items, more, err := s.search(withWaiting(ctx, waiting), e, q)
		if err != nil {
			return nil, false, err
		}
		return rank(label(items), q), more, nil
	}
	pol := cachePolicy{ttl: resultTTL, disk: true}
	modeKey := "full"
	if q.suggest {
		pol = cachePolicy{ttl: suggestTTL}
		modeKey = "suggest"
	}
	key := "src|" + s.ID + "|" + modeKey + "|" + q.Lang + "|" + q.norm
	v, err := e.do(ctx, key, pol, waiting, func(fctx context.Context) (any, error) {
		items, more, err := s.search(fctx, e, q)
		if err != nil {
			return nil, err
		}
		return cached{items: label(items), more: more}, nil
	})
	if err != nil {
		return nil, false, err
	}
	c := v.(cached)
	// a private copy: rank reorders it and thumbnails are filled into it
	return rank(append([]Item(nil), c.items...), q), c.more, nil
}

type waitingKey struct{}

// withWaiting carries a source's "waiting" notice to the per-engine flights
// a direct source starts.
func withWaiting(ctx context.Context, fn func()) context.Context {
	return context.WithValue(ctx, waitingKey{}, fn)
}

func waitingOf(ctx context.Context) func() {
	fn, _ := ctx.Value(waitingKey{}).(func())
	return fn
}

func (e *Service) fillThumbs(ctx context.Context, items []Item) {
	if e.thumbDir == "" {
		return
	}
	var wg sync.WaitGroup
	for i := range items {
		if items[i].thumbURL == "" {
			continue
		}
		wg.Add(1)
		go func(it *Item) {
			defer wg.Done()
			tctx, cancel := context.WithTimeout(ctx, e.thumbTimeout)
			defer cancel()
			client := e.client
			if it.thumbPage {
				client = e.pageClient
			}
			if p, err := thumbs.Fetch(tctx, client, e.thumbDir, it.thumbURL); err == nil {
				it.Thumb = p
			}
		}(&items[i])
	}
	wg.Wait()
}
