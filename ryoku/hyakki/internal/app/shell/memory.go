package shell

import (
	"strings"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

// MemoryRoute reads a Memory page route: memory, memory/<filter> or
// memory/<id>.
func MemoryRoute(route string) (env.MemoryPlace, bool) {
	parts := strings.Split(strings.Trim(route, "/"), "/")
	if parts[0] != protocol.TabMemory {
		return env.MemoryPlace{}, false
	}
	var place env.MemoryPlace
	if len(parts) > 1 {
		if env.ValidMemoryFilter(parts[1]) {
			place.Filter = parts[1]
		} else {
			place.ID = parts[1]
		}
	}
	return place, true
}
