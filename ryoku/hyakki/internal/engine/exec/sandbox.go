package exec

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"sync"
	"syscall"
	"time"

	"ryoku-rashin/internal/rpcpipe"
)

// StartToolHost starts one sandboxed tool host, returning its process and a
// bidirectional pipe connected to its stdin and stdout. The executor owns
// waiting for and stopping the process and closing the pipe.
type StartToolHost func(context.Context) (*os.Process, io.ReadWriteCloser, error)

// Sandboxed starts a tool host on the first call and shares it across a
// session's file and process operations. After Close, a later call starts a
// fresh host. The starter must set the child's process group independently.
func Sandboxed(start StartToolHost) Executor {
	return &sandboxed{start: start, callbacks: make(map[string]OutputFunc), jobs: make(map[string]bool)}
}

type sandboxed struct {
	mu        sync.Mutex
	start     StartToolHost
	child     *os.Process
	pipe      io.ReadWriteCloser
	conn      *rpcpipe.Conn
	waited    chan struct{}
	epoch     uint64
	callbacks map[string]OutputFunc
	jobs      map[string]bool
}

func (s *sandboxed) ensure(ctx context.Context) (*rpcpipe.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil && s.child != nil {
		select {
		case <-s.conn.Done():
			stopToolHost(s.child, s.pipe, s.conn, s.waited)
			s.child, s.pipe, s.conn, s.waited = nil, nil, nil, nil
		default:
			return s.conn, nil
		}
	}
	if s.start == nil {
		return nil, &Error{Code: CodeUnavailable, Message: "tool host has no starter"}
	}
	child, pipe, err := s.start(ctx)
	if err != nil {
		return nil, err
	}
	if child == nil || pipe == nil {
		if pipe != nil {
			_ = pipe.Close()
		}
		if child != nil {
			_ = syscall.Kill(-child.Pid, syscall.SIGKILL)
			_, _ = child.Wait()
		}
		return nil, &Error{Code: CodeUnavailable, Message: "tool host starter returned no process or pipe"}
	}
	s.epoch++
	epoch := s.epoch
	conn := rpcpipe.New(pipe, pipe, rpcpipe.Options{Notify: func(method string, raw json.RawMessage) { s.notify(epoch, method, raw) }})
	s.conn, s.child, s.pipe = conn, child, pipe
	s.waited = make(chan struct{})
	s.callbacks = make(map[string]OutputFunc)
	s.jobs = make(map[string]bool)
	go func(done chan struct{}) { _, _ = child.Wait(); close(done); _ = conn.Close() }(s.waited)
	return conn, nil
}

func (s *sandboxed) call(ctx context.Context, method string, params, result any) error {
	c, err := s.ensure(ctx)
	if err != nil {
		return err
	}
	return callToolHost(ctx, c, method, params, result)
}

func callToolHost(ctx context.Context, c *rpcpipe.Conn, method string, params, result any) error {
	err := c.Call(ctx, method, params, result)
	if err != nil {
		var rpc *rpcpipe.Error
		if errors.As(err, &rpc) && rpc.Code == WireErrorCode {
			var e Error
			if json.Unmarshal(rpc.Data, &e) == nil && e.Code != "" {
				return &e
			}
		}
		if errors.Is(err, rpcpipe.ErrClosed) || errors.Is(err, io.EOF) {
			return &Error{Code: CodeUnavailable, Message: "tool host stopped"}
		}
	}
	return err
}

func (s *sandboxed) notify(epoch uint64, method string, raw json.RawMessage) {
	if method != NotifyOutput {
		return
	}
	var output Output
	if json.Unmarshal(raw, &output) != nil {
		return
	}
	s.mu.Lock()
	if s.epoch != epoch {
		s.mu.Unlock()
		return
	}
	fn := s.callbacks[output.ID]
	s.mu.Unlock()
	if fn != nil {
		fn(output)
	}
}

