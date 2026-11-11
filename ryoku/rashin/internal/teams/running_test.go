package teams

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

func TestATeamWorksOnAsManyTasksAsItsLimitSays(t *testing.T) {
	h := newHarnessWith(t, func(s *protocol.Settings) {
		s.Teams.MaxRunning = 6
		s.Teams.PerTeam = 1
	})
	alpha := h.newTeam(t, "Alpha", nil)
	beta := h.newTeam(t, "Beta", func(p *protocol.TeamCreateParams) { p.Running = 3 })
	a1, a2 := h.newTask(t, alpha, "First."), h.newTask(t, alpha, "Second.")
	b := []protocol.TeamTask{h.newTask(t, beta, "One."), h.newTask(t, beta, "Two."), h.newTask(t, beta, "Three."), h.newTask(t, beta, "Four.")}
	for _, task := range []protocol.TeamTask{a1, b[0], b[1], b[2]} {
		h.agent(t, task.ID)
	}
	if got := h.task(t, a2.ID).Status; got != protocol.TaskReady {
		t.Fatalf("a second task of a team on the setting's one at a time is %q, want queued", got)
	}
	if got := h.task(t, b[3].ID).Status; got != protocol.TaskReady {
		t.Fatalf("a fourth task of a team of three at a time is %q, want queued", got)
	}
	do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: alpha.ID, Patch: mustJSON(t, map[string]any{"running": 2})})
	h.agent(t, a2.ID)
	refused(t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: alpha.ID, Patch: mustJSON(t, map[string]any{"running": protocol.TeamsPerTeamMax + 1})}, protocol.CodeBadRequest)
}

func TestPausingEveryTeamStopsTheirWorkAndRoutines(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Watch", nil)
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "Hourly check", true))
	waitFor(t, "the wake unit to turn on", func() bool { return len(h.wake.seen()) == 2 })
	working := h.newTask(t, team, "Look around.")
	ref := h.agent(t, working.ID)

	h.setTeams(func(s *protocol.TeamsSettings) { s.Paused = true })
	stopped := h.task(t, working.ID)
	if stopped.Status != protocol.TaskStopped || stopped.Reason != "Every team was paused." {
		t.Fatalf("a working task of a paused computer is %q: %q", stopped.Status, stopped.Reason)
	}
	if h.s.Armed() {
		t.Fatal("routines still keep Rashin up while every team is paused")
	}
	waitFor(t, "the wake unit to turn off", func() bool { return len(h.wake.seen()) == 3 })
	queued := h.newTask(t, team, "Wait for it.")
	if queued.Status != protocol.TaskReady {
		t.Fatalf("a task given while every team is paused is %q, want queued", queued.Status)
	}
	refused(t, h, protocol.MTeamsRoutineRun, protocol.RoutineRef{ID: routine.ID}, protocol.CodeConflict)
	if listed := h.s.listRoutines(team.ID).Routines; listed[0].NextRun != nil {
		t.Fatal("a routine shows a next run while every team is paused")
	}
	if !h.s.Hold(h.sessions.info(ref.Session)) {
		t.Fatal("a session may go on while every team is paused")
	}

	h.setTeams(func(s *protocol.TeamsSettings) { s.Paused = false })
	h.agent(t, queued.ID)
	if !h.s.Armed() {
		t.Fatal("routines do not run again once the teams work again")
	}
	if h.ledgerCount(t, "teams.pause", "") != 1 || h.ledgerCount(t, "teams.resume", "") != 1 {
		t.Fatal("the pause and the resume are not both in the ledger")
	}
}

func TestTurningRoutinesWakeOffKeepsTheWakeUnitOff(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Night", nil)
	do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "Nightly", true))
	waitFor(t, "the wake unit to turn on", func() bool { return len(h.wake.seen()) == 2 })
	h.setTeams(func(s *protocol.TeamsSettings) { s.RoutinesWake = false })
	waitFor(t, "the wake unit to turn off", func() bool { return len(h.wake.seen()) == 3 })
	if !h.s.Armed() {
		t.Fatal("with the wake unit off a routine no longer keeps a running Rashin up")
	}
}

// The minutes a task may work leave out the time it waited on the user.
func TestATaskThatWorksPastItsMinutesIsBlocked(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Slow", func(p *protocol.TeamCreateParams) {
		p.Budget = protocol.TeamBudget{TaskTurns: 10, TaskMinutes: 2}
	})
	task := h.newTask(t, team, "Take your time.")
	ref := h.agent(t, task.ID)
	h.s.mu.Lock()
	armed := h.s.runs[task.ID].timer != nil
	h.s.mu.Unlock()
	if !armed {
		t.Fatal("a task with a time limit has no timer")
	}

	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: ref, Title: "Delete the cache"})
	asked := h.pending(t)
	h.clock.advance(10 * time.Minute)
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	h.s.overrun(task.ID)
	if got := h.task(t, task.ID).Status; got != protocol.TaskWorking {
		t.Fatalf("a task that waited ten minutes on the user is %q after one minute of work, want working", got)
	}

	h.clock.advance(3 * time.Minute)
	h.s.overrun(task.ID)
	blocked := h.task(t, task.ID)
	if blocked.Status != protocol.TaskBlocked || blocked.Reason != "It worked past its 2 minutes." {
		t.Fatalf("a task past its minutes is %q: %q", blocked.Status, blocked.Reason)
	}
	waitFor(t, "its session to stop", func() bool { return h.sessions.wasStopped(ref.Session) })
}

// After a restart a task's minutes count on from when it started, so one
// already past them ends at once.
func TestARestartArmsTheTimeLimitFromTheStart(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Slow", func(p *protocol.TeamCreateParams) {
		p.Budget = protocol.TeamBudget{TaskTurns: 10, TaskMinutes: 5}
	})
	task := h.newTask(t, team, "Take your time.")
	h.agent(t, task.ID)
	h.clock.advance(6 * time.Minute)

	keys := fixedKeeper{key: bytes.Repeat([]byte{7}, 32)}
	again, err := open(h.dir, vault.OpenWith(filepath.Join(t.TempDir(), "vault.db"), keys), h.wake.set, h.events, h.currentSettings)
	if err != nil {
		t.Fatal(err)
	}
	again.now = h.clock.now
	again.defaultHarness = h.s.defaultHarness
	t.Cleanup(func() { _ = again.Close() })
	again.Bind(h.sessions)
	waitFor(t, "the restarted task to run out of time", func() bool {
		recovered, _ := again.state.task(task.ID)
		return recovered.Status == protocol.TaskBlocked
	})
}
