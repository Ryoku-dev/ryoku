package app

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"ryoku-rashin/internal/oni/capture"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/window"
)

// Shooter takes one picture of a window with masks painted out, as PNG
// bytes in memory, and a clear that zeroes them.
type Shooter interface {
	Shoot(ctx context.Context, w window.Window, masks []window.Box) (png []byte, clear func(), err error)
}

// Pictures is the Shooter for a capture.Service: the window's own toplevel
// where the desktop offers one, otherwise the crop of the screen under it
// with masks painted white.
func Pictures(svc capture.Service) Shooter { return pictures{svc} }

type pictures struct{ svc capture.Service }

func (p pictures) Shoot(ctx context.Context, w window.Window, masks []window.Box) ([]byte, func(), error) {
	shot, _, err := p.svc.Capture(ctx, w, masks)
	if err != nil {
		return nil, nil, err
	}
	png, err := shot.PNG()
	if err != nil {
		shot.Clear()
		return nil, nil, err
	}
	return png, shot.Clear, nil
}

// Readback is what reading typed text back found. The zero value is
// Unknown, so a read-back nobody judged never passes.
type Readback uint8

const (
	// Unknown: the picture or its words could not tell. It never passes;
	// a second Unknown in one task ends the task.
	Unknown Readback = iota
	// Yes: every typed word is in the window.
	Yes
	// No: the window's words were read and none of the typed words is
	// among them.
	No
)

func (r Readback) String() string {
	switch r {
	case Yes:
		return "yes"
	case No:
		return "no"
	}
	return "unknown"
}

// Frame is one look at the picked window: the masked picture and the words
// read in it, with boxes in fractions of the picture. Read is false when
// the text reader failed. Clear zeroes the picture.
type Frame struct {
	PNG   []byte
	Words []screenask.Word
	Read  bool
	clear func()
}

// Clear zeroes the picture.
func (f Frame) Clear() {
	if f.clear != nil {
		f.clear()
	}
}

// Look takes the masked picture of the picked window and reads its words,
// the guard checking before and after the picture.
func (d *Driver) Look(ctx context.Context) (Frame, error) {
	d.act.Lock()
	defer d.act.Unlock()
	if err := d.begin(); err != nil {
		return Frame{}, err
	}
	png, clear, err := d.picture(ctx)
	if err != nil {
		d.log.Info("hands app", "action", "look", "pause", pauseOf(err), "ok", false)
		return Frame{}, err
	}
	rctx, cancel := context.WithTimeout(ctx, d.cfg.Limits.Readback)
	defer cancel()
	words, werr := d.cfg.OCR.Words(rctx, png)
	d.log.Info("hands app", "action", "look", "words", len(words), "ok", true)
	return Frame{PNG: png, Words: words, Read: werr == nil, clear: clear}, nil
}

// ReadBack looks for the words of typed in a masked picture of the picked
// window. The words read stay here; only the verdict leaves. Unknown never
// passes, and the second Unknown in a task ends it: it returns Unknown and
// ErrEnded, and every later call ErrEnded. A check that fails around the
// picture pauses with no verdict counted.
func (d *Driver) ReadBack(ctx context.Context, typed string) (Readback, error) {
	d.act.Lock()
	defer d.act.Unlock()
	if err := d.begin(); err != nil {
		return Unknown, err
	}
	want := tokens(typed)
	verdict := Unknown
	if len(want) > 0 {
		rctx, cancel := context.WithTimeout(ctx, d.cfg.Limits.Readback)
		defer cancel()
		// the typing process can exit before the target renders its last keys
		for rctx.Err() == nil {
			png, clear, err := d.picture(rctx)
			var paused *Paused
			if errors.As(err, &paused) {
				d.log.Info("hands app", "action", "readback", "pause", pauseOf(err), "ok", false)
				return Unknown, err
			}
			if err != nil {
				break
			}
			words, werr := d.cfg.OCR.Words(rctx, png)
			clear()
			if werr != nil {
				break
			}
			verdict = judge(want, words)
			if verdict != Unknown {
				break
			}
		}
		// a cancelled read-back is no read at all
		if ctx.Err() != nil {
			return Unknown, ctx.Err()
		}
	}
	if verdict == Unknown {
		d.unknowns++
		if d.unknowns >= 2 {
			d.end()
			d.log.Info("hands app", "action", "readback", "verdict", verdict.String(), "ended", true)
			return Unknown, ErrEnded
		}
	}
	d.log.Info("hands app", "action", "readback", "verdict", verdict.String())
	return verdict, nil
}

// picture checks, takes the masked picture of the target, and checks again,
// so the picture is of the window the checks saw; act is held.
func (d *Driver) picture(ctx context.Context) ([]byte, func(), error) {
	s, err := d.check(ctx)
	if err != nil {
		return nil, nil, err
	}
	w := d.target(s)
	var masks []window.Box
	if d.cfg.Masks != nil {
		masks = d.cfg.Masks(w.Output)
	}
	rctx, cancel := context.WithTimeout(ctx, d.cfg.Limits.Readback)
	png, clear, err := d.cfg.Pictures.Shoot(rctx, w, masks)
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, err
	}
	if _, err := d.check(ctx); err != nil {
		clear()
		return nil, nil, err
	}
	return png, clear, nil
}

// tokens are the readable words of s, folded: lower case, letters and
// digits only. A word with neither is not readable and not looked for.
func tokens(s string) []string {
	var out []string
	for _, f := range strings.Fields(s) {
		if t := fold(f); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func fold(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// judge finds want among the words read: Yes when every typed word is
// there (each read word counted once), No when words were read and none of
// the typed ones is among them, Unknown otherwise.
func judge(want []string, words []screenask.Word) Readback {
	have := map[string]int{}
	for _, w := range words {
		for _, t := range tokens(w.Text) {
			have[t]++
		}
	}
	found := 0
	for _, t := range want {
		if have[t] > 0 {
			have[t]--
			found++
		}
	}
	clear(have)
	switch {
	case found == len(want):
		return Yes
	case found == 0 && len(words) > 0:
		return No
	}
	return Unknown
}
