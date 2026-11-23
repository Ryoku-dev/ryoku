// Package engine owns ryoget's downloads: the task list and its persistence,
// the queue, the run of each task through the resolver, fetchers, finisher or
// torrent client, the progress sampling behind the wire's task and stats
// events, and the settings file.
package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	i18n "ryoku-i18n"

	"ryoget/internal/httpdl"
	"ryoget/internal/paths"
	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

const (
	tickEvery   = 200 * time.Millisecond
	emitEvery   = 200 * time.Millisecond
	saveDelay   = 500 * time.Millisecond
	speedTau    = 2.0
	historyLen  = 60
	limitBurst  = 256 << 10
	tempPrefix  = ".ryoget-"
	stateFormat = 1
)

const (
	wantPaused  = "paused"
	wantStopped = "stopped"
	wantRemoved = "removed"
	wantQueued  = "queued"
)

type Options struct {
	StateFile  string
	ConfigFile string
	// YTDLP, FFmpeg and Cache travel to the resolvers; the engine fills HTTP and Limit
	Env plan.Env
	// called with the engine lock held: marshal or copy synchronously, never call back in
	OnEvent func(name string, data any)
	// called with the lock held as well; hand slow work to a goroutine
	OnFinish func(proto.Task)
	OnFail   func(proto.Task)
	Log      *log.Logger
}

