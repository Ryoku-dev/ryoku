// Package toolhost runs file and process operations inside a team task's sandbox.
package toolhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/rpcpipe"
)

// Serve handles JSON-RPC over stdin/stdout until the input closes or ctx ends.
// All filesystem and process work happens in this process's namespace.
func Serve(ctx context.Context, input io.Reader, output io.Writer) error {
	server := &server{host: exec.Host(exec.HostOptions{}), ready: make(chan struct{}), streams: make(map[string]chan struct{})}
	conn := rpcpipe.New(input, output, rpcpipe.Options{Handle: server.handle, Notify: server.notify})
	server.conn = conn
	close(server.ready)
	select {
	case <-ctx.Done():
		_ = conn.Close()
	case <-conn.Done():
	}
	_ = server.host.Close()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := conn.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

type server struct {
	host    exec.Executor
	conn    *rpcpipe.Conn
	ready   chan struct{}
	mu      sync.Mutex
	streams map[string]chan struct{}
}

func (s *server) notify(method string, raw json.RawMessage) {
	if method != exec.NotifyReady {
		return
	}
	var p exec.ProcessParams
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	s.mu.Lock()
	ready := s.streams[p.ID]
	delete(s.streams, p.ID)
	if ready != nil {
		close(ready)
	}
	s.mu.Unlock()
}

func (s *server) handle(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	<-s.ready
	decode := func(v any) error {
		if err := json.Unmarshal(raw, v); err != nil {
			return &rpcpipe.Error{Code: rpcpipe.CodeInvalidParams, Message: err.Error()}
		}
		return nil
	}
	switch method {
	case exec.MethodStat:
		var p exec.PathParams
		if err := decode(&p); err != nil {
			return nil, err
		}
		v, err := s.host.Stat(ctx, p.Path)
		return v, exec.WireError(err)
	case exec.MethodRead:
		var p exec.ReadRequest
		if err := decode(&p); err != nil {
			return nil, err
		}
		v, err := s.host.Read(ctx, p)
		return v, exec.WireError(err)
	case exec.MethodWrite:
		var p exec.WriteRequest
		if err := decode(&p); err != nil {
			return nil, err
		}
		v, err := s.host.Write(ctx, p)
		return v, exec.WireError(err)
	case exec.MethodEdit:
		var p exec.EditRequest
		if err := decode(&p); err != nil {
			return nil, err
		}
		v, err := s.host.Edit(ctx, p)
		return v, exec.WireError(err)
	case exec.MethodRemove:
		var p exec.RemoveRequest
		if err := decode(&p); err != nil {
			return nil, err
		}
		return nil, exec.WireError(s.host.Remove(ctx, p))
	case exec.MethodList:
		var p exec.ListRequest
		if err := decode(&p); err != nil {
			return nil, err
		}
		v, err := s.host.List(ctx, p)
		return v, exec.WireError(err)
	case exec.MethodGlob:
		var p exec.GlobRequest
		if err := decode(&p); err != nil {
			return nil, err
		}
		v, err := s.host.Glob(ctx, p)
		return v, exec.WireError(err)
	case exec.MethodGrep:
		var p exec.GrepRequest
		if err := decode(&p); err != nil {
			return nil, err
		}
		v, err := s.host.Grep(ctx, p)
		return v, exec.WireError(err)
	case exec.MethodStart:
		var p exec.StartRequest
		if err := decode(&p); err != nil {
			return nil, err
		}
		ready := make(chan struct{})
		id, err := s.host.Start(ctx, p, func(o exec.Output) {
			select {
			case <-ready:
				_ = s.conn.Notify(exec.NotifyOutput, o)
			case <-s.conn.Done():
			}
		})
		if err == nil && ctx.Err() != nil {
			close(ready)
			_ = s.host.Kill(context.Background(), id)
			_ = s.host.Release(context.Background(), id)
			return nil, ctx.Err()
		}
		if err == nil {
			s.mu.Lock()
			s.streams[id] = ready
			s.mu.Unlock()
			go func() {
				exit, err := s.host.Wait(context.Background(), id)
				if err == nil {
					exit.Output = ""
					_ = s.conn.Notify(exec.NotifyExit, exit)
				}
			}()
		}
		return exec.StartResult{ID: id}, exec.WireError(err)
	case exec.MethodWait, exec.MethodPeek, exec.MethodKill, exec.MethodRelease:
		var p exec.ProcessParams
		if err := decode(&p); err != nil {
			return nil, err
		}
		switch method {
		case exec.MethodWait:
			v, err := s.host.Wait(ctx, p.ID)
			return v, exec.WireError(err)
		case exec.MethodPeek:
			v, err := s.host.Peek(ctx, p.ID)
			return v, exec.WireError(err)
		case exec.MethodKill:
			return nil, exec.WireError(s.host.Kill(ctx, p.ID))
		default:
			s.mu.Lock()
			ready := s.streams[p.ID]
			delete(s.streams, p.ID)
			if ready != nil {
				close(ready)
			}
			s.mu.Unlock()
			return nil, exec.WireError(s.host.Release(ctx, p.ID))
		}
	}
	return nil, &rpcpipe.Error{Code: rpcpipe.CodeMethodNotFound, Message: fmt.Sprintf("unknown method %s", method)}
}
