package main

import (
	"encoding/json"
	"strings"

	wm "ryoku-wm"
)

// runBinds prints the effective keybind legend: every catalogue row with its
// chord resolved by the same code that writes rebinds.toml. A rebind the writer
// rejected, or a disabled chord, is reported the same way here, so the Hub and
// Super+K show what the compositor actually does.
func runBinds(args []string) error {
	path := ""
	if len(args) > 0 {
		path = args[0]
	}
	store := map[string]any{}
	if path != "" {
		s, err := readStore(path)
		if err != nil {
			return err
		}
		store = s
	}
	rebinds, disabled := storeBinds(store)
	lines, unhonored := resolvedBinds(rebinds, disabled)
	effective := map[string]string{}
	reason := map[string]string{}
	for _, l := range lines {
		effective[l.chord] = l.value
	}
	for _, u := range unhonored {
		reason[strings.TrimPrefix(u.Key, "desktop.keybindRebinds.")] = u.Reason
	}

	shipped := wm.ShippedBinds()
	rows := make([]wm.BindRow, 0, len(shipped))
	for _, b := range shipped {
		eff := b.Chord
		if to, ok := rebinds[b.Chord]; ok {
			// Only a chord that survived the writer's resolution is effective;
			// a rejected rebind keeps the shipped chord.
			if _, kept := effective[mustUmbrielChord(to)]; kept {
				eff = to
			} else if _, kept := effective[mustUmbrielChord(b.Chord)]; !kept {
				// The original was dropped (rebound onto a rejected target).
				eff = b.Chord
			}
		}
		if _, present := effective[mustUmbrielChord(b.Chord)]; !present {
			if _, viaRebind := rebinds[b.Chord]; !viaRebind {
				// Disabled: no effective line for this shipped chord.
				rows = append(rows, wm.BindRow{
					ID: b.ID, Category: b.Category, Label: b.Label, Hint: b.Hint,
					Default: b.Chord, Kind: b.Kind, Taps: b.Taps,
					WhileHighlighting: b.WhileHighlighting,
					Rebindable:        !b.Locked, Locked: b.Locked, Disabled: true,
				})
				continue
			}
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
		if r, ok := reason[b.Chord]; ok {
			row.Unhonored = r
		}
		rows = append(rows, row)
	}
	return json.NewEncoder(stdout).Encode(rows)
}

func mustUmbrielChord(chord string) string {
	c, ok := umbrielChord(chord)
	if !ok {
		return chord
	}
	return c
}
