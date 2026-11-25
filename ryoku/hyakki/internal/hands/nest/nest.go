// Package nest runs the separate desktop Hands acts on: niri inside a
// headless weston, owned by Hyakki, on a private runtime dir, under a niri
// config Hyakki writes. Anything a task does that needs no live window
// happens here.
//
// Nothing in it reaches the user's session. Every child gets an environment
// built from scratch, never the daemon's, so the user's display, compositor
// sockets and message bus are never named; the runtime dir holds no bus, so
// the default bus path leads nowhere. The config has no key binds and starts
// nothing, so nothing on the desktop can open a program but Open. A missing
// program is reported by Available and Start refuses; nothing falls back to
// the live session.
//
// The desktop lives and dies with the daemon: its processes get SIGKILL when
// the daemon dies, Stop ends every process that carries its runtime dir and
// removes the dir, and Sweep clears what a killed daemon left behind.
package nest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The size a desktop has when Options leaves it out, and the bounds a size
// must keep.
const (
	DefaultWidth  = 1280
	DefaultHeight = 800
	minWidth      = 320
	minHeight     = 240
	maxWidth      = 7680
	maxHeight     = 4320
)

// The wait for the desktop to come up and the wait between asking its
// processes to end and killing them, when Options leaves them out.
const (
	DefaultReady     = 20 * time.Second
	DefaultStopGrace = 3 * time.Second
)

const (
	// parentSocket is weston's display, which only niri connects to.
	parentSocket = "wayland-parent"
	configName   = "niri.kdl"
	// marker names a dir as a desktop's, so Start and Sweep never empty a
	// dir that is not one.
	marker = ".hyakki-nest"
	// maxDir keeps niri's IPC socket path, niri.<display>.<pid>.sock, inside
	// the 107 bytes a Unix socket path may have.
	maxDir = 72
)

// config is niri's whole config: one output filled by the focused window,
// no animation, no decoration, no hot corner and no key bind, and nothing
// started with it.
// Private windows float so niri reports the positions needed to check clicks
const config = `input {
    keyboard {
        xkb {
            layout "us"
        }
    }
}
output "winit" {
    scale 1.0
}
layout {
    gaps 0
    focus-ring { off; }
    border { off; }
}
window-rule {
    open-floating true
    default-column-width {
        proportion 1.0
    }
    default-window-height {
        proportion 1.0
    }
    default-floating-position x=0 y=0 relative-to="top-left"
}
gestures {
    hot-corners {
        off
    }
}
prefer-no-csd
animations {
    off
}
hotkey-overlay {
    skip-at-startup
}
xwayland-satellite {
    off
}
`

// tools are the programs the desktop needs: the two that run it and the one
// that captures it.
var tools = []string{"weston", "niri", "grim"}

// Reason says whether the separate desktop can run here, and if not, which
// programs are missing.
type Reason struct {
	Missing []string
}

// OK is true when every program the desktop needs is installed.
func (r Reason) OK() bool { return len(r.Missing) == 0 }

// String is a sentence for Settings, empty when the desktop can run.
func (r Reason) String() string {
	switch n := len(r.Missing); n {
	case 0:
		return ""
	case 1:
		return r.Missing[0] + " is not installed"
	default:
		return strings.Join(r.Missing[:n-1], ", ") + " and " + r.Missing[n-1] + " are not installed"
	}
}

// Available looks the desktop's programs up on PATH. It starts nothing and
// never looks at the live session.
func Available() Reason {
	var r Reason
	for _, name := range tools {
		if _, err := exec.LookPath(name); err != nil {
			r.Missing = append(r.Missing, name)
		}
	}
	return r
}

// Options places a desktop.
type Options struct {
	// Dir becomes the desktop's runtime dir: an absolute path of at most 72
	// bytes, such as DefaultDir, whose parent exists. Start creates it and
	// Stop removes it.
	Dir string
	// Home is the HOME of the desktop's programs, kept apart from the user's.
	Home string
	// Width and Height are the output's size; zero takes the default.
	Width, Height int
	// Ready bounds the start, and StopGrace the wait between asking the
	// desktop's processes to end and killing them; zero takes the default.
	Ready, StopGrace time.Duration
}

// DefaultDir is a runtime dir in shared memory that holds this user's id.
func DefaultDir() string { return fmt.Sprintf("/dev/shm/hyakki-nest-%d", os.Getuid()) }

