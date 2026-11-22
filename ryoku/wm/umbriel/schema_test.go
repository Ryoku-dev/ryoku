package main

import (
	"encoding/json"
	"testing"
)

func TestSchemaRowsUseUmbrielDomain(t *testing.T) {
	body, err := schemaFS.ReadFile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 7 {
		t.Fatalf("got %d schema rows, want 7", len(rows))
	}
	for _, row := range rows {
		key, _ := row["key"].(string)
		if len(key) < len("wm.umbriel.") || key[:len("wm.umbriel.")] != "wm.umbriel." {
			t.Fatalf("schema key %q is not in the Umbriel domain", key)
		}
	}
}
