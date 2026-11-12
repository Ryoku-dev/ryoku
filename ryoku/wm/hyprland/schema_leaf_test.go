package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The Animations page lists every leaf the shipped config overrides and names
// it from this schema's "[leaf=<name>]" rows; a leaf with no row shows its raw
// name ("layersIn"), so every leaf the Lua sets needs a labelled row.
func TestEveryShippedAnimationLeafHasALabel(t *testing.T) {
	var rows []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(schemaRows, &rows); err != nil {
		t.Fatal(err)
	}
	named := map[string]string{}
	leafKey := regexp.MustCompile(`\[leaf=([A-Za-z]+)\]`)
	for _, r := range rows {
		if m := leafKey.FindStringSubmatch(r.Key); m != nil {
			named[m[1]] = strings.Split(r.Label, " \u00b7 ")[0]
		}
	}

	set := regexp.MustCompile(`animation\(\{\s*leaf\s*=\s*"([A-Za-z]+)"`)
	seen := 0
	err := filepath.WalkDir(filepath.Join("..", "..", "hyprland"), func(f string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(f, ".lua") {
			return err
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		for _, m := range set.FindAllStringSubmatch(string(b), -1) {
			seen++
			if name := named[m[1]]; name == "" || name == m[1] {
				t.Errorf("%s sets the animation %q, which has no labelled [leaf=%s] row", f, m[1], m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("found no animation({ leaf = ... }) in the shipped modules")
	}
}
