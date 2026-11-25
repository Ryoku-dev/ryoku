// Package app drives one window the person picked for one Hands task: it
// types into it, presses a few safe keys in it, clicks inside it, and reads
// it back. It is the executor of the live app scope; the daemon decides what
// to do and this package only does it, inside these rules:
//
//   - The window is bound at the pick (Select) as a discipline.Baseline.
//     Hyakki's own windows, private windows, hidden ones and windows that
//     cannot be pinned are refused there.
//   - Every action runs the guard's check before it and again after it, on a
//     fresh read of the windows. Any change of focus, title, geometry,
//     output or visibility, a covering window, a private or own window in
//     front, an unreadable window list, and any key the person pressed
//     pauses the task with that reason. A paused driver does nothing until
//     Resume.
//   - Text goes in chunks of at most Limits.ChunkRunes characters, each
//     checked before and after; a failed check stops at a chunk boundary
//     and the untyped rest goes back to the caller. Text with a line break
//     or shaped like a secret is refused, and there is no clipboard path.
//   - Keys are only the app rules' safe set: moving, deleting the character
//     just typed, undo.
//   - A click lands only in a window whose box the desktop reports (floating
//     or otherwise placed), at a point inside that box that no other window
//     may cover, after the check passed. The pointer is opened for the click
//     and closed after it: one move, one press, one release, never a drag.
//   - Read-back finds the typed words in a masked picture of the window.
//     Unknown never passes, and a second Unknown ends the task.
//   - Nothing is ever started but the input package's keyboard tool. No
//     program, script or file is opened or run here.
//
// Typed text stays in memory: it is never written to a log or put into an
// error, and read-back words never leave this package.
package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/oni/hands/discipline"
	"ryoku-rashin/internal/oni/ocr"
	"ryoku-rashin/internal/oni/window"
)

// Windows reads the windows of the display the driver acts on. A
// window.Provider is one; the driver only takes snapshots of it.
type Windows interface {
	Snapshot(ctx context.Context) (window.Snapshot, error)
}

// Limits are the spec's hands.app numbers (spec.Hands.App), passed in by the
// daemon.
type Limits struct {
	// GeometryTolPx is how far any edge of the window may move.
	GeometryTolPx int
	// Verify bounds one read of the windows for a check.
	Verify time.Duration
	// Chunk bounds one typing call.
	Chunk time.Duration
	// ChunkRunes is the most characters one typing call carries, at most
	// input.MaxChunk.
	ChunkRunes int
	// MaxTypeRunes is the most characters one Type carries.
	MaxTypeRunes int
	// Readback bounds capture and text-reading work.
	Readback time.Duration
}

func (l Limits) ok() bool {
	return l.GeometryTolPx >= 0 && l.Verify > 0 && l.Chunk > 0 && l.Readback > 0 &&
		l.ChunkRunes >= 1 && l.ChunkRunes <= input.MaxChunk && l.MaxTypeRunes >= l.ChunkRunes
}

// Config is what a Driver acts with.
type Config struct {
	// Display is where input goes: LiveDisplay for the person's session,
	// NestDisplay for Hyakki's separate nested desktop. The keyboard tool's
	// environment and the pointer's socket come from it and nothing else.
	Display Display
	// Windows reads Display's windows.
	Windows Windows
	// Pictures takes the masked picture of the window.
	Pictures Shooter
	// OCR reads the words in a picture.
	OCR ocr.Reader
	// Masks returns Oni's own rectangles on an output (the glow strips, the
	// presenter), in output-local logical pixels; they are painted out of
	// every picture. nil means none are drawn.
	Masks func(output string) []window.Box
	// Keys starts the key-edge reader (keys.StartAnyKey) with onKey and
	// returns its stop. It is started at Select and stopped at Close; until
	// it runs no check passes.
	Keys func(onKey func()) (stop func(), err error)
	// Self lists Hyakki's own app ids.
	Self []string
	// Limits are the spec's numbers.
	Limits Limits
	// Log receives one record per action with counts and reasons only;
	// nil discards them.
	Log *slog.Logger
	// OnPause, when set, is told each time the driver pauses, including from
	// the key reader's goroutine. It must not call back into the Driver.
	OnPause func(discipline.Pause)
	// Run builds the keyboard tool's runner for an environment; nil is
	// input.Exec. Tests pass a fake.
	Run func(env []string) input.Runner
	// Dial opens the pointer on a socket; nil is input.DialPointer. Tests
	// pass a fake.
	Dial func(socket string) (input.Pointer, error)
}

