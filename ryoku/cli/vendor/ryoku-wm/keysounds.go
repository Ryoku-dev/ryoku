package wm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// KeysoundsUserDir is where a user's own key sound profiles live, one folder
// each, beside the shipped ones; every provider's key sound engine reads it.
func KeysoundsUserDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "ryoku", "keysounds")
}

// KeysoundsShippedDir is where the profiles package installs its profiles.
const KeysoundsShippedDir = "/usr/share/ryoku/keysounds"

// WithInstalledKeysounds makes the key sound profile row (the row keyed key)
// offer every profile installed under roots, so a pack the user adds shows in
// Settings without a schema edit. The shipped names stay first and are the
// whole list when nothing else is installed.
func WithInstalledKeysounds(rows []json.RawMessage, key string, roots ...string) []json.RawMessage {
	for i, raw := range rows {
		var row map[string]json.RawMessage
		if json.Unmarshal(raw, &row) != nil {
			continue
		}
		var k string
		if json.Unmarshal(row["key"], &k) != nil || k != key {
			continue
		}
		var opts []string
		if json.Unmarshal(row["opts"], &opts) != nil {
			continue
		}
		if b, err := json.Marshal(appendMissing(opts, installedKeysounds(roots))); err == nil {
			row["opts"] = b
		}
		if b, err := json.Marshal(row); err == nil {
			rows[i] = b
		}
	}
	return rows
}

func installedKeysounds(roots []string) []string {
	names := []string{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
	}
	sort.Strings(names)
	return names
}

func appendMissing(base, extra []string) []string {
	seen := make(map[string]bool, len(base))
	for _, s := range base {
		seen[s] = true
	}
	for _, s := range extra {
		if !seen[s] {
			seen[s] = true
			base = append(base, s)
		}
	}
	return base
}
