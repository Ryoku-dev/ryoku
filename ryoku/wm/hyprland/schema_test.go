package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The schema verb's sound-profile picker lists what is installed, as niri's
// does, so a pack the user adds can be chosen; the shipped names stay.
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
	var rows []struct {
		Key  string   `json:"key"`
		Opts []string `json:"opts"`
	}
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Key != profileRowKey {
			continue
		}
		found := map[string]bool{}
		for _, o := range r.Opts {
			found[o] = true
		}
		if !found["my-pack"] || !found["cherry-mx-brown"] {
			t.Errorf("profiles = %v, want the shipped ones and my-pack", r.Opts)
		}
		return
	}
	t.Fatal("no keysounds profile row in the schema output")
}

// the picker shows these labels; without one a shipped switch reads as its raw
// directory name ("Cherry-mx-brown")
func TestSchemaLabelsShippedProfiles(t *testing.T) {
	var rows []struct {
		Key       string            `json:"key"`
		Opts      []string          `json:"opts"`
		OptLabels map[string]string `json:"optLabels"`
	}
	if err := json.Unmarshal(schemaRows, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Key != "wm.hyprland.plugins.keysounds.profile" {
			continue
		}
		if len(row.Opts) == 0 {
			t.Fatal("profile row ships no options")
		}
		for _, o := range row.Opts {
			if row.OptLabels[o] == "" {
				t.Errorf("shipped profile %q has no label", o)
			}
		}
		return
	}
	t.Fatal("no keysounds profile row in the schema")
}