// Errors. None of them carries typed text or window text.
var (
	ErrConfig      = errors.New("hands app: incomplete driver configuration")
	ErrNotSelected = errors.New("hands app: no window was picked")
	ErrEnded       = errors.New("hands app: the task has ended")
	ErrNoKeys      = errors.New("hands app: the keyboards cannot be read, so a key press could not pause the task")
	ErrWindows     = errors.New("hands app: the windows could not be read")
	ErrKeyboard    = errors.New("hands app: the keyboard tool failed")
	ErrPointer     = errors.New("hands app: the pointer failed")
)

// Paused is the error of an action the guard stopped. Reason names the rule,
// never window text.
type Paused struct{ Reason discipline.Pause }

func (p *Paused) Error() string { return "hands app: paused (" + string(p.Reason) + ")" }

// Refused is the error of a pick or an action the rules do not allow. Why
// names the rule: a discipline.Reason for a pick, an apprules Why for text
// and keys, and for a click "unplaced", "region", "covered" or "hidden".
// "long" is text over Limits.MaxTypeRunes.
type Refused struct{ Why string }

func (r *Refused) Error() string { return "hands app: refused (" + r.Why + ")" }

// Cand is one window the person may pick.
type Cand struct {
	ID  window.ID
	App string
	// Title is window text, for the picker only.
	Title string
	// Clickable is false for a window whose box the desktop does not
	// report, such as a tiled niri window: Hands may only type into it.
	Clickable bool
}

type state uint8

const (
	idle state = iota
	active
	ended
)

// Driver drives one picked window. Its methods may be called from several
// goroutines; actions run one at a time.
type Driver struct {
	cfg  Config
	kb   input.Keyboard
	dial func(string) (input.Pointer, error)
	log  *slog.Logger

	// act keeps actions, Select, Resume and Close one at a time, and with
	// them the guard's Check and Resume.
	act      sync.Mutex
	base     discipline.Baseline
	guard    *discipline.Guard
	stopKeys func()
	unknowns int

	// mu guards what the key reader touches.
	mu    sync.Mutex
	state state
	pause discipline.Pause
}

// New builds a Driver. Nothing is read or started until Select.
func New(cfg Config) (*Driver, error) {
	if cfg.Windows == nil || cfg.Pictures == nil || cfg.OCR == nil || cfg.Keys == nil || !cfg.Limits.ok() {
		return nil, ErrConfig
	}
	env, err := cfg.Display.env()
	if err != nil {
		return nil, err
	}
	run := cfg.Run
	if run == nil {
		run = input.Exec
	}
	dial := cfg.Dial
	if dial == nil {
		dial = input.DialPointer
	}
	log := cfg.Log
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	cfg.Self = append([]string(nil), cfg.Self...)
	return &Driver{
		cfg:  cfg,
		kb:   input.Keyboard{Run: run(env), ChunkTimeout: cfg.Limits.Chunk},
		dial: dial,
		log:  log,
	}, nil
}

// Windows lists the windows the person may pick: shown, not Hyakki's own,
// not private, and pinned by a process or an app id of their own.
func (d *Driver) Windows(ctx context.Context) ([]Cand, error) {
	if d.current() == ended {
		return nil, ErrEnded
	}
	s, err := d.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	var out []Cand
	for _, w := range s.Windows {
		if _, why := discipline.Capture(s, w.ID, d.cfg.Self); why != discipline.Accepted {
			continue
		}
		out = append(out, Cand{ID: w.ID, App: w.App, Title: w.Title, Clickable: w.Box != nil})
	}
	return out, nil
}

// Select binds the task to window id and starts the key reader. A second
// Select replaces the first pick.
func (d *Driver) Select(ctx context.Context, id window.ID) (discipline.Baseline, error) {
	d.act.Lock()
	defer d.act.Unlock()
	if d.current() == ended {
		return discipline.Baseline{}, ErrEnded
	}
	d.release()
	s, err := d.snapshot(ctx)
	if err != nil {
		return discipline.Baseline{}, err
	}
	b, why := discipline.Capture(s, id, d.cfg.Self)
	if why != discipline.Accepted {
		d.log.Info("hands app", "action", "select", "refused", string(why))
		return discipline.Baseline{}, &Refused{Why: string(why)}
	}
	g := discipline.NewGuard(b, d.cfg.Self, d.cfg.Limits.GeometryTolPx)
	stop, err := d.cfg.Keys(func() {
		g.KeyDown()
		d.pauseFor(discipline.PauseKey)
	})
	if err != nil {
		d.log.Info("hands app", "action", "select", "refused", "keys")
		return discipline.Baseline{}, errors.Join(ErrNoKeys, err)
	}
	g.Listening(true)
	d.base, d.guard, d.stopKeys, d.unknowns = b, g, stop, 0
	d.mu.Lock()
	d.state, d.pause = active, discipline.OK
	d.mu.Unlock()
	d.log.Info("hands app", "action", "select", "app", b.App, "clickable", b.Box != nil)
	return b, nil
}

