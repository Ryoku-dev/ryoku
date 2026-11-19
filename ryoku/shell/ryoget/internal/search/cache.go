package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	resultTTL   = 30 * time.Minute
	suggestTTL  = 10 * time.Minute
	negativeTTL = 5 * time.Minute
	detailTTL   = 30 * time.Minute
)

type cached struct {
	items []Item
	more  bool
}

// diskItem keeps the fields Item hides from the wire.
type diskItem struct {
	Item
	ThumbURL  string  `json:"thumbURL,omitempty"`
	ThumbPage bool    `json:"thumbPage,omitempty"`
	Pop       float64 `json:"pop,omitempty"`
}

type diskEntry struct {
	Exp   time.Time  `json:"exp"`
	More  bool       `json:"more"`
	Items []diskItem `json:"items"`
}

type memEntry struct {
	val any
	exp time.Time
}

// store is the result cache: memory for everything, plus one file per
// full-search result so a restarted daemon does not ask the engines again.
type store struct {
	mu  sync.Mutex
	mem map[string]memEntry
	dir string
	now func() time.Time
}

func newStore(dir string, now func() time.Time) *store {
	s := &store{mem: map[string]memEntry{}, dir: dir, now: now}
	s.prune()
	return s
}

func (s *store) path(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}

func (s *store) get(key string, disk bool) (any, bool) {
	now := s.now()
	s.mu.Lock()
	e, ok := s.mem[key]
	s.mu.Unlock()
	if ok && now.Before(e.exp) {
		return e.val, true
	}
	if !disk || s.dir == "" {
		return nil, false
	}
	b, err := os.ReadFile(s.path(key))
	if err != nil {
		return nil, false
	}
	var d diskEntry
	if json.Unmarshal(b, &d) != nil || !now.Before(d.Exp) {
		return nil, false
	}
	items := make([]Item, len(d.Items))
	for i, di := range d.Items {
		it := di.Item
		it.thumbURL, it.thumbPage, it.pop = di.ThumbURL, di.ThumbPage, di.Pop
		items[i] = it
	}
	val := cached{items: items, more: d.More}
	s.mu.Lock()
	s.mem[key] = memEntry{val: val, exp: d.Exp}
	s.mu.Unlock()
	return val, true
}

func (s *store) put(key string, val any, ttl time.Duration, disk bool) {
	exp := s.now().Add(ttl)
	s.mu.Lock()
	if len(s.mem) >= 1024 {
		now := s.now()
		for k, e := range s.mem {
			if !now.Before(e.exp) {
				delete(s.mem, k)
			}
		}
	}
	s.mem[key] = memEntry{val: val, exp: exp}
	s.mu.Unlock()
	c, ok := val.(cached)
	if !disk || !ok || s.dir == "" {
		return
	}
	d := diskEntry{Exp: exp, More: c.more, Items: make([]diskItem, len(c.items))}
	for i, it := range c.items {
		it.Thumb = ""
		d.Items[i] = diskItem{Item: it, ThumbURL: it.thumbURL, ThumbPage: it.thumbPage, Pop: it.pop}
	}
	b, err := json.Marshal(d)
	if err != nil || os.MkdirAll(s.dir, 0o700) != nil {
		return
	}
	tmp, err := os.CreateTemp(s.dir, ".entry-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(b)
	if cerr := tmp.Close(); werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return
	}
	if os.Rename(tmp.Name(), s.path(key)) != nil {
		os.Remove(tmp.Name())
	}
}

// prune drops expired result files; it runs once per daemon start.
func (s *store) prune() {
	if s.dir == "" {
		return
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	now := s.now()
	for _, e := range entries {
		p := filepath.Join(s.dir, e.Name())
		info, err := e.Info()
		if err != nil || e.IsDir() {
			continue
		}
		if now.Sub(info.ModTime()) > resultTTL+time.Hour {
			os.Remove(p)
		}
	}
}

// flight is one shared fetch. Every client asking the same thing waits on it;
// when the last one leaves before a request went out, it is abandoned. Once a
// request is on the wire it always finishes into the cache.
type flight struct {
	done chan struct{}
	val  any
	err  error

	mu      sync.Mutex
	waiters int
	onWire  bool
	waiting bool
	subs    map[int]func()
	next    int
	cancel  context.CancelFunc
}

type flightKey struct{}

func flightOf(ctx context.Context) *flight {
	f, _ := ctx.Value(flightKey{}).(*flight)
	return f
}

func (f *flight) wire() {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.onWire = true
	f.mu.Unlock()
}

func (f *flight) notifyWaiting() {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.waiting = true
	subs := make([]func(), 0, len(f.subs))
	for _, fn := range f.subs {
		subs = append(subs, fn)
	}
	f.mu.Unlock()
	for _, fn := range subs {
		fn()
	}
}

type cachePolicy struct {
	ttl  time.Duration
	disk bool
}

// do returns the cached value for key, or joins (or starts) the one flight
// computing it. waiting is told when the flight is held by a rate budget.
func (e *Service) do(ctx context.Context, key string, pol cachePolicy, waiting func(), fn func(ctx context.Context) (any, error)) (any, error) {
	for {
		if v, ok := e.cache.get(key, pol.disk); ok {
			return v, nil
		}
		f := e.join(key, pol, fn)
		f.mu.Lock()
		f.waiters++
		id := f.next
		f.next++
		if waiting != nil {
			f.subs[id] = waiting
		}
		already := f.waiting
		f.mu.Unlock()
		if already && waiting != nil {
			waiting()
		}
		select {
		case <-f.done:
			// joined a flight its last caller had just abandoned
			if errors.Is(f.err, context.Canceled) && ctx.Err() == nil {
				continue
			}
			return f.val, f.err
		case <-ctx.Done():
			f.mu.Lock()
			f.waiters--
			delete(f.subs, id)
			if f.waiters == 0 && !f.onWire {
				f.cancel()
			}
			f.mu.Unlock()
			return nil, ctx.Err()
		}
	}
}

func (e *Service) join(key string, pol cachePolicy, fn func(ctx context.Context) (any, error)) *flight {
	e.fmu.Lock()
	defer e.fmu.Unlock()
	if f, ok := e.flights[key]; ok {
		return f
	}
	fctx, cancel := context.WithCancel(e.base)
	f := &flight{done: make(chan struct{}), subs: map[int]func(){}, cancel: cancel}
	fctx = context.WithValue(fctx, flightKey{}, f)
	e.flights[key] = f
	e.running.Add(1)
	go func() {
		defer e.running.Done()
		f.val, f.err = fn(fctx)
		if f.err == nil {
			ttl := pol.ttl
			if c, ok := f.val.(cached); ok && len(c.items) == 0 {
				ttl = negativeTTL
			}
			e.cache.put(key, f.val, ttl, pol.disk && ttl == pol.ttl)
		}
		e.fmu.Lock()
		delete(e.flights, key)
		e.fmu.Unlock()
		cancel()
		close(f.done)
	}()
	return f
}
