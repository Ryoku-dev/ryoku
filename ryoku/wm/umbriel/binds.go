package main

import (
	"encoding/json"
	"strings"

	wm "ryoku-wm"
)

// runBinds prints the effective keybind legend: every catalogue row with its
// chord resolved against the store's rebinds and disabled list. The Hub and
// Super+K both read this, so a rebind or a disabled chord is reflected in the
// sheet as well as in the generated rebinds.toml the compositor uses.
func runBinds(args []string) error {
	path := ""
	if len(args) > 0 {
		path = args[0]
	}
	store := map[string]any{}
	if path != "" {
		if s, err := readStore(path); err == nil {
			store = s
		}
	}
	rebinds, disabled := storeBinds(store)
	off := map[string]bool{}
	for _, chord := range disabled {
		for _, alias := range chordAliases(chord) {
			off[alias] = true
		}
	}

	shipped := wm.ShippedBinds()
	rows := make([]wm.BindRow, 0, len(shipped))
	for _, b := range shipped {
		eff := b.Chord
		if b.Family {
			if to, ok := rebinds[b.Chord]; ok && strings.TrimSpace(to) != "" {
				if _, changed := wm.FamilyRebind(b.Chord, rebinds); changed {
					eff = to
				}
			}
		} else if to, ok := rebinds[b.Chord]; ok && strings.TrimSpace(to) != "" {
			eff = to
		}
		row := wm.BindRow{
			ID:                b.ID,
			Category:          b.Category,
			Label:             b.Label,
			Hint:              b.Hint,
			Keys:              wm.DisplayKeys(eff),
			Default:           b.Chord,
			Chord:             eff,
			Kind:              b.Kind,
			Taps:              b.Taps,
			WhileHighlighting: b.WhileHighlighting,
			Rebindable:        !b.Locked,
			Locked:            b.Locked,
		}
		if len(chordAliases(b.Chord)) > 0 && off[chordAliases(b.Chord)[0]] {
			row.Disabled = true
			row.Keys = nil
			row.Chord = ""
		}
		rows = append(rows, row)
	}
	return json.NewEncoder(stdout).Encode(rows)
}
