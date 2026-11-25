// Package chrome starts the Hands browser: a Chromium-family browser on its
// own profile, driven only over --remote-debugging-pipe. The argv is one
// exact shape; nothing else is ever passed. A distribution launcher that
// reads the user's flag files (or the environment) is skipped by starting
// the real browser binary it would have started, and flatpak, snap and
// shells are refused. No network port is opened.
//
// This is the only package under internal/hands that starts a process.
package chrome

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"ryoku-rashin/internal/hands/cdp"
)

// ProfileName is the last element every Hands profile directory must have.
const ProfileName = "hands-browser"

// The fixed flags, in order.
const (
	flagProfile   = "--user-data-dir="
	flagPipe      = "--remote-debugging-pipe"
	flagClass     = "--class="
	flagFirstRun  = "--no-first-run"
	flagDefault   = "--no-default-browser-check"
	flagExtension = "--disable-extensions"
	flagHeadless  = "--headless"
)

var (
	// ErrExec means the catalog command is not one plain browser program.
	ErrExec = errors.New("chrome: the browser command is not a plain program")
	// ErrSandboxed means a flatpak or snap browser, which cannot be
	// handed the pipe.
	ErrSandboxed = errors.New("chrome: flatpak and snap browsers cannot be driven")
	// ErrShell means the command is a shell or a command runner.
	ErrShell = errors.New("chrome: a shell is never started")
	// ErrWrapper means a launcher whose real browser could not be found.
	ErrWrapper = errors.New("chrome: the browser launcher has no known real program")
	// ErrOptions means a profile, class or start address outside the shape.
	ErrOptions = errors.New("chrome: options outside the allowed shape")
)

// Options are the launch's only inputs.
type Options struct {
	Exec       []string // the catalog entry's argv: exactly one program
	ProfileDir string   // absolute, ending in ProfileName
	Class      string   // unique window class
	StartURL   string   // http, https, file or about:blank
	// Headless adds --headless. Only the engine test and the lab set it;
	// the daemon never does.
	Headless bool
	// Env is the environment to filter (nil: this process's).
	Env []string
}

var classRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Command resolves o into the exact argv Launch starts.
func Command(o Options) ([]string, error) { return resolver{}.command(o) }

type resolver struct {
	root string // test tree prefix; empty is /
	path string // PATH override for tests
	min  int64  // smallest browser program; 0 is MinBrowserBytes
}

func (r resolver) command(o Options) ([]string, error) {
	if len(o.Exec) != 1 || o.Exec[0] == "" || strings.HasPrefix(o.Exec[0], "-") {
		return nil, ErrExec
	}
	real, err := r.realBinary(o.Exec[0], o.Env)
	if err != nil {
		return nil, err
	}
	p := o.ProfileDir
	if !filepath.IsAbs(p) || filepath.Clean(p) != p || filepath.Base(p) != ProfileName || strings.ContainsAny(p, "\x00\n") {
		return nil, fmt.Errorf("%w: profile %q", ErrOptions, p)
	}
	if !classRE.MatchString(o.Class) {
		return nil, fmt.Errorf("%w: class %q", ErrOptions, o.Class)
	}
	if !startURL(o.StartURL) {
		return nil, fmt.Errorf("%w: start address", ErrOptions)
	}
	argv := []string{real, flagProfile + p, flagPipe, flagClass + o.Class, flagFirstRun, flagDefault, flagExtension}
	if o.Headless {
		argv = append(argv, flagHeadless)
	}
	return append(argv, o.StartURL), nil
}

// Allowed says whether argv has the one launch shape.
func Allowed(argv []string) bool {
	if len(argv) != 8 && len(argv) != 9 {
		return false
	}
	if !filepath.IsAbs(argv[0]) || !strings.HasPrefix(argv[1], flagProfile) || !strings.HasPrefix(argv[3], flagClass) {
		return false
	}
	p := strings.TrimPrefix(argv[1], flagProfile)
	if !filepath.IsAbs(p) || filepath.Clean(p) != p || filepath.Base(p) != ProfileName {
		return false
	}
	if !classRE.MatchString(strings.TrimPrefix(argv[3], flagClass)) {
		return false
	}
	want := []string{flagPipe, flagFirstRun, flagDefault, flagExtension}
	if argv[2] != want[0] || !slices.Equal(argv[4:7], want[1:]) {
		return false
	}
	if len(argv) == 9 && argv[7] != flagHeadless {
		return false
	}
	return startURL(argv[len(argv)-1])
}

func startURL(s string) bool {
	if s == "about:blank" {
		return true
	}
	u, err := url.Parse(s)
	if err != nil || strings.HasPrefix(s, "-") || strings.ContainsAny(s, " \t\n\x00") {
		return false
	}
	switch u.Scheme {
	case "http", "https":
		return u.Host != "" && u.User == nil
	case "file":
		return u.Host == "" && strings.HasPrefix(u.Path, "/")
	}
	return false
}

