package env

import (
	"testing"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the user's pickers read the user's own list and a team's read the team's: a dream's two models never replace
// the user's 681, the user's never stand in for a team that has not read its own, and no team at all reads none
func TestPickersReadTheListOfTheirOwnHome(t *testing.T) {
	user := &protocol.HarnessCapabilities{ModelList: []protocol.Choice{{ID: "a"}, {ID: "b"}, {ID: "c"}}, DefaultModel: "a"}
	dream := &protocol.HarnessCapabilities{ModelList: []protocol.Choice{{ID: "local"}}, DefaultModel: "local"}
	snapshot := state.Snapshot{Harnesses: []protocol.HarnessInfo{{
		ID: "omp", Installed: true, Capabilities: user,
		Setup: []protocol.HarnessFact{{Label: "Model", Value: "a"}},
		Teams: &protocol.HarnessTeams{Homes: []protocol.TeamHome{{Team: "team-dream", Capabilities: dream}, {Team: "team-new"}}},
	}}}
	var reader HarnessProbe
	if got := reader.Info(snapshot, "omp").Capabilities; got != user {
		t.Fatalf("the user's picker reads %+v", got)
	}
	if got := reader.TeamInfo(snapshot, "omp", "team-dream").Capabilities; got != dream {
		t.Fatalf("the dream's picker reads %+v", got)
	}
	for _, team := range []string{"team-new", "team-unknown", ""} {
		info := reader.TeamInfo(snapshot, "omp", team)
		if info.Capabilities != nil || HarnessDefaultModel(info) != "" {
			t.Fatalf("team %q borrows the user's list: %+v, default %q", team, info.Capabilities, HarnessDefaultModel(info))
		}
	}
}
