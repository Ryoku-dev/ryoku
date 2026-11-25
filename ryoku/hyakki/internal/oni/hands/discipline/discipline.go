// Package discipline decides, before and after every Hands action outside a
// browser tab, whether the place Hands acts on is still the place the person
// chose. It only reads window facts it is handed; it starts nothing and
// reaches no compositor.
//
// The live app scope drives one window the person picked. That window is
// recorded at the pick as a Baseline, and every check after it is stricter
// than dictation's target check: the same id, app and process; the same
// output; shown and not minimized; exactly one focused window, and it is the
// target; the title unchanged; the box, floating and fullscreen state
// unchanged within a pixel tolerance; and no other window drawn over it. Any
// difference is a Pause with a reason, never a guess. A Guard adds the
// person's own keys: any key going down while Hands acts pauses it, and the
// key reader says only that a key went down, never which.
//
// The desktop scope drives a separate nested desktop that Hyakki owns, never
// the person's live desktop. Its away rule pauses on a lost presenter, a
// change in the nested outputs, and the person focusing Hyakki's own window.
//
// Private windows are refused at every layer this package sees: Hyakki's own
// windows, password managers and secret stores, polkit agents and other
// credential prompts, and lock screens. Wayland names no text field, so these
// window-level rules are the floor: a password field inside an ordinary
// window cannot be told apart here.
package discipline

import (
	"math"

	"ryoku-rashin/internal/oni/window"
)

// Pause is why a check did not pass. The zero value OK means the check
// passed; every other value is the reason a paused frame shows. It names a
// rule, never window text, so it may be kept in the activity log.
type Pause string

const (
	// OK: the check passed.
	OK Pause = ""
	// PauseClosed: the target is gone, or its id now belongs to another
	// app or process.
	PauseClosed Pause = "closed"
	// PauseFocus: another window, or none, has focus.
	PauseFocus Pause = "focus"
	// PauseWindow: the windows could not be read, more than one window
	// reports focus, or the target can no longer be told apart.
	PauseWindow Pause = "window"
	// PauseHidden: the target is minimized or not on a visible workspace.
	PauseHidden Pause = "hidden"
	// PauseOutput: the target moved to another output, or the nested
	// desktop's outputs changed.
	PauseOutput Pause = "output"
	// PauseTitle: the target's title changed.
	PauseTitle Pause = "title"
	// PauseGeometry: the target moved, resized, or changed between floating,
	// tiled and fullscreen.
	PauseGeometry Pause = "geometry"
	// PauseCovered: another window may be drawn over the target.
	PauseCovered Pause = "covered"
	// PauseSelf: one of Hyakki's own windows has focus or covers the target.
	PauseSelf Pause = "self"
	// PausePrivate: a private window has focus or covers the target.
	PausePrivate Pause = "private"
	// PauseKey: the person pressed a key, or no key reader is listening.
	PauseKey Pause = "key"
	// PausePresenter: no presenter is drawn.
	PausePresenter Pause = "presenter"
)

// Reason is why a window or a nested desktop was refused at the pick. The
// zero value Accepted means it was not refused.
type Reason string

const (
	// Accepted: nothing refused the pick.
	Accepted Reason = ""
	// ReasonMissing: no window has that id, or the nested desktop has no
	// output.
	ReasonMissing Reason = "missing"
	// ReasonUnpinned: the window has no process id and its app id does not
	// name it alone, so a reused id could not be told from it.
	ReasonUnpinned Reason = "unpinned"
	// ReasonHidden: the window is minimized or not on a visible workspace.
	ReasonHidden Reason = "hidden"
	// ReasonSelf: the window is one of Hyakki's own.
	ReasonSelf Reason = "self"
	// ReasonPrivate: the window is a private one.
	ReasonPrivate Reason = "private"
)

// Baseline is the live window the person picked, as it was at the pick.
// Title is window text: it is held in memory for the title check and never
// written anywhere.
type Baseline struct {
	ID     window.ID
	App    string
	PID    int
	Output string
	Title  string
	// Box is nil when the provider cannot place the window, as for a tiled
	// niri window: Hands may then only type into it, never click.
	Box                  *window.Box
	Floating, Fullscreen bool
}

