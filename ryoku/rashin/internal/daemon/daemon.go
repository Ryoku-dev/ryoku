package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const shutdownTimeout = 5 * time.Second

type Deps struct {
	Gateway   core.Gateway
	Asker     core.Asker
	Harnesses core.Harnesses
	Indexer   core.Indexer
	Work      core.Work
	Version   string
	Events    *Broadcaster
	Lock      *os.File
}

type Daemon struct {
	deps       Deps
	started    time.Time
	mu         sync.RWMutex
	settingsMu sync.Mutex
	settings   protocol.Settings
	listener   net.Listener
	bound      bool
	lock       *os.File
	clients    map[*connection]struct{}
	manager    *sessionManager
	events     *Broadcaster
	removeFeed func()
	stop       chan struct{}
	stopOnce   sync.Once
	idle       *time.Timer
	logger     *daemonLogger
	api        apiServer
}

func New(deps Deps) *Daemon {
	events := deps.Events
	if events == nil {
		events = NewBroadcaster()
	}
	d := &Daemon{deps: deps, started: time.Now(), clients: make(map[*connection]struct{}), events: events, stop: make(chan struct{})}
	d.manager = newSessionManager(deps.Asker, deps.Harnesses, deps.Work, d.Settings, d.Broadcast, d.stateChanged, d.warn)
	d.manager.brief = d.agentBrief
	d.manager.prepare = d.indexSession
	if deps.Work != nil {
		deps.Work.Bind(d.manager)
	}
	d.removeFeed = events.add(d.deliverGlobal)
	return d
}

func (d *Daemon) Settings() protocol.Settings {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.settings
}

func (d *Daemon) Broadcast(event string, data any) { d.events.Broadcast(event, data) }

// ProvidersChanged publishes the gateway's latest asynchronous health result.
func (d *Daemon) ProvidersChanged() { d.broadcastProviders(context.Background()) }

// Run serves until shutdown or ctx cancellation.
func (d *Daemon) Run(ctx context.Context) error {
	settings, err := config.Load()
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.settings = settings
	d.mu.Unlock()
	if !settings.Enabled {
		return d.closeAll()
	}
	if err := d.runMigrations(ctx); err != nil {
		d.warn("startup migration: %v", err)
	}
	listener, bound, lock, err := listen(d.deps.Lock)
	if err != nil {
		return err
	}
	if listener == nil {
		if lock != nil {
			_ = lock.Close()
		}
		return nil
	}
	d.mu.Lock()
	d.listener = listener
	d.bound = bound
	d.lock = lock
	d.mu.Unlock()
	go func() { <-ctx.Done(); d.Shutdown() }()
	d.applyAPI()
	d.stateChanged()
	for {
		nc, acceptErr := listener.Accept()
		if acceptErr != nil {
			select {
			case <-d.stop:
				return d.closeAll()
			default:
			}
			if errors.Is(acceptErr, net.ErrClosed) {
				return d.closeAll()
			}
			d.warn("accept: %v", acceptErr)
			continue
		}
		if err := verifyPeerUID(nc); err != nil {
			d.warn("client credentials: %v", err)
			_ = nc.Close()
			continue
		}
		conn := newConnection(d, nc)
		d.mu.Lock()
		d.clients[conn] = struct{}{}
		d.mu.Unlock()
		d.stateChanged()
		go conn.serve()
	}
}

