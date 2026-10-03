package main

import (
	"encoding/json"
	"sort"
	"strconv"

	wm "ryoku-wm"
)

// runState is one snapshot for callers that ask once and exit: three gets
// (monitors, clients, layouts) folded into the neutral shape. mango answers per
// query, so there is no stream to wait on and no replay to fold.
func runState() error {
	snap, err := snapshot()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}

// snapshot reads the whole picture once. Shared by state and by the watch
// session's initial sweep.
func snapshot() (wm.Snapshot, error) {
	var mons monitorsEnvelope
	if err := get("all-monitors", &mons); err != nil {
		return wm.Snapshot{}, err
	}
	var clients clientsEnvelope
	if err := get("all-clients", &clients); err != nil {
		return wm.Snapshot{}, err
	}
	var layouts layoutEnvelope
	if err := get("layouts", &layouts); err != nil {
		return wm.Snapshot{}, err
	}
	names := layoutNames(layouts)

	var focusedOut string
	randr := readRandr()
	outputs := make([]wm.Output, 0, len(mons.Monitors))
	workspaceCount := len(mons.Monitors)
	for _, m := range mons.Monitors {
		workspaceCount += len(m.Tags)
	}
	workspaces := make([]wm.Workspace, 0, workspaceCount)
	for _, m := range mons.Monitors {
		if m.Active {
			focusedOut = m.Name
		}
		outputs = append(outputs, outputFrameWithRandr(m, true, randr))
		for _, t := range m.Tags {
			workspaces = append(workspaces, wm.Workspace{
				ID:      strconv.Itoa(t.Index),
				Name:    strconv.Itoa(t.Index),
				Output:  m.Name,
				Active:  t.IsActive,
				Windows: t.ClientCount,
				Layout:  names[t.Layout],
			})
		}
		workspaces = append(workspaces, specialWorkspaceFrame(m, clients.Clients, names[m.LayoutSymbol]))
	}

	sort.Slice(clients.Clients, func(i, j int) bool {
		return clients.Clients[i].ID < clients.Clients[j].ID
	})
	focusedID, haveFocused := focusedSnapshotClient(mons.Monitors, clients.Clients)
	windows := make([]wm.Window, 0, len(clients.Clients))
	nextOrder := 1
	for _, c := range clients.Clients {
		order := nextOrder
		if haveFocused && c.ID == focusedID {
			order = 0
		} else {
			nextOrder++
		}
		windows = append(windows, clientFrame(c, order, currentTags(mons.Monitors)))
	}

	kbLayout, kbLayouts := keyboardLayouts()
	return wm.Snapshot{
		FocusedOutput:   focusedOut,
		Outputs:         outputs,
		Workspaces:      workspaces,
		Windows:         windows,
		KeyboardLayout:  kbLayout,
		KeyboardLayouts: kbLayouts,
	}, nil
}

func focusedSnapshotClient(mons []mangoMonitor, clients []mangoClient) (int, bool) {
	for _, m := range mons {
		if m.Active && m.ActiveClient.ID != nil {
			return *m.ActiveClient.ID, true
		}
	}
	for _, c := range clients {
		if c.IsFocused {
			return c.ID, true
		}
	}
	return 0, false
}

// currentTags maps each monitor to its active tag index, so a window shown on
// the current view reports the workspace the bar is looking at rather than its
// lowest tag number.
func currentTags(mons []mangoMonitor) map[string]int {
	out := map[string]int{}
	for _, m := range mons {
		out[m.Name] = activeTagIndex(m.Tags)
	}
	return out
}

func activeTagIndex(tags []mangoTag) int {
	for _, t := range tags {
		if t.IsActive {
			return t.Index
		}
	}
	return 0
}

// outputFrame renders one mango monitor. full adds the editor detail
// (advertised modes, position, transform, EDID identity), which mango does NOT
// carry in IPC: it is read from wlr-randr over the live display, because mango
// serves wlr-output-management-v1 (verified in src/main.c).
func outputFrame(m mangoMonitor, full bool) wm.Output {
	randr := randrRead{state: randrUnavailable}
	if full {
		randr = readRandr()
	}
	return outputFrameWithRandr(m, full, randr)
}

func outputFrameWithRandr(m mangoMonitor, full bool, randr randrRead) wm.Output {
	o := wm.Output{
		Name:            m.Name,
		Width:           m.Width,
		Height:          m.Height,
		Scale:           m.Scale,
		X:               m.X,
		Y:               m.Y,
		Focused:         m.Active,
		VRR:             m.IsVRR,
		ActiveWorkspace: strconv.Itoa(activeTagIndex(m.Tags)),
	}
	if !full || randr.state == randrUnavailable {
		return o
	}
	// Physical size is not in wlr-randr's output; the EDID make and model are
	// what doctor's phantom-output heuristic reads.
	if d, ok := randr.detail(m.Name); ok {
		o.X, o.Y = d.x, d.y
		o.Transform = d.transform
		o.Mode = d.mode
		o.Modes = d.modes
		o.Make, o.Model = d.make, d.model
		o.Disabled = !d.enabled
	}
	return o
}

func keyboardLayouts() (string, []string) {
	var k struct {
		Layout string `json:"layout"`
	}
	if err := get("keyboardlayout", &k); err != nil {
		return "", nil
	}
	// `get keyboardlayout` returns only the ACTIVE layout name, not the loaded
	// set, and the name is human-readable ("English (US)"), not an XKB id. The
	// loaded set is read from the machine seed (xkb_rules_layout) instead, so the
	// bar indicator can still list what switching cycles through.
	return k.Layout, seededLayouts()
}
