package window

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"sync/atomic"

	"ryoku-rashin/internal/oni/spec"
)

// niri, read through `niri msg --json`. Released niri reports a position in
// the workspace view only for floating windows; tiled windows get one where
// niri fills tile_pos_in_workspace_view, and that is used whenever present,
// with no version check.

type niriLayout struct {
	TileSize               [2]float64  `json:"tile_size"`
	WindowSize             [2]int      `json:"window_size"`
	TilePosInWorkspaceView *[2]float64 `json:"tile_pos_in_workspace_view"`
	WindowOffsetInTile     [2]float64  `json:"window_offset_in_tile"`
}

type niriTimestamp struct {
	Secs  uint64 `json:"secs"`
	Nanos uint32 `json:"nanos"`
}

func (t *niriTimestamp) after(o *niriTimestamp) bool {
	switch {
	case t == nil:
		return false
	case o == nil:
		return true
	case t.Secs != o.Secs:
		return t.Secs > o.Secs
	}
	return t.Nanos > o.Nanos
}

type niriWindow struct {
	ID             uint64         `json:"id"`
	Title          *string        `json:"title"`
	AppID          *string        `json:"app_id"`
	PID            *int           `json:"pid"`
	WorkspaceID    *uint64        `json:"workspace_id"`
	IsFocused      bool           `json:"is_focused"`
	IsFloating     bool           `json:"is_floating"`
	Layout         *niriLayout    `json:"layout"`
	FocusTimestamp *niriTimestamp `json:"focus_timestamp"`
}

type niriWorkspace struct {
	ID        uint64  `json:"id"`
	Output    *string `json:"output"`
	IsActive  bool    `json:"is_active"`
	IsFocused bool    `json:"is_focused"`
}

type niriOutput struct {
	Name    string `json:"name"`
	Logical *struct {
		X      int     `json:"x"`
		Y      int     `json:"y"`
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Scale  float64 `json:"scale"`
	} `json:"logical"`
}

type niriState struct {
	outputs    map[string]niriOutput
	workspaces []niriWorkspace
	windows    []niriWindow
}

type niri struct {
	run    Runner
	stream Stream
	// tiledPos is set once niri has placed a tiled window, so Caps reports
	// what this niri actually does rather than what its version promises.
	tiledPos atomic.Bool
}

func newNiri(run Runner, stream Stream) *niri {
	return &niri{run: guard(run), stream: guardStream(stream)}
}

// Niri reads the windows of the niri env names, and nothing else: no seam
// probe, no other desktop. Hands reads its own nested desktop with it, env
// naming only that desktop's runtime dir and sockets.
func Niri(env []string) Provider { return newNiri(Exec(env), ExecStream(env)) }

func (p *niri) Caps() Caps {
	geom := GeomFloating
	if p.tiledPos.Load() {
		geom = GeomAll
	}
	return Caps{Provider: ProviderNiri, Geometry: geom, Events: true}
}

func (p *niri) read(ctx context.Context, verb string, v any) error {
	out, err := p.run(ctx, ProviderNiri, "msg", "--json", verb)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("niri %s: %w", verb, err)
	}
	return nil
}

func (p *niri) Snapshot(ctx context.Context) (Snapshot, error) {
	var st niriState
	if err := p.read(ctx, "outputs", &st.outputs); err != nil {
		return Snapshot{}, err
	}
	if err := p.read(ctx, "workspaces", &st.workspaces); err != nil {
		return Snapshot{}, err
	}
	if err := p.read(ctx, "windows", &st.windows); err != nil {
		return Snapshot{}, err
	}
	return p.convert(st), nil
}

// Watch folds niri's event stream. Layout changes arrive as events, so a
// perched window's moves need no poll and Geometry is a no-op. niri sends no
// output event; outputs are re-read whenever the workspace set changes, which
// is what an output appearing or leaving does.
func (p *niri) Watch(ctx context.Context, fn func(Snapshot)) error {
	var st niriState
	if err := p.read(ctx, "outputs", &st.outputs); err != nil {
		return err
	}
	rc, err := p.stream(ctx, ProviderNiri, "msg", "--json", "event-stream")
	if err != nil {
		return err
	}
	haveWindows, haveSpaces := false, false
	e := emitter{fn: fn}
	return readLines(ctx, rc, func(line []byte) error {
		var ev map[string]json.RawMessage
		if err := json.Unmarshal(line, &ev); err != nil {
			return err
		}
		changed := false
		for kind, body := range ev {
			ok, err := p.fold(ctx, &st, kind, body)
			if err != nil {
				return fmt.Errorf("niri %s: %w", kind, err)
			}
			changed = changed || ok
			haveWindows = haveWindows || kind == "WindowsChanged"
			haveSpaces = haveSpaces || kind == "WorkspacesChanged"
		}
		if changed && haveWindows && haveSpaces {
			e.emit(p.convert(st))
		}
		return nil
	})
}

