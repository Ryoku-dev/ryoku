package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The Hub draws these rows on shared pages, so a row that names a key the
// store does not have becomes a control that silently does nothing, and a row
// missing its page or control becomes one it cannot draw. This pins every row
// to its default, its page and a control the renderer knows.
func TestSchemaRowsResolveAgainstStore(t *testing.T) {
	rows, err := loadSchemaRows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no schema rows")
	}
	defaults, err := splitStore(defaultStore())
	if err != nil {
		t.Fatal(err)
	}
	blob, err := json.Marshal(defaults)
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(blob, &tree); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, raw := range rows {
		var row struct {
			Key, Label, Desc, Ctl, Src, Page, Tab, Group string
			Opts                                         []string
			OptLabels                                    map[string]string
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatalf("row does not decode: %v", err)
		}
		switch {
		case row.Key == "":
			t.Error("row without a key")
		case !strings.HasPrefix(row.Key, "wm.wayfire."):
			t.Errorf("row key %q is outside the wayfire namespace", row.Key)
		case seen[row.Key]:
			t.Errorf("row key %q appears twice", row.Key)
		}
		seen[row.Key] = true
		if row.Label == "" || row.Desc == "" || row.Tab == "" || row.Group == "" {
			t.Errorf("row %q is missing its label, desc, tab or group", row.Key)
		}
		if row.Page == "" || row.Src != "desktop.json" {
			t.Errorf("row %q must name its page and read desktop.json, got page=%q src=%q",
				row.Key, row.Page, row.Src)
		}

		switch row.Ctl {
		case "sw", "slid", "step":
		case "chips":
			if len(row.Opts) == 0 {
				t.Errorf("row %q offers chips but no options", row.Key)
			}
			for _, o := range row.Opts {
				if row.OptLabels[o] == "" {
					t.Errorf("row %q has no label for option %q", row.Key, o)
				}
			}
		default:
			t.Errorf("row %q uses unknown control %q", row.Key, row.Ctl)
		}

		var node any = tree
		for _, part := range strings.Split(row.Key, ".") {
			level, ok := node.(map[string]any)
			if !ok {
				t.Errorf("row key %q walks off the store at %q", row.Key, part)
				break
			}
			if node, ok = level[part]; !ok {
				t.Errorf("row key %q has no default in the store", row.Key)
				break
			}
		}
	}
}
