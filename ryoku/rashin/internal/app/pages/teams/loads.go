package teams

import (
	"fmt"

	"ryoku-rashin/internal/app/env"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a fetch made once per key, so the page asks the daemon again only when what it shows could have changed
type load struct {
	key  string
	busy bool
}

// a failed fetch is not retried until its key changes, so an error is told once and not every frame
func (l *load) want(e *env.Env, key string, fetch func(done func())) {
	if e.Snapshot().Connection != appstate.Ready || l.busy || l.key == key {
		return
	}
	l.key, l.busy = key, true
	fetch(func() { l.busy = false })
}

// what a team's own figures depend on: the connection, the team and how its work stands
func teamKey(epoch int, team protocol.Team) string {
	stats := team.Stats
	return fmt.Sprintf("%s/%d/%d/%d/%d/%d/%d/%d",
		team.ID, epoch, team.Updated.UnixNano(), stats.MonthTokens, stats.Working, stats.Queued, stats.NeedsYou, stats.Done)
}

func templateByID(templates []protocol.TeamTemplate, id string) (protocol.TeamTemplate, bool) {
	for _, template := range templates {
		if template.ID == id {
			return template, true
		}
	}
	return protocol.TeamTemplate{}, false
}
