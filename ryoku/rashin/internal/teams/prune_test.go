package teams

import (
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// finishedTask gives team a task and has its agent report it done.
func finishedTask(t *testing.T, h *harness, team protocol.Team, brief string) (protocol.TeamTask, string) {
	t.Helper()
	task := h.newTask(t, team, brief)
	ref := h.agent(t, task.ID)
	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Body: "Done."})
	return task, ref.Session
}

func TestPruningDeletesOldFinishedTasksWithTheirSessions(t *testing.T) {
	h := newHarnessWith(t, func(s *protocol.Settings) { s.Teams.KeepDays = 30 })
	team := h.newTeam(t, "Archive", nil)
	old, oldSession := finishedTask(t, h, team, "Long ago.")
	h.clock.advance(31 * 24 * time.Hour)
	recent, recentSession := finishedTask(t, h, team, "Lately.")
	going := h.newTask(t, team, "Still going.")
	h.agent(t, going.ID)

	do[protocol.Empty](t, h, protocol.MTeamsPrune, protocol.TeamPruneParams{Team: team.ID})
	if _, kept := h.s.state.task(old.ID); kept {
		t.Fatal("a task finished past the kept days is still kept")
	}
	if _, kept := h.s.state.task(recent.ID); !kept {
		t.Fatal("a task finished within the kept days was deleted")
	}
	h.sessions.mu.Lock()
	deleted := slices.Clone(h.sessions.deleted)
	h.sessions.mu.Unlock()
	if !slices.Contains(deleted, oldSession) || slices.Contains(deleted, recentSession) {
		t.Fatalf("the sessions deleted are %v, want only %s", deleted, oldSession)
	}
	if h.ledgerCount(t, "task.prune", "") != 1 {
		t.Fatal("the prune left no record")
	}

	do[protocol.Empty](t, h, protocol.MTeamsPrune, protocol.TeamPruneParams{Team: team.ID, All: true})
	if _, kept := h.s.state.task(recent.ID); kept {
		t.Fatal("clearing every finished task kept one")
	}
	if got := h.task(t, going.ID).Status; got != protocol.TaskWorking {
		t.Fatalf("clearing finished tasks touched a working one: %q", got)
	}
	refused(t, h, protocol.MTeamsPrune, protocol.TeamPruneParams{Team: "team-missing"}, protocol.CodeNotFound)
}

func TestKeepingFewerDaysPrunesAtOnce(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Archive", nil)
	old, _ := finishedTask(t, h, team, "Long ago.")
	h.clock.advance(40 * 24 * time.Hour)
	do[protocol.Empty](t, h, protocol.MTeamsPrune, protocol.TeamPruneParams{})
	if _, kept := h.s.state.task(old.ID); !kept {
		t.Fatal("a prune deleted a task while finished tasks are kept forever")
	}
	h.setTeams(func(s *protocol.TeamsSettings) { s.KeepDays = 30 })
	if _, kept := h.s.state.task(old.ID); kept {
		t.Fatal("keeping finished tasks 30 days left one 40 days old")
	}
}
