package main

import (
	"embed"
	"encoding/json"
	"sort"
	"strings"

	wm "ryoku-wm"
)

//go:embed schema*.json
var schemaFS embed.FS

// runSchema prints niri's exclusive settings rows, in the shape the Hub's
// settings renderer already consumes. The rows live in topical files
// (schema.json plus schema_layout.json, schema_input.json, schema_anim.json,
// schema_rules.json), one concern each; this concatenates them into the single
// rows array the Hub expects, so niri's exclusives surface without a Hub edit,
// the same way a third compositor's would. Each row carries its own page tag,
// so window-manager rows land on the shared window-manager page while the
// animation, layer-rule and window-rule rows reach their bespoke pages.
func runSchema() error {
	rows, err := loadSchemaRows()
	if err != nil {
		return err
	}
	rows = wm.WithInstalledKeysounds(rows, profileRowKey, wm.KeysoundsUserDir(), wm.KeysoundsShippedDir)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// The key-sounds profile row offers what is installed, not only the names
// written in the schema, so a pack the user adds is selectable without a schema
// edit.
const profileRowKey = "wm.niri.keysounds.profile"

// loadSchemaRows reads every embedded schema*.json array and concatenates them in
// a stable filename order, so the row set is deterministic across builds.
func loadSchemaRows() ([]json.RawMessage, error) {
	entries, err := schemaFS.ReadDir(".")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, "schema") && strings.HasSuffix(n, ".json") {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	rows := []json.RawMessage{}
	for _, n := range names {
		b, err := schemaFS.ReadFile(n)
		if err != nil {
			return nil, err
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(b, &arr); err != nil {
			return nil, err
		}
		rows = append(rows, arr...)
	}
	return rows, nil
}