// Resume clears a pause after the person pressed Resume: keys pressed while
// paused no longer count. The next action checks the window afresh.
func (d *Driver) Resume() {
	d.act.Lock()
	defer d.act.Unlock()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state != active {
		return
	}
	d.guard.Resume()
	d.pause = discipline.OK
}

// Paused returns the reason the driver is paused, or OK.
func (d *Driver) Paused() discipline.Pause {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pause
}

// Close ends the task: the key reader stops and every later call returns
// ErrEnded. It is safe to call again.
func (d *Driver) Close() {
	d.act.Lock()
	defer d.act.Unlock()
	d.end()
}

// end stops the key reader and ends the task; act is held.
func (d *Driver) end() {
	d.release()
	d.mu.Lock()
	d.state = ended
	d.mu.Unlock()
}

// release stops the key reader of the current pick, if any; act is held.
func (d *Driver) release() {
	if d.guard != nil {
		d.guard.Listening(false)
	}
	if d.stopKeys != nil {
		d.stopKeys()
	}
	d.guard, d.stopKeys = nil, nil
}

func (d *Driver) current() state {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state
}

// begin is the gate of every action; act is held.
func (d *Driver) begin() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.state == ended:
		return ErrEnded
	case d.state == idle:
		return ErrNotSelected
	case d.pause != discipline.OK:
		return &Paused{Reason: d.pause}
	}
	return nil
}

// pauseFor pauses the driver for p unless it is paused already, and tells
// OnPause. It returns the error of the action p stopped.
func (d *Driver) pauseFor(p discipline.Pause) error {
	d.mu.Lock()
	first := d.state == active && d.pause == discipline.OK
	if first {
		d.pause = p
	}
	reason := d.pause
	if reason == discipline.OK {
		reason = p
	}
	d.mu.Unlock()
	if first && d.cfg.OnPause != nil {
		d.cfg.OnPause(p)
	}
	return &Paused{Reason: reason}
}

// snapshot reads the windows within Limits.Verify.
func (d *Driver) snapshot(ctx context.Context) (window.Snapshot, error) {
	cctx, cancel := context.WithTimeout(ctx, d.cfg.Limits.Verify)
	defer cancel()
	s, err := d.cfg.Windows.Snapshot(cctx)
	if err != nil {
		if ctx.Err() != nil {
			return window.Snapshot{}, ctx.Err()
		}
		return window.Snapshot{}, ErrWindows
	}
	return s, nil
}

// check is the guard's check on a fresh read of the windows, run before and
// after every action; act is held. It returns the read it passed on.
func (d *Driver) check(ctx context.Context) (window.Snapshot, error) {
	if p := d.Paused(); p != discipline.OK {
		return window.Snapshot{}, &Paused{Reason: p}
	}
	s, err := d.snapshot(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return window.Snapshot{}, err
		}
		return window.Snapshot{}, d.pauseFor(discipline.PauseWindow)
	}
	if p := d.guard.Check(s); p != discipline.OK {
		return window.Snapshot{}, d.pauseFor(p)
	}
	// a key may have gone down while the windows were read
	if p := d.Paused(); p != discipline.OK {
		return window.Snapshot{}, &Paused{Reason: p}
	}
	return s, nil
}

// verify is check as the keyboard's Verify.
func (d *Driver) verify(ctx context.Context) error {
	_, err := d.check(ctx)
	return err
}

// target is the picked window in s, which a passed check holds.
func (d *Driver) target(s window.Snapshot) window.Window {
	for _, w := range s.Windows {
		if w.ID == d.base.ID {
			return w
		}
	}
	return window.Window{}
}

// pauseOf is the reason in err, for the log.
func pauseOf(err error) string {
	var p *Paused
	if errors.As(err, &p) {
		return string(p.Reason)
	}
	return ""
}
