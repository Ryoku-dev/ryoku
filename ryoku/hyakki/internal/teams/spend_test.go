package teams

import (
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// spendDay is one day of a month's spend, empty when nothing was kept for it.
func spendDay(spend protocol.TeamSpend, day string) protocol.DaySpend {
	for _, d := range spend.Days {
		if d.Day == day {
			return d
		}
	}
	return protocol.DaySpend{Day: day}
}

func TestADayCountsTheTasksThatEndedOnIt(t *testing.T) {
	h := newHarness(t, 0)
	desk := h.newTeam(t, "Desk", nil)
	ops := h.newTeam(t, "Ops", nil)

	done := h.newTask(t, desk, "Draft the notes.")
	ref := h.agent(t, done.ID)
	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Title: "Notes", Body: "The notes are drafted."})
	blocked := h.newTask(t, desk, "Fix the printer.")
	ref = h.agent(t, blocked.ID)
	h.s.GoalEnded(h.sessions.info(ref.Session), protocol.Goal{Status: protocol.GoalBlocked, Reason: "No printer is on the network."})
	stopped := h.newTask(t, ops, "Rotate the logs.")
	h.agent(t, stopped.ID)
	do[protocol.TeamTask](t, h, protocol.MTeamsTaskStop, protocol.TeamTaskRef{ID: stopped.ID})

	spend := func(team string) protocol.TeamSpend {
		t.Helper()
		return do[protocol.TeamSpend](t, h, protocol.MTeamsSpend, protocol.TeamSpendParams{Team: team})
	}
	if got := spendDay(spend(desk.ID), "2026-11-11"); got.Tasks != 2 || got.Routines != 0 {
		t.Fatalf("the desk's day counts %d tasks and %d routine runs, want the done and the blocked one", got.Tasks, got.Routines)
	}
	if got := spendDay(spend(ops.ID), "2026-11-11"); got.Tasks != 0 {
		t.Fatalf("a stopped task counts as %d ended tasks, want none", got.Tasks)
	}
	if got := spendDay(spend(""), "2026-11-11"); got.Tasks != 2 {
		t.Fatalf("every team's day counts %d tasks, want 2", got.Tasks)
	}

	h.clock.advance(24 * time.Hour)
	do[protocol.TeamTask](t, h, protocol.MTeamsTaskRetry, protocol.TeamTaskRetryParams{ID: blocked.ID})
	ref = h.agent(t, blocked.ID)
	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Title: "Printer", Body: "The printer is back."})
	all := spend("")
	if first, second := spendDay(all, "2026-11-11"), spendDay(all, "2026-11-12"); first.Tasks != 2 || second.Tasks != 1 {
		t.Fatalf("a retried task that ended again counts %d then %d, want 2 on its first day and 1 on its new one", first.Tasks, second.Tasks)
	}
}

func TestARoutineRunCountsAsATaskAndARoutineRun(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Watch", nil)
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "Hourly check", false))
	task := do[protocol.TeamTask](t, h, protocol.MTeamsRoutineRun, protocol.RoutineRef{ID: routine.ID})
	ref := h.agent(t, task.ID)
	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Title: "No change", Body: "Nothing new."})

	for _, p := range []protocol.TeamSpendParams{{Team: team.ID}, {}} {
		got := spendDay(do[protocol.TeamSpend](t, h, protocol.MTeamsSpend, p), "2026-11-11")
		if got.Tasks != 1 || got.Routines != 1 {
			t.Fatalf("spend of team %q counts %d tasks and %d routine runs, want the run in both", p.Team, got.Tasks, got.Routines)
		}
	}
}
