// Package daemon serves the ryoget socket: one JSON object per line, requests
// answered by id, events pushed to connections that said hello. It runs while
// something needs it and exits once nobody is connected and nothing is left to do.
package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	i18n "ryoku-i18n"

	"ryoget/internal/engine"
	"ryoget/internal/links"
	"ryoget/internal/notify"
	"ryoget/internal/paths"
	"ryoget/internal/proto"
)

var ErrRunning = errors.New("another ryoget daemon is running")

const (
	defaultIdle  = 60 * time.Second
	outQueue     = 512
	writeTimeout = 10 * time.Second
	resolveLimit = 90 * time.Second
)

type Options struct {
	Socket string
	Lock   string
	// 0 reads RYOGET_IDLE_SECONDS, else 60 s
	IdleAfter time.Duration
	Engine    engine.Options
	// builds the concrete parts once the engine exists (its Env and limiters)
	Parts func(e *engine.Engine) engine.Parts
	Log   *log.Logger
}

type Server struct {
	opts   Options
	eng    *engine.Engine
	notes  *notify.Notifier
	log    *log.Logger
	notify atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.Mutex
	conns map[*conn]struct{}

	poke    chan struct{}
	idleMu  sync.Mutex
	idle    *time.Timer
	idleFor time.Duration
}

func idleFromEnv() time.Duration {
	if v := os.Getenv("RYOGET_IDLE_SECONDS"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			return time.Duration(n * float64(time.Second))
		}
	}
	return defaultIdle
}

// Run serves until ctx ends or the daemon has been idle long enough.
func Run(ctx context.Context, opts Options) error {
	if opts.Socket == "" {
		opts.Socket = paths.Socket()
	}
	if opts.Lock == "" {
		opts.Lock = paths.Lock()
	}
	if opts.IdleAfter <= 0 {
		opts.IdleAfter = idleFromEnv()
	}
	if opts.Log == nil {
		opts.Log = log.New(io.Discard, "", 0)
	}
	if err := os.MkdirAll(filepath.Dir(opts.Socket), 0o700); err != nil {
		return err
	}
	lock, err := takeLock(opts.Lock)
	if err != nil {
		return err
	}
	defer lock.Close()

	s := &Server{opts: opts, log: opts.Log, conns: map[*conn]struct{}{}, poke: make(chan struct{}, 1), idleFor: opts.IdleAfter}
	s.ctx, s.cancel = context.WithCancel(ctx)
	defer s.cancel()
	s.notes = notify.New(s.onAction, s.kick)

	eo := opts.Engine
	eo.OnEvent = s.broadcast
	eo.OnFinish = s.onFinish
	eo.OnFail = s.onFail
	if eo.Log == nil {
		eo.Log = opts.Log
	}
	s.eng = engine.New(eo)
	if opts.Parts != nil {
		s.eng.Use(opts.Parts(s.eng))
	}
	if err := s.eng.Start(); err != nil {
		return err
	}
	s.notify.Store(s.eng.Settings().Notify)

	// the lock is ours, so a socket file left here belongs to a dead daemon
	_ = os.Remove(opts.Socket)
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: opts.Socket, Net: "unix"})
	if err != nil {
		s.eng.Close()
		return err
	}
	ln.SetUnlinkOnClose(true)
	if err := os.Chmod(opts.Socket, 0o600); err != nil {
		ln.Close()
		s.eng.Close()
		return err
	}
	s.log.Printf("ryoget listening on %s", opts.Socket)

	go s.accept(ln)
	go s.idleLoop()
	s.kick()
	<-s.ctx.Done()

	ln.Close()
	s.mu.Lock()
	for c := range s.conns {
		c.close()
	}
	s.mu.Unlock()
	s.eng.Close()
	s.notes.Close()
	s.log.Printf("ryoget stopped")
	return nil
}

func takeLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrRunning
		}
		return nil, err
	}
	return f, nil
}

func (s *Server) accept(ln *net.UnixListener) {
	for {
		nc, err := ln.AcceptUnix()
		if err != nil {
			if s.ctx.Err() == nil && !errors.Is(err, net.ErrClosed) {
				s.log.Printf("accept: %v", err)
			}
			return
		}
		if !samePeer(nc) {
			nc.Close()
			continue
		}
		c := &conn{s: s, nc: nc, out: make(chan []byte, outQueue), closed: make(chan struct{})}
		s.mu.Lock()
		s.conns[c] = struct{}{}
		s.mu.Unlock()
		s.kick()
		go c.writer()
		go c.reader()
	}
}

