// Package client connects to rashind: it dials the socket, starts the daemon
// when Rashin is on and nothing listens, correlates responses with requests and
// delivers events. The command line and the app both use it.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// ErrDisabled means Rashin is turned off.
var ErrDisabled = errors.New(i18n.T("Rashin is off. Turn it on in Ryoku Settings > Apps > Rashin, or run rashin on."))

// ErrUnreachable means Rashin is on but its daemon did not answer.
var ErrUnreachable = errors.New(i18n.Tf("Rashin's daemon did not start. Check %s.", paths.DaemonLog()))

// Options configure Dial.
type Options struct {
	// Client is the protocol client kind sent in hello.
	Client string
	// Socket overrides paths.Socket().
	Socket string
	// Start launches the daemon when Rashin is on and the socket is absent.
	Start bool
	// StartTimeout bounds the wait for a started daemon (default 3 s).
	StartTimeout time.Duration
}

func (o Options) socket() string {
	if o.Socket != "" {
		return o.Socket
	}
	return paths.Socket()
}

// Conn is one connection to the daemon. Call is safe for concurrent use.
type Conn struct {
	nc    net.Conn
	enc   *protocol.Encoder
	Hello protocol.HelloResult

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan protocol.Frame
	err     error
	done    chan struct{}

	qmu    sync.Mutex
	queue  []protocol.Event
	signal chan struct{}
	events chan protocol.Event
}

// Dial connects, starting the daemon when allowed, and sends hello.
func Dial(ctx context.Context, opts Options) (*Conn, error) {
	nc, err := dialSocket(ctx, opts.socket())
	if err != nil && opts.Start {
		if err := Ensure(ctx, opts); err != nil {
			return nil, err
		}
		nc, err = dialSocket(ctx, opts.socket())
	}
	if err != nil {
		s, _ := config.Load()
		if !s.Enabled {
			return nil, ErrDisabled
		}
		return nil, fmt.Errorf("%w (%v)", ErrUnreachable, err)
	}
	c := newConn(nc)
	kind := opts.Client
	if kind == "" {
		kind = protocol.ClientCLI
	}
	if err := c.Call(ctx, protocol.MHello, protocol.HelloParams{
		Client: kind, Version: protocol.Version, PID: os.Getpid(),
	}, &c.Hello); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func dialSocket(ctx context.Context, path string) (net.Conn, error) {
	if err := verifySocketDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	if err := verifyPeerUID(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func verifyPeerUID(conn net.Conn) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return errors.New(i18n.T("The connection is not a Unix socket."))
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return err
	}
	var cred *syscall.Ucred
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		cred, socketErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return err
	}
	if socketErr != nil {
		return socketErr
	}
	if cred == nil || cred.Uid != uint32(os.Getuid()) {
		return errors.New(i18n.T("The daemon uid does not match the client uid."))
	}
	return nil
}

func verifySocketDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s", i18n.Tf("The socket parent is not a directory: %s.", path))
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("%s", i18n.Tf("The socket parent is not owned by uid %d: %s.", os.Getuid(), path))
	}
	if info.Mode().Perm() != 0o700 {
		return fmt.Errorf("%s", i18n.Tf("The socket parent permissions are %o, want 700: %s.", info.Mode().Perm(), path))
	}
	return nil
}

func newConn(nc net.Conn) *Conn {
	c := &Conn{
		nc:      nc,
		enc:     protocol.NewEncoder(nc),
		pending: map[int64]chan protocol.Frame{},
		done:    make(chan struct{}),
		signal:  make(chan struct{}, 1),
		events:  make(chan protocol.Event),
	}
	go c.readLoop()
	go c.pump()
	return c
}

func (c *Conn) readLoop() {
	dec := protocol.NewDecoder(c.nc)
	for {
		f, err := dec.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.EOF
			}
			c.fail(err)
			return
		}
		switch {
		case f.IsEvent():
			c.qmu.Lock()
			c.queue = append(c.queue, protocol.Event{Event: f.Event, Data: f.Data})
			c.qmu.Unlock()
			select {
			case c.signal <- struct{}{}:
			default:
			}
		case f.ID != nil:
			c.mu.Lock()
			ch := c.pending[*f.ID]
			delete(c.pending, *f.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- f
			}
		}
	}
}

