package main

import (
	"encoding/json"

	wm "ryoku-wm"
)

// runBinds prints the effective keybind legend. Until apply generates rebinds
// from desktop.json, the shipped chord is the effective chord. The Hub and
// Super+K both read this, so the legend matches the catalogue rather than the
// empty set the test provider used to return.
func runBinds([]string) error {
	shipped := wm.ShippedBinds()
	rows := make([]wm.BindRow, 0, len(shipped))
	for _, b := range shipped {
		rows = append(rows, wm.BindRow{
			ID:                b.ID,
			Category:          b.Category,
			Label:             b.Label,
			Hint:              b.Hint,
			Keys:              wm.DisplayKeys(b.Chord),
			Default:           b.Chord,
			Chord:             b.Chord,
			Kind:              b.Kind,
			Taps:              b.Taps,
			WhileHighlighting: b.WhileHighlighting,
			Rebindable:        !b.Locked,
			Locked:            b.Locked,
		})
	}
	return json.NewEncoder(stdout).Encode(rows)
}
