package search

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
)

const (
	ProtocolVersion = 2
	maxLine         = 1 << 20
)

type readyMsg struct {
	Ready   bool         `json:"ready"`
	Version int          `json:"version"`
	Sources []SourceInfo `json:"sources"`
	Engines []EngineInfo `json:"engines"`
}

type enginesMsg struct {
	Engines []EngineInfo `json:"engines"`
}

type inMsg struct {
	Hello *struct {
		Client  string `json:"client"`
		Version int    `json:"version"`
		Lang    string `json:"lang"`
	} `json:"hello"`
	ID     *int64  `json:"id"`
	Q      *string `json:"q"`
	Scope  string  `json:"scope"`
	Mode   string  `json:"mode"`
	Detail *struct {
		Source string `json:"source"`
		Key    string `json:"key"`
	} `json:"detail"`
	Cancel *int64 `json:"cancel"`
}

type client struct {
	mu     sync.Mutex
	w      *bufio.Writer
	enc    *json.Encoder
	broken bool
	ready  bool
	// cancels every job of the client once its connection stops taking lines
	gone context.CancelFunc
}

// write drops lines for work that was cancelled meanwhile, so a superseded
// query never paints over a newer one.
func (c *client) write(ctx context.Context, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.broken || ctx.Err() != nil {
		return
	}
	if c.enc.Encode(v) != nil || c.w.Flush() != nil {
		c.broken = true
		c.gone()
	}
}

func (e *Service) broadcastEngines(infos []EngineInfo) {
	e.cmu.Lock()
	cs := make([]*client, 0, len(e.clients))
	for c := range e.clients {
		cs = append(cs, c)
	}
	e.cmu.Unlock()
	for _, c := range cs {
		c.mu.Lock()
		ready := c.ready
		c.mu.Unlock()
		if ready {
			c.write(context.Background(), enginesMsg{Engines: infos})
		}
	}
}

type job struct {
	cancel context.CancelFunc
	query  bool
}

// Serve speaks protocol v2 with one client until it closes its side or ctx
// ends. Many Serve calls share the governor, the caches and the requests on
// the wire. Serve never closes conn.
func (e *Service) Serve(ctx context.Context, conn io.ReadWriter) error {
	ctx, gone := context.WithCancel(ctx)
	defer gone()
	bw := bufio.NewWriter(conn)
	c := &client{w: bw, enc: json.NewEncoder(bw), gone: gone}
	c.enc.SetEscapeHTML(false)
	e.cmu.Lock()
	e.clients[c] = true
	e.cmu.Unlock()
	defer func() {
		e.cmu.Lock()
		delete(e.clients, c)
		e.cmu.Unlock()
	}()

	lines := make(chan []byte)
	scanErr := make(chan error, 1)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 0, 64<<10), maxLine)
		for sc.Scan() {
			b := append([]byte(nil), sc.Bytes()...)
			select {
			case lines <- b:
			case <-ctx.Done():
				return
			}
		}
		scanErr <- sc.Err()
	}()

	var (
		mu     sync.Mutex
		jobs   = map[int64]*job{}
		wg     sync.WaitGroup
		locale = "en-US"
	)
	cancelWhere := func(match func(id int64, j *job) bool) {
		mu.Lock()
		defer mu.Unlock()
		for id, j := range jobs {
			if match(id, j) {
				j.cancel()
				delete(jobs, id)
			}
		}
	}
	start := func(id int64, query bool, run func(ctx context.Context, emit func(any))) {
		jctx, cancel := context.WithCancel(ctx)
		j := &job{cancel: cancel, query: query}
		mu.Lock()
		if old, ok := jobs[id]; ok {
			old.cancel()
		}
		jobs[id] = j
		mu.Unlock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			run(jctx, func(v any) { c.write(jctx, v) })
			mu.Lock()
			if jobs[id] == j {
				delete(jobs, id)
			}
			mu.Unlock()
			cancel()
		}()
	}

	for {
		select {
		case <-ctx.Done():
			cancelWhere(func(int64, *job) bool { return true })
			wg.Wait()
			return nil
		case line, ok := <-lines:
			if !ok {
				// the client closed its side: let its running work answer
				wg.Wait()
				select {
				case err := <-scanErr:
					return err
				default:
					return nil
				}
			}
			var m inMsg
			if json.Unmarshal(line, &m) != nil {
				continue
			}
			switch {
			case m.Hello != nil:
				if m.Hello.Lang != "" {
					locale = normLocale(m.Hello.Lang)
				}
				c.mu.Lock()
				c.ready = true
				c.mu.Unlock()
				c.write(ctx, readyMsg{Ready: true, Version: ProtocolVersion, Sources: e.Sources(), Engines: e.gov.infos()})
			case m.Cancel != nil:
				n := *m.Cancel
				cancelWhere(func(id int64, _ *job) bool { return id == n })
			case m.ID != nil && m.Q != nil:
				n := *m.ID
				// a newer query supersedes everything older, details included
				cancelWhere(func(id int64, _ *job) bool { return id < n })
				text, scope, mode, loc := *m.Q, m.Scope, m.Mode, locale
				start(n, true, func(ctx context.Context, emit func(any)) {
					e.query(ctx, n, text, scope, mode, loc, emit)
				})
			case m.ID != nil && m.Detail != nil:
				n := *m.ID
				// moving the selection supersedes older details, not the running query
				cancelWhere(func(id int64, j *job) bool { return id < n && !j.query })
				src, key, loc := m.Detail.Source, m.Detail.Key, locale
				start(n, false, func(ctx context.Context, emit func(any)) {
					d, ok := e.detail(ctx, src, key, loc)
					if ok {
						emit(detailMsg{ID: n, Detail: d})
					}
				})
			}
		}
	}
}