// record is one persisted row: the wire Task plus what a later run needs.
type record struct {
	proto.Task
	Sel []int `json:"sel,omitempty"`
	// the link has been resolved to its kind
	Known bool `json:"known,omitempty"`
	// for direct files the resolver may point somewhere else than the pasted link
	FetchURL string            `json:"fetchUrl,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	// this task is one item of a post that was split into a task per item; it never splits again
	Expanded bool `json:"expanded,omitempty"`
}

type stateFile struct {
	Version int      `json:"version"`
	Tasks   []record `json:"tasks"`
}

type runState struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type task struct {
	record
	run       *runState
	want      string
	delFile   bool
	prog      *progress
	tor       Torrent
	speed     float64
	lastMoved int64
	emitted   time.Time
	pending   bool
	sig       string
	// one sample per second of the running task's speeds, oldest first; memory only
	histDown []int64
	histUp   []int64
}

type Engine struct {
	opts  Options
	env   *plan.Env
	dl    *httpdl.Downloader
	down  *rate.Limiter
	up    *rate.Limiter
	parts Parts
	log   *log.Logger

	ctx  context.Context
	stop context.CancelFunc
	wg   sync.WaitGroup

	saveMu sync.Mutex

	mu           sync.Mutex
	settings     proto.Settings
	tasks        map[string]*task
	order        []string
	torrents     Torrents
	torResolving int
	closing      bool
	ticking      bool
	ticks        int
	history      []int64
	saveTimer    *time.Timer
}

func New(opts Options) *Engine {
	if opts.StateFile == "" {
		opts.StateFile = paths.TasksFile()
	}
	if opts.ConfigFile == "" {
		opts.ConfigFile = paths.ConfigFile()
	}
	e := &Engine{
		opts:     opts,
		down:     rate.NewLimiter(rate.Inf, limitBurst),
		up:       rate.NewLimiter(rate.Inf, limitBurst),
		settings: DefaultSettings(),
		tasks:    map[string]*task{},
		history:  make([]int64, historyLen),
		log:      opts.Log,
	}
	if e.log == nil {
		e.log = log.New(io.Discard, "", 0)
	}
	e.ctx, e.stop = context.WithCancel(context.Background())
	client := httpdl.NewClient()
	env := opts.Env
	env.HTTP = client
	env.Limit = e.down
	if env.Cache == "" {
		env.Cache = paths.CacheDir()
	}
	e.env = &env
	e.dl = &httpdl.Downloader{
		Client: client,
		Limit:  e.down,
		Conns: func() int {
			e.mu.Lock()
			defer e.mu.Unlock()
			return e.settings.Connections
		},
		MinSplit: func() int64 {
			e.mu.Lock()
			defer e.mu.Unlock()
			return int64(e.settings.MinSplitMiB) << 20
		},
	}
	return e
}

// Env is what resolvers and fetchers share with the engine: its HTTP client
// and the global download limiter.
func (e *Engine) Env() *plan.Env { return e.env }

func (e *Engine) Downloader() *httpdl.Downloader { return e.dl }

func (e *Engine) Limiters() (down, up *rate.Limiter) { return e.down, e.up }

// Use installs the concrete parts; call it before Start.
func (e *Engine) Use(p Parts) {
	if p.Resolver == nil {
		p.Resolver = directResolver{client: e.env}
	}
	fetchers := map[plan.PartKind]Fetcher{plan.PartHTTP: e.dl}
	for k, f := range p.Fetchers {
		fetchers[k] = f
	}
	p.Fetchers = fetchers
	e.parts = p
}

// Start loads settings and tasks and picks up where the last run stopped.
func (e *Engine) Start() error {
	if e.parts.Fetchers == nil {
		e.Use(Parts{})
	}
	s, err := LoadSettings(e.opts.ConfigFile)
	if err != nil {
		e.log.Printf("settings: %v (using defaults)", err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.settings = s
	e.applyLimits()
	if err := e.load(); err != nil {
		e.log.Printf("tasks: %v", err)
	}
	e.pruneLocked(time.Now())
	e.schedule()
	return nil
}

func (e *Engine) load() error {
	b, err := os.ReadFile(e.opts.StateFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var sf stateFile
	if err := json.Unmarshal(b, &sf); err != nil {
		return err
	}
	for _, r := range sf.Tasks {
		if r.ID == "" || e.tasks[r.ID] != nil {
			continue
		}
		t := &task{record: r}
		t.Speed, t.UpSpeed, t.Conns, t.ETA, t.Peers, t.Seeds = 0, 0, 0, -1, 0, 0
		if t.Pieces == "" {
			t.Pieces = emptyPieces
		}
		if t.Files == nil {
			t.Files = []proto.File{}
		}
		switch t.State {
		case proto.StateQueued, proto.StateResolving, proto.StateActive, proto.StateMerging, proto.StateSeeding:
			t.Stage = ""
			if e.settings.ResumeOnStart {
				t.State = proto.StateQueued
			} else {
				t.State = proto.StatePaused
			}
		}
		e.tasks[t.ID] = t
		e.order = append(e.order, t.ID)
	}
	return nil
}

// Close stops every run, keeps their states for the next start, and saves.
func (e *Engine) Close() {
	e.mu.Lock()
	e.closing = true
	for _, t := range e.tasks {
		if t.run != nil {
			t.run.cancel()
		}
	}
	e.mu.Unlock()
	e.stop()
	e.wg.Wait()
	e.mu.Lock()
	tc := e.torrents
	e.torrents = nil
	if e.saveTimer != nil {
		e.saveTimer.Stop()
		e.saveTimer = nil
	}
	e.mu.Unlock()
	if tc != nil {
		_ = tc.Close()
	}
	e.flush()
}

func (e *Engine) logf(format string, a ...any) { e.log.Printf(format, a...) }

func perr(code, msg string) *proto.Error { return &proto.Error{Code: code, Message: msg} }

func notFound() *proto.Error {
	return perr(proto.CodeNotFound, i18n.T("No such download."))
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// snap copies a task for the wire, so later in-place edits cannot race a marshal.
func (t *task) snap() proto.Task {
	c := t.Task
	c.Files = append([]proto.File{}, t.Files...)
	if t.Error != nil {
		er := *t.Error
		c.Error = &er
	}
	if t.Format != nil {
		f := *t.Format
		c.Format = &f
	}
	return c
}

func (e *Engine) event(name string, data any) {
	if e.opts.OnEvent != nil {
		e.opts.OnEvent(name, data)
	}
}

// emitTask sends a task at most every 200 ms; a change inside that window is
// coalesced into one later send carrying the latest state.
func (e *Engine) emitTask(t *task) {
	if e.tasks[t.ID] != t {
		return
	}
	since := time.Since(t.emitted)
	if since >= emitEvery {
		t.pending = false
		t.emitted = time.Now()
		e.event("task", t.snap())
		return
	}
	if t.pending {
		return
	}
	t.pending = true
	time.AfterFunc(emitEvery-since, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if t.pending && e.tasks[t.ID] == t {
			t.pending = false
			t.emitted = time.Now()
			e.event("task", t.snap())
		}
	})
}

func (e *Engine) with(t *task, f func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f()
	e.dirty()
	e.emitTask(t)
}

func (e *Engine) setState(t *task, state, stage string) {
	e.with(t, func() {
		t.State, t.Stage = state, stage
		if t.prog != nil {
			t.prog.setStage(stage)
		}
	})
}

func (e *Engine) dirty() {
	if e.saveTimer == nil && !e.closing {
		e.saveTimer = time.AfterFunc(saveDelay, e.flush)
	}
}

func (e *Engine) flush() {
	e.saveMu.Lock()
	defer e.saveMu.Unlock()
	e.mu.Lock()
	e.saveTimer = nil
	sf := stateFile{Version: stateFormat, Tasks: make([]record, 0, len(e.order))}
	for _, id := range e.order {
		t := e.tasks[id]
		r := t.record
		r.Task = t.snap()
		sf.Tasks = append(sf.Tasks, r)
	}
	e.mu.Unlock()
	b, err := json.Marshal(sf)
	if err == nil {
		err = writeAtomic(e.opts.StateFile, b)
	}
	if err != nil {
		e.logf("save tasks: %v", err)
	}
}

func limitFor(kib int64) rate.Limit {
	if kib <= 0 {
		return rate.Inf
	}
	return rate.Limit(kib * 1024)
}

func (e *Engine) applyLimits() {
	e.down.SetLimit(limitFor(e.settings.DownLimit))
	e.up.SetLimit(limitFor(e.settings.UpLimit))
}

func running(state string) bool {
	return state == proto.StateResolving || state == proto.StateActive || state == proto.StateMerging
}

// Busy reports whether anything is queued, running or seeding.
func (e *Engine) Busy() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, t := range e.tasks {
		if t.run != nil || t.State == proto.StateQueued || running(t.State) || t.State == proto.StateSeeding {
			return true
		}
	}
	return false
}

func (e *Engine) Settings() proto.Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.settings
}

func (e *Engine) SetSettings(patch json.RawMessage) (proto.Settings, error) {
	e.mu.Lock()
	next, err := ApplyPatch(e.settings, patch)
	if err != nil {
		e.mu.Unlock()
		return e.Settings(), err
	}
	if err := SaveSettings(e.opts.ConfigFile, next); err != nil {
		e.mu.Unlock()
		return e.Settings(), perr(proto.CodeInternal, err.Error())
	}
	prev := e.settings
	e.settings = next
	e.applyLimits()
	if e.torrents != nil && (prev.Seed != next.Seed || prev.SeedRatio != next.SeedRatio || prev.SeedMinutes != next.SeedMinutes) {
		e.torrents.SetSeed(next.Seed, next.SeedRatio, next.SeedMinutes)
	}
	// a new port or DHT choice reaches the client when it is next built
	stale := e.releaseTorrents()
	e.pruneLocked(time.Now())
	e.schedule()
	e.event("settings", next)
	e.mu.Unlock()
	if stale != nil {
		_ = stale.Close()
	}
	return next, nil
}

// releaseTorrents gives up the torrent client once no torrent task runs or
// seeds and no paste card is resolving one, so DHT and listeners never stay up
// on a box with nothing to share. The caller closes it outside the lock;
// paused torrents are added again when they resume.
func (e *Engine) releaseTorrents() Torrents {
	if e.torrents == nil || e.torResolving > 0 {
		return nil
	}
	for _, t := range e.tasks {
		if t.run != nil && t.Kind == "torrent" {
			return nil
		}
	}
	for _, t := range e.tasks {
		t.tor = nil
	}
	tc := e.torrents
	e.torrents = nil
	return tc
}

// Snapshot hands f the current state under the same lock events are sent
// under, so a subscriber added inside f misses nothing and sees nothing twice.
func (e *Engine) Snapshot(f func(proto.Settings, []proto.Task, proto.Stats)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pruneLocked(time.Now())
	tasks := make([]proto.Task, 0, len(e.order))
	for _, id := range e.order {
		tasks = append(tasks, e.tasks[id].snap())
	}
	f(e.settings, tasks, e.stats())
}

func (e *Engine) torrentClient() (Torrents, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.torrents != nil {
		return e.torrents, nil
	}
	if e.parts.Torrents == nil {
		return nil, perr(proto.CodeUnsupported, i18n.T("Torrents are not available."))
	}
	tc, err := e.parts.Torrents(e.settings, e.down, e.up)
	if err != nil {
		return nil, err
	}
	tc.SetSeed(e.settings.Seed, e.settings.SeedRatio, e.settings.SeedMinutes)
	e.torrents = tc
	return tc, nil
}

func (e *Engine) Tasks() []proto.Task {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]proto.Task, 0, len(e.order))
	for _, id := range e.order {
		out = append(out, e.tasks[id].snap())
	}
	return out
}

func (e *Engine) Task(id string) (proto.Task, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	t := e.tasks[id]
	if t == nil {
		return proto.Task{}, notFound()
	}
	return t.snap(), nil
}

// History is the task's speed over its last 60 seconds while it runs or seeds;
// empty for any other task.
func (e *Engine) History(id string) (down, up []int64, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	t := e.tasks[id]
	if t == nil {
		return nil, nil, notFound()
	}
	return append([]int64{}, t.histDown...), append([]int64{}, t.histUp...), nil
}

func pushSample(s []int64, v int64) []int64 {
	if len(s) >= historyLen {
		copy(s, s[1:])
		s[len(s)-1] = v
		return s
	}
	return append(s, v)
}

func (e *Engine) Stats() proto.Stats {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stats()
}

func (e *Engine) stats() proto.Stats {
	st := proto.Stats{History: append([]int64(nil), e.history...)}
	for _, t := range e.tasks {
		switch {
		case t.State == proto.StateQueued:
			st.Queued++
		case t.State == proto.StateSeeding:
			st.Seeding++
		case running(t.State):
			st.Active++
		}
		if t.run != nil {
			st.Down += t.Speed
			st.Up += t.UpSpeed
		}
	}
	return st
}

// classify returns "torrent" for magnets and local .torrent files, "" for web links.
func classify(raw string) (kind, normalized string, err error) {
	if strings.HasPrefix(strings.ToLower(raw), "magnet:") {
		return "torrent", raw, nil
	}
	if p, ok := strings.CutPrefix(raw, "file://"); ok {
		raw = p
	}
	if filepath.IsAbs(raw) {
		if strings.HasSuffix(strings.ToLower(raw), ".torrent") {
			if fi, err := os.Stat(raw); err == nil && fi.Mode().IsRegular() {
				return "torrent", raw, nil
			}
		}
		return "", "", perr(proto.CodeInvalid, i18n.T("Only links and .torrent files can be downloaded."))
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "", perr(proto.CodeInvalid, i18n.T("That is not a link ryoget can download."))
	}
	return "", u.String(), nil
}

func provisionalTitle(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Scheme == "magnet" {
		if dn := u.Query().Get("dn"); dn != "" {
			return dn
		}
		return raw
	}
	if u.Host == "" {
		return filepath.Base(raw)
	}
	if b := httpdl.Sanitize(filepath.Base(u.Path)); b != "" && b != "." {
		return b
	}
	return u.Hostname()
}

func (e *Engine) nameTaken(dir, self string) func(string) bool {
	return func(n string) bool {
		for _, t := range e.tasks {
			if t.ID != self && t.Dir == dir && t.Name == n && t.State != proto.StateDone {
				return true
			}
		}
		return false
	}
}

func (e *Engine) Add(p proto.AddParams) (proto.Task, error) {
	kind, link, err := classify(strings.TrimSpace(p.URL))
	if err != nil {
		return proto.Task{}, err
	}
	mode := p.Mode
	switch mode {
	case "":
		mode = "auto"
	case "auto", "audio", "mute":
	default:
		return proto.Task{}, perr(proto.CodeInvalid, i18n.T("Unknown download mode."))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	dir := e.settings.Dir
	if p.Dir != "" {
		dir = filepath.Clean(paths.Expand(p.Dir))
		if !filepath.IsAbs(dir) {
			return proto.Task{}, perr(proto.CodeInvalid, i18n.T("The folder must be an absolute path."))
		}
	}
	host := ""
	if u, err := url.Parse(link); err == nil {
		host = u.Hostname()
	}
	t := &task{record: record{Task: proto.Task{
		ID:      newID(),
		URL:     link,
		Source:  proto.Source{Name: host},
		Kind:    "file",
		Title:   provisionalTitle(link),
		Dir:     dir,
		Size:    -1,
		ETA:     -1,
		State:   proto.StateQueued,
		Created: now(),
		Pieces:  emptyPieces,
		Files:   []proto.File{},
		Mode:    mode,
	}, Sel: p.Files}}
	if kind == "torrent" {
		t.Kind, t.Known = "torrent", true
		t.Source = proto.Source{ID: "torrent", Name: "BitTorrent"}
	}
	if p.Format != "" {
		t.Format = &proto.FormatRef{ID: p.Format}
	}
	if title := clientTitle(p.Title); title != "" {
		t.Title = title
	}
	if p.Name != "" {
		name := httpdl.Sanitize(p.Name)
		if name == "" {
			return proto.Task{}, perr(proto.CodeInvalid, i18n.T("That file name cannot be used."))
		}
		t.Name = httpdl.Unique(dir, name, e.nameTaken(dir, t.ID))
		t.Path = filepath.Join(dir, t.Name)
	}
	if p.Paused {
		t.State = proto.StatePaused
	}
	e.tasks[t.ID] = t
	e.order = append(e.order, t.ID)
	e.dirty()
	e.emitTask(t)
	e.schedule()
	return t.snap(), nil
}

// clientTitle tidies a title a client sends along: one line, at most 300 characters.
func clientTitle(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 300 {
		s = string(r[:300])
	}
	return s
}

// maxBatch bounds one addMany; a paste of thousands of links is a mistake, not a queue.
const maxBatch = 500

// AddMany adds each item on its own: a bad link becomes an entry in Errors
// (with its index) and never stops the others.
func (e *Engine) AddMany(p proto.AddManyParams) (proto.AddManyResult, error) {
	if len(p.Items) > maxBatch {
		return proto.AddManyResult{}, perr(proto.CodeInvalid, i18n.Tf("At most %d links can be added at once.", maxBatch))
	}
	res := proto.AddManyResult{Tasks: []proto.Task{}, Errors: []proto.ItemError{}}
	for i, it := range p.Items {
		t, err := e.Add(proto.AddParams{URL: it.URL, Mode: it.Mode, Format: it.Format, Files: it.Files, Title: it.Title, Dir: p.Dir, Paused: p.Paused})
		if err != nil {
			pe := mapErr(err)
			res.Errors = append(res.Errors, proto.ItemError{Index: i, Code: pe.Code, Message: pe.Message})
			continue
		}
		res.Tasks = append(res.Tasks, t)
	}
	return res, nil
}

func (e *Engine) each(id string, all bool, f func(t *task) error) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if all {
		for _, oid := range slices.Clone(e.order) {
			if t := e.tasks[oid]; t != nil {
				_ = f(t)
			}
		}
		e.schedule()
		return nil
	}
	t := e.tasks[id]
	if t == nil {
		return notFound()
	}
	err := f(t)
	e.schedule()
	return err
}

func (e *Engine) Pause(id string, all bool) error {
	return e.each(id, all, func(t *task) error {
		switch {
		case t.run != nil:
			t.want = wantPaused
			t.run.cancel()
		case t.State == proto.StateQueued:
			t.State, t.Seen = proto.StatePaused, ""
			e.dirty()
			e.emitTask(t)
		}
		return nil
	})
}

func (e *Engine) Resume(id string, all bool) error {
	return e.each(id, all, func(t *task) error {
		e.requeue(t)
		return nil
	})
}

func (e *Engine) requeue(t *task) bool {
	if t.run != nil {
		if t.want == wantPaused || t.want == wantStopped {
			t.want = wantQueued
		}
		return true
	}
	switch t.State {
	case proto.StatePaused, proto.StateStopped, proto.StateError:
		t.State, t.Stage, t.Error, t.Finished, t.Seen = proto.StateQueued, "", nil, "", ""
		e.dirty()
		e.emitTask(t)
		return true
	}
	return false
}

func (e *Engine) Retry(id string) (proto.Task, error) {
	var out proto.Task
	err := e.each(id, false, func(t *task) error {
		if t.State == proto.StateDone {
			return perr(proto.CodeInvalid, i18n.T("This download already finished."))
		}
		e.requeue(t)
		out = t.snap()
		return nil
	})
	if err != nil {
		return proto.Task{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if t := e.tasks[id]; t != nil {
		out = t.snap()
	}
	return out, nil
}

func (e *Engine) Cancel(id string) error {
	return e.each(id, false, func(t *task) error {
		if t.State == proto.StateDone {
			return perr(proto.CodeInvalid, i18n.T("This download already finished."))
		}
		if t.run != nil {
			t.want = wantStopped
			t.run.cancel()
			return nil
		}
		e.dropPartial(t)
		t.State, t.Stage, t.Error, t.Finished, t.Seen = proto.StateStopped, "", nil, now(), ""
		e.dirty()
		e.emitTask(t)
		return nil
	})
}

func (e *Engine) Remove(id string, deleteFile bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	t := e.tasks[id]
	if t == nil {
		return notFound()
	}
	e.forget(t)
	if t.run != nil {
		t.want, t.delFile = wantRemoved, deleteFile
		t.run.cancel()
	} else {
		if t.State == proto.StateDone {
			if deleteFile {
				removeFinal(t)
			}
			if t.tor != nil {
				tor := t.tor
				go func() { _ = tor.Drop(false) }()
			}
			e.forgetTorrent(t)
		} else {
			e.dropPartial(t)
		}
	}
	e.schedule()
	return nil
}

func (e *Engine) forget(t *task) {
	delete(e.tasks, t.ID)
	e.order = slices.DeleteFunc(e.order, func(id string) bool { return id == t.ID })
	t.pending = false
	e.dirty()
	e.event("removed", map[string]string{"id": t.ID})
}

// drop forgets a task that is not running: a finished file stays, partial data goes.
func (e *Engine) drop(t *task) {
	switch t.State {
	case proto.StateDone, proto.StateStopped:
	default:
		e.dropPartial(t)
	}
	if t.tor != nil {
		tor := t.tor
		go func() { _ = tor.Drop(false) }()
	}
	if t.State == proto.StateDone {
		e.forgetTorrent(t)
	}
	e.forget(t)
}

func (e *Engine) Clear(states []string) error {
	if len(states) == 0 {
		states = []string{proto.StateDone, proto.StateError, proto.StateStopped}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, id := range slices.Clone(e.order) {
		t := e.tasks[id]
		if t.run != nil || !slices.Contains(states, t.State) {
			continue
		}
		e.drop(t)
	}
	return nil
}

func settled(state string) bool {
	return state == proto.StateDone || state == proto.StateError || state == proto.StateStopped
}

// tasks saved before failures were stamped fall back to when they started
func stoppedAt(t *task) time.Time {
	for _, s := range []string{t.Finished, t.Started, t.Created} {
		if at, err := time.Parse(time.RFC3339, s); err == nil {
			return at
		}
	}
	return time.Time{}
}

// pruneLocked runs at start, after a task settles and on hello, so retention never needs a timer
func (e *Engine) pruneLocked(now time.Time) {
	days := e.settings.KeepDays
	if days <= 0 {
		return
	}
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	for _, id := range slices.Clone(e.order) {
		t := e.tasks[id]
		if t.run != nil || !settled(t.State) {
			continue
		}
		if at := stoppedAt(t); !at.IsZero() && at.Before(cutoff) {
			e.drop(t)
		}
	}
}

// Seen stamps settled or paused tasks with when a home list first showed them. The earlier
// stamp wins, so a client may backdate one to take it off its list at once.
func (e *Engine) Seen(ids []string, at time.Time) error {
	stamp := at.UTC().Format(time.RFC3339)
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, id := range ids {
		t := e.tasks[id]
		if t == nil || t.run != nil || !(settled(t.State) || t.State == proto.StatePaused) {
			continue
		}
		if prev, err := time.Parse(time.RFC3339, t.Seen); err == nil && !at.Before(prev) {
			continue
		}
		t.Seen = stamp
		e.dirty()
		e.emitTask(t)
	}
	return nil
}

// Move places a queued task at index among the queued tasks.
func (e *Engine) Move(id string, index int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	t := e.tasks[id]
	if t == nil {
		return notFound()
	}
	if t.State != proto.StateQueued || t.run != nil {
		return perr(proto.CodeInvalid, i18n.T("Only waiting downloads can be reordered."))
	}
	var slots []int
	var queued []string
	for i, oid := range e.order {
		if q := e.tasks[oid]; q.State == proto.StateQueued && q.run == nil {
			slots = append(slots, i)
			queued = append(queued, oid)
		}
	}
	queued = slices.DeleteFunc(queued, func(s string) bool { return s == id })
	index = max(0, min(index, len(queued)))
	queued = slices.Insert(queued, index, id)
	for i, slot := range slots {
		e.order[slot] = queued[i]
	}
	e.dirty()
	return nil
}

// Files changes a torrent's file selection.
func (e *Engine) Files(id string, selected []int) (proto.Task, error) {
	e.mu.Lock()
	t := e.tasks[id]
	if t == nil {
		e.mu.Unlock()
		return proto.Task{}, notFound()
	}
	if t.Kind != "torrent" {
		e.mu.Unlock()
		return proto.Task{}, perr(proto.CodeInvalid, i18n.T("Only torrents have files to choose."))
	}
	if len(selected) == 0 {
		e.mu.Unlock()
		return proto.Task{}, perr(proto.CodeInvalid, i18n.T("Choose at least one file."))
	}
	for _, i := range selected {
		if i < 0 || (len(t.Files) > 0 && i >= len(t.Files)) {
			e.mu.Unlock()
			return proto.Task{}, perr(proto.CodeInvalid, i18n.T("No such file in this torrent."))
		}
	}
	tor := t.tor
	e.mu.Unlock()
	if tor != nil {
		if err := tor.Select(selected); err != nil {
			return proto.Task{}, err
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	t.Sel = slices.Clone(selected)
	for i := range t.Files {
		t.Files[i].Selected = slices.Contains(selected, i)
	}
	if t.State == proto.StateDone && tor == nil {
		// a finished torrent that now wants more files runs again
		t.State = proto.StateQueued
	}
	e.dirty()
	e.emitTask(t)
	e.schedule()
	return t.snap(), nil
}

// Locate returns what open and reveal act on.
func (e *Engine) Locate(id string) (path, dir, state string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	t := e.tasks[id]
	if t == nil {
		return "", "", "", notFound()
	}
	return t.Path, t.Dir, t.State, nil
}

func safeName(n string) bool {
	return n != "" && n != "." && n != ".." && !strings.ContainsRune(n, '/')
}

// forgetTorrent clears a dropped torrent's kept state: through the open client
// when there is one, since it holds that state open, otherwise directly. Under
// the lock, so no client opens meanwhile.
func (e *Engine) forgetTorrent(t *task) {
	if t.Kind != "torrent" {
		return
	}
	src := t.FetchURL
	if src == "" {
		src = t.URL
	}
	var err error
	switch {
	case e.torrents != nil:
		err = e.torrents.Forget(src)
	case e.parts.Forget != nil:
		err = e.parts.Forget(src)
	}
	if err != nil {
		e.logf("forget torrent %s: %v", t.ID, err)
	}
}

// dropPartial deletes what an unfinished run left behind; finished files stay.
func (e *Engine) dropPartial(t *task) {
	switch {
	case t.Kind == "torrent":
		if t.Size > 0 && t.Done >= t.Size {
			// a complete torrent paused while seeding: its files are finished files
			if t.tor != nil {
				tor := t.tor
				t.tor = nil
				go func() { _ = tor.Drop(false) }()
			}
			e.forgetTorrent(t)
			return
		}
		if t.tor != nil {
			tor := t.tor
			t.tor = nil
			go func() { _ = tor.Drop(true) }()
		} else if safeName(t.Name) && t.Known {
			// the client is closed while nothing runs; unfinished single files carry .part
			base := filepath.Join(t.Dir, t.Name)
			_ = os.RemoveAll(base)
			_ = os.Remove(base + ".part")
			e.forgetTorrent(t)
		}
	case t.Kind == "media":
		_ = os.RemoveAll(filepath.Join(t.Dir, tempPrefix+t.ID))
	default:
		if safeName(t.Name) {
			part := filepath.Join(t.Dir, t.Name) + ".part"
			_ = os.Remove(part)
			_ = os.Remove(part + httpdl.SidecarExt)
		}
		_ = os.RemoveAll(filepath.Join(t.Dir, tempPrefix+t.ID))
	}
	t.Done = 0
	t.Pieces = emptyPieces
}

func removeFinal(t *task) {
	if t.Path == "" {
		return
	}
	rel, err := filepath.Rel(t.Dir, t.Path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return
	}
	_ = os.RemoveAll(t.Path)
}

// schedule starts queued tasks while fewer than maxActive run; seeding does not count.
func (e *Engine) schedule() {
	if e.closing {
		return
	}
	active := 0
	for _, t := range e.tasks {
		if t.run != nil && t.State != proto.StateSeeding {
			active++
		}
	}
	for _, id := range e.order {
		if active >= e.settings.MaxActive {
			return
		}
		t := e.tasks[id]
		if t.State == proto.StateQueued && t.run == nil {
			e.start(t)
			active++
		}
	}
}

func (e *Engine) start(t *task) {
	ctx, cancel := context.WithCancel(e.ctx)
	rs := &runState{cancel: cancel, done: make(chan struct{})}
	t.run, t.want, t.delFile = rs, "", false
	t.State, t.Stage, t.Error = proto.StateResolving, "", nil
	if t.Started == "" {
		t.Started = now()
	}
	t.prog = newProgress()
	t.lastMoved, t.speed = 0, 0
	e.dirty()
	e.emitTask(t)
	e.kick()
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		err := e.work(ctx, t)
		cancel()
		e.mu.Lock()
		e.finish(t, err)
		stale := e.releaseTorrents()
		e.mu.Unlock()
		close(rs.done)
		if stale != nil {
			_ = stale.Close()
		}
	}()
}

func (e *Engine) finish(t *task, err error) {
	e.sampleTask(t, tickEvery.Seconds())
	t.run = nil
	t.Speed, t.UpSpeed, t.Conns, t.ETA, t.speed = 0, 0, 0, -1, 0
	t.histDown, t.histUp = nil, nil
	want := t.want
	t.want = ""
	if want == wantRemoved {
		switch {
		case err == nil && t.delFile:
			removeFinal(t)
		case err != nil && t.Kind != "torrent":
			e.dropPartial(t)
		}
		return
	}
	switch {
	case want == wantPaused:
		t.State, t.Stage, t.Seen = proto.StatePaused, "", ""
	case want == wantQueued:
		t.State, t.Stage = proto.StateQueued, ""
	case want == wantStopped:
		e.dropPartial(t)
		t.State, t.Stage, t.Finished, t.Seen = proto.StateStopped, "", now(), ""
	case e.closing && (err == nil && t.State == proto.StateSeeding || errors.Is(err, context.Canceled)):
		// left as it was so the next start picks it up
	case err == nil:
		t.State, t.Stage, t.Error, t.Seen = proto.StateDone, "", nil, ""
		t.Finished = now()
		if t.Size <= 0 && t.Path != "" {
			if fi, serr := os.Stat(t.Path); serr == nil && fi.Mode().IsRegular() {
				t.Size = fi.Size()
			}
		}
		if t.Size > 0 {
			t.Done = t.Size
		}
		t.Pieces = fullPieces
		if e.opts.OnFinish != nil {
			e.opts.OnFinish(t.snap())
		}
	case errors.Is(err, context.Canceled):
		t.State, t.Stage, t.Seen = proto.StatePaused, "", ""
	default:
		t.State, t.Stage, t.Error, t.Finished, t.Seen = proto.StateError, "", mapErr(err), now(), ""
		e.logf("task %s: %v", t.ID, err)
		if e.opts.OnFail != nil {
			e.opts.OnFail(t.snap())
		}
	}
	e.dirty()
	t.emitted = time.Time{}
	e.emitTask(t)
	e.pruneLocked(time.Now())
	e.schedule()
}

func (e *Engine) kick() {
	if !e.ticking {
		e.ticking = true
		go e.loop()
	}
}

// loop samples running tasks at 5 Hz and sends stats at 1 Hz; it ends, with
// one last stats event, as soon as nothing runs.
func (e *Engine) loop() {
	tk := time.NewTicker(tickEvery)
	defer tk.Stop()
	last := time.Now()
	for range tk.C {
		e.mu.Lock()
		dt := time.Since(last).Seconds()
		last = time.Now()
		moving := false
		for _, id := range e.order {
			if t := e.tasks[id]; t.run != nil {
				moving = true
				e.sampleTask(t, dt)
			}
		}
		e.ticks++
		if !moving {
			e.event("stats", e.stats())
			e.ticking = false
			e.mu.Unlock()
			return
		}
		// keeps the listed progress close to the sidecars if the daemon is killed
		if e.ticks%25 == 0 {
			e.dirty()
		}
		if e.ticks%5 == 0 {
			for _, t := range e.tasks {
				if t.run != nil && (t.State == proto.StateActive || t.State == proto.StateMerging || t.State == proto.StateSeeding) {
					t.histDown = pushSample(t.histDown, t.Speed)
					t.histUp = pushSample(t.histUp, t.UpSpeed)
				}
			}
			st := e.stats()
			copy(e.history, e.history[1:])
			e.history[historyLen-1] = st.Down
			st.History = append([]int64(nil), e.history...)
			e.event("stats", st)
		}
		e.mu.Unlock()
	}
}

func (e *Engine) sampleTask(t *task, dt float64) {
	if dt <= 0 {
		dt = tickEvery.Seconds()
	}
	switch {
	case t.tor != nil && t.Known && t.Kind == "torrent":
		st := t.tor.Stats()
		if st.Size >= 0 {
			t.Size = st.Size
		}
		t.Done, t.Speed, t.UpSpeed, t.Up = st.Done, st.Down, st.Up, st.Uploaded
		t.Peers, t.Seeds, t.Conns, t.Ratio = st.Peers, st.Seeds, st.Conns, st.Ratio
		t.Pieces = encodePieces(st.Pieces)
		if e.ticks%5 == 0 {
			files := t.tor.Files()
			if len(files) > 0 {
				t.Files = files
			}
		}
	case t.prog != nil:
		sm := t.prog.sample()
		if sm.total >= 0 {
			t.Size = sm.total
		}
		t.Done = sm.done
		inst := max(0, float64(sm.moved-t.lastMoved)/dt)
		t.lastMoved = sm.moved
		t.speed += (inst - t.speed) * (1 - math.Exp(-dt/speedTau))
		if t.speed < 1 {
			t.speed = 0
		}
		t.Speed = int64(t.speed)
		t.Conns = sm.conns
		t.Pieces = encodePieces(sm.pieces)
		if sm.stage != "" && t.run != nil {
			t.Stage = sm.stage
		}
	}
	t.ETA = -1
	if t.Speed > 0 && t.Size > 0 && t.Done < t.Size {
		t.ETA = (t.Size - t.Done) / t.Speed
	}
	sig := fmt.Sprint(t.Done, t.Speed, t.UpSpeed, t.Conns, t.Peers, t.Seeds, t.Pieces, t.Stage, t.Size, t.State)
	if sig != t.sig {
		t.sig = sig
		e.emitTask(t)
	}
}
