package main

import (
	"encoding/json"
)

// pageTitles are the full-page windows fitted to the work area on open, as
// Hyprland's window.open hook fits them. Their window rule maximizes them
// first, so without this verb they still open inside the work area.
var pageTitles = map[string]bool{"Ryoku Settings": true}

// runPages fits each page window once when it maps or takes its title, then
// exits with the compositor that autostarted it.
func runPages() error {
	conn, err := dialWatch()
	if err != nil {
		return err
	}
	defer conn.Close()

	fitted := map[int64]bool{}
	for {
		frame, err := readFrame(conn)
		if err != nil {
			return nil
		}
		var e struct {
			Event string      `json:"event"`
			View  wayfireView `json:"view"`
		}
		if json.Unmarshal(frame, &e) != nil {
			continue
		}
		switch e.Event {
		case "view-unmapped":
			delete(fitted, e.View.ID)
		case "view-mapped", "view-title-changed":
			if !e.View.isWindow() || fitted[e.View.ID] || !pageTitles[e.View.Title] {
				continue
			}
			fitted[e.View.ID] = true
			_ = fitPage(e.View)
		}
	}
}

func fitPage(v wayfireView) error {
	out, err := readOutput(v.OutputName)
	if err != nil {
		return err
	}
	return configurePlaced(v.ID, fitRect(out, readFitInsets()))
}