func (p *niri) fold(ctx context.Context, st *niriState, kind string, body json.RawMessage) (bool, error) {
	switch kind {
	case "WorkspacesChanged":
		var v struct {
			Workspaces []niriWorkspace `json:"workspaces"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return false, err
		}
		st.workspaces = v.Workspaces
		outputs := map[string]niriOutput{}
		if err := p.read(ctx, "outputs", &outputs); err != nil {
			return false, err
		}
		st.outputs = outputs
	case "WorkspaceActivated":
		var v struct {
			ID      uint64 `json:"id"`
			Focused bool   `json:"focused"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return false, err
		}
		i := slices.IndexFunc(st.workspaces, func(w niriWorkspace) bool { return w.ID == v.ID })
		if i < 0 {
			return false, nil
		}
		out := st.workspaces[i].Output
		for j := range st.workspaces {
			w := &st.workspaces[j]
			if w.Output != nil && out != nil && *w.Output == *out {
				w.IsActive = j == i
			}
			if v.Focused {
				w.IsFocused = j == i
			}
		}
	case "WindowsChanged":
		var v struct {
			Windows []niriWindow `json:"windows"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return false, err
		}
		st.windows = v.Windows
	case "WindowOpenedOrChanged":
		var v struct {
			Window niriWindow `json:"window"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return false, err
		}
		i := slices.IndexFunc(st.windows, func(w niriWindow) bool { return w.ID == v.Window.ID })
		if i < 0 {
			st.windows = append(st.windows, v.Window)
		} else {
			st.windows[i] = v.Window
		}
		if v.Window.IsFocused {
			for j := range st.windows {
				st.windows[j].IsFocused = st.windows[j].ID == v.Window.ID
			}
		}
	case "WindowClosed":
		var v struct {
			ID uint64 `json:"id"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return false, err
		}
		st.windows = slices.DeleteFunc(st.windows, func(w niriWindow) bool { return w.ID == v.ID })
	case "WindowFocusChanged":
		var v struct {
			ID *uint64 `json:"id"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return false, err
		}
		for j := range st.windows {
			st.windows[j].IsFocused = v.ID != nil && st.windows[j].ID == *v.ID
		}
	case "WindowFocusTimestampChanged":
		var v struct {
			ID             uint64         `json:"id"`
			FocusTimestamp *niriTimestamp `json:"focus_timestamp"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return false, err
		}
		for j := range st.windows {
			if st.windows[j].ID == v.ID {
				st.windows[j].FocusTimestamp = v.FocusTimestamp
			}
		}
	case "WindowLayoutsChanged":
		var v struct {
			Changes [][2]json.RawMessage `json:"changes"`
		}
		if err := json.Unmarshal(body, &v); err != nil {
			return false, err
		}
		for _, c := range v.Changes {
			var id uint64
			var layout niriLayout
			if err := json.Unmarshal(c[0], &id); err != nil {
				return false, err
			}
			if err := json.Unmarshal(c[1], &layout); err != nil {
				return false, err
			}
			for j := range st.windows {
				if st.windows[j].ID == id {
					l := layout
					st.windows[j].Layout = &l
				}
			}
		}
	default:
		return false, nil
	}
	return true, nil
}

func (p *niri) Geometry(ID) {}

// Pointer: niri needs no pointer locate (its surfaces get pointer enter on
// map), and its IPC has no read for it.
func (p *niri) Pointer(context.Context) (spec.Point, bool, error) {
	return spec.Point{}, false, nil
}

func (p *niri) convert(st niriState) Snapshot {
	var s Snapshot
	for _, o := range st.outputs {
		if o.Logical == nil {
			continue
		}
		l := o.Logical
		s.Outputs = append(s.Outputs, Output{Name: o.Name, X: l.X, Y: l.Y, W: l.Width, H: l.Height, Scale: l.Scale})
	}
	slices.SortFunc(s.Outputs, func(a, b Output) int {
		if a.X != b.X {
			return a.X - b.X
		}
		return a.Y - b.Y
	})
	spaces := map[uint64]niriWorkspace{}
	for _, w := range st.workspaces {
		spaces[w.ID] = w
	}
	// Focus history: the focused window first, then by niri's focus
	// timestamps, newest first.
	order := slices.Clone(st.windows)
	slices.SortStableFunc(order, func(a, b niriWindow) int {
		switch {
		case a.IsFocused != b.IsFocused:
			if a.IsFocused {
				return -1
			}
			return 1
		case a.FocusTimestamp.after(b.FocusTimestamp):
			return -1
		case b.FocusTimestamp.after(a.FocusTimestamp):
			return 1
		}
		return 0
	})
	for i, nw := range order {
		w := Window{
			ID: ID(strconv.FormatUint(nw.ID, 10)), Floating: nw.IsFloating,
			Focused: nw.IsFocused, FocusOrder: i,
		}
		if nw.Title != nil {
			w.Title = *nw.Title
		}
		if nw.AppID != nil {
			w.App = *nw.AppID
		}
		if nw.PID != nil {
			w.PID = *nw.PID
		}
		if nw.WorkspaceID != nil {
			if ws, ok := spaces[*nw.WorkspaceID]; ok {
				w.Visible = ws.IsActive
				if ws.Output != nil {
					w.Output = *ws.Output
				}
			}
		}
		_, haveOut := outputByName(s.Outputs, w.Output)
		var pos *[2]float64
		if nw.Layout != nil {
			pos = nw.Layout.TilePosInWorkspaceView
		}
		switch {
		case !w.Visible:
			w.Unplaced = UnplacedOffView
		case pos != nil && haveOut:
			l := nw.Layout
			w.Box = &Box{
				X: pos[0] + l.WindowOffsetInTile[0], Y: pos[1] + l.WindowOffsetInTile[1],
				W: float64(l.WindowSize[0]), H: float64(l.WindowSize[1]),
			}
			if !nw.IsFloating {
				p.tiledPos.Store(true)
			}
		case !nw.IsFloating:
			w.Unplaced = UnplacedTiled
		default:
			w.Unplaced = UnplacedNoGeom
		}
		s.Windows = append(s.Windows, w)
	}
	sortWindows(s.Windows)
	return s
}
