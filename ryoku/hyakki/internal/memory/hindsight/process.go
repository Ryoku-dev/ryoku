package hindsight

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/childproc"
	"ryoku-rashin/internal/memory/backend"
	"ryoku-rashin/internal/protocol"
)

// instance names pg0's Postgres instance and its database user.
const instance = "rashin"

// Stable credential IDs keep existing encrypted API and database secrets usable.
const (
	KeyName      = "RASHIN_HINDSIGHT_KEY"
	DatabaseName = "RASHIN_HINDSIGHT_DB"
)

// How long a start may take before it fails, and how long a stop waits after
// SIGTERM before SIGKILL. The worker hands its tasks back within 20 s of a
// SIGTERM.
const (
	startWait    = 60 * time.Second
	setupWait    = 15 * time.Minute
	stopWait     = 25 * time.Second
	healthPeriod = 250 * time.Millisecond
)

// Config is what a local Hindsight needs from Rashin.
type Config struct {
	Layout  Layout
	Secrets backend.Secrets
	Gateway Gateway
	// Model returns the model memory learns with now, given the model that
	// answered the conversation Hindsight learns from last, or "" while
	// none may.
	Model func(ctx context.Context, answered string) string
}

// Process runs a local hindsight-api for as long as Rashin needs it. It
// starts on the first call that needs it, never before, and Stop ends it
// with the Postgres it started.
type Process struct {
	cfg       Config
	stopWait  time.Duration
	startWait time.Duration

	mu       sync.Mutex
	run      *running
	answered string
}

// running is one start of the child. ready closes once the start finished;
// err says why it failed.
type running struct {
	ready  chan struct{}
	err    error
	cancel context.CancelFunc
	base   string
	key    string
	cmd    *exec.Cmd
	exited chan struct{}
	models *modelListener
	log    *os.File
}

// crashed reports a child that started and has since exited on its own.
func (r *running) crashed() bool {
	select {
	case <-r.ready:
	default:
		return false
	}
	select {
	case <-r.exited:
		return r.err == nil
	default:
		return false
	}
}

// NewProcess prepares a local Hindsight without starting anything.
func NewProcess(cfg Config) *Process {
	return &Process{cfg: cfg, stopWait: stopWait, startWait: startWait}
}

// setAnswered records the model that answered the conversation handed to
// Hindsight last; the model listener learns with it when the policy allows.
func (p *Process) setAnswered(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.answered = model
}

func (p *Process) lastAnswered() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.answered
}

// Layout is where this Hindsight keeps its pieces.
func (p *Process) Layout() Layout { return p.cfg.Layout }

// endpoint returns the running API's address and key, starting it first when
// it is not running. A caller whose context ends stops waiting; the start
// goes on.
func (p *Process) endpoint(ctx context.Context) (base, key string, err error) {
	r := p.ensure(false)
	select {
	case <-r.ready:
	case <-ctx.Done():
		return "", "", ctx.Err()
	}
	if r.err != nil {
		return "", "", r.err
	}
	return r.base, r.key, nil
}