// samePeer admits only processes of the user running the daemon.
func samePeer(nc *net.UnixConn) bool {
	raw, err := nc.SyscallConn()
	if err != nil {
		return false
	}
	var cred *syscall.Ucred
	var cerr error
	if err := raw.Control(func(fd uintptr) {
		cred, cerr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || cerr != nil {
		return false
	}
	return int(cred.Uid) == os.Getuid()
}

// broadcast runs under the engine lock: marshal once, queue without blocking.
func (s *Server) broadcast(name string, data any) {
	if name == "settings" {
		if st, ok := data.(proto.Settings); ok {
			s.notify.Store(st.Notify)
		}
	}
	b, err := json.Marshal(proto.Event{Event: name, Data: data})
	if err != nil {
		return
	}
	b = append(b, '\n')
	s.mu.Lock()
	for c := range s.conns {
		if c.sub.Load() {
			c.send(b)
		}
	}
	s.mu.Unlock()
	if name == "task" || name == "removed" {
		s.kick()
	}
}

func (s *Server) dropConn(c *conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
	s.kick()
}

// kick asks the idle loop to look again; it never blocks.
func (s *Server) kick() {
	select {
	case s.poke <- struct{}{}:
	default:
	}
}

func (s *Server) busy() bool {
	s.mu.Lock()
	n := len(s.conns)
	s.mu.Unlock()
	return n > 0 || s.eng.Busy() || s.notes.Pending()
}

func (s *Server) idleLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.poke:
		}
		s.idleMu.Lock()
		if s.busy() {
			if s.idle != nil {
				s.idle.Stop()
				s.idle = nil
			}
		} else if s.idle == nil {
			s.idle = time.AfterFunc(s.idleFor, func() {
				if !s.busy() {
					s.log.Printf("idle for %s, exiting", s.idleFor)
					s.cancel()
					return
				}
				s.idleMu.Lock()
				s.idle = nil
				s.idleMu.Unlock()
			})
		}
		s.idleMu.Unlock()
	}
}

func (s *Server) onFinish(t proto.Task) {
	if !s.notify.Load() {
		return
	}
	title := t.Title
	if title == "" {
		title = t.Name
	}
	n := notify.Notice{Title: i18n.T("Download finished"), Body: title, Path: t.Path, Dir: t.Dir}
	go s.post(n)
}

func (s *Server) onFail(t proto.Task) {
	if !s.notify.Load() {
		return
	}
	body := t.Title
	if t.Error != nil && t.Error.Message != "" {
		body = fmt.Sprintf("%s\n%s", t.Title, t.Error.Message)
	}
	go s.post(notify.Notice{Title: i18n.T("Download failed"), Body: body})
}

func (s *Server) post(n notify.Notice) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	if err := s.notes.Post(ctx, n); err != nil {
		s.log.Printf("notify: %v", err)
	}
}

func (s *Server) onAction(n notify.Notice, action string) {
	switch action {
	case notify.ActOpen:
		_ = openPath(n.Path)
	case notify.ActFolder:
		reveal(n.Path, n.Dir)
	}
}

type conn struct {
	s      *Server
	nc     *net.UnixConn
	out    chan []byte
	sub    atomic.Bool
	closed chan struct{}
	once   sync.Once
}

func (c *conn) close() {
	c.once.Do(func() {
		close(c.closed)
		c.nc.Close()
	})
}

// send queues a frame; a client too slow to keep up is dropped rather than
// allowed to stall every other one.
func (c *conn) send(b []byte) {
	select {
	case <-c.closed:
	case c.out <- b:
	default:
		c.close()
	}
}

func (c *conn) writer() {
	for {
		select {
		case <-c.closed:
			return
		case b := <-c.out:
			_ = c.nc.SetWriteDeadline(time.Now().Add(writeTimeout))
			if _, err := c.nc.Write(b); err != nil {
				c.close()
				return
			}
		}
	}
}

func (c *conn) reader() {
	defer c.s.dropConn(c)
	defer c.close()
	sc := bufio.NewScanner(c.nc)
	sc.Buffer(make([]byte, 0, 64<<10), proto.MaxFrame)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req proto.Request
		if err := json.Unmarshal(line, &req); err != nil {
			c.reply(proto.Response{Error: &proto.Error{Code: proto.CodeInvalid, Message: "bad frame"}})
			continue
		}
		if req.Method == "hello" {
			c.hello(req)
			continue
		}
		go func() {
			result, err := c.s.dispatch(req)
			resp := proto.Response{ID: req.ID, Result: result}
			if err != nil {
				resp.Result = nil
				resp.Error = toError(err)
			}
			c.reply(resp)
			c.s.kick()
		}()
	}
}

