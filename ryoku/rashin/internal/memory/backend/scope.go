package backend

import (
	"encoding/hex"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// idPrefix starts every name Rashin gives a scope on an engine, so its own
// banks and peers stand apart from anything else on a shared server.
const idPrefix = "rashin-"

// ScopeID names a scope on an engine in the characters every engine accepts
// (letters, digits, - and _): rashin-you, rashin-project-<key> or
// rashin-team-<id>. A key or id with other characters is written in hex
// under rashin-projectx- or rashin-teamx-, so the name maps back exactly.
func ScopeID(scope string) string {
	switch {
	case scope == protocol.MemoryYou:
		return idPrefix + "you"
	case strings.HasPrefix(scope, protocol.MemoryProjectPrefix):
		return named("project", strings.TrimPrefix(scope, protocol.MemoryProjectPrefix))
	case strings.HasPrefix(scope, protocol.MemoryTeamPrefix):
		return named("team", strings.TrimPrefix(scope, protocol.MemoryTeamPrefix))
	}
	return ""
}

func named(kind, key string) string {
	if protocol.ValidHonchoID(key) {
		return idPrefix + kind + "-" + key
	}
	return idPrefix + kind + "x-" + hex.EncodeToString([]byte(key))
}

// ScopeOf is the inverse of ScopeID; ok is false for a name Rashin did not
// give.
func ScopeOf(id string) (scope string, ok bool) {
	rest, found := strings.CutPrefix(id, idPrefix)
	if !found {
		return "", false
	}
	if rest == "you" {
		return protocol.MemoryYou, true
	}
	for kind, prefix := range map[string]string{"project": protocol.MemoryProjectPrefix, "team": protocol.MemoryTeamPrefix} {
		if key, found := strings.CutPrefix(rest, kind+"-"); found && key != "" {
			return prefix + key, true
		}
		if encoded, found := strings.CutPrefix(rest, kind+"x-"); found {
			key, err := hex.DecodeString(encoded)
			if err != nil || len(key) == 0 {
				return "", false
			}
			return prefix + string(key), true
		}
	}
	return "", false
}

// InGroup reports whether an exact scope falls under what a list asked for:
// every scope, protocol.MemoryYou, MemoryProjects, MemoryTeams or the scope
// itself.
func InGroup(scope, group string) bool {
	switch group {
	case "":
		return true
	case protocol.MemoryProjects:
		return strings.HasPrefix(scope, protocol.MemoryProjectPrefix)
	case protocol.MemoryTeams:
		return strings.HasPrefix(scope, protocol.MemoryTeamPrefix)
	}
	return scope == group
}

// Exact reports whether a list's scope names one scope rather than a group.
func Exact(group string) bool {
	return group != "" && group != protocol.MemoryProjects && group != protocol.MemoryTeams
}

// Page cuts one page out of a whole list, for engines that cannot page
// across several of their own lists.
func Page(all []protocol.Memory, offset, limit int) []protocol.Memory {
	offset = max(offset, 0)
	if limit <= 0 || offset >= len(all) {
		return []protocol.Memory{}
	}
	return all[offset:min(offset+limit, len(all))]
}

// Interleave merges ranked lists from several scopes: the first of each,
// then the second of each, up to limit.
func Interleave(lists [][]protocol.Memory, limit int) []protocol.Memory {
	out := []protocol.Memory{}
	for rank := 0; len(out) < limit; rank++ {
		took := false
		for _, list := range lists {
			if rank < len(list) && len(out) < limit {
				out = append(out, list[rank])
				took = true
			}
		}
		if !took {
			return out
		}
	}
	return out
}
