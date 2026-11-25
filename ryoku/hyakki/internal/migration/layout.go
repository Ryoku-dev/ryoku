// Package migration performs the one-time Rashin-to-Hyakki data cutover.
package migration

import (
	"path/filepath"
	"strings"
)

// Move binds an original app-owned path to its relocated destination.
// Neither the shared Ryoku vault nor existing Git worktrees belongs in this list.
type Move struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// RebasePath changes only an exact relocated path or one of its descendants.
// Longest match wins; unrelated user locations and non-path strings stay intact.
func RebasePath(path string, moves []Move) string {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return path
	}
	best := -1
	for i := range moves {
		source := moves[i].Source
		if source == "" || !filepath.IsAbs(source) || !filepath.IsAbs(moves[i].Destination) {
			continue
		}
		if path == source || strings.HasPrefix(path, source+string(filepath.Separator)) {
			if best < 0 || len(source) > len(moves[best].Source) {
				best = i
			}
		}
	}
	if best < 0 {
		return path
	}
	if path == moves[best].Source {
		return moves[best].Destination
	}
	return filepath.Join(moves[best].Destination, path[len(moves[best].Source)+1:])
}
