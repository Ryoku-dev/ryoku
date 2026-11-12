package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	wm "ryoku-wm"
)

// Wayfire's maximize fills the bare work area and restores a window that
// opened maximized to its 100x100 placeholder, so Ryoku fits instead: the work
// area inside the outer gaps and the decor border, as niri and Hyprland do.

const (
	fitTolerance  = 2
	restoreWidth  = 1000
	restoreHeight = 660
)

type fitInsets struct {
	horiz, vert float64
}

func readFitInsets() fitInsets {
	border := configNumber("ryoku-decor/border_width")
	return fitInsets{
		horiz: configNumber("simple-tile/outer_horiz_gap_size") + border,
		vert:  configNumber("simple-tile/outer_vert_gap_size") + border,
	}
}

// configNumber reads one numeric option; an unloaded plugin's option is zero.
func configNumber(name string) float64 {
	raw, err := request("wayfire/get-config-option", map[string]any{"option": name})
	if err != nil {
		return 0
	}
	var reply struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &reply) != nil {
		return 0
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(reply.Value), 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func workArea(out wayfireOutput) wayfireGeometry {
	if out.Workarea.Width > 0 && out.Workarea.Height > 0 {
		return out.Workarea
	}
	return wayfireGeometry{Width: out.Geometry.Width, Height: out.Geometry.Height}
}

// fitRect is output-local, the space view geometry is reported in.
func fitRect(out wayfireOutput, in fitInsets) wayfireGeometry {
	wa := workArea(out)
	return wayfireGeometry{
		X:      math.Round(wa.X + in.horiz),
		Y:      math.Round(wa.Y + in.vert),
		Width:  math.Round(math.Max(wa.Width-2*in.horiz, 1)),
		Height: math.Round(math.Max(wa.Height-2*in.vert, 1)),
	}
}

func restoreRect(out wayfireOutput) wayfireGeometry {
	wa := workArea(out)
	w := math.Min(restoreWidth, math.Round(wa.Width*0.92))
	h := math.Min(restoreHeight, math.Round(wa.Height*0.88))
	return wayfireGeometry{
		X:      math.Round(wa.X + (wa.Width-w)/2),
		Y:      math.Round(wa.Y + (wa.Height-h)/2),
		Width:  w,
		Height: h,
	}
}

func nearRect(a, b wayfireGeometry) bool {
	return math.Abs(a.X-b.X) <= fitTolerance && math.Abs(a.Y-b.Y) <= fitTolerance &&
		math.Abs(a.Width-b.Width) <= fitTolerance && math.Abs(a.Height-b.Height) <= fitTolerance
}

func isMaximized(v wayfireView, fit wayfireGeometry) bool {
	return v.TiledEdges == 15 || nearRect(v.Geometry, fit)
}

// usableRestore rejects a remembered size too small to use or off screen.
func usableRestore(r wayfireGeometry, out wayfireOutput) bool {
	wa := workArea(out)
	if r.Width < 320 || r.Height < 240 {
		return false
	}
	return r.X < wa.X+wa.Width && r.X+r.Width > wa.X && r.Y < wa.Y+wa.Height && r.Y+r.Height > wa.Y
}

// maximizeFocused toggles a floating window between the fit and the size it
// had. A tiled leaf is left to simple-tile's own toggle on the same chord.
func maximizeFocused(act wm.Action) error {
	st, err := readStage()
	if err != nil {
		return err
	}
	origin, err := focusedWindow(act)
	if err != nil {
		return err
	}
	if origin.Fullscreen {
		return nil
	}
	_, layout, hasLayout, err := readSimpleTileLayout(st, origin)
	if err != nil {
		return err
	}
	if hasLayout && layout.containsView(origin.ID) {
		return nil
	}
	out, ok := st.outputByName(origin.OutputName)
	if !ok {
		return fmt.Errorf("act %s: the window's screen is gone", act)
	}
	fit := fitRect(out, readFitInsets())

	saved := readRestoreState(st.views)
	key := strconv.FormatInt(origin.ID, 10)
	target := fit
	if isMaximized(origin, fit) {
		back, ok := saved[key]
		delete(saved, key)
		if !ok || !usableRestore(back, out) {
			back = restoreRect(out)
		}
		target = back
	} else {
		saved[key] = origin.Geometry
	}
	if err := writeRestoreState(saved); err != nil {
		return err
	}
	return configurePlaced(origin.ID, target)
}

// Remembered sizes live in the runtime dir: view ids restart with wayfire.
func restoreStatePath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "ryoku", "wayfire-maximize.json")
}

func readRestoreState(views []wayfireView) map[string]wayfireGeometry {
	state := map[string]wayfireGeometry{}
	if body, err := os.ReadFile(restoreStatePath()); err == nil {
		_ = json.Unmarshal(body, &state)
	}
	open := map[string]bool{}
	for _, v := range views {
		open[strconv.FormatInt(v.ID, 10)] = true
	}
	for id := range state {
		if !open[id] {
			delete(state, id)
		}
	}
	return state
}

func writeRestoreState(state map[string]wayfireGeometry) error {
	if err := os.MkdirAll(filepath.Dir(restoreStatePath()), 0o700); err != nil {
		return err
	}
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(restoreStatePath(), body, 0o600)
}
