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
	"ryoku-rashin/internal/skills"
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
	Teams     core.Teams
	// Memory is what Rashin remembers between conversations; nil runs the
	// daemon without it.
	Memory core.Memory
	// Skills are the built-in skill sets; nil runs the daemon without them.
	Skills  *skills.Library
	Version string
	Events  *Broadcaster
	Lock    *os.File
	// Program is the file this daemon was started from, watched so a newer
	// build takes over once nothing is running. Empty watches nothing.
	Program string
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
	screens    *screenManager
	events     *Broadcaster
	removeFeed func()
	stop       chan struct{}
	stopOnce   sync.Once
	idle       *time.Timer
	program    *programWatch
	retireOnce sync.Once
	logger     *daemonLogger
	api        apiServer
	// appPeer reports whether a connecting process is the Rashin app, which
	// alone may call teams.UserOnly methods.
	appPeer func(pid int32) bool
	// agents are the team sessions' own sockets
	agents agentSockets
	// learned is the pending harnesses.changed after capabilities were
	// recorded
	learnedMu sync.Mutex
	learned   *time.Timer
	// migrations are the session scrub passes running in the background
	migrations migrations
}

func New(deps Deps) *Daemon {
	events := deps.Events
	if events == nil {
		events = NewBroadcaster()
	}
	d := &Daemon{deps: deps, started: time.Now(), clients: make(map[*connection]struct{}), events: events, stop: make(chan struct{}), appPeer: appPeer(deps.Program)}
	d.manager = newSessionManager(deps.Asker, deps.Harnesses, deps.Work, d.Settings, d.Broadcast, d.stateChanged, d.warn)
	d.manager.brief = d.agentBrief
	d.manager.tools = d.agentTools
	d.manager.prepare = d.indexSession
	d.screens = newScreenManager(d.Broadcast, d.manager.ask, screenable)
	d.manager.sandbox = d.agentSandbox
	d.screens.sandbox = d.screenSandbox
	d.manager.ended = func(id string) {
		d.screens.stop(id)
		d.closeAgentSocket(id)
	}
	d.manager.watched = d.watched
	if deps.Work != nil {
		deps.Work.Bind(d.manager)
	}
	if deps.Teams != nil {
		d.manager.teams = deps.Teams
		d.screens.granted = func(id string) bool {
			info, ok := d.manager.Info(id)
			return ok && deps.Teams.ScreenGranted(info)
		}
		deps.Teams.Bind(d.manager)
	}
	if deps.Memory != nil {
		d.manager.memory = deps.Memory
		deps.Memory.Bind(d.manager)
	}
	if deps.Harnesses != nil {
		deps.Harnesses.OnLearned(d.harnessesLearned)
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

// watched reports whether a Rashin window is connected, so Teams records
// what happens without one as unwatched and notifies the desktop instead.
func (d *Daemon) watched() bool {
	d.mu.RLock()
	clients := make([]*connection, 0, len(d.clients))
	for c := range d.clients {
		clients = append(clients, c)
	}
	d.mu.RUnlock()
	for _, c := range clients {
		c.mu.RLock()
		app := c.kind == protocol.ClientApp && c.session == ""
		c.mu.RUnlock()
		if app {
			return true
		}
	}
	return false
}

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
	d.scrubInBackground()
	// sockets a daemon that died left behind lead nowhere
	_ = os.RemoveAll(paths.AgentSocketsDir())
	// nor do the checkpoint refs of sessions that are gone, temporary ones a crash ended included
	go d.manager.sweepCheckpoints(ctx)
	go func() { <-ctx.Done(); d.Shutdown() }()
	if d.deps.Program != "" {
		if watch, watchErr := watchProgram(d.deps.Program, d.retire); watchErr != nil {
			d.warn("watch %s: %v", d.deps.Program, watchErr)
		} else {
			d.mu.Lock()
			d.program = watch
			d.mu.Unlock()
			defer watch.close()
		}
	}
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
		pid, err := verifyPeer(nc)
		if err != nil {
			d.warn("client credentials: %v", err)
			_ = nc.Close()
			continue
		}
		conn := newConnection(d, nc)
		// read once at accept, leaving the connecting process the least time to exec another program
		conn.app = d.appPeer != nil && d.appPeer(pid)
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
		d.migrations.stop()
		d.manager.shutdown(shutdownTimeout - time.Second)
		d.screens.stopAll()
		if d.deps.Work != nil {
			_ = d.deps.Work.Close()
		}
		if d.deps.Teams != nil {
			_ = d.deps.Teams.Close()
		}
		if d.deps.Memory != nil {
			_ = d.deps.Memory.Close()
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
	d.closeAgentSockets()
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
	program := d.program
	d.mu.Unlock()
	// a turn that just ended may have been all that kept an outdated daemon
	// up; asked on its own goroutine, since callers hold session locks
	if program.isReplaced() {
		go d.retire()
	}
}

// onIdleDeadline stops what idled out and, once nothing is busy, lets go of
// what is loaded and exits. Only when the daemon would otherwise exit does
// memory learn from the conversations still due, so a connected window never
// cuts a conversation's quiet time short; the daemon then stays up while
// memory learns and looks again at the next deadline.
func (d *Daemon) onIdleDeadline() {
	d.manager.idleStop(time.Now())
	d.screens.idleStop(time.Now())
	if !d.restingBesideMemory() {
		d.stateChanged()
		return
	}
	if d.deps.Memory != nil {
		d.deps.Memory.Settle()
		if d.deps.Memory.Busy() {
			d.stateChanged()
			return
		}
	}
	if d.deps.Indexer != nil {
		d.deps.Indexer.Release()
	}
	if d.deps.Memory != nil {
		d.deps.Memory.Release()
	}
	// other programs hold the address of the local API, and the routines that
	// are on run from the daemon's own clock, so either keeps it up at rest
	if d.api.listening() || (d.deps.Teams != nil && d.deps.Teams.Armed()) {
		return
	}
	d.Shutdown()
}

// Idle reports the exit predicate for tests and lifecycle diagnostics.
func (d *Daemon) Idle() bool {
	return d.restingBesideMemory() && (d.deps.Memory == nil || !d.deps.Memory.Busy())
}

// restingBesideMemory reports that no client is connected and nothing but
// memory works.
func (d *Daemon) restingBesideMemory() bool {
	d.mu.RLock()
	connections := len(d.clients)
	d.mu.RUnlock()
	return connections == 0 && !d.manager.busy() && !d.screens.busy() && (d.deps.Indexer == nil || !d.deps.Indexer.Busy()) && (d.deps.Work == nil || !d.deps.Work.Busy()) && (d.deps.Teams == nil || !d.deps.Teams.Busy())
}
