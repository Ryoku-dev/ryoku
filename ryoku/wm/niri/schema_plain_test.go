package main

import (
	"encoding/json"
	"regexp"
	"testing"

	wm "ryoku-wm"
)

// This window manager loads no plugins and Settings offers no way to import a
// sound pack, so no row it shows may promise either.
func TestSchemaPromisesNoPluginsOrImports(t *testing.T) {
	if (wm.Caps{Supports: capsManifest}).Has(wm.CapPlugins) {
		t.Skip("claims plugins")
	}
	rows, err := loadSchemaRows()
	if err != nil {
		t.Fatal(err)
	}
	promise := regexp.MustCompile(`(?i)\bplug-?ins?\b|\bimport`)
	for _, raw := range rows {
		var row struct {
			Key, Label, Desc string
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{row.Label, row.Desc} {
			if m := promise.FindString(s); m != "" {
				t.Errorf("row %q: %q mentions %q", row.Key, s, m)
			}
		}
	}
}
