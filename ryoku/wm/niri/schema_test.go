package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The schema verb's sound-profile picker lists what is installed, so a pack the
// user adds can be chosen; the shipped names stay and nothing is listed twice.
func TestSchemaListInstalledProfiles(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	if err := os.MkdirAll(filepath.Join(data, "ryoku", "keysounds", "my-pack"), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	prev := stdout
	stdout = bufio.NewWriter(&buf)
	defer func() { stdout = prev }()
	if err := runSchema(); err != nil {
		t.Fatal(err)
	}
	stdout.Flush()
	var rows []json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	opts := profileRowOpts(t, rows)

	for _, name := range []string{"my-pack", "cherry-mx-brown"} {
		if !contains(opts, name) {
			t.Errorf("profile %q missing from %v", name, opts)
		}
	}
	seen := map[string]bool{}
	for _, o := range opts {
		if seen[o] {
			t.Errorf("profile %q listed twice in %v", o, opts)
		}
		seen[o] = true
	}
}

// the picker shows these labels; without one a shipped switch reads as its raw
// directory name ("Cherry-mx-brown")
func TestSchemaLabelsShippedProfiles(t *testing.T) {
	row := schemaRow(t, profileRowKey)
	if len(row.Opts) == 0 {
		t.Fatal("profile row ships no options")
	}
	for _, o := range row.Opts {
		if row.OptLabels[o] == "" {
			t.Errorf("shipped profile %q has no label", o)
		}
	}
}

// Screen capture's own folder row is titled "Save screenshots to"; this row
// covers niri's built-in screenshot only, so it must not read as the same one.
func TestSchemaScreenshotPathNamesItsOwner(t *testing.T) {
	if got := schemaRow(t, "wm.niri.screenshotPath").Label; got == "Save screenshots to" {
		t.Errorf("niri's screenshot row reuses the screenshot tool's title %q", got)
	}
}

type schemaRowFields struct {
	Key       string            `json:"key"`
	Label     string            `json:"label"`
	Opts      []string          `json:"opts"`
	OptLabels map[string]string `json:"optLabels"`
}

func schemaRow(t *testing.T, key string) schemaRowFields {
	t.Helper()
	rows, err := loadSchemaRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range rows {
		var row schemaRowFields
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		if row.Key == key {
			return row
		}
	}
	t.Fatalf("no %s row in the schema", key)
	return schemaRowFields{}
}

func profileRowOpts(t *testing.T, rows []json.RawMessage) []string {
	t.Helper()
	for _, raw := range rows {
		var row struct {
			Key  string   `json:"key"`
			Opts []string `json:"opts"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		if row.Key == profileRowKey {
			return row.Opts
		}
	}
	t.Fatalf("no %s row in the schema", profileRowKey)
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