// Capture records window id from s as the target of a live app task. The
// window need not have focus: the person picks it from Hyakki's own window.
// self lists Hyakki's own app ids. It refuses a missing window, one that is
// not shown, one of Hyakki's own, a private one, and one that cannot be
// pinned by its process id or by an app id no other window shares.
func Capture(s window.Snapshot, id window.ID, self []string) (Baseline, Reason) {
	if id == "" {
		return Baseline{}, ReasonMissing
	}
	w, ok := find(s, id)
	if !ok {
		return Baseline{}, ReasonMissing
	}
	switch {
	case Self(w, self):
		return Baseline{}, ReasonSelf
	case Private(w):
		return Baseline{}, ReasonPrivate
	case !shown(w):
		return Baseline{}, ReasonHidden
	case w.PID == 0 && !alone(s, w):
		return Baseline{}, ReasonUnpinned
	}
	b := Baseline{
		ID: w.ID, App: w.App, PID: w.PID, Output: w.Output, Title: w.Title,
		Floating: w.Floating, Fullscreen: w.Fullscreen,
	}
	if w.Box != nil {
		box := *w.Box
		b.Box = &box
	}
	return b, Accepted
}

// Check compares s with the baseline and returns OK only when every rule
// holds; otherwise the first rule that failed, in the order the constants
// above describe them. self lists Hyakki's own app ids, and tolPx is how far
// in logical pixels any edge of the box may move before it counts as moved;
// a negative tolerance counts as none. The zero Baseline never passes.
func (b Baseline) Check(s window.Snapshot, self []string, tolPx int) Pause {
	if b.ID == "" {
		return PauseClosed
	}
	w, ok := find(s, b.ID)
	// Hyprland's ids are addresses and can be reused once a window closes,
	// so another app or process under the same id is a closed target.
	if !ok || w.App != b.App || (b.PID != 0 && w.PID != b.PID) {
		return PauseClosed
	}
	// a target pinned by its app id alone stops being pinned once another
	// window shares that app id
	if b.PID == 0 && (w.PID != 0 || !alone(s, w)) {
		return PauseWindow
	}
	if w.Output != b.Output {
		return PauseOutput
	}
	if !shown(w) {
		return PauseHidden
	}
	f, n := focused(s)
	switch {
	case n > 1:
		return PauseWindow
	case n == 0:
		return PauseFocus
	case f.ID != b.ID:
		return foreign(f, self, PauseFocus)
	}
	if w.Title != b.Title {
		return PauseTitle
	}
	if w.Floating != b.Floating || w.Fullscreen != b.Fullscreen || moved(b.Box, w.Box, tolPx) {
		return PauseGeometry
	}
	for _, c := range s.Windows {
		if c.ID != b.ID && covers(c, w) {
			return foreign(c, self, PauseCovered)
		}
	}
	return OK
}

// foreign names why window c, which is not the target, stops the task: self
// for Hyakki's own, private for a private one, and otherwise fallback.
func foreign(c window.Window, self []string, fallback Pause) Pause {
	switch {
	case Self(c, self):
		return PauseSelf
	case Private(c):
		return PausePrivate
	}
	return fallback
}

func find(s window.Snapshot, id window.ID) (window.Window, bool) {
	for _, c := range s.Windows {
		if c.ID == id {
			return c, true
		}
	}
	return window.Window{}, false
}

// alone says whether w has an app id and no other window in s shares it.
func alone(s window.Snapshot, w window.Window) bool {
	if w.App == "" {
		return false
	}
	for _, c := range s.Windows {
		if c.ID != w.ID && c.App == w.App {
			return false
		}
	}
	return true
}

// focused returns the focused window in s and how many report focus.
func focused(s window.Snapshot) (window.Window, int) {
	var (
		w window.Window
		n int
	)
	for _, c := range s.Windows {
		if c.Focused {
			w = c
			n++
		}
	}
	return w, n
}

func shown(w window.Window) bool { return w.Visible && !w.Minimized }

// moved says whether box now differs from box then by more than tolPx on
// any edge, or one of them is placed and the other is not.
func moved(then, now *window.Box, tolPx int) bool {
	if then == nil || now == nil {
		return then != now
	}
	tol := float64(max(tolPx, 0))
	for _, d := range [...]float64{now.X - then.X, now.Y - then.Y, now.W - then.W, now.H - then.H} {
		// a NaN never compares within tolerance
		if !(math.Abs(d) <= tol) {
			return true
		}
	}
	return false
}

// covers says whether window c may be drawn over target w. There is no
// stacking order in a snapshot, so a shown window on the same output covers
// the target when it is fullscreen, or floating and either overlapping the
// target's box or unplaced; a floating window over an unplaced target covers
// it too, because where the target sits is unknown.
func covers(c, w window.Window) bool {
	if !shown(c) || c.Output != w.Output {
		return false
	}
	if c.Fullscreen {
		return true
	}
	if !c.Floating {
		return false
	}
	if c.Box == nil || w.Box == nil {
		return true
	}
	return overlap(*c.Box, *w.Box)
}

// overlap says whether a and b share an area; edges that only touch do not.
func overlap(a, b window.Box) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}
