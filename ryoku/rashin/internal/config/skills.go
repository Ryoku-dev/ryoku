package config

import (
	"slices"
	"strings"
)

// cleanIDs trims a list of set ids, drops empty and repeated ones and sorts
// it, so a settings file names each turned-off set once.
func cleanIDs(ids []string) []string {
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}
