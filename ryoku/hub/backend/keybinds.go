package main

import (
	"encoding/json"
	"os"

	wm "ryoku-wm"
)

// The keybind legend the Hub keybinds page and the Super+K cheatsheet draw. It
// is the effective legend the active window-manager provider reports through the
// seam: the shipped Ryoku catalogue resolved against the user's rebinds, each
// row struck or annotated where the running compositor cannot honour it, then
// that compositor's own exclusive binds and the user's custom binds. The
// provider owns the parse now, so the two surfaces read one source instead of
// each parsing a compositor's own config and drifting apart.

// bind is one legend row for the two surfaces: every wm.BindRow field, plus desc
// and combo so the Hub page and the cheatsheet keep the field names they already
// read (desc is the row's label, combo the shipped default chord).
type bind struct {
	wm.BindRow
	Desc  string `json:"desc"`
	Combo string `json:"combo"`
}

type category struct {
	Name  string `json:"name"`
	Binds []bind `json:"binds"`
}

type legend struct {
	Categories  []category      `json:"categories"`
	GestureKeys []wm.GestureKey `json:"gestureKeys"`
}

// keybinds groups the provider's effective legend into sections in the order the
// rows arrive: a category opens the first time one of its rows appears, and every
// later row for it lands under that same header, so the sheet's sections read in
// catalogue order. On a provider error there is no honest legend to draw, so the
// result is the empty set and the Hub shows its empty state.
func keybinds() legend {
	l := legend{Categories: []category{}, GestureKeys: wm.GestureKeys}
	rows, err := wm.Open().Binds()
	if err != nil {
		return l
	}
	at := map[string]int{}
	tapKey, highlightKey, inPlaceKey := sightGestureKeys()
	for _, r := range rows {
		switch r.ID {
		case "shell.sight":
			r.Chord = tapKey
			r.Keys = wm.DisplayKeys(tapKey)
		case "shell.sightHighlight":
			r.Chord = highlightKey
			r.Keys = wm.DisplayKeys(highlightKey)
		case "shell.sightInPlace":
			r.Chord = inPlaceKey
			r.Keys = wm.DisplayKeys(inPlaceKey)
		}
		i, ok := at[r.Category]
		if !ok {
			i = len(l.Categories)
			at[r.Category] = i
			l.Categories = append(l.Categories, category{Name: r.Category, Binds: []bind{}})
		}
		l.Categories[i].Binds = append(l.Categories[i].Binds, bind{
			BindRow: r,
			Desc:    r.Label,
			Combo:   r.Default,
		})
	}
	return l
}

func sightGestureKeys() (string, string, string) {
	var settings struct {
		Sight struct {
			TapKey       string `json:"tapKey"`
			HighlightKey string `json:"highlightKey"`
			InPlace      struct {
				Key string `json:"key"`
			} `json:"inplace"`
		} `json:"sight"`
	}
	if raw, err := os.ReadFile(shellStorePath()); err == nil {
		_ = json.Unmarshal(raw, &settings)
	}
	allowed := func(name, fallback string) string {
		for _, key := range wm.GestureKeys {
			if key.Name == name {
				return name
			}
		}
		return fallback
	}
	return allowed(settings.Sight.TapKey, "Alt_R"), allowed(settings.Sight.HighlightKey, "Alt_L"), allowed(settings.Sight.InPlace.Key, "Control_R")
}
