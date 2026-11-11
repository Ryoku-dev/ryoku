// Package screen gives an agent session a screen of its own: a hidden nested
// session of the user's compositor, started through the window-manager seam,
// that the agent opens apps on, clicks, types into and looks at, and that the
// user can watch and take over. Nothing on it reaches the user's desktop.
package screen

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/sandbox"
)

// Available says this desktop can host a screen: the window-manager seam's
// provider for the running compositor claims nested sessions.
func Available(ctx context.Context) bool {
	output, err := exec.CommandContext(ctx, "ryoku", "wm", "caps").Output()
	if err != nil {
		return false
	}
	var caps struct {
		Supports []string `json:"supports"`
	}
	return json.Unmarshal(output, &caps) == nil && slices.Contains(caps.Supports, "nestedSession")
}

// The size every screen has: wide enough for a desktop web page, small
// enough that a model reads a whole frame.
const (
	Width  = 1280
	Height = 800
)

// Options places a screen.
type Options struct {
	// Dir becomes the nested session's runtime dir; the screen creates it and
	// removes it when it stops.
	Dir string
	// Home is the HOME of the apps opened on the screen, kept apart from the
	// user's so the agent drives its own logins and profiles.
	Home string
	// Sandbox, set for a team session's screen, is the sandbox every app
	// opened on it runs in; the screen adds its home, display and bus. Nil
	// runs apps as the user.
	Sandbox *sandbox.Spec
}

// Screen is one running screen.
type Screen struct {
	dir, home, display string
	width, height      int
	nest, bus          *exec.Cmd
	nestIn             io.WriteCloser
	pointer            *pointer
	done               chan struct{}
	sandbox            *sandbox.Spec

	mu     sync.Mutex
	apps   []*exec.Cmd
	x, y   int
	closed bool
}

// Start brings a screen up: a private message bus when the system has one to
// run, then the nested session through `ryoku wm nest`, then the pointer.
func Start(ctx context.Context, options Options) (*Screen, error) {
	s := &Screen{dir: options.Dir, home: options.Home, width: Width, height: Height, done: make(chan struct{}), sandbox: options.Sandbox}
	_ = os.RemoveAll(s.dir)
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.home, 0o700); err != nil {
		return nil, err
	}
	if path, err := exec.LookPath("dbus-daemon"); err == nil {
		s.bus = exec.Command(path, "--session", "--nofork", "--nopidfile", "--address=unix:path="+filepath.Join(s.dir, "bus"))
		s.bus.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := s.bus.Start(); err != nil {
			s.bus = nil
		} else if !waitSocket(ctx, filepath.Join(s.dir, "bus")) {
			s.Stop()
			return nil, errors.New("the screen's message bus did not start")
		}
	}
	s.nest = exec.Command("ryoku", "wm", "nest", s.dir, fmt.Sprintf("%dx%d", s.width, s.height))
	s.nest.Env = s.withBus(os.Environ())
	s.nest.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stderr bytes.Buffer
	s.nest.Stderr = &stderr
	in, err := s.nest.StdinPipe()
	if err != nil {
		s.Stop()
		return nil, err
	}
	s.nestIn = in
	out, err := s.nest.StdoutPipe()
	if err != nil {
		s.Stop()
		return nil, err
	}
	if err := s.nest.Start(); err != nil {
		s.Stop()
		return nil, err
	}
	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(out).ReadString('\n')
		ready <- line
		_ = s.nest.Wait()
		close(s.done)
	}()
	select {
	case line := <-ready:
		var answer struct {
			Display string `json:"display"`
		}
		if json.Unmarshal([]byte(line), &answer) != nil || answer.Display == "" {
			s.Stop()
			reason := strings.TrimSpace(stderr.String())
			if reason == "" {
				reason = "the nested session did not start"
			}
			return nil, errors.New(reason)
		}
		s.display = answer.Display
	case <-ctx.Done():
		s.Stop()
		return nil, ctx.Err()
	}
	if s.pointer, err = dialPointer(filepath.Join(s.dir, s.display)); err != nil {
		s.Stop()
		return nil, err
	}
	return s, nil
}

