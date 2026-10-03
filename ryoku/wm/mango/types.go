package main

import (
	"strconv"

	wm "ryoku-wm"
)

// The wire structs every read path shares. Field names are mango's own
// (src/ipc/ipc.c build_client_json / build_monitor_json / build_tags_json) and
// are pinned by wire_test against captured live replies.

type mangoClient struct {
	ID           int     `json:"id"`
	PID          int     `json:"pid"`
	Title        string  `json:"title"`
	AppID        string  `json:"appid"`
	Monitor      string  `json:"monitor"`
	Tags         []int   `json:"tags"`
	IsXWayland   bool    `json:"is_xwayland"`
	IsVisible    bool    `json:"is_visible"`
	IsFocused    bool    `json:"is_focused"`
	IsFullscreen bool    `json:"is_fullscreen"`
	IsFloating   bool    `json:"is_floating"`
	IsMaximized  bool    `json:"is_maximized"`
	IsGlobal     bool    `json:"is_global"`
	IsUrgent     bool    `json:"is_urgent"`
	IsMinimized  bool    `json:"is_minimized"`
	IsScratchpad bool    `json:"is_scratchpad"`
	X            int     `json:"x"`
	Y            int     `json:"y"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Proportion   float64 `json:"scroller_proportion"`
}

type mangoClientRef struct {
	ID *int `json:"id"`
}

type mangoTag struct {
	Index       int    `json:"index"`
	IsActive    bool   `json:"is_active"`
	IsUrgent    bool   `json:"is_urgent"`
	Layout      string `json:"layout"`
	ClientCount int    `json:"client_count"`
}

type mangoMonitor struct {
	Name         string         `json:"name"`
	Active       bool           `json:"active"`
	IsHDR        bool           `json:"is_hdr"`
	IsVRR        bool           `json:"is_vrr"`
	X            int            `json:"x"`
	Y            int            `json:"y"`
	Width        int            `json:"width"`
	Height       int            `json:"height"`
	Scale        float64        `json:"scale"`
	LayoutSymbol string         `json:"layout_symbol"`
	ActiveClient mangoClientRef `json:"active_client"`
	Tags         []mangoTag     `json:"tags"`
}

// clientsEnvelope wraps `get all-clients`; monitorsEnvelope wraps
// `get all-monitors`; tagsEnvelope wraps `get all-tags`; layoutEnvelope wraps
// `get layouts`.
type clientsEnvelope struct {
	Clients []mangoClient `json:"clients"`
}

type monitorsEnvelope struct {
	Monitors []mangoMonitor `json:"monitors"`
}

type tagsEnvelope struct {
	AllTags []struct {
		Monitor string     `json:"monitor"`
		Tags    []mangoTag `json:"tags"`
	} `json:"all_tags"`
}

type layoutEnvelope struct {
	Layouts []struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
	} `json:"layouts"`
}

// layoutNames maps a mango layout symbol ("T", "S", "G", …) to its
// config-name spelling ("tile", "scroller", "grid", …) as `get layouts`
// reports it. Tag frames carry symbols; setlayout wants names.
func layoutNames(env layoutEnvelope) map[string]string {
	out := map[string]string{}
	for _, l := range env.Layouts {
		out[l.Symbol] = l.Name
	}
	return out
}

func specialWorkspaceFrame(m mangoMonitor, clients []mangoClient, layout string) wm.Workspace {
	frame := wm.Workspace{
		ID:      "0",
		Name:    "0",
		Output:  m.Name,
		Special: true,
		Layout:  layout,
	}
	for _, c := range clients {
		if c.Monitor != m.Name {
			continue
		}
		onSpecial := false
		for _, tag := range c.Tags {
			if tag == 0 {
				onSpecial = true
				break
			}
		}
		if !onSpecial {
			continue
		}
		frame.Windows++
		frame.Active = frame.Active || c.IsVisible
	}
	return frame
}

// clientFrame renders one window. mango reports full on-screen geometry on
// every client (verified against a live instance), so CapWindowGeometry is
// honoured and consumers can draw windows where they are. FocusOrder comes
// from the caller's observed or snapshot ordering.
func clientFrame(c mangoClient, focusOrder int, currentTag map[string]int) wm.Window {
	w := wm.Window{
		ID:         strconv.Itoa(c.ID),
		AppID:      c.AppID,
		Title:      c.Title,
		FocusOrder: focusOrder,
		Floating:   c.IsFloating,
		X:          c.X,
		Y:          c.Y,
		Width:      c.Width,
		Height:     c.Height,
		Output:     c.Monitor,
	}
	// A window's tags[] is the set of tags it lives on. The workspace a pill
	// counts it on is the monitor's currently viewed tag when the window shares
	// it; otherwise its lowest tag. Tag 0 is mango's special workspace handle,
	// and stays 0 exactly as ordinary workspace handles stay numeric text.
	for _, t := range c.Tags {
		if t == 0 {
			w.Workspace = "0"
			return w
		}
	}
	tag, haveTag := 0, false
	for _, t := range c.Tags {
		if !haveTag || t < tag {
			tag, haveTag = t, true
		}
	}
	if cur, ok := currentTag[c.Monitor]; ok {
		for _, t := range c.Tags {
			if t == cur {
				tag, haveTag = t, true
				break
			}
		}
	}
	if haveTag {
		w.Workspace = strconv.Itoa(tag)
	}
	return w
}