// Workspace is one running desktop.
type Workspace struct {
	dir, home, display, ipc string
	width, height           int
	grace                   time.Duration
	grim                    string
	weston, niri            *proc

	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
	err      error

	mu     sync.Mutex
	apps   []*proc
	closed bool
}

// Start brings a desktop up: weston on its own socket, then niri on weston,
// and returns once niri has its display and IPC sockets. A start that fails
// or does not finish within Ready leaves no process and no dir.
func Start(ctx context.Context, o Options) (*Workspace, error) {
	width, height, err := size(o.Width, o.Height)
	if err != nil {
		return nil, err
	}
	dir, err := checkDir(o.Dir)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(o.Home) {
		return nil, errors.New("nest: the home must be an absolute path")
	}
	home := filepath.Clean(o.Home)
	if home == dir || strings.HasPrefix(home, dir+string(filepath.Separator)) {
		return nil, errors.New("nest: the home must lie outside the runtime dir")
	}
	ready, grace := o.Ready, o.StopGrace
	if ready <= 0 {
		ready = DefaultReady
	}
	if grace <= 0 {
		grace = DefaultStopGrace
	}
	if r := Available(); !r.OK() {
		return nil, errors.New("nest: the separate desktop is unavailable: " + r.String())
	}
	westonPath, _ := exec.LookPath("weston")
	niriPath, _ := exec.LookPath("niri")
	grimPath, _ := exec.LookPath("grim")
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := reset(dir); err != nil {
		return nil, err
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, fmt.Errorf("nest: make the runtime dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, marker), nil, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, configName), []byte(config), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	w := &Workspace{
		dir: dir, home: home, width: width, height: height, grace: grace, grim: grimPath,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
	deadline := time.Now().Add(ready)
	w.weston, err = spawn(westonPath, westonArgs(width, height), w.baseEnv(), dir, true)
	if err != nil {
		w.teardown()
		return nil, fmt.Errorf("nest: start weston: %w", err)
	}
	if err := w.await(ctx, deadline, ready, func() bool { return isSocket(filepath.Join(dir, parentSocket)) }); err != nil {
		w.teardown()
		return nil, err
	}
	w.niri, err = spawn(niriPath, []string{"niri", "-c", filepath.Join(dir, configName)},
		append(w.baseEnv(), "WAYLAND_DISPLAY="+parentSocket), dir, true)
	if err != nil {
		w.teardown()
		return nil, fmt.Errorf("nest: start niri: %w", err)
	}
	if err := w.await(ctx, deadline, ready, w.found); err != nil {
		w.teardown()
		return nil, err
	}
	go w.supervise()
	return w, nil
}

func westonArgs(width, height int) []string {
	return []string{"weston", "--backend=headless", "--shell=kiosk", "--idle-time=0", "--no-config",
		"--width=" + strconv.Itoa(width), "--height=" + strconv.Itoa(height), "--socket=" + parentSocket}
}

func size(width, height int) (int, int, error) {
	if width == 0 && height == 0 {
		return DefaultWidth, DefaultHeight, nil
	}
	if width < minWidth || height < minHeight || width > maxWidth || height > maxHeight {
		return 0, 0, fmt.Errorf("nest: size %dx%d is outside %dx%d to %dx%d", width, height, minWidth, minHeight, maxWidth, maxHeight)
	}
	return width, height, nil
}

// checkDir takes an absolute, short runtime dir that is not the daemon's
// own runtime dir or a filesystem root.
func checkDir(dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		return "", errors.New("nest: the runtime dir must be an absolute path")
	}
	dir = filepath.Clean(dir)
	if len(dir) > maxDir {
		return "", fmt.Errorf("nest: the runtime dir is longer than %d bytes", maxDir)
	}
	if dir == filepath.Dir(dir) {
		return "", errors.New("nest: the runtime dir cannot be a root")
	}
	if host := os.Getenv("XDG_RUNTIME_DIR"); host != "" && filepath.Clean(host) == dir {
		return "", errors.New("nest: the runtime dir cannot be the session's own")
	}
	return dir, nil
}

// reset ends what an earlier desktop left in dir and removes it. A dir that
// is this user's, a real dir and either empty or marked as a desktop's is
// cleared; anything else is refused untouched.
func reset(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		sweep(dir)
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("nest: %s is not a directory", dir)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("nest: %s belongs to someone else", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		if _, err := os.Lstat(filepath.Join(dir, marker)); err != nil {
			return fmt.Errorf("nest: %s is not a separate desktop's dir", dir)
		}
	}
	sweep(dir)
	return os.RemoveAll(dir)
}

// Sweep ends every process a desktop on dir left behind, as after the daemon
// was killed, and removes the dir. It refuses a dir that is not a desktop's.
func Sweep(dir string) error {
	dir, err := checkDir(dir)
	if err != nil {
		return err
	}
	return reset(dir)
}

// baseEnv is what weston and niri run with: PATH, the desktop's HOME and
// runtime dir, and the language and renderer choices, nothing else.
func (w *Workspace) baseEnv() []string {
	env := []string{"PATH=" + searchPath(), "HOME=" + w.home, "XDG_RUNTIME_DIR=" + w.dir}
	return append(env, pick("LANG", "LC_ALL", "LIBGL_ALWAYS_SOFTWARE", "GALLIUM_DRIVER", "MESA_LOADER_DRIVER_OVERRIDE")...)
}

func searchPath() string {
	if p := os.Getenv("PATH"); p != "" {
		return p
	}
	return "/usr/bin:/bin"
}

func pick(names ...string) []string {
	var env []string
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

// found reads niri's display from its IPC socket's name,
// niri.<display>.<pid>.sock, once both sockets exist.
func (w *Workspace) found() bool {
	matches, _ := filepath.Glob(filepath.Join(w.dir, "niri.*.sock"))
	suffix := "." + strconv.Itoa(w.niri.cmd.Process.Pid) + ".sock"
	for _, socket := range matches {
		name := filepath.Base(socket)
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		display := strings.TrimSuffix(strings.TrimPrefix(name, "niri."), suffix)
		if display == "" || display == parentSocket || strings.ContainsAny(display, "/.") {
			continue
		}
		if isSocket(socket) && isSocket(filepath.Join(w.dir, display)) {
			w.display, w.ipc = display, socket
			return true
		}
	}
	return false
}

// await polls ready until it holds, the deadline passes, ctx ends or weston
// or niri exits.
func (w *Workspace) await(ctx context.Context, deadline time.Time, limit time.Duration, ready func() bool) error {
	for {
		if ready() {
			return nil
		}
		for _, p := range []*proc{w.weston, w.niri} {
			if p != nil && p.ended() {
				return p.exitError()
			}
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("nest: the separate desktop did not come up in %s", limit)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// supervise ends the desktop on Stop or when weston or niri exits.
func (w *Workspace) supervise() {
	var err error
	select {
	case <-w.stop:
	case <-w.weston.exited:
		err = w.weston.exitError()
	case <-w.niri.exited:
		err = w.niri.exitError()
	}
	w.teardown()
	w.err = err
	close(w.done)
}

// teardown asks every process to end, kills what is left after the grace,
// then every process still holding the runtime dir, and removes the dir.
func (w *Workspace) teardown() {
	w.mu.Lock()
	w.closed = true
	procs := slices.Clone(w.apps)
	w.mu.Unlock()
	for _, p := range []*proc{w.niri, w.weston} {
		if p != nil {
			procs = append(procs, p)
		}
	}
	for _, p := range procs {
		p.signal(syscall.SIGTERM)
	}
	if !waitAll(procs, w.grace) {
		for _, p := range procs {
			p.signal(syscall.SIGKILL)
		}
	}
	sweep(w.dir)
	waitAll(procs, w.grace)
	_ = os.RemoveAll(w.dir)
}

// Env reaches the desktop and nothing else: PATH, its runtime dir, its
// display and niri's IPC socket. It is for programs that talk to the
// desktop, such as a window list or a capture.
func (w *Workspace) Env() []string {
	return []string{"PATH=" + searchPath(), "XDG_RUNTIME_DIR=" + w.dir, "WAYLAND_DISPLAY=" + w.display, "NIRI_SOCKET=" + w.ipc}
}

// Dir is the desktop's runtime dir.
func (w *Workspace) Dir() string { return w.dir }

// Display is the bare name of niri's Wayland socket in Dir.
func (w *Workspace) Display() string { return w.display }

// Socket is the path of niri's Wayland socket.
func (w *Workspace) Socket() string { return filepath.Join(w.dir, w.display) }

// Size is the output's size in pixels.
func (w *Workspace) Size() (int, int) { return w.width, w.height }

// Done is closed once the desktop has ended and been cleared.
func (w *Workspace) Done() <-chan struct{} { return w.done }

// Wait blocks until the desktop has ended. It returns nil after Stop, and
// says which compositor exited when one ended by itself.
func (w *Workspace) Wait() error {
	<-w.done
	return w.err
}

// Stop ends the desktop's programs, niri and weston, and removes the
// runtime dir. It returns once nothing is left and is safe to call again.
func (w *Workspace) Stop() {
	w.stopOnce.Do(func() { close(w.stop) })
	<-w.done
}

func (w *Workspace) ended() bool {
	select {
	case <-w.done:
		return true
	default:
		return false
	}
}

// Capture reads the desktop's output as one PNG, through grim on the
// desktop's display only.
func (w *Workspace) Capture(ctx context.Context) ([]byte, error) {
	if w.ended() {
		return nil, errors.New("nest: the separate desktop has ended")
	}
	command := exec.CommandContext(ctx, w.grim, "-t", "png", "-l", "3", "-")
	command.Args[0] = "grim"
	command.Env = w.Env()
	command.Dir = w.dir
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	var stdout bytes.Buffer
	stderr := &tail{}
	command.Stdout, command.Stderr = &stdout, stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("nest: capture the separate desktop: %v%s", err, stderr.last())
	}
	frame := stdout.Bytes()
	if !bytes.HasPrefix(frame, []byte("\x89PNG\r\n\x1a\n")) {
		return nil, errors.New("nest: the capture is not a PNG")
	}
	return frame, nil
}

// noShell are programs that would run a command line or another program
// chosen by their arguments; Open never starts one.
var noShell = []string{
	"sh", "bash", "dash", "zsh", "fish", "ksh", "mksh", "oksh", "csh", "tcsh", "nu", "elvish", "xonsh",
	"busybox", "toybox", "env", "sudo", "doas", "pkexec", "su", "run0", "setsid", "nohup", "xargs",
}

// Open starts one program on the desktop with exactly argv: no shell, the
// desktop's HOME as its HOME and working dir, and the desktop's display.
// argv[0] is an absolute path or a name on PATH; a shell, a program-runner
// or anything inside the desktop's home or runtime dir is refused.
func (w *Workspace) Open(argv []string) error {
	if len(argv) == 0 || argv[0] == "" {
		return errors.New("nest: open needs a program")
	}
	for _, arg := range argv {
		if strings.IndexByte(arg, 0) >= 0 {
			return errors.New("nest: an argument holds a NUL byte")
		}
	}
	name := argv[0]
	if strings.ContainsRune(name, '/') && !filepath.IsAbs(name) {
		return errors.New("nest: the program must be an absolute path or a name on PATH")
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("nest: %s is not an installed program", filepath.Base(name))
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if slices.Contains(noShell, filepath.Base(path)) || slices.Contains(noShell, filepath.Base(resolved)) {
		return fmt.Errorf("nest: %s runs other programs and is never opened", filepath.Base(path))
	}
	for _, inside := range []string{w.home, w.dir} {
		root, err := filepath.EvalSymlinks(inside)
		if err != nil {
			root = inside
		}
		if strings.HasPrefix(resolved, root+string(filepath.Separator)) {
			return errors.New("nest: a program inside the separate desktop's own files is never run")
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return errors.New("nest: the separate desktop has ended")
	}
	p, err := spawn(path, slices.Clone(argv), w.appEnv(), w.home, false)
	if err != nil {
		return fmt.Errorf("nest: open %s: %w", filepath.Base(path), err)
	}
	w.apps = slices.DeleteFunc(w.apps, (*proc).ended)
	w.apps = append(w.apps, p)
	return nil
}

// appEnv is what a program on the desktop runs with: the user's PATH,
// language and time zone, the desktop's HOME, runtime dir and display, and
// every toolkit told to use Wayland, since the desktop has no X server.
func (w *Workspace) appEnv() []string {
	env := []string{"PATH=" + searchPath()}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch {
		case name == "LANG", name == "LANGUAGE", name == "TZ", name == "USER", name == "LOGNAME", strings.HasPrefix(name, "LC_"):
			env = append(env, entry)
		}
	}
	env = append(env, pick("LIBGL_ALWAYS_SOFTWARE", "GALLIUM_DRIVER", "MESA_LOADER_DRIVER_OVERRIDE")...)
	return append(env,
		"HOME="+w.home, "XDG_RUNTIME_DIR="+w.dir, "WAYLAND_DISPLAY="+w.display, "XDG_SESSION_TYPE=wayland",
		"MOZ_ENABLE_WAYLAND=1", "QT_QPA_PLATFORM=wayland", "GDK_BACKEND=wayland", "SDL_VIDEODRIVER=wayland",
		"ELECTRON_OZONE_PLATFORM_HINT=wayland",
	)
}