func waitSocket(ctx context.Context, path string) bool {
	for range 100 {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
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

// Size is the screen's size in pixels.
func (s *Screen) Size() (int, int) { return s.width, s.height }

// Done is closed once the nested session has ended.
func (s *Screen) Done() <-chan struct{} { return s.done }

// Capture reads one frame as PNG.
func (s *Screen) Capture(ctx context.Context) ([]byte, error) {
	command := exec.CommandContext(ctx, "grim", "-t", "png", "-l", "3", "-")
	command.Env = s.toolEnv()
	var stderr bytes.Buffer
	command.Stderr = &stderr
	frame, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("capture the screen: %v %s", err, strings.TrimSpace(stderr.String()))
	}
	return frame, nil
}

// Settled captures until two frames a moment apart match, so a click's
// result is on the frame rather than half drawn; a screen that keeps moving
// gives its latest frame after two seconds.
func (s *Screen) Settled(ctx context.Context) ([]byte, error) {
	deadline := time.Now().Add(2 * time.Second)
	time.Sleep(150 * time.Millisecond)
	previous, err := s.Capture(ctx)
	if err != nil {
		return nil, err
	}
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return previous, nil
		case <-time.After(250 * time.Millisecond):
		}
		frame, err := s.Capture(ctx)
		if err != nil {
			return previous, nil
		}
		if bytes.Equal(frame, previous) {
			return frame, nil
		}
		previous = frame
	}
	return previous, nil
}

func (s *Screen) inside(x, y int) error {
	if x < 0 || y < 0 || x >= s.width || y >= s.height {
		return fmt.Errorf("%d,%d is off the %dx%d screen", x, y, s.width, s.height)
	}
	return nil
}

// Move puts the pointer at x,y.
func (s *Screen) Move(x, y int) error {
	if err := s.inside(x, y); err != nil {
		return err
	}
	s.mu.Lock()
	s.x, s.y = x, y
	s.mu.Unlock()
	return s.pointer.move(x, y, s.width, s.height)
}

// Click presses a button count times at x,y: left, right or middle.
func (s *Screen) Click(x, y int, button string, count int) error {
	code := uint32(buttonLeft)
	switch button {
	case "", "left":
	case "right":
		code = buttonRight
	case "middle":
		code = buttonMiddle
	default:
		return fmt.Errorf("%q is not a mouse button", button)
	}
	if err := s.Move(x, y); err != nil {
		return err
	}
	for range max(1, min(count, 3)) {
		if err := s.pointer.press(code, true); err != nil {
			return err
		}
		if err := s.pointer.press(code, false); err != nil {
			return err
		}
		time.Sleep(40 * time.Millisecond)
	}
	return nil
}

// Press holds or lets go of the left button where the pointer is, for a
// person dragging on the live view.
func (s *Screen) Press(down bool) error { return s.pointer.press(buttonLeft, down) }

// Drag holds the left button from x,y to toX,toY, moving in steps so the app
// sees a drag and not a jump.
func (s *Screen) Drag(x, y, toX, toY int) error {
	if err := s.inside(toX, toY); err != nil {
		return err
	}
	if err := s.Move(x, y); err != nil {
		return err
	}
	if err := s.pointer.press(buttonLeft, true); err != nil {
		return err
	}
	const steps = 12
	for step := 1; step <= steps; step++ {
		if err := s.Move(x+(toX-x)*step/steps, y+(toY-y)*step/steps); err != nil {
			return err
		}
		time.Sleep(15 * time.Millisecond)
	}
	return s.pointer.press(buttonLeft, false)
}

// Scroll turns the wheel at x,y by notches, down and right positive.
func (s *Screen) Scroll(x, y, dx, dy int) error {
	if err := s.Move(x, y); err != nil {
		return err
	}
	return s.pointer.scroll(dx, dy)
}

// Type types text into whatever has the keyboard.
func (s *Screen) Type(ctx context.Context, text string) error {
	if text == "" {
		return nil
	}
	return s.wtype(ctx, "--", text)
}

// Key sends one chord, such as ctrl+l or Return.
func (s *Screen) Key(ctx context.Context, chord string) error {
	args, err := keyArgs(chord)
	if err != nil {
		return err
	}
	return s.wtype(ctx, args...)
}

