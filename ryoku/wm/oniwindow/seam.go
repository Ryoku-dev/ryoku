package window

import (
	"context"
	"encoding/json"
	"slices"

	"ryoku-rashin/internal/oni/spec"
)

// The Ryoku window-manager seam, read as a client of the `ryoku wm` CLI.
// These are the JSON shapes `ryoku wm caps|state|watch` print; Hyakki keeps
// its own copy so it never imports Ryoku.

type seamCaps struct {
	Name     string   `json:"name"`
	Supports []string `json:"supports"`
}

type seamOutput struct {
	Name            string  `json:"name"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	Scale           float64 `json:"scale"`
	ActiveWorkspace string  `json:"activeWorkspace"`
	Disabled        bool    `json:"disabled"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
}

type seamWorkspace struct {
	ID         string `json:"id"`
	Output     string `json:"output"`
	Active     bool   `json:"active"`
	Fullscreen bool   `json:"fullscreen"`
}

type seamWindow struct {
	ID         string `json:"id"`
	AppID      string `json:"appId"`
	Title      string `json:"title"`
	Workspace  string `json:"workspace"`
	Output     string `json:"output"`
	Pid        int    `json:"pid"`
	FocusOrder int    `json:"focusOrder"`
	Floating   bool   `json:"floating"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type seamState struct {
	Outputs    []seamOutput    `json:"outputs"`
	Workspaces []seamWorkspace `json:"workspaces"`
	Windows    []seamWindow    `json:"windows"`
}

type seamFrame struct {
	Kind string `json:"kind"`
	seamState
}

type seam struct {
	caps   seamCaps
	run    Runner
	stream Stream
}

func newSeam(caps seamCaps, run Runner, stream Stream) *seam {
	return &seam{caps: caps, run: guard(run), stream: guardStream(stream)}
}

func (p *seam) Caps() Caps {
	geom := GeomFloating
	if slices.Contains(p.caps.Supports, "windowGeometry") {
		geom = GeomAll
	}
	// The seam locates the pointer only through an action verb, which is
	// outside this package's read-only allowlist.
	return Caps{Provider: "ryoku", Geometry: geom, Events: true}
}

func (p *seam) Snapshot(ctx context.Context) (Snapshot, error) {
	out, err := p.run(ctx, "ryoku", "wm", "state")
	if err != nil {
		return Snapshot{}, err
	}
	var st seamState
	if err := json.Unmarshal(out, &st); err != nil {
		return Snapshot{}, err
	}
	return p.convert(st), nil
}

// Watch folds `ryoku wm watch` frames. The seam already keeps geometry fresh
// (its Hyprland provider samples only while a floating window is present; its
// niri provider folds layout events), so Geometry needs no poll here.
func (p *seam) Watch(ctx context.Context, fn func(Snapshot)) error {
	rc, err := p.stream(ctx, "ryoku", "wm", "watch", "windows", "outputs", "workspaces", "focus")
	if err != nil {
		return err
	}
	var st seamState
	ready := false
	e := emitter{fn: fn}
	return readLines(ctx, rc, func(line []byte) error {
		var f seamFrame
		if err := json.Unmarshal(line, &f); err != nil {
			return err
		}
		switch f.Kind {
		case "outputs":
			st.Outputs = f.Outputs
		case "workspaces":
			st.Workspaces = f.Workspaces
		case "windows":
			st.Windows = f.Windows
		case "ready":
			ready = true
		default:
			return nil
		}
		if ready {
			e.emit(p.convert(st))
		}
		return nil
	})
}

func (p *seam) Geometry(ID) {}

func (p *seam) Pointer(context.Context) (spec.Point, bool, error) {
	return spec.Point{}, false, nil
}

func (p *seam) convert(st seamState) Snapshot {
	var s Snapshot
	active := map[string]bool{}
	for _, o := range st.Outputs {
		if o.Disabled {
			continue
		}
		s.Outputs = append(s.Outputs, Output{Name: o.Name, X: o.X, Y: o.Y, W: o.Width, H: o.Height, Scale: o.Scale})
		if o.ActiveWorkspace != "" {
			active[o.ActiveWorkspace] = true
		}
	}
	spaces := map[string]seamWorkspace{}
	for _, w := range st.Workspaces {
		spaces[w.ID] = w
		if w.Active {
			active[w.ID] = true
		}
	}
	history := slices.Contains(p.caps.Supports, "focusHistory")
	all := p.Caps().Geometry == GeomAll
	for _, sw := range st.Windows {
		w := Window{
			ID: ID(sw.ID), App: sw.AppID, Title: sw.Title, Output: sw.Output,
			Floating: sw.Floating, FocusOrder: sw.FocusOrder, PID: sw.Pid,
			Focused: history && sw.FocusOrder == 0,
			Visible: sw.Workspace == "" || len(spaces) == 0 || active[sw.Workspace],
		}
		if w.Output == "" {
			if ws, ok := spaces[sw.Workspace]; ok {
				w.Output = ws.Output
			}
		}
		out, haveOut := outputByName(s.Outputs, w.Output)
		if !haveOut && sw.Width > 0 && sw.Height > 0 {
			out, haveOut = outputAt(s.Outputs, float64(sw.X)+float64(sw.Width)/2, float64(sw.Y)+float64(sw.Height)/2)
			w.Output = out.Name
		}
		switch {
		case !w.Visible:
			w.Unplaced = UnplacedOffView
		case sw.Width > 0 && sw.Height > 0 && haveOut:
			w.Box = &Box{X: float64(sw.X - out.X), Y: float64(sw.Y - out.Y), W: float64(sw.Width), H: float64(sw.Height)}
			w.Fullscreen = spaces[sw.Workspace].Fullscreen && sw.Width >= out.W && sw.Height >= out.H
		case !sw.Floating && !all:
			w.Unplaced = UnplacedTiled
		default:
			w.Unplaced = UnplacedNoGeom
		}
		s.Windows = append(s.Windows, w)
	}
	sortWindows(s.Windows)
	return s
}
