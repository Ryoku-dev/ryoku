package teams

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// hourly is a routine of team that runs at the start of every hour.
func hourly(team protocol.Team, title string, on bool) protocol.RoutineCreateParams {
	return protocol.RoutineCreateParams{Team: team.ID, Title: title, Brief: "Look at what changed.", Schedule: "0 * * * *", Enabled: on}
}

func turnRoutine(t *testing.T, h *harness, id string, on bool) protocol.Routine {
	t.Helper()
	patch := json.RawMessage(fmt.Sprintf(`{"enabled":%t}`, on))
	return do[protocol.Routine](t, h, protocol.MTeamsRoutineUpdate, protocol.RoutineUpdateParams{ID: id, Patch: patch})
}

// at is a time on the day the harness's clock starts.
func at(hour, minute, second int) time.Time {
	return time.Date(2026, time.November, 11, hour, minute, second, 0, time.Local)
}

func TestADueRoutineStartsATaskAndSkipsWhatItCannotRun(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Watch", nil)
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "Hourly check", true))
	if routine.NextRun == nil || !routine.NextRun.Equal(at(11, 0, 0)) {
		t.Fatalf("the routine next runs at %v, want 11:00", routine.NextRun)
	}
	runs := func() []protocol.TeamTask {
		return h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Routine == routine.ID })
	}
	lastStatus := func() string {
		r, _ := h.s.state.routine(routine.ID)
		return r.LastStatus
	}

	h.clock.set(at(11, 0, 30))
	h.s.clock.tick()
	first := runs()
	if len(first) != 1 || first[0].Origin != protocol.TaskFromRoutine || first[0].Status != protocol.TaskWorking {
		t.Fatalf("a due routine made %+v, want one working task from the routine", first)
	}
	ref := h.agent(t, first[0].ID)

	// the last run still works, so the next is skipped
	h.clock.set(at(12, 0, 30))
	h.s.clock.tick()
	if len(runs()) != 1 || lastStatus() != routineSkipped {
		t.Fatalf("a run due while the last works made %d tasks and left %q", len(runs()), lastStatus())
	}

	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Body: "Nothing changed."})
	// found five minutes late: the computer was off, so the run is missed
	h.clock.set(at(13, 5, 0))
	h.s.clock.tick()
	if len(runs()) != 1 || lastStatus() != routineSkipped {
		t.Fatalf("a missed run made %d tasks and left %q", len(runs()), lastStatus())
	}

	// a minute late still counts as on time
	h.clock.set(at(14, 1, 0))
	h.s.clock.tick()
	latest := runs()
	if len(latest) != 2 || latest[0].Status != protocol.TaskWorking {
		t.Fatalf("a run a minute late made %d tasks, want a second one", len(latest))
	}
	h.agent(t, latest[0].ID)
}

func TestARoutineThatFailsFiveRunsInARowTurnsItselfOff(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Flaky", nil)
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "Sync", true))
	waitFor(t, "the wake unit to turn on", func() bool { return len(h.wake.seen()) == 2 })

	for run := 1; run <= protocol.RoutineFailuresMax; run++ {
		task := do[protocol.TeamTask](t, h, protocol.MTeamsRoutineRun, protocol.RoutineRef{ID: routine.ID})
		ref := h.agent(t, task.ID)
		h.s.GoalEnded(h.sessions.info(ref.Session), protocol.Goal{Status: protocol.GoalBlocked, Reason: "The API refused the login."})
		got, _ := h.s.state.routine(routine.ID)
		if got.Failures != run {
			t.Fatalf("after %d failed runs the routine counts %d", run, got.Failures)
		}
		if on := run < protocol.RoutineFailuresMax; got.Enabled != on {
			t.Fatalf("after %d failed runs the routine is on: %v, want %v", run, got.Enabled, on)
		}
	}
	if h.ledgerCount(t, "routine.off", "") != 1 {
		t.Fatal("a routine that turned itself off left no record of it")
	}
	waitFor(t, "the wake unit to turn off", func() bool { return len(h.wake.seen()) == 3 })

	again := turnRoutine(t, h, routine.ID, true)
	if !again.Enabled || again.Failures != 0 {
		t.Fatalf("a routine turned back on is %+v, want on with no failures", again)
	}
}

func TestRoutineLimits(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Busy", nil)

	tooOften := hourly(team, "Every five minutes", true)
	tooOften.Schedule = "*/5 * * * *"
	refused(t, h, protocol.MTeamsRoutineCreate, tooOften, protocol.CodeBadRequest)
	quarter := hourly(team, "Every quarter hour", true)
	quarter.Schedule = "*/15 * * * *"
	do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, quarter)

	for i := 2; i <= protocol.RoutinesEnabledMax; i++ {
		do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, fmt.Sprintf("Check %d", i), true))
	}
	refused(t, h, protocol.MTeamsRoutineCreate, hourly(team, "One too many", true), protocol.CodeConflict)
	off := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "Waiting its turn", false))
	refused(t, h, protocol.MTeamsRoutineUpdate, protocol.RoutineUpdateParams{ID: off.ID, Patch: json.RawMessage(`{"enabled":true}`)}, protocol.CodeConflict)
}

func TestTheWakeUnitIsOnWhileAnyRoutineIs(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Night", nil)
	first := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "First", true))
	waitFor(t, "the wake unit to turn on", func() bool { return len(h.wake.seen()) == 2 })
	second := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "Second", true))
	turnRoutine(t, h, first.ID, false)
	turnRoutine(t, h, second.ID, false)
	waitFor(t, "the wake unit to turn off", func() bool { return len(h.wake.seen()) == 3 })
	if got := h.wake.seen(); !slices.Equal(got, []bool{false, true, false}) {
		t.Fatalf("the wake unit was set %v, want off, on with the first routine, off with the last", got)
	}
}