// ensure returns the current start, beginning one when there is none.
func (p *Process) ensure(online bool) *running {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.run != nil && !p.run.crashed() {
		return p.run
	}
	if p.run != nil {
		// a child that died on its own leaves only what serves it to close
		p.terminate(p.run)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{ready: make(chan struct{}), cancel: cancel, exited: make(chan struct{})}
	p.run = r
	wait := p.startWait
	if online {
		wait = setupWait
	}
	go func() {
		r.err = p.start(ctx, r, !online, wait)
		if r.err != nil {
			p.mu.Lock()
			if p.run == r {
				p.run = nil
			}
			p.mu.Unlock()
		}
		close(r.ready)
	}()
	return r
}

// state says whether the child is starting or running, without starting it.
func (p *Process) state() (starting, running bool) {
	p.mu.Lock()
	r := p.run
	p.mu.Unlock()
	if r == nil {
		return false, false
	}
	select {
	case <-r.ready:
		return false, r.err == nil
	default:
		return true, false
	}
}

// start launches the child and waits until it answers. On failure it leaves
// nothing running.
func (p *Process) start(ctx context.Context, r *running, offline bool, wait time.Duration) (err error) {
	layout := p.cfg.Layout
	if !layout.Installed() {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Set up Hindsight first, in Settings > Memory."))
	}
	key, password, err := p.secrets()
	if err != nil {
		return err
	}
	modelKey, err := p.cfg.Gateway.APIKey(ctx, false)
	if err != nil {
		return err
	}
	// Postgres left by a stop that went wrong would hold the instance
	p.reap()
	for _, dir := range []string{layout.home(), layout.run(), layout.Cache} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}
	port, err := childproc.FreePort()
	if err != nil {
		return err
	}
	dbPort, err := childproc.FreePort()
	if err != nil {
		return err
	}
	models, err := listenModels(p.cfg.Gateway.API(), func(ctx context.Context) string { return p.cfg.Model(ctx, p.lastAnswered()) })
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			models.close()
		}
	}()
	log, err := os.OpenFile(layout.log(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = log.Close()
		}
	}()
	l := launch{port: port, dbPort: dbPort, key: key, password: password, models: models.base(), modelKey: modelKey, offline: offline}
	cmd := exec.Command(layout.program(), l.args()...)
	cmd.Env = l.env(layout)
	// a private 0700 folder, because Hindsight reads a .env from where it runs
	cmd.Dir = layout.run()
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = childproc.Attr()
	if err := childproc.LaunchLocked(cmd, r.exited); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("Hindsight did not start"), err)
	}
	r.cmd, r.models, r.log = cmd, models, log
	r.base, r.key = "http://"+net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), key
	if err := waitHealthy(ctx, r, wait); err != nil {
		p.terminate(r)
		r.cmd, r.models, r.log = nil, nil, nil
		return err
	}
	return nil
}

// waitHealthy polls /health until it answers 200, the child exits, the start
// is cancelled or wait passes.
func waitHealthy(ctx context.Context, r *running, wait time.Duration) error {
	err := childproc.WaitHealthy(ctx, childproc.Health{URL: r.base + "/health", Bearer: r.key, Exited: r.exited, Wait: wait, Period: healthPeriod})
	switch {
	case errors.Is(err, childproc.ErrExited):
		return errors.New(i18n.Tf("Hindsight stopped while starting. Its log is %s.", r.log.Name()))
	case errors.Is(err, childproc.ErrTimeout):
		return errors.New(i18n.Tf("Hindsight did not answer within %s. Its log is %s.", wait, r.log.Name()))
	}
	return err
}

// secrets reads the API key and database password from the vault.
func (p *Process) secrets() (key, password string, err error) {
	if key, err = p.cfg.Secrets.Secret(KeyName); err != nil {
		return "", "", err
	}
	if password, err = p.cfg.Secrets.Secret(DatabaseName); err != nil {
		return "", "", err
	}
	return key, password, nil
}

// Stop ends the child: SIGTERM to its process group, SIGKILL after the wait,
// then pg0's own stop if Postgres outlived it. A start still under way is
// cancelled. The next call starts it again.
func (p *Process) Stop() {
	p.mu.Lock()
	r := p.run
	p.run = nil
	p.mu.Unlock()
	if r != nil {
		r.cancel()
		<-r.ready
		if r.err == nil {
			p.terminate(r)
		}
	}
	p.reap()
}

// terminate stops one started child and what serves it.
func (p *Process) terminate(r *running) {
	if r.cmd != nil {
		childproc.Terminate(r.cmd, r.exited, p.stopWait)
	}
	if r.models != nil {
		r.models.close()
	}
	if r.log != nil {
		_ = r.log.Close()
	}
}

// reap stops a Postgres that outlived Hindsight, which pg0 started apart
// from Hindsight's process group.
func (p *Process) reap() {
	layout := p.cfg.Layout
	if _, err := os.Stat(layout.postmaster()); err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// pg0 ships no command in the environment, only its Python package
	cmd := exec.CommandContext(ctx, layout.python(), "-I", "-c", "from pg0 import Pg0; Pg0(name='"+instance+"').stop()")
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + layout.home()}
	cmd.Dir = layout.run()
	_ = cmd.Run()
}

// newSecret is setup.go's name for a fresh secret.
func newSecret() (string, error) { return childproc.NewSecret() }
