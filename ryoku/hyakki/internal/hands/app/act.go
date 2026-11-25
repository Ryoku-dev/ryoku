package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"unicode/utf8"

	"ryoku-rashin/internal/hands/input"
	"ryoku-rashin/internal/oni/hands/apprules"
	"ryoku-rashin/internal/oni/window"
)

// Typed is what a Type got done. Rest is the text not confirmed typed and
// stays in memory: String, GoString and LogValue give counts only, so a
// Typed printed or logged never shows it.
type Typed struct {
	// Landed is the number of characters in chunks the tool finished.
	Landed int
	// Rest is the text minus its first Landed characters, for the caller to
	// hand back; it is never typed anywhere else.
	Rest string
	// Unsure is the size of the chunk the tool failed on, some of which may
	// have reached the window; 0 unless the error is ErrKeyboard.
	Unsure int
}

func (t Typed) String() string {
	return fmt.Sprintf("landed %d, rest %d, unsure %d", t.Landed, utf8.RuneCountInString(t.Rest), t.Unsure)
}

// GoString is String, so %#v shows no text either.
func (t Typed) GoString() string { return t.String() }

// LogValue logs the counts only.
func (t Typed) LogValue() slog.Value {
	return slog.GroupValue(slog.Int("landed", t.Landed), slog.Int("rest", utf8.RuneCountInString(t.Rest)), slog.Int("unsure", t.Unsure))
}

// Type types text into the picked window in chunks of Limits.ChunkRunes
// characters, the guard checking before and after each one. The first
// failed check, a key the person pressed included, stops typing at a chunk
// boundary; Rest holds what was not typed. Text the app rules do not call
// safe (a line break, a control character, a secret) or longer than
// Limits.MaxTypeRunes types nothing.
func (d *Driver) Type(ctx context.Context, text string) (Typed, error) {
	d.act.Lock()
	defer d.act.Unlock()
	res := Typed{Rest: text}
	if err := d.begin(); err != nil {
		return res, err
	}
	if v := apprules.Text(text); v.Kind != apprules.KindSafe {
		d.log.Info("hands app", "action", "type", "refused", v.Why)
		return res, &Refused{Why: v.Why}
	}
	if utf8.RuneCountInString(text) > d.cfg.Limits.MaxTypeRunes {
		d.log.Info("hands app", "action", "type", "refused", "long")
		return res, &Refused{Why: "long"}
	}
	off := 0
	for _, piece := range input.Chunks(text, d.cfg.Limits.ChunkRunes) {
		t, err := d.kb.TypeChecked(ctx, piece, d.verify)
		res.Landed += t.Landed
		res.Rest = text[off+len(piece)-len(t.Rest):]
		if err != nil {
			res.Unsure = t.Unsure
			err = typingError(ctx, err)
			d.log.Info("hands app", "action", "type", "typed", res, "pause", pauseOf(err), "ok", false)
			return res, err
		}
		off += len(piece)
	}
	d.log.Info("hands app", "action", "type", "typed", res, "ok", true)
	return res, nil
}

// typingError keeps the guard's and the context's errors and replaces any
// other with one that cannot carry the tool's words.
func typingError(ctx context.Context, err error) error {
	var p *Paused
	switch {
	case errors.As(err, &p):
		return err
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, input.ErrText):
		return &Refused{Why: apprules.WhyControl}
	}
	return ErrKeyboard
}

// Press presses one chord from the app rules' safe set: Tab, Shift+Tab,
// the arrows, Home, End, Page Up and Down, BackSpace and Ctrl+Z. Anything
// else, Enter included, is refused.
func (d *Driver) Press(ctx context.Context, chord string) error {
	d.act.Lock()
	defer d.act.Unlock()
	if err := d.begin(); err != nil {
		return err
	}
	c, v := apprules.PressChord(chord)
	if v.Kind != apprules.KindSafe {
		d.log.Info("hands app", "action", "press", "refused", v.Why)
		return &Refused{Why: v.Why}
	}
	if _, err := d.check(ctx); err != nil {
		d.log.Info("hands app", "action", "press", "pause", pauseOf(err), "ok", false)
		return err
	}
	if err := d.kb.Chord(ctx, c.String()); err != nil {
		d.log.Info("hands app", "action", "press", "ok", false)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrKeyboard
	}
	_, err := d.check(ctx)
	d.log.Info("hands app", "action", "press", "pause", pauseOf(err), "ok", err == nil)
	return err
}