func (s *Screen) wtype(ctx context.Context, args ...string) error {
	command := exec.CommandContext(ctx, "wtype", args...)
	command.Env = s.toolEnv()
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("type on the screen: %v %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Open runs a command line on the screen, as the user would from a launcher,
// with the screen's own HOME, in the screen's sandbox when it has one.
func (s *Screen) Open(command string) error {
	argv, err := s.appCommand(command)
	if err != nil {
		return err
	}
	app := exec.Command(argv[0], argv[1:]...)
	app.Env = s.appEnv()
	app.Dir = s.home
	app.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := app.Start(); err != nil {
		return err
	}
	s.mu.Lock()
	s.apps = append(s.apps, app)
	s.mu.Unlock()
	go func() { _ = app.Wait() }()
	return nil
}

// appCommand is how a command line runs on the screen: through sh, and in a
// sandbox that sees the screen's home, its runtime dir with the display and
// bus, and the GPU, under the app environment.
func (s *Screen) appCommand(command string) ([]string, error) {
	argv := []string{"sh", "-c", command}
	if s.sandbox == nil {
		return argv, nil
	}
	if err := sandbox.Available(); err != nil {
		return nil, err
	}
	spec := *s.sandbox
	dir := paths.Resolved(s.dir)
	spec.Folder = paths.Resolved(s.home)
	spec.Binds = append(slices.Clone(spec.Binds), sandbox.Bind{Source: dir, Target: dir})
	spec.Devices = true
	spec.Env = make(map[string]string)
	for _, entry := range s.appEnv() {
		name, value, _ := strings.Cut(entry, "=")
		spec.Env[name] = value
	}
	if s.bus == nil {
		spec.Unset = append(slices.Clone(spec.Unset), "DBUS_SESSION_BUS_ADDRESS")
	}
	return sandbox.Command(spec, argv)
}

// Stop ends the apps, the nested session and the bus, and removes the
// session's dir.
func (s *Screen) Stop() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	apps := s.apps
	s.mu.Unlock()
	for _, app := range apps {
		if app.Process != nil {
			_ = syscall.Kill(-app.Process.Pid, syscall.SIGTERM)
		}
	}
	if s.pointer != nil {
		s.pointer.close()
	}
	if s.nestIn != nil {
		_ = s.nestIn.Close()
	}
	if s.nest != nil && s.nest.Process != nil {
		select {
		case <-s.done:
		case <-time.After(4 * time.Second):
			_ = syscall.Kill(-s.nest.Process.Pid, syscall.SIGKILL)
		}
	}
	if s.bus != nil && s.bus.Process != nil {
		_ = syscall.Kill(-s.bus.Process.Pid, syscall.SIGTERM)
		_ = s.bus.Wait()
	}
	for _, app := range apps {
		if app.Process != nil {
			_ = syscall.Kill(-app.Process.Pid, syscall.SIGKILL)
		}
	}
	_ = os.RemoveAll(s.dir)
}

func (s *Screen) withBus(env []string) []string {
	if s.bus == nil {
		return env
	}
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "DBUS_SESSION_BUS_ADDRESS=") {
			out = append(out, entry)
		}
	}
	return append(out, "DBUS_SESSION_BUS_ADDRESS=unix:path="+filepath.Join(s.dir, "bus"))
}

// toolEnv reaches the screen and nothing else, for grim and wtype
func (s *Screen) toolEnv() []string {
	return []string{"PATH=" + os.Getenv("PATH"), "XDG_RUNTIME_DIR=" + s.dir, "WAYLAND_DISPLAY=" + s.display}
}

// appEnv is what an app on the screen runs with: the user's language and
// PATH, the screen's display, bus and HOME, and every toolkit told to use
// Wayland, since the screen has no X server
func (s *Screen) appEnv() []string {
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		switch {
		case name == "PATH", name == "LANG", name == "LANGUAGE", name == "TZ", name == "USER", name == "LOGNAME", name == "SHELL", strings.HasPrefix(name, "LC_"):
			env = append(env, entry)
		}
	}
	env = append(env,
		"HOME="+s.home, "XDG_RUNTIME_DIR="+s.dir, "WAYLAND_DISPLAY="+s.display, "XDG_SESSION_TYPE=wayland",
		"MOZ_ENABLE_WAYLAND=1", "QT_QPA_PLATFORM=wayland", "GDK_BACKEND=wayland", "SDL_VIDEODRIVER=wayland",
		"ELECTRON_OZONE_PLATFORM_HINT=wayland",
	)
	return s.withBus(env)
}
