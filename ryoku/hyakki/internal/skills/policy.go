package skills

import (
	"slices"

	"ryoku-rashin/internal/protocol"
)

// Restrict adds the sets outside an explicit allowlist to off. It never
// re-enables a globally or team-disabled set and does not touch the filesystem.
func (l *Library) Restrict(off, allowed []string) []string {
	restricted := make([]string, len(off), len(off)+len(l.catalog)+1)
	copy(restricted, off)
	for _, set := range l.catalog {
		if !slices.Contains(allowed, set.id) && !slices.Contains(restricted, set.id) {
			restricted = append(restricted, set.id)
		}
	}
	if !slices.Contains(allowed, protocol.ManagedSkillSet) && !slices.Contains(restricted, protocol.ManagedSkillSet) {
		restricted = append(restricted, protocol.ManagedSkillSet)
	}
	return restricted
}
