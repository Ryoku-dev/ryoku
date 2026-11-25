package daemon

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/hands/chrome"
	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/hands/nest"
	"ryoku-rashin/internal/hands/page"
	"ryoku-rashin/internal/oni/hands/task"
	"ryoku-rashin/internal/oni/keys"
	"ryoku-rashin/internal/oni/presence"
	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/reflex/browser"
	"ryoku-rashin/internal/reflex/launch"
)

// OniHandsDeps are what Hands drives the Hands browser, the picked window
// and the nested desktop through; each nil field is the real one, built on
// first use and never while Hands is off. The windows Hands watches are
// Deps.OniWindows, the perch's, and pictures of a live window come from
// Deps.OniCapture.
type OniHandsDeps struct {
	// Find names the Chromium-family browser the Hands browser is started
	// from; nil reads the desktop entries. A browser Hands cannot drive
	// (Firefox, a Flatpak or snap build) is an error whose text says so.
	Find func() (OniHandsProgram, error)
	// Launch starts the Hands browser with exactly o and drives it over
	// its pipe; nil is chrome.Launch with page.Open on its connection.
	Launch func(ctx context.Context, o chrome.Options) (OniHandsBrowser, error)
	// Stop reads the stop key (a bare Esc, never grabbed) while a task
	// runs and calls onStop on each press; nil is keys.StartStop, reading
	// the lab's own FIFO (HYAKKI_ONI_STOP_FIFO) only under HYAKKI_ONI_LAB=1.
	Stop func(onStop func()) (stop func(), err error)
	// AnyKey reads every key edge (never which key) while a task works in
	// a live window, calling onKey on each; nil is keys.StartAnyKey,
	// reading the lab's own FIFO (HYAKKI_ONI_ANYKEY_FIFO) only under
	// HYAKKI_ONI_LAB=1.
	AnyKey func(onKey func()) (stop func(), err error)
	// Nest starts the separate nested desktop; nil is nest.Start.
	Nest func(ctx context.Context, o nest.Options) (OniHandsNest, error)
	// NestMissing names the programs the nested desktop needs and this
	// computer lacks; nil is nest.Available.
	NestMissing func() []string
	// NestWindows reads the nested desktop's windows with env, its own
	// environment; nil is window.Niri.
	NestWindows func(env []string) window.Provider
	// Keyboard runs the keyboard tool with env; nil is input.Exec.
	Keyboard func(env []string) input.Runner
	// Pointer dials the virtual pointer on a socket; nil is
	// input.DialPointer.
	Pointer func(socket string) (input.Pointer, error)
	// Journal is the task journal's file; empty is task.DefaultPath().
	Journal string
	// Roots are the folders find_files searches; nil is the person's
	// Documents, Downloads and Desktop that exist.
	Roots []string
	// AppDirs are the applications directories the nested desktop opens
	// apps and the file manager from; nil is launch.Dirs.
	AppDirs []string
	// Clock times the task's waits; nil is the wall clock.
	Clock presence.Clock
}

// OniHandsNest is one running nested desktop: a *nest.Workspace.
type OniHandsNest interface {
	Env() []string
	Dir() string
	Display() string
	Size() (int, int)
	Done() <-chan struct{}
	Capture(ctx context.Context) ([]byte, error)
	Open(argv []string) error
	Stop()
}

// OniHandsProgram is the browser the Hands browser starts from: Name its
// short code name, Exec the one program its desktop entry runs.
type OniHandsProgram struct {
	Name string
	Exec []string
}

// OniHandsBrowser is one running Hands browser.
type OniHandsBrowser interface {
	// Driver drives its tabs.
	Driver() page.Driver
	// PID is its process id, the one its window reports.
	PID() int
	// Done closes once the browser has exited.
	Done() <-chan struct{}
	// Close ends it.
	Close() error
}

// The honest lines for a browser Hands cannot drive.
var (
	errHandsSandboxed = errors.New(i18n.T("Hands can't drive a Flatpak or snap browser. Install Chromium or Chrome from your distribution."))
	errHandsNoBrowser = errors.New(i18n.T("Hands needs a Chromium-family browser, such as Chromium or Chrome. Firefox can't be driven."))
)

// handsProfile is the Hands browser's own profile directory.
func handsProfile() string { return filepath.Join(paths.StateDir(), chrome.ProfileName) }

func (d *Daemon) handsFind() (OniHandsProgram, error) {
	if f := d.deps.OniHands.Find; f != nil {
		return f()
	}
	return findHandsProgram(reflexEnv(os.Environ()))
}

// findHandsProgram takes the first desktop browser, the default one first,
// whose program is a Chromium-family browser chrome can start as is.
func findHandsProgram(env []string) (OniHandsProgram, error) {
	desktops := strings.Split(lookupEnv(env, "XDG_CURRENT_DESKTOP"), ":")
	cat := launch.Scan(launch.ScanOptions{Dirs: launch.Dirs(env), Max: reflexLimits().CatalogMax, Desktops: desktops, LookPath: exec.LookPath})
	var entries []launch.Entry
	if e, ok := browser.Default(cat, browser.Mimeapps(env)); ok {
		entries = append(entries, e)
	}
	for _, e := range cat.Entries() {
		if e.Browser() {
			entries = append(entries, e)
		}
	}
	sandboxed := false
	for _, e := range entries {
		argv := e.Argv()
		if len(argv) == 0 {
			continue
		}
		o := chrome.Options{Exec: argv[:1], ProfileDir: handsProfile(), Class: "hyakki-hands-probe", StartURL: "about:blank", Env: env}
		if _, err := chrome.Command(o); err != nil {
			sandboxed = sandboxed || errors.Is(err, chrome.ErrSandboxed)
			continue
		}
		if name := handsProgramName(argv[0]); name != "" {
			return OniHandsProgram{Name: name, Exec: argv[:1]}, nil
		}
	}
	if sandboxed {
		return OniHandsProgram{}, errHandsSandboxed
	}
	return OniHandsProgram{}, errHandsNoBrowser
}