// envKeep are the variables the browser may see. Every flag variable a
// launcher or the browser could read (CHROMIUM_FLAGS and its kin) is left
// out by not being here.
var envKeep = []string{
	"HOME", "PATH", "LANG", "LANGUAGE", "TZ", "DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY",
	"XDG_RUNTIME_DIR", "XDG_SESSION_TYPE", "XDG_CURRENT_DESKTOP", "XDG_DATA_DIRS",
	"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "FONTCONFIG_FILE",
}

// Environ filters env down to what the browser may see.
func Environ(env []string) []string {
	var out []string
	for _, kv := range env {
		k, _, ok := strings.Cut(kv, "=")
		if ok && (slices.Contains(envKeep, k) || strings.HasPrefix(k, "LC_")) {
			out = append(out, kv)
		}
	}
	return out
}

// Browser is one running Hands browser.
type Browser struct {
	cmd  *exec.Cmd
	argv []string
	conn *cdp.Conn
	exit chan struct{} // closed once the browser is reaped
	err  error         // its exit, set before exit closes
	once sync.Once
}

type pipes struct {
	r *os.File // the browser's answers (its fd 4)
	w *os.File // our commands (its fd 3)
}

func (p pipes) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p pipes) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p pipes) Close() error                { return errors.Join(p.w.Close(), p.r.Close()) }

// Launch starts the browser with the exact argv from Command, fd 3 reading
// our commands and fd 4 writing its answers, nothing on stdin, stdout or
// stderr, in its own process group.
func Launch(ctx context.Context, o Options) (*Browser, error) { return launch(ctx, o, resolver{}) }

func launch(ctx context.Context, o Options, r resolver) (*Browser, error) {
	argv, err := r.command(o)
	if err != nil {
		return nil, err
	}
	if !Allowed(argv) {
		return nil, ErrOptions
	}
	if err := profileDir(o.ProfileDir); err != nil {
		return nil, err
	}
	cmdR, cmdW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	ansR, ansW, err := os.Pipe()
	if err != nil {
		cmdR.Close()
		cmdW.Close()
		return nil, err
	}
	env := o.Env
	if env == nil {
		env = os.Environ()
	}
	cmd := &exec.Cmd{
		Path:        argv[0],
		Args:        slices.Clone(argv),
		Env:         Environ(env),
		ExtraFiles:  []*os.File{cmdR, ansW},
		SysProcAttr: &syscall.SysProcAttr{Setpgid: true},
	}
	if err := ctx.Err(); err != nil {
		cmdR.Close()
		cmdW.Close()
		ansR.Close()
		ansW.Close()
		return nil, err
	}
	err = cmd.Start()
	cmdR.Close()
	ansW.Close()
	if err != nil {
		cmdW.Close()
		ansR.Close()
		return nil, err
	}
	b := &Browser{cmd: cmd, argv: argv, exit: make(chan struct{})}
	b.conn = cdp.Dial(pipes{r: ansR, w: cmdW})
	go func() {
		b.err = cmd.Wait()
		b.conn.Close()
		close(b.exit)
	}()
	return b, nil
}

func profileDir(p string) error {
	if err := os.MkdirAll(p, 0o700); err != nil {
		return err
	}
	fi, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%w: profile is not a directory", ErrOptions)
	}
	if fi.Mode().Perm() != 0o700 {
		return os.Chmod(p, 0o700)
	}
	return nil
}

// PID is the browser's process id (and its process group's).
func (b *Browser) PID() int { return b.cmd.Process.Pid }

// Argv is what was started.
func (b *Browser) Argv() []string { return slices.Clone(b.argv) }

// Conn is the pipe's protocol connection.
func (b *Browser) Conn() *cdp.Conn { return b.conn }

// Wait delivers the browser's exit when it is reaped; every call gets its
// own channel.
func (b *Browser) Wait() <-chan error {
	ch := make(chan error, 1)
	go func() {
		<-b.exit
		ch <- b.err
	}()
	return ch
}

// CloseGrace is how long Close waits after ending the pipe, and again
// after asking the process group to stop, before killing it.
const CloseGrace = 5 * time.Second

var closeGrace = CloseGrace // tests shorten it

// Close ends the pipe, which a Chromium-family browser takes as its cue to
// quit; a browser still running after CloseGrace has its process group
// stopped, then killed. Helpers left in the group after the browser exits
// are killed with it.
func (b *Browser) Close() error {
	var err error
	b.once.Do(func() {
		b.conn.Close()
		if !b.exited(closeGrace) {
			_ = syscall.Kill(-b.PID(), syscall.SIGTERM)
			if !b.exited(closeGrace) {
				_ = syscall.Kill(-b.PID(), syscall.SIGKILL)
				if !b.exited(closeGrace) {
					err = errors.New("chrome: the browser did not exit")
					return
				}
			}
		}
		_ = syscall.Kill(-b.PID(), syscall.SIGKILL)
	})
	return err
}

func (b *Browser) exited(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-b.exit:
		return true
	case <-t.C:
		return false
	}
}

var _ io.ReadWriteCloser = pipes{}
