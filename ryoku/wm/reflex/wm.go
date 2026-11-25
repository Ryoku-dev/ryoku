// Package wm carries out a reflex's window verbs, and only where a
// provider honestly supports them:
//
//   - snap left and snap right go through the Ryoku window-manager seam's
//     window.place, which floats the focused window over one half of its
//     output; it needs the seam to report windowFloat and focusHistory;
//   - maximize is niri's own maximize-column on the focused column;
//   - minimize is offered nowhere: no provider here can hide a window and
//     give it back the way a minimize does.
//
// Anything else answers ErrUnsupported, never a pretend success. Every
// command is checked against Allowed before it runs.
package wm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"ryoku-rashin/internal/oni/window"
	"ryoku-rashin/internal/reflex/proc"
)

// Verb is one window verb.
type Verb string

const (
	SnapLeft  Verb = "snap_left"
	SnapRight Verb = "snap_right"
	Maximize  Verb = "maximize"
	Minimize  Verb = "minimize"
)

var (
	// ErrUnsupported means no provider in this session does the verb.
	ErrUnsupported = errors.New("wm: not supported here")
	// ErrNoWindow means the seam reports no focused window to act on.
	ErrNoWindow = errors.New("wm: no focused window")
)

// probeTimeout bounds the one capability read Supports makes.
const probeTimeout = 2 * time.Second

// maxCoord bounds a placement number.
const maxCoord = 100000

// Allowed reports whether name and args are one of this package's
// commands: the seam's caps and state reads, the seam's window.place with
// a well-formed id, four non-negative numbers and an output name, or
// niri's maximize-column.
func Allowed(name string, args ...string) bool {
	switch name {
	case "ryoku":
		switch {
		case slices.Equal(args, []string{"wm", "caps"}), slices.Equal(args, []string{"wm", "state"}):
			return true
		case len(args) == 9 && args[0] == "wm" && args[1] == "act" && args[2] == "window.place":
			if !handle(args[3]) || !handle(args[8]) {
				return false
			}
			for _, n := range args[4:8] {
				v, err := strconv.Atoi(n)
				if err != nil || v < 0 || v > maxCoord || strconv.Itoa(v) != n {
					return false
				}
			}
			return true
		}
	case window.ProviderNiri:
		return slices.Equal(args, []string{"msg", "action", "maximize-column"})
	}
	return false
}

// handle accepts a seam window id or output name: 1 to 128 letters,
// digits and . _ : -, not starting with a dash.
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

// WM runs window verbs for one session.
type WM struct {
	env []string
	run proc.Runner
}

// New returns a WM for the session env describes. run starts commands
// (proc.Exec(env) for the real one); every call is checked against
// Allowed.
func New(env []string, run proc.Runner) *WM { return &WM{env: env, run: proc.Guard(Allowed, run)} }

type seamCaps struct {
	Name     string   `json:"name"`
	Supports []string `json:"supports"`
}

// Supports reports which verbs this session can do. It reads the seam's
// capabilities once; a seam that does not answer offers nothing.
func (w *WM) Supports(ctx context.Context) map[Verb]bool {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	snap := w.canPlace(ctx)
	return map[Verb]bool{SnapLeft: snap, SnapRight: snap, Maximize: w.niri(), Minimize: false}
}

func (w *WM) niri() bool { return lookup(w.env, window.EnvNiriSocket) != "" }

func (w *WM) canPlace(ctx context.Context) bool {
	out, err := w.run(ctx, "ryoku", "wm", "caps")
	if err != nil {
		return false
	}
	var caps seamCaps
	if json.Unmarshal(out, &caps) != nil || caps.Name == "" {
		return false
	}
	return slices.Contains(caps.Supports, "windowFloat") && slices.Contains(caps.Supports, "focusHistory")
}

// Do carries out v, or answers ErrUnsupported.
func (w *WM) Do(ctx context.Context, v Verb) error {
	switch v {
	case SnapLeft, SnapRight:
		if !w.canPlace(ctx) {
			return ErrUnsupported
		}
		return w.snap(ctx, v == SnapLeft)
	case Maximize:
		if !w.niri() {
			return ErrUnsupported
		}
		_, err := w.run(ctx, window.ProviderNiri, "msg", "action", "maximize-column")
		return err
	}
	return ErrUnsupported
}

type seamOutput struct {
	Name     string `json:"name"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Disabled bool   `json:"disabled"`
}

type seamWorkspace struct {
	ID     string `json:"id"`
	Output string `json:"output"`
}

type seamWindow struct {
	ID         string `json:"id"`
	Workspace  string `json:"workspace"`
	Output     string `json:"output"`
	FocusOrder int    `json:"focusOrder"`
}

type seamState struct {
	Outputs    []seamOutput    `json:"outputs"`
	Workspaces []seamWorkspace `json:"workspaces"`
	Windows    []seamWindow    `json:"windows"`
}

// snap places the focused window over the left or right half of its
// output, in the output's logical pixels.
func (w *WM) snap(ctx context.Context, left bool) error {
	out, err := w.run(ctx, "ryoku", "wm", "state")
	if err != nil {
		return err
	}
	var st seamState
	if err := json.Unmarshal(out, &st); err != nil {
		return fmt.Errorf("wm: state: %w", err)
	}
	var focused *seamWindow
	for i := range st.Windows {
		if st.Windows[i].FocusOrder == 0 {
			focused = &st.Windows[i]
			break
		}
	}
	if focused == nil {
		return ErrNoWindow
	}
	name := focused.Output
	if name == "" {
		for _, ws := range st.Workspaces {
			if ws.ID == focused.Workspace {
				name = ws.Output
			}
		}
	}
	i := slices.IndexFunc(st.Outputs, func(o seamOutput) bool { return o.Name == name && !o.Disabled })
	if i < 0 || st.Outputs[i].Width < 2 || st.Outputs[i].Height < 1 {
		return ErrNoWindow
	}
	o := st.Outputs[i]
	half := o.Width / 2
	x, width := 0, half
	if !left {
		x, width = half, o.Width-half
	}
	_, err = w.run(ctx, "ryoku", "wm", "act", "window.place", focused.ID,
		strconv.Itoa(x), "0", strconv.Itoa(width), strconv.Itoa(o.Height), o.Name)
	return err
}

func lookup(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if k, v, ok := strings.Cut(env[i], "="); ok && k == key {
			return v
		}
	}
	return ""
}