func (s *sandboxed) Stat(ctx context.Context, path string) (v FileInfo, err error) {
	err = s.call(ctx, MethodStat, PathParams{Path: path}, &v)
	return
}
func (s *sandboxed) Read(ctx context.Context, req ReadRequest) (v ReadResult, err error) {
	err = s.call(ctx, MethodRead, req, &v)
	return
}
func (s *sandboxed) Write(ctx context.Context, req WriteRequest) (v WriteResult, err error) {
	err = s.call(ctx, MethodWrite, req, &v)
	return
}
func (s *sandboxed) Edit(ctx context.Context, req EditRequest) (v EditResult, err error) {
	err = s.call(ctx, MethodEdit, req, &v)
	return
}
func (s *sandboxed) Remove(ctx context.Context, req RemoveRequest) error {
	return s.call(ctx, MethodRemove, req, nil)
}
func (s *sandboxed) List(ctx context.Context, req ListRequest) (v ListResult, err error) {
	err = s.call(ctx, MethodList, req, &v)
	return
}
func (s *sandboxed) Glob(ctx context.Context, req GlobRequest) (v GlobResult, err error) {
	err = s.call(ctx, MethodGlob, req, &v)
	return
}
func (s *sandboxed) Grep(ctx context.Context, req GrepRequest) (v GrepResult, err error) {
	err = s.call(ctx, MethodGrep, req, &v)
	return
}

func (s *sandboxed) Start(ctx context.Context, req StartRequest, out OutputFunc) (string, error) {
	c, err := s.ensure(ctx)
	if err != nil {
		return "", err
	}
	var started StartResult
	startCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	if err := callToolHost(startCtx, c, MethodStart, req, &started); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		discardStart(c, started.ID)
		return "", err
	}
	s.mu.Lock()
	if s.conn != c {
		s.mu.Unlock()
		discardStart(c, started.ID)
		return "", &Error{Code: CodeUnavailable, Message: "tool host stopped"}
	}
	s.jobs[started.ID] = true
	if out != nil {
		s.callbacks[started.ID] = out
	}
	s.mu.Unlock()
	if err := c.Notify(NotifyReady, ProcessParams{ID: started.ID}); err != nil {
		s.mu.Lock()
		delete(s.jobs, started.ID)
		delete(s.callbacks, started.ID)
		s.mu.Unlock()
		discardStart(c, started.ID)
		return "", &Error{Code: CodeUnavailable, Message: "tool host stopped"}
	}
	if err := ctx.Err(); err != nil {
		s.mu.Lock()
		delete(s.jobs, started.ID)
		delete(s.callbacks, started.ID)
		s.mu.Unlock()
		discardStart(c, started.ID)
		return "", err
	}
	return started.ID, nil
}

func discardStart(c *rpcpipe.Conn, id string) {
	if id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = callToolHost(ctx, c, MethodKill, ProcessParams{ID: id}, nil)
	_ = callToolHost(ctx, c, MethodRelease, ProcessParams{ID: id}, nil)
}

func (s *sandboxed) Wait(ctx context.Context, id string) (v Exit, err error) {
	err = s.call(ctx, MethodWait, ProcessParams{ID: id}, &v)
	return
}
func (s *sandboxed) Peek(ctx context.Context, id string) (v Exit, err error) {
	err = s.call(ctx, MethodPeek, ProcessParams{ID: id}, &v)
	return
}
func (s *sandboxed) Kill(ctx context.Context, id string) error {
	return s.call(ctx, MethodKill, ProcessParams{ID: id}, nil)
}
func (s *sandboxed) Release(ctx context.Context, id string) error {
	err := s.call(ctx, MethodRelease, ProcessParams{ID: id}, nil)
	if err == nil {
		s.mu.Lock()
		delete(s.jobs, id)
		delete(s.callbacks, id)
		s.mu.Unlock()
	}
	return err
}
func (s *sandboxed) Running() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.jobs))
	for id := range s.jobs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
func (s *sandboxed) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return false
	}
	select {
	case <-s.conn.Done():
		return false
	default:
		return true
	}
}

func (s *sandboxed) Close() error {
	s.mu.Lock()
	child, pipe, conn, waited := s.child, s.pipe, s.conn, s.waited
	s.child, s.pipe, s.conn, s.waited = nil, nil, nil, nil
	s.epoch++
	s.jobs = make(map[string]bool)
	s.callbacks = make(map[string]OutputFunc)
	s.mu.Unlock()
	stopToolHost(child, pipe, conn, waited)
	return nil
}

func stopToolHost(child *os.Process, pipe io.ReadWriteCloser, conn *rpcpipe.Conn, waited <-chan struct{}) {
	if child == nil {
		return
	}
	// EOF lets the server release its jobs; the group signal also covers
	// descendants that have inherited the pipe.
	_ = pipe.Close()
	_ = conn.Close()
	select {
	case <-waited:
		return
	default:
	}
	_ = syscall.Kill(-child.Pid, syscall.SIGTERM)
	select {
	case <-waited:
	case <-time.After(killGrace):
		_ = syscall.Kill(-child.Pid, syscall.SIGKILL)
		<-waited
	}
}

var _ Executor = (*sandboxed)(nil)
