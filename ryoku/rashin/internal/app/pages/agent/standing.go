package agent

import (
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// how a session stands for someone who is not looking at it
type Standing struct {
	Needs, Failed, Working bool
	Title                  string
	// what it asks, why it failed or what it said last, and where it works
	Matter, Place string
}

func Standings(snapshot state.Snapshot) map[string]Standing {
	standings := make(map[string]Standing)
	for id, info := range snapshot.Sessions {
		if info.Kind != protocol.KindAgent {
			continue
		}
		standings[id] = Standing{
			Needs: needsYou(snapshot, info), Failed: info.State == protocol.StateFailed, Working: info.State == protocol.StateWorking,
			Title: sessionTitle(info), Matter: sessionMatter(snapshot, info), Place: sessionPlace(snapshot, info),
		}
	}
	return standings
}
