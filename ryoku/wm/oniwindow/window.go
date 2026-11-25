// Package window tells the Oni daemon which windows exist, where they sit and
// which one has focus. It reads compositor state only: every provider runs
// read verbs through a Runner, and the argv allowlist below is the complete
// set of commands this package may ever start.
package window

import (
	"context"
	"errors"
	"io"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/spec"
)

// Provider identities and session handles are shared with native consumers.
const (
	ProviderNiri        = "niri"
	ProviderHyprland    = "hyprland"
	EnvNiriSocket       = "NIRI_SOCKET"
	EnvHyprlandInstance = "HYPRLAND_INSTANCE_SIGNATURE"
	CommandHyprland     = "hyprctl"
)

// ID is the compositor's own handle: niri's numbers, Hyprland's 0x addresses,
// the seam's ids. It matches the feature frames' token rule.
type ID string

// Box is output-local logical pixels.
type Box struct{ X, Y, W, H float64 }

// Window is one toplevel as a provider sees it.
type Window struct {
	ID     ID
	App    string
	Title  string
	Output string
	// Box is nil when this provider cannot place the window; Unplaced says why.
	Box      *Box
	Unplaced string

	Focused, Floating, Fullscreen, Minimized, Visible bool
	// FocusOrder is 0 for the focused window, counting back through history.
	FocusOrder int
	PID        int
}

// Output is one monitor in the global logical layout.
type Output struct {
	Name       string
	X, Y, W, H int
	Scale      float64
}

// GeomCap is how much of the window layout a provider can place.
type GeomCap uint8

const (
	GeomNone GeomCap = iota
	GeomFloating
	GeomAll
)

// Caps describes one provider.
type Caps struct {
	Provider string
	Geometry GeomCap
	// Events is true when Watch is driven by compositor events rather than
	// a poll.
	Events bool
	// Pointer is true when Pointer can locate the cursor.
	Pointer bool
}

// Snapshot is one consistent read of outputs and windows.
type Snapshot struct {
	Outputs []Output
	Windows []Window
}

// Provider reads one compositor. Watch blocks until ctx ends; Geometry names
// the perched window whose geometry a provider without move events samples,
// and "" ends the sampling when the perch ends.
type Provider interface {
	Caps() Caps
	Snapshot(ctx context.Context) (Snapshot, error)
	Watch(ctx context.Context, fn func(Snapshot)) error
	Geometry(id ID)
	Pointer(ctx context.Context) (spec.Point, bool, error)
}

// Runner runs one read verb and returns its stdout. The real one is
// exec-backed; tests pass a fake, so no compositor is ever asked.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Reasons a window has no Box. They are user copy: the daemon shows them when
// a perch is refused with no-geometry.
var (
	UnplacedTiled   = i18n.T("This desktop does not report where tiled windows sit, so Oni can only perch on a floating window.")
	UnplacedNoGeom  = i18n.T("This desktop does not report where windows sit, so Oni cannot perch here.")
	UnplacedOffView = i18n.T("That window is not on a visible workspace.")
)

// Stream opens one long-lived read: an event-stream command's stdout, or a
// compositor's event socket. It passes the same allowlist as Runner.
type Stream func(ctx context.Context, name string, args ...string) (io.ReadCloser, error)

// ErrNotAllowed is returned for any argv outside the allowlist.
var ErrNotAllowed = errors.New("window: command not in the read-only allowlist")

// ErrNoStream is returned by Watch when the provider was built without a
// Stream.
var ErrNoStream = errors.New("window: no event stream")
