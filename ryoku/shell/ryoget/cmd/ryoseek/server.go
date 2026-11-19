package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"ryoget/internal/search"
)

var errRunning = errors.New("another ryoseek daemon is running")

const defaultIdle = 5 * time.Minute

type serverOptions struct {
	Socket string
	Lock   string
	Idle   time.Duration
	// called only once the lock is ours, so a refused second daemon never
	// touches the shared state file
	NewService func() (*search.Service, error)
	Log        *log.Logger
	// the uid peers must run as; the daemon's own uid outside tests
	UID int
}

func idleFromEnv() time.Duration {
	if v := os.Getenv("RYOSEEK_IDLE_SECONDS"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			return time.Duration(n * float64(time.Second))
		}
	}
	return defaultIdle
}

// runServer serves until ctx ends or nothing has happened for opts.Idle.
func runServer(ctx context.Context, opts serverOptions) error {
	if err := os.MkdirAll(filepath.Dir(opts.Socket), 0o700); err != nil {
		return err
	}
	lock, err := takeLock(opts.Lock)
	if err != nil {
		return err
	}
	defer lock.Close()

	svc, err := opts.NewService()
	if err != nil {
		return err
	}
	// the lock is ours, so a socket file left here belongs to a dead daemon
	_ = os.Remove(opts.Socket)
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: opts.Socket, Net: "unix"})
	if err != nil {
		svc.Close()
		return err
	}
	ln.SetUnlinkOnClose(true)
	if err := os.Chmod(opts.Socket, 0o600); err != nil {
		ln.Close()
		svc.Close()
		return err
	}
	opts.Log.Printf("listening on %s", opts.Socket)

	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s := &server{svc: svc, uid: opts.UID, log: opts.Log, conns: map[net.Conn]struct{}{}}
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		s.accept(sctx, ln)
	}()

	reason := s.waitIdle(sctx, opts.Idle)
	opts.Log.Print(reason)
	cancel()
	ln.Close()
	<-acceptDone
	s.mu.Lock()
	for nc := range s.conns {
		nc.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
	if err := svc.Close(); err != nil {
		opts.Log.Printf("close: %v", err)
	}
	opts.Log.Print("stopped")
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
			return nil, errRunning
		}
		return nil, err
	}
	return f, nil
}

type server struct {
	svc *search.Service
	uid int
	log *log.Logger

	wg    sync.WaitGroup
	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

func (s *server) accept(ctx context.Context, ln *net.UnixListener) {
	for {
		nc, err := ln.AcceptUnix()
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, net.ErrClosed) {
				s.log.Printf("accept: %v", err)
			}
			return
		}
		if !peerIs(nc, s.uid) {
			nc.Close()
			continue
		}
		s.mu.Lock()
		s.conns[nc] = struct{}{}
		s.mu.Unlock()
		s.wg.Add(1)
		go s.handle(ctx, nc)
	}
}

func (s *server) handle(ctx context.Context, nc *net.UnixConn) {
	defer s.wg.Done()
	if err := s.svc.Serve(ctx, nc); err != nil && ctx.Err() == nil {
		s.log.Printf("client: %v", err)
	}
	nc.Close()
	s.mu.Lock()
	delete(s.conns, nc)
	s.mu.Unlock()
}

// busy counts accepted connections too: a client can sit between accept and
// Serve where the service does not see it yet.
func (s *server) busy() bool {
	s.mu.Lock()
	n := len(s.conns)
	s.mu.Unlock()
	return n > 0 || !s.svc.Idle()
}

// waitIdle polls because the service has no change signal; the poll is coarse
// against the idle window so a quiet daemon barely wakes.
func (s *server) waitIdle(ctx context.Context, idle time.Duration) string {
	every := min(max(idle/10, 50*time.Millisecond), 5*time.Second)
	t := time.NewTicker(every)
	defer t.Stop()
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return "shutting down"
		case now := <-t.C:
			if s.busy() {
				last = now
			} else if now.Sub(last) >= idle {
				return "idle for " + idle.String() + ", exiting"
			}
		}
	}
}

// peerIs admits only processes running as uid.
func peerIs(nc *net.UnixConn, uid int) bool {
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
	return int(cred.Uid) == uid
}