// AcquireLock serializes daemon startup before any mutable stores are opened.
func AcquireLock() (*os.File, bool, error) {
	if err := secureDir(paths.RuntimeDir()); err != nil {
		return nil, false, err
	}
	lock, err := os.OpenFile(paths.DaemonLock(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if err := lock.Truncate(0); err == nil {
		_, err = lock.WriteString(strconv.Itoa(os.Getpid()) + "\n")
	}
	if err != nil {
		lock.Close()
		return nil, false, err
	}
	return lock, true, nil
}

func listen(lock *os.File) (net.Listener, bool, *os.File, error) {
	if lock == nil {
		var acquired bool
		var err error
		lock, acquired, err = AcquireLock()
		if err != nil || !acquired {
			return nil, false, lock, err
		}
	}
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) {
		fds, _ := strconv.Atoi(os.Getenv("LISTEN_FDS"))
		if fds >= 1 {
			file := os.NewFile(3, "rashind.socket")
			if file == nil {
				lock.Close()
				return nil, false, nil, fmt.Errorf("%s", i18n.T("Socket activation did not provide fd 3."))
			}
			listener, err := net.FileListener(file)
			file.Close()
			if err != nil {
				lock.Close()
				return nil, false, nil, err
			}
			return listener, false, lock, nil
		}
	}
	if err := secureDir(filepath.Dir(paths.Socket())); err != nil {
		lock.Close()
		return nil, false, nil, err
	}
	if stat, statErr := os.Lstat(paths.Socket()); statErr == nil {
		if stat.Mode()&os.ModeSocket == 0 {
			lock.Close()
			return nil, false, nil, fmt.Errorf("%s", i18n.Tf("The socket path is not a socket: %s.", paths.Socket()))
		}
		if err := os.Remove(paths.Socket()); err != nil {
			lock.Close()
			return nil, false, nil, err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		lock.Close()
		return nil, false, nil, statErr
	}
	listener, err := net.Listen("unix", paths.Socket())
	if err != nil {
		lock.Close()
		return nil, false, nil, err
	}
	if err := os.Chmod(paths.Socket(), 0o600); err != nil {
		listener.Close()
		lock.Close()
		return nil, false, nil, err
	}
	return listener, true, lock, nil
}

func secureDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s", i18n.Tf("The runtime path is not a directory: %s.", path))
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("%s", i18n.Tf("The runtime directory is not owned by uid %d: %s.", os.Getuid(), path))
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(path, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func verifyPeerUID(conn net.Conn) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("%s", i18n.T("The connection is not a Unix socket."))
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
		return fmt.Errorf("%s", i18n.T("The peer uid does not match the daemon uid."))
	}
	return nil
}

func (d *Daemon) Shutdown() {
	d.stopOnce.Do(func() {
		close(d.stop)
		d.mu.Lock()
		if d.idle != nil {
			d.idle.Stop()
		}
		if d.listener != nil {
			_ = d.listener.Close()
		}
		clients := make([]*connection, 0, len(d.clients))
		for c := range d.clients {
			clients = append(clients, c)
		}
		d.mu.Unlock()
		d.closeAPI()
		for _, c := range clients {
			c.close()
		}
	})
}

func (d *Daemon) closeAll() error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.manager.shutdown(shutdownTimeout - time.Second)
		if d.deps.Work != nil {
			_ = d.deps.Work.Close()
		}
		if d.deps.Indexer != nil {
			_ = d.deps.Indexer.Close()
		}
		if d.deps.Gateway != nil {
			_ = d.deps.Gateway.Close()
		}
		if d.removeFeed != nil {
			d.removeFeed()
		}
		if d.logger != nil {
			_ = d.logger.Close()
		}
	}()
	timer := time.NewTimer(shutdownTimeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
	d.releaseTransport()
	return nil
}

func (d *Daemon) releaseTransport() {
	d.mu.Lock()
	bound, lock := d.bound, d.lock
	d.bound = false
	d.lock = nil
	d.mu.Unlock()
	if bound {
		_ = os.Remove(paths.Socket())
	}
	if lock != nil {
		_ = lock.Close()
	}
}

func (d *Daemon) disconnected(c *connection) {
	c.dropWatches()
	d.manager.detachAll(c)
	d.mu.Lock()
	delete(d.clients, c)
	d.mu.Unlock()
	d.stateChanged()
}

func (d *Daemon) stateChanged() {
	d.mu.Lock()
	if d.idle != nil {
		d.idle.Stop()
	}
	minutes := d.settings.Daemon.IdleExitMinutes
	if minutes <= 0 {
		minutes = 5
	}
	agentMinutes := d.settings.Agent.IdleStopMinutes
	if agentMinutes > 0 && agentMinutes < minutes {
		minutes = agentMinutes
	}
	d.idle = time.AfterFunc(time.Duration(minutes)*time.Minute, d.onIdleDeadline)
	d.mu.Unlock()
}

func (d *Daemon) onIdleDeadline() {
	d.manager.idleStop(time.Now())
	if !d.Idle() {
		d.stateChanged()
		return
	}
	if d.deps.Indexer != nil {
		d.deps.Indexer.Release()
	}
	// other programs hold the address of the local API, so a daemon that serves it stays up at rest
	if d.api.listening() {
		return
	}
	d.Shutdown()
}

// Idle reports the exit predicate for tests and lifecycle diagnostics.
func (d *Daemon) Idle() bool {
	d.mu.RLock()
	connections := len(d.clients)
	d.mu.RUnlock()
	return connections == 0 && !d.manager.busy() && (d.deps.Indexer == nil || !d.deps.Indexer.Busy()) && (d.deps.Work == nil || !d.deps.Work.Busy())
}
