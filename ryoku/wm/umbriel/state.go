package main

import (
	"encoding/json"
	"fmt"

	wm "ryoku-wm"
)

type uOutput struct {
	Name        string `json:"name"`
	LogicalSize struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"logical_size"`
	Position struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"position"`
	Scale           float64 `json:"scale"`
	Focused         bool    `json:"focused"`
	ActiveWorkspace string  `json:"active_workspace"`
	Enabled         bool    `json:"enabled"`
	Transform       string  `json:"transform"`
	VRR             *bool   `json:"adaptive_sync"`
	Modes           []struct {
		Width      int  `json:"width"`
		Height     int  `json:"height"`
		RefreshMHz int  `json:"refresh_mhz"`
		Current    bool `json:"current"`
	} `json:"modes"`
}

type uWorkspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Output      string `json:"output"`
	Index       int    `json:"index"`
	Active      bool   `json:"active"`
	WindowCount int    `json:"window_count"`
	Layout      string `json:"layout"`
}

type uWindow struct {
	ID         string `json:"id"`
	AppID      string `json:"app_id"`
	Title      string `json:"title"`
	Workspace  string `json:"workspace"`
	Output     string `json:"output"`
	Pid        int    `json:"pid"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	W          int    `json:"w"`
	H          int    `json:"h"`
	Floating   bool   `json:"floating"`
	Focused    bool   `json:"focused"`
	Fullscreen bool   `json:"fullscreen"`
}

type uState struct {
	Outputs    []uOutput    `json:"outputs"`
	Workspaces []uWorkspace `json:"workspaces"`
	Windows    []uWindow    `json:"windows"`
}

// activeWorkspaceName maps Umbriel's workspace id (e.g. "eDP-1:1") to its
// display name (e.g. "1"). Consumers such as the shell overview key the active
// workspace numerically, so publishing the id there makes it resolve to -1.
func activeWorkspaceName(s uState, id string) string {
	for _, w := range s.Workspaces {
		if w.ID == id {
			return w.Name
		}
	}
	return ""
}

func outputsOf(s uState) []wm.Output {
	out := make([]wm.Output, 0, len(s.Outputs))
	for _, o := range s.Outputs {
		v := wm.Output{
			Name: o.Name, Width: o.LogicalSize.Width, Height: o.LogicalSize.Height,
			Scale: o.Scale, Focused: o.Focused, ActiveWorkspace: o.ActiveWorkspace,
			X: o.Position.X, Y: o.Position.Y, Disabled: !o.Enabled,
			Transform: transformOf(o.Transform),
		}
		if o.VRR != nil {
			v.VRR = *o.VRR
		}
		for _, m := range o.Modes {
			mode := fmt.Sprintf("%dx%d", m.Width, m.Height)
			if m.RefreshMHz > 0 {
				mode += fmt.Sprintf("@%g", float64(m.RefreshMHz)/1000)
			}
			v.Modes = append(v.Modes, mode)
			if m.Current {
				v.Mode = mode
			}
		}
		out = append(out, v)
	}
	return out
}

func transformOf(name string) int {
	switch name {
	case "90":
		return 1
	case "180":
		return 2
	case "270":
		return 3
	default:
		return 0
	}
}

func workspacesOf(s uState) []wm.Workspace {
	out := make([]wm.Workspace, 0, len(s.Workspaces))
	for _, w := range s.Workspaces {
		out = append(out, wm.Workspace{
			ID: w.ID, Name: w.Name, Output: w.Output, Index: w.Index,
			Active: w.Active, Windows: w.WindowCount, Layout: w.Layout,
		})
	}
	return out
}

func windowsOf(s uState) []wm.Window {
	out := make([]wm.Window, 0, len(s.Windows))
	for i, w := range s.Windows {
		out = append(out, wm.Window{
			ID: w.ID, AppID: w.AppID, Title: w.Title, Workspace: w.Workspace,
			Output: w.Output, Pid: w.Pid, X: w.X, Y: w.Y, Width: w.W, Height: w.H,
			Floating: w.Floating, FocusOrder: i,
		})
	}
	return out
}

func focusedOutput(s uState) string {
	for _, o := range s.Outputs {
		if o.Focused {
			return o.Name
		}
	}
	return ""
}

func runState() error {
	var s uState
	for _, family := range []string{"outputs", "workspaces", "windows"} {
		raw, err := query(family)
		if err != nil {
			return err
		}
		switch family {
		case "outputs":
			err = json.Unmarshal(raw, &s.Outputs)
		case "workspaces":
			err = json.Unmarshal(raw, &s.Workspaces)
		case "windows":
			err = json.Unmarshal(raw, &s.Windows)
		}
		if err != nil {
			return err
		}
	}
	return json.NewEncoder(stdout).Encode(wm.Snapshot{
		FocusedOutput: focusedOutput(s), Outputs: outputsOf(s),
		Workspaces: workspacesOf(s), Windows: windowsOf(s),
	})
}