func (c *conn) reply(r proto.Response) {
	if r.Error == nil && r.Result == nil {
		r.Result = struct{}{}
	}
	b, err := json.Marshal(r)
	if err != nil {
		b, _ = json.Marshal(proto.Response{ID: r.ID, Error: &proto.Error{Code: proto.CodeInternal, Message: err.Error()}})
	}
	c.send(append(b, '\n'))
}

func (c *conn) hello(req proto.Request) {
	var p struct {
		Client  string `json:"client"`
		Version int    `json:"version"`
	}
	_ = json.Unmarshal(req.Params, &p)
	c.s.eng.Snapshot(func(s proto.Settings, tasks []proto.Task, st proto.Stats) {
		c.sub.Store(true)
		c.reply(proto.Response{ID: req.ID, Result: proto.Hello{Version: proto.Version, Settings: s, Tasks: tasks, Stats: st}})
	})
}

func toError(err error) *proto.Error {
	var pe *proto.Error
	if errors.As(err, &pe) {
		return pe
	}
	return &proto.Error{Code: proto.CodeInternal, Message: err.Error()}
}

func decode(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return &proto.Error{Code: proto.CodeInvalid, Message: err.Error()}
	}
	return nil
}

func (s *Server) dispatch(req proto.Request) (any, error) {
	e := s.eng
	switch req.Method {
	case "resolve":
		var p struct {
			URL string `json:"url"`
		}
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(s.ctx, resolveLimit)
		defer cancel()
		return e.Resolve(ctx, p.URL)
	case "links":
		var p struct {
			Text string `json:"text"`
		}
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		return links.Parse(p.Text), nil
	case "addMany":
		var p proto.AddManyParams
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		return e.AddMany(p)
	case "add":
		var p proto.AddParams
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		return e.Add(p)
	case "pause", "resume":
		var p proto.IDParams
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		if req.Method == "pause" {
			return nil, e.Pause(p.ID, p.All)
		}
		return nil, e.Resume(p.ID, p.All)
	case "cancel":
		var p proto.IDParams
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		return nil, e.Cancel(p.ID)
	case "remove":
		var p struct {
			ID         string `json:"id"`
			DeleteFile bool   `json:"deleteFile"`
		}
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		return nil, e.Remove(p.ID, p.DeleteFile)
	case "retry":
		var p proto.IDParams
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		return e.Retry(p.ID)
	case "move":
		var p struct {
			ID    string `json:"id"`
			Index int    `json:"index"`
		}
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		return nil, e.Move(p.ID, p.Index)
	case "files":
		var p struct {
			ID       string `json:"id"`
			Selected []int  `json:"selected"`
		}
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		return e.Files(p.ID, p.Selected)
	case "open", "reveal":
		var p proto.IDParams
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		path, dir, state, err := e.Locate(p.ID)
		if err != nil {
			return nil, err
		}
		if req.Method == "reveal" {
			if state != proto.StateDone {
				path = ""
			}
			reveal(path, dir)
			return nil, nil
		}
		if state != proto.StateDone || path == "" {
			return nil, &proto.Error{Code: proto.CodeInvalid, Message: i18n.T("This download has not finished yet.")}
		}
		return nil, openPath(path)
	case "clear":
		var p struct {
			States []string `json:"states"`
		}
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		return nil, e.Clear(p.States)
	case "seen":
		var p struct {
			IDs []string `json:"ids"`
			At  string   `json:"at"`
		}
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		at := time.Now()
		if p.At != "" {
			t, err := time.Parse(time.RFC3339, p.At)
			if err != nil {
				return nil, &proto.Error{Code: proto.CodeInvalid, Message: "at is not an RFC 3339 time"}
			}
			at = t
		}
		return nil, e.Seen(p.IDs, at)
	case "history":
		var p proto.IDParams
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		down, up, err := e.History(p.ID)
		if err != nil {
			return nil, err
		}
		return map[string][]int64{"down": down, "up": up}, nil
	case "settings.get":
		return e.Settings(), nil
	case "settings.set":
		var p struct {
			Patch json.RawMessage `json:"patch"`
		}
		if err := decode(req.Params, &p); err != nil {
			return nil, err
		}
		if len(p.Patch) == 0 {
			return nil, &proto.Error{Code: proto.CodeInvalid, Message: "missing patch"}
		}
		return e.SetSettings(p.Patch)
	}
	return nil, &proto.Error{Code: proto.CodeInvalid, Message: "unknown method " + req.Method}
}
