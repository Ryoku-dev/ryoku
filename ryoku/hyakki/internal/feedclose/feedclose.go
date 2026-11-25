// Package feedclose closes one window for the focus guard, and only where
// that is honestly possible: through the Ryoku window-manager seam's
// window.close, after the seam's own state still shows the same window under
// the same app. Nothing here closes a tab, presses a key or speaks to a
// compositor directly; without the seam it closes nothing and says so.
// Every command is checked against Allowed before it runs.
package feedclose

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"ryoku-rashin/internal/reflex/proc"
)

var (
	// ErrUnsupported means no seam answers in this session, so nothing can
	// be closed.
	ErrUnsupported = errors.New("feedclose: closing windows is not supported here")
	// ErrChanged means the window is gone, or its id now names another
	// app's window; nothing was closed.
	ErrChanged = errors.New("feedclose: the window changed")
)

// probeTimeout bounds the seam reads Close makes before it acts.
const probeTimeout = 2 * time.Second

// Allowed reports whether name and args are one of this package's commands:
// the seam's caps and state reads, and its window.close with one
// well-formed window id.
func Allowed(name string, args ...string) bool {
	if name != "ryoku" {
		return false
	}
	switch {
	case slices.Equal(args, []string{"wm", "caps"}), slices.Equal(args, []string{"wm", "state"}):
		return true
	case len(args) == 4 && args[0] == "wm" && args[1] == "act" && args[2] == "window.close":
		return handle(args[3])
	}
	return false
}

// handle accepts a seam window id: 1 to 128 letters, digits and . _ : -,
// not starting with a dash.
func handle(s string) bool {
	if s == "" || len(s) > 128 || s[0] == '-' {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._:-", r)) {
			return false
		}
	}
	return true
}

// Closer closes windows through the seam.
type Closer struct{ run proc.Runner }

// New returns a Closer whose commands run is checked against Allowed;
// proc.Exec(env) is the real one.
func New(run proc.Runner) *Closer { return &Closer{run: proc.Guard(Allowed, run)} }

type seamCaps struct {
	Name string `json:"name"`
}

type seamWindow struct {
	ID    string `json:"id"`
	AppID string `json:"appId"`
}

type seamState struct {
	Windows []seamWindow `json:"windows"`
}

// Supported reports whether a seam answers in this session.
func (c *Closer) Supported(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	out, err := c.run(ctx, "ryoku", "wm", "caps")
	if err != nil {
		return false
	}
	var caps seamCaps
	return json.Unmarshal(out, &caps) == nil && caps.Name != ""
}

// Close closes the window the seam knows as id, only while the seam's state
// still shows it under app. A window gone or renamed to another app closes
// nothing and answers ErrChanged.
func (c *Closer) Close(ctx context.Context, id, app string) error {
	if !handle(id) || app == "" {
		return ErrChanged
	}
	if !c.Supported(ctx) {
		return ErrUnsupported
	}
	read, cancel := context.WithTimeout(ctx, probeTimeout)
	out, err := c.run(read, "ryoku", "wm", "state")
	cancel()
	if err != nil {
		return err
	}
	var st seamState
	if err := json.Unmarshal(out, &st); err != nil {
		return err
	}
	i := slices.IndexFunc(st.Windows, func(w seamWindow) bool { return w.ID == id })
	if i < 0 || st.Windows[i].AppID != app {
		return ErrChanged
	}
	_, err = c.run(ctx, "ryoku", "wm", "act", "window.close", id)
	return err
}