// pump moves queued events to the events channel so a slow reader never
// stalls responses.
func (c *Conn) pump() {
	defer close(c.events)
	for {
		c.qmu.Lock()
		if len(c.queue) == 0 {
			c.qmu.Unlock()
			select {
			case <-c.signal:
				continue
			case <-c.done:
				return
			}
		}
		ev := c.queue[0]
		c.queue = c.queue[1:]
		c.qmu.Unlock()
		select {
		case c.events <- ev:
		case <-c.done:
			return
		}
	}
}

func (c *Conn) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return
	}
	c.err = err
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	close(c.done)
}

// Call runs method and decodes its result into result (which may be nil).
func (c *Conn) Call(ctx context.Context, method string, params, result any) error {
	raw, err := protocol.Marshal(params)
	if err != nil {
		return err
	}
	ch := make(chan protocol.Frame, 1)
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return fmt.Errorf("%s", i18n.Tf("Rashin connection closed: %v", err))
	}
	c.nextID++
	id := c.nextID
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.enc.Encode(protocol.Request{ID: id, Method: method, Params: raw}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return err
	}
	select {
	case f, ok := <-ch:
		if !ok {
			return fmt.Errorf("%s", i18n.Tf("Rashin connection closed: %v", c.Err()))
		}
		if f.Error != nil {
			return f.Error
		}
		if result == nil {
			return nil
		}
		if err := json.Unmarshal(f.Result, result); err != nil {
			return fmt.Errorf("%s", i18n.Tf("Could not decode the %s result: %v", method, err))
		}
		return nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return ctx.Err()
	}
}

// Events delivers pushed events in order. It closes when the connection does.
func (c *Conn) Events() <-chan protocol.Event { return c.events }

// Done closes when the connection ends.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Err is why the connection ended.
func (c *Conn) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Close ends the connection.
func (c *Conn) Close() error {
	err := c.nc.Close()
	c.fail(io.EOF)
	return err
}

// Reachable reports whether a daemon answers on the socket right now. It never
// starts one.
func Reachable(ctx context.Context, socket string) bool {
	if socket == "" {
		socket = paths.Socket()
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	nc, err := dialSocket(ctx, socket)
	if err != nil {
		return false
	}
	nc.Close()
	return true
}

// Ensure makes sure a daemon listens while Rashin is on. Under systemd an
// enabled rashind.socket already listens; otherwise rashind is started
// detached and Ensure waits for its socket.
func Ensure(ctx context.Context, opts Options) error {
	s, err := config.Load()
	if err != nil {
		return err
	}
	if !s.Enabled {
		return ErrDisabled
	}
	socket := opts.socket()
	if Reachable(ctx, socket) {
		return nil
	}
	if startSocketUnit(ctx) && waitSocket(ctx, socket, opts.StartTimeout) {
		return nil
	}
	if err := spawnDaemon(); err != nil {
		return fmt.Errorf("%w (%v)", ErrUnreachable, err)
	}
	if !waitSocket(ctx, socket, opts.StartTimeout) {
		return ErrUnreachable
	}
	return nil
}

// startSocketUnit starts rashind.socket when a user systemd manager knows it.
func startSocketUnit(ctx context.Context) bool {
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return false
	}
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return exec.CommandContext(cctx, systemctl, "--user", "start", "rashind.socket").Run() == nil
}

func waitSocket(ctx context.Context, socket string, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if Reachable(ctx, socket) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(50 * time.Millisecond):
		}
	}
	return false
}

// DaemonPath finds rashind: RASHIN_DAEMON, beside this executable, then PATH.
func DaemonPath() (string, error) {
	if p := os.Getenv("RASHIN_DAEMON"); p != "" {
		return p, nil
	}
	if self, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(self), "rashind")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return exec.LookPath("rashind")
}

func spawnDaemon() error {
	bin, err := DaemonPath()
	if err != nil {
		return err
	}
	if err := paths.EnsureDir(paths.StateDir()); err != nil {
		return err
	}
	logf, err := os.OpenFile(paths.DaemonLog(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer devnull.Close()
	cmd := exec.Command(bin)
	cmd.Dir = paths.Home()
	cmd.Stdin = devnull
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
