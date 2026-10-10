package main

import (
	"embed"
	"encoding/json"
	"sort"
	"strings"
)

//go:embed schema*.json
var schemaFS embed.FS

// runSchema prints wayfire's exclusive settings rows, in the shape the Hub's
// settings renderer already consumes. The rows live in topical files
// (schema.json for the depth deck, schema_anim.json for the animation
// durations and curves), one concern each; this concatenates them into the
// single rows array the Hub expects, so wayfire's exclusives surface without
// a Hub edit, the same way the siblings' do. Each row carries its own page
// tag, so the depth-deck rows land on the shared window-manager page while
// the animation rows reach the animation page.
func runSchema() error {
	rows, err := loadSchemaRows()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// loadSchemaRows reads every embedded schema*.json array and concatenates them
// in a stable filename order, so the row set is deterministic across builds.
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