// Click presses and releases the left button once at fx, fy, fractions of
// the picked window's box (0 ≤ f < 1). The window must have a box (floating
// or placed); a tiled window the desktop does not place takes keys only.
// The point must fall inside the box and on an output, and no other window
// may cover it. The pointer is opened only after the check passed; the
// check runs again after the move, before the press, and after the release.
func (d *Driver) Click(ctx context.Context, fx, fy float64) error {
	d.act.Lock()
	defer d.act.Unlock()
	if err := d.begin(); err != nil {
		return err
	}
	if d.base.Box == nil {
		d.log.Info("hands app", "action", "click", "refused", "unplaced")
		return &Refused{Why: "unplaced"}
	}
	if !(fx >= 0 && fx < 1 && fy >= 0 && fy < 1) {
		d.log.Info("hands app", "action", "click", "refused", "region")
		return &Refused{Why: "region"}
	}
	s, err := d.check(ctx)
	if err != nil {
		d.log.Info("hands app", "action", "click", "pause", pauseOf(err), "ok", false)
		return err
	}
	x, y, width, height, err := d.hit(s, fx, fy)
	if err != nil {
		d.log.Info("hands app", "action", "click", "refused", err.(*Refused).Why)
		return err
	}
	// A new virtual pointer needs motion to pick the surface. Approach
	// from an uncovered point inside the same window, never outside it.
	b := d.target(s).Box
	var ax, ay int
	approach := false
	for _, delta := range [4][2]float64{{1 / b.W, 0}, {-1 / b.W, 0}, {0, 1 / b.H}, {0, -1 / b.H}} {
		mx, my, _, _, moveErr := d.hit(s, fx+delta[0], fy+delta[1])
		if moveErr == nil && (mx != x || my != y) {
			ax, ay, approach = mx, my, true
			break
		}
	}
	if !approach {
		return &Refused{Why: "region"}
	}
	p, err := d.dial(d.cfg.Display.Socket())
	if err != nil {
		d.log.Info("hands app", "action", "click", "ok", false)
		return ErrPointer
	}
	defer p.Close()
	if err := p.Move(ax, ay, width, height); err != nil {
		return ErrPointer
	}
	if _, err := d.check(ctx); err != nil {
		return err
	}
	if err := p.Move(x, y, width, height); err != nil {
		d.log.Info("hands app", "action", "click", "ok", false)
		return ErrPointer
	}
	// the move itself may hand focus to another window
	if _, err := d.check(ctx); err != nil {
		d.log.Info("hands app", "action", "click", "pause", pauseOf(err), "ok", false)
		return err
	}
	if err := p.Button(input.ButtonLeft, true); err != nil {
		d.log.Info("hands app", "action", "click", "ok", false)
		return ErrPointer
	}
	if err := p.Button(input.ButtonLeft, false); err != nil {
		d.log.Info("hands app", "action", "click", "ok", false)
		return ErrPointer
	}
	_, err = d.check(ctx)
	d.log.Info("hands app", "action", "click", "pause", pauseOf(err), "ok", err == nil)
	return err
}

// hit is the click's hit-test on s: the point fx, fy of the target's box,
// which must hold it with no other window over it, placed in the layout the
// pointer's absolute motion spans (every output's logical box).
func (d *Driver) hit(s window.Snapshot, fx, fy float64) (x, y, width, height int, err error) {
	w := d.target(s)
	lx, ly, rx, ry, out, ok := layout(s, w.Output)
	if !ok {
		return 0, 0, 0, 0, &Refused{Why: "region"}
	}
	px, py, cerr := apprules.Click(w, apprules.Region{X: fx, Y: fy}, s.Windows)
	switch {
	case errors.Is(cerr, apprules.ErrUnplaced):
		return 0, 0, 0, 0, &Refused{Why: "unplaced"}
	case errors.Is(cerr, apprules.ErrCovered):
		return 0, 0, 0, 0, &Refused{Why: "covered"}
	case errors.Is(cerr, apprules.ErrHidden):
		return 0, 0, 0, 0, &Refused{Why: "hidden"}
	case cerr != nil:
		return 0, 0, 0, 0, &Refused{Why: "region"}
	}
	b := w.Box
	// inside the box, half-open as a pixel grid is, and on its output
	if !(px >= b.X && px < b.X+b.W && py >= b.Y && py < b.Y+b.H) ||
		!(px >= 0 && px < float64(out.W) && py >= 0 && py < float64(out.H)) {
		return 0, 0, 0, 0, &Refused{Why: "region"}
	}
	gx := math.Floor(float64(out.X)+px) - float64(lx)
	gy := math.Floor(float64(out.Y)+py) - float64(ly)
	return int(gx), int(gy), rx - lx, ry - ly, nil
}

// layout returns the logical box every output spans and the output named.
func layout(s window.Snapshot, name string) (lx, ly, rx, ry int, out window.Output, ok bool) {
	seen := false
	for _, o := range s.Outputs {
		if o.W <= 0 || o.H <= 0 {
			continue
		}
		if !seen {
			lx, ly, rx, ry, seen = o.X, o.Y, o.X+o.W, o.Y+o.H, true
		}
		lx, ly = min(lx, o.X), min(ly, o.Y)
		rx, ry = max(rx, o.X+o.W), max(ry, o.Y+o.H)
		if o.Name == name {
			out, ok = o, true
		}
	}
	return lx, ly, rx, ry, out, ok
}