// handsProgramName is the code name of program: its file name, lowercase,
// kept to what the wire allows.
func handsProgramName(program string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(filepath.Base(program)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	name := b.String()
	if len(name) > 32 {
		name = name[:32]
	}
	if !protocol.ValidOniHandsBrowser(name) {
		return ""
	}
	return name
}

func (d *Daemon) handsLaunch(ctx context.Context, o chrome.Options) (OniHandsBrowser, error) {
	if f := d.deps.OniHands.Launch; f != nil {
		return f(ctx, o)
	}
	b, err := chrome.Launch(ctx, o)
	if err != nil {
		return nil, err
	}
	s, err := page.Open(ctx, b.Conn(), handsPageLimits())
	if err != nil {
		_ = b.Close()
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		<-b.Wait()
		close(done)
	}()
	return &chromeHands{b: b, s: s, done: done}, nil
}

// chromeHands is the real Hands browser.
type chromeHands struct {
	b    *chrome.Browser
	s    *page.Session
	done chan struct{}
}

func (c *chromeHands) Driver() page.Driver   { return c.s }
func (c *chromeHands) PID() int              { return c.b.PID() }
func (c *chromeHands) Done() <-chan struct{} { return c.done }
func (c *chromeHands) Close() error          { return errors.Join(c.s.Close(), c.b.Close()) }

// handsPageLimits bound one reading and one step, from the spec.
func handsPageLimits() page.Limits {
	s, err := oniGuideSpec()
	if err != nil {
		return page.Limits{}
	}
	h := s.Hands
	return page.Limits{
		MaxControls: h.Page.MaxControls, MaxNameRunes: h.Page.MaxNameRunes,
		MaxTextRunes: h.Page.MaxTextRunes, MaxFillRunes: h.Page.MaxFillRunes,
		Act: h.Act() + h.Readback(),
	}
}

// handsStopKeys starts the stop key reader for onStop.
func (d *Daemon) handsStopKeys(onStop func()) (func(), error) {
	if f := d.deps.OniHands.Stop; f != nil {
		return f(onStop)
	}
	opts := keys.Options{Getenv: os.Getenv}
	if os.Getenv("HYAKKI_ONI_LAB") == "1" {
		opts.FIFO = os.Getenv("HYAKKI_ONI_STOP_FIFO")
	}
	s, err := keys.StartStop(opts, onStop)
	if err != nil {
		return nil, err
	}
	return s.Stop, nil
}

func (d *Daemon) handsClock() presence.Clock {
	if c := d.deps.OniHands.Clock; c != nil {
		return c
	}
	return presence.SystemClock()
}

// handsAnyKey starts the key-edge reader for onKey.
func (d *Daemon) handsAnyKey(onKey func()) (func(), error) {
	if f := d.deps.OniHands.AnyKey; f != nil {
		return f(onKey)
	}
	s, err := keys.StartAnyKey(keys.AnyKeyOptions(os.Getenv), onKey)
	if err != nil {
		return nil, err
	}
	return s.Stop, nil
}

// handsNestStart starts the nested desktop.
func (d *Daemon) handsNestStart(ctx context.Context, o nest.Options) (OniHandsNest, error) {
	if f := d.deps.OniHands.Nest; f != nil {
		return f(ctx, o)
	}
	return nest.Start(ctx, o)
}

// handsNestMissing names what the nested desktop lacks; nothing is
// started to find out.
func (d *Daemon) handsNestMissing() []string {
	if f := d.deps.OniHands.NestMissing; f != nil {
		return f()
	}
	return nest.Available().Missing
}

// handsNestWindows reads the nested desktop's windows with its env alone.
func (d *Daemon) handsNestWindows(env []string) window.Provider {
	if f := d.deps.OniHands.NestWindows; f != nil {
		return f(env)
	}
	return window.Niri(env)
}

// handsNestDir is the nested desktop's runtime dir: nest.DefaultDir, or in
// the Oni lab only the absolute dir HYAKKI_ONI_HANDS_NEST_DIR names, so a
// lab never touches the real user's.
func handsNestDir(getenv func(string) string) string {
	if getenv("HYAKKI_ONI_LAB") == "1" {
		if dir := getenv("HYAKKI_ONI_HANDS_NEST_DIR"); filepath.IsAbs(dir) && len(dir) <= 72 {
			return filepath.Clean(dir)
		}
	}
	return nest.DefaultDir()
}

// handsNestHome is the HOME of the nested desktop's programs, apart from
// the person's own.
func handsNestHome() string { return filepath.Join(paths.StateDir(), "oni-hands-desktop") }

// handsJournalPath is the task journal's file.
func (d *Daemon) handsJournalPath() string {
	if p := d.deps.OniHands.Journal; p != "" {
		return p
	}
	return task.DefaultPath()
}

// handsRoots are the folders find_files searches: the person's Documents,
// Downloads and Desktop that exist, never a whole home.
func (d *Daemon) handsRoots() []string {
	if r := d.deps.OniHands.Roots; r != nil {
		return slices.Clone(r)
	}
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return nil
	}
	var roots []string
	for _, name := range []string{"Documents", "Downloads", "Desktop"} {
		dir := filepath.Join(home, name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			roots = append(roots, dir)
		}
	}
	return roots
}
