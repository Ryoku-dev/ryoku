// Package target records the window the user was in when dictation started
// and checks, before any text is typed, that it is still that window.
//
// The check is window-level only. Wayland names no text field, so a target is
// a window's id, app, process and output plus its focus: a password field
// inside the right window cannot be told apart from any other field in it.
package target

import (
	"context"
	"errors"
	"sync/atomic"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/window"
)

// Source reads the current windows. Every window.Provider is one; tests pass
// a fake.
type Source interface {
	Snapshot(ctx context.Context) (window.Snapshot, error)
}

// Target is the window that had focus when dictation started.
type Target struct {
	Window window.ID
	App    string
	Title  string
	Output string
	// PID is 0 when the provider does not report one.
	PID int
	// Seq numbers captures within this process, starting at 1, so a caller
	// can tell a stale target from the current one. The zero Target never
	// verifies.
	Seq uint64
}

// Reason is why a target was refused.
type Reason string

const (
	// ReasonClosed: no window with the target's id is left, or the id now
	// belongs to a different app or process.
	ReasonClosed Reason = "closed"
	// ReasonUnfocused: the window still exists but no longer has focus.
	ReasonUnfocused Reason = "unfocused"
	// ReasonMovedOutput: the window is now on a different output.
	ReasonMovedOutput Reason = "moved-output"
	// ReasonHidden: the provider reports the window minimized or not on a
	// visible workspace.
	ReasonHidden Reason = "hidden"
	// ReasonUnavailable: the windows could not be read, or focus could not be
	// pinned to exactly one window.
	ReasonUnavailable Reason = "unavailable"
)

// Refusal is the error Capture and Verify return. Err is the underlying
// failure for ReasonUnavailable and nil otherwise.
type Refusal struct {
	Reason Reason
	Err    error
}

func (r *Refusal) Error() string {
	s := "target: " + string(r.Reason)
	if r.Err != nil {
		s += ": " + r.Err.Error()
	}
	return s
}

func (r *Refusal) Unwrap() error { return r.Err }

// Message is the user copy for the refusal. It never names the dictated text.
func (r *Refusal) Message() string {
	switch r.Reason {
	case ReasonClosed:
		return i18n.T("The window you were dictating into has closed, so typing stopped. The text stays here to copy.")
	case ReasonUnfocused:
		return i18n.T("The window you were dictating into lost focus, so typing stopped. The text stays here to copy.")
	case ReasonMovedOutput:
		return i18n.T("The window you were dictating into moved to another screen, so typing stopped. The text stays here to copy.")
	case ReasonHidden:
		return i18n.T("The window you were dictating into is hidden or minimized, so typing stopped. The text stays here to copy.")
	default:
		return i18n.T("Hyakki could not confirm which window has focus, so nothing more was typed. The text stays here to copy.")
	}
}

// ReasonOf returns the refusal reason carried by err: "" for nil, and
// ReasonUnavailable for any error that is not a Refusal, so an unexpected
// failure still reads as a refusal.
func ReasonOf(err error) Reason {
	if err == nil {
		return ""
	}
	var r *Refusal
	if errors.As(err, &r) {
		return r.Reason
	}
	return ReasonUnavailable
}

var seq atomic.Uint64

func refuse(reason Reason, err error) error { return &Refusal{Reason: reason, Err: err} }

var (
	errNoSource  = errors.New("no window source")
	errNoFocus   = errors.New("no window has focus")
	errManyFocus = errors.New("more than one window reports focus")
	errNoID      = errors.New("the focused window has no id")
	errZero      = errors.New("no target was captured")
)

// focused returns the one focused window in s.
func focused(s window.Snapshot) (window.Window, error) {
	var (
		w     window.Window
		count int
	)
	for _, c := range s.Windows {
		if c.Focused {
			w = c
			count++
		}
	}
	switch count {
	case 0:
		return w, errNoFocus
	case 1:
		return w, nil
	default:
		return w, errManyFocus
	}
}

func shown(w window.Window) bool { return w.Visible && !w.Minimized }

// Capture records the focused window. It refuses with ReasonUnavailable when
// the windows cannot be read or focus is not on exactly one window with an
// id, and with ReasonHidden when that window is not shown.
func Capture(ctx context.Context, src Source) (Target, error) {
	if src == nil {
		return Target{}, refuse(ReasonUnavailable, errNoSource)
	}
	s, err := src.Snapshot(ctx)
	if err != nil {
		return Target{}, refuse(ReasonUnavailable, err)
	}
	w, err := focused(s)
	if err != nil {
		return Target{}, refuse(ReasonUnavailable, err)
	}
	if w.ID == "" {
		return Target{}, refuse(ReasonUnavailable, errNoID)
	}
	if !shown(w) {
		return Target{}, refuse(ReasonHidden, nil)
	}
	return Target{
		Window: w.ID, App: w.App, Title: w.Title, Output: w.Output, PID: w.PID,
		Seq: seq.Add(1),
	}, nil
}

// Verify reads the windows again and returns nil only when the target window
// still exists as the same app and process, is on the same output, is shown,
// and is the one focused window. A title change is allowed: terminals and
// editors retitle themselves while text is typed.
func (t Target) Verify(ctx context.Context, src Source) error {
	if t.Window == "" || t.Seq == 0 {
		return refuse(ReasonUnavailable, errZero)
	}
	if src == nil {
		return refuse(ReasonUnavailable, errNoSource)
	}
	s, err := src.Snapshot(ctx)
	if err != nil {
		return refuse(ReasonUnavailable, err)
	}
	var (
		w     window.Window
		found bool
	)
	for _, c := range s.Windows {
		if c.ID == t.Window {
			w, found = c, true
			break
		}
	}
	// Hyprland's ids are addresses and can be reused once a window closes,
	// so a different app or process under the same id is a closed target.
	if !found || w.App != t.App || (t.PID != 0 && w.PID != 0 && w.PID != t.PID) {
		return refuse(ReasonClosed, nil)
	}
	if w.Output != t.Output {
		return refuse(ReasonMovedOutput, nil)
	}
	if !shown(w) {
		return refuse(ReasonHidden, nil)
	}
	f, err := focused(s)
	if errors.Is(err, errManyFocus) {
		return refuse(ReasonUnavailable, err)
	}
	if err != nil || f.ID != t.Window {
		return refuse(ReasonUnfocused, nil)
	}
	return nil
}
