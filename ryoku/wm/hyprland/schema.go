package main

import (
	_ "embed"
	"encoding/json"

	wm "ryoku-wm"
)

//go:embed schema.json
var schemaRows []byte

// profileRowKey is the key-sounds profile row, which offers what is installed,
// not only the names written in the schema, so a pack the user adds is
// selectable without a schema edit.
const profileRowKey = "wm.hyprland.plugins.keysounds.profile"

// runSchema prints the Hyprland exclusive settings rows, in the shape the Hub's
// settings renderer already consumes. The rows are the single source: they were
// hardcoded in the Hub, and moving them here is what lets a compositor's
// settings arrive by shipping this file rather than editing the Hub. Each row's
// `page` names the surface that draws it: the shared window-manager page, or a
// capability-gated bespoke page (plugins, animations, layerrules) that keeps its
// own interactions and reads its rows from here.
func runSchema() error {
	var rows []json.RawMessage
	if err := json.Unmarshal(schemaRows, &rows); err != nil {
		return err
	}
	rows = wm.WithInstalledKeysounds(rows, profileRowKey, wm.KeysoundsUserDir(), keysoundsShippedDir())
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}
