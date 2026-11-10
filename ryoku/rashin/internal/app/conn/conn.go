// Package conn keeps the Rashin app subscribed across daemon restarts.
package conn

import (
	"context"
	"errors"
	"sync"
	"time"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

const retryDelay = 2 * time.Second

// Client is a reconnecting wrapper over internal/client.
type Client struct {
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	store   *state.Store
	options client.Options

	mu      sync.RWMutex
	current *client.Conn
	wake    chan struct{}
	watchMu sync.Mutex
	watches map[string]int
}

// New starts reconnecting immediately.
func New(store *state.Store) *Client {
	return NewOptions(store, client.Options{Client: protocol.ClientApp, Start: true})
}

// NewOptions starts a client with caller-selected dial options for demos and tests.
func NewOptions(store *state.Store, options client.Options) *Client {
	if options.Client == "" {
		options.Client = protocol.ClientApp
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{ctx: ctx, cancel: cancel, done: make(chan struct{}), store: store, options: options, wake: make(chan struct{}, 1), watches: make(map[string]int)}
	go c.run()
	return c
}

func (c *Client) Close() {
	c.cancel()
	c.mu.RLock()
	current := c.current
	c.mu.RUnlock()
	if current != nil {
		_ = current.Close()
	}
	<-c.done
}

func (c *Client) Reconnect() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Client) Connected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current != nil
}

// Call runs an RPC on the current connection. UI surfaces call it from a
// worker goroutine so a daemon response can never block Gio's frame loop.
func (c *Client) Call(ctx context.Context, method string, params, result any) error {
	c.mu.RLock()
	current := c.current
	c.mu.RUnlock()
	if current == nil {
		return client.ErrUnreachable
	}
	return current.Call(ctx, method, params, result)
}

// GoCall starts a bounded RPC away from the UI goroutine.
func (c *Client) GoCall(method string, params, result any, done func(error)) {
	go func() {
		ctx, cancel := context.WithTimeout(c.ctx, 45*time.Second)
		defer cancel()
		err := c.Call(ctx, method, params, result)
		if done != nil {
			done(err)
		}
	}()
}

// Attach opens a transcript and starts its stream.
func (c *Client) Attach(ctx context.Context, id string) error {
	return c.attachPages(ctx, c.Call, id, c.store.Open(id))
}

type caller func(ctx context.Context, method string, params, result any) error

// attachPages folds a session's replay page by page; the daemon starts the
// live stream only after the last page.
func (c *Client) attachPages(ctx context.Context, call caller, id string, since int64) error {
	for {
		var result protocol.SessionAttachResult
		if err := call(ctx, protocol.MSessionAttach, protocol.SessionAttachParams{ID: id, Since: since}, &result); err != nil {
			return err
		}
		c.store.ApplyAttach(since, result)
		if !result.More || len(result.Events) == 0 {
			return nil
		}
		since = result.Events[len(result.Events)-1].Seq
	}
}

func (c *Client) Detach(ctx context.Context, id string) error {
	var result protocol.Empty
	err := c.Call(ctx, protocol.MSessionDetach, protocol.SessionRef{ID: id}, &result)
	if err == nil {
		c.store.Close(id)
	}
	return err
}

// WatchProject keeps one daemon project watch active while at least one page
// needs it. The first caller sends watch=true.
func (c *Client) WatchProject(project string) {
	if project == "" {
		return
	}
	c.watchMu.Lock()
	c.watches[project]++
	first := c.watches[project] == 1
	c.watchMu.Unlock()
	if first {
		c.sendWatch(project, true)
	}
}

// UnwatchProject releases one project watch. Extra releases are harmless.
func (c *Client) UnwatchProject(project string) {
	if project == "" {
		return
	}
	c.watchMu.Lock()
	count := c.watches[project]
	if count <= 1 {
		delete(c.watches, project)
	} else {
		c.watches[project] = count - 1
	}
	last := count == 1
	c.watchMu.Unlock()
	if last {
		c.sendWatch(project, false)
	}
}

func (c *Client) sendWatch(project string, watch bool) {
	c.GoCall(protocol.MProjectsWatch, protocol.WatchParams{Project: project, Watch: watch}, new(protocol.Empty), nil)
}

func (c *Client) watchedProjects() []string {
	c.watchMu.Lock()
	defer c.watchMu.Unlock()
	projects := make([]string, 0, len(c.watches))
	for project := range c.watches {
		projects = append(projects, project)
	}
	return projects
}

func (c *Client) run() {
	defer close(c.done)
	for c.ctx.Err() == nil {
		c.store.SetConnection(state.Connecting)
		dialCtx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
		connection, err := client.Dial(dialCtx, c.options)
		cancel()
		if err != nil {
			if errors.Is(err, client.ErrDisabled) {
				c.store.SetConnection(state.Off)
			} else {
				c.store.SetConnection(state.Unreachable)
			}
			if !c.waitRetry() {
				return
			}
			continue
		}
		c.install(connection)
		if err := c.subscribe(connection); err != nil {
			c.lose(connection)
			if !c.waitRetry() {
				return
			}
			continue
		}
		c.store.SetConnection(state.Ready)
		if !c.consume(connection) {
			return
		}
		c.store.SetConnection(state.Connecting)
		if !c.waitRetry() {
			return
		}
	}
}

func (c *Client) subscribe(connection *client.Conn) error {
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()
	var snapshot protocol.Snapshot
	if err := connection.Call(ctx, protocol.MSubscribe, protocol.Empty{}, &snapshot); err != nil {
		return err
	}
	c.store.ApplySnapshot(snapshot)
	for id, since := range c.store.OpenSessions() {
		if err := c.attachPages(ctx, connection.Call, id, since); err != nil {
			return err
		}
	}
	for _, project := range c.watchedProjects() {
		var result protocol.Empty
		if err := connection.Call(ctx, protocol.MProjectsWatch, protocol.WatchParams{Project: project, Watch: true}, &result); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) consume(connection *client.Conn) bool {
	defer c.lose(connection)
	for {
		select {
		case <-c.ctx.Done():
			return false
		case event, ok := <-connection.Events():
			if !ok {
				return true
			}
			_ = c.store.Apply(event)
		case <-connection.Done():
			return true
		case <-c.wake:
			_ = connection.Close()
			return true
		}
	}
}

func (c *Client) install(connection *client.Conn) {
	c.mu.Lock()
	c.current = connection
	c.mu.Unlock()
}

func (c *Client) lose(connection *client.Conn) {
	c.mu.Lock()
	if c.current == connection {
		c.current = nil
	}
	c.mu.Unlock()
	_ = connection.Close()
}

func (c *Client) waitRetry() bool {
	timer := time.NewTimer(retryDelay)
	defer timer.Stop()
	select {
	case <-c.ctx.Done():
		return false
	case <-timer.C:
		return true
	case <-c.wake:
		return true
	}
}
