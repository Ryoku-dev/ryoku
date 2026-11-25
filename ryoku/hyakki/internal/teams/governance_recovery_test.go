package teams

import (
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestQuietRoutineMayCompleteWithoutInventedReportText(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Quiet observations", nil)
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "Observe changes", false))
	task := do[protocol.TeamTask](t, h, protocol.MTeamsRoutineRun, protocol.RoutineRef{ID: routine.ID})
	ref := h.agent(t, task.ID)
	h.answer(t, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Quiet: true})
	completed := h.task(t, task.ID)
	if completed.Status != protocol.TaskDone || !completed.Read || completed.Report == nil || !completed.Report.Quiet || completed.Report.Body != "" {
		t.Fatalf("quiet completion failed or invented report text: %+v", completed)
	}
	current, _ := h.s.state.routine(routine.ID)
	if current.QuietStreak != 1 {
		t.Fatal("a valid empty quiet completion did not count for its routine")
	}
	ordinary := h.newTask(t, team, "A real assigned report is required")
	ordinaryRef := h.agent(t, ordinary.ID)
	refused(t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ordinaryRef, Quiet: true}, protocol.CodeBadRequest)
	if unchanged := h.task(t, ordinary.ID); unchanged.Status != protocol.TaskWorking || unchanged.Report != nil {
		t.Fatal("quiet syntax let an ordinary assignment complete without a result")
	}
}

func TestRecoveryDeliversChildrenCommittedBeforeParentRelease(t *testing.T) {
	h, team := governedTeam(t)
	root := h.newTask(t, team, "Receive the committed child result")
	rootRef := h.agent(t, root.ID)
	child := governedPiece(t, h, root, rootRef, team.Members[1].ID)
	h.agent(t, child.ID)
	const result = "durable-child-result-before-crash"
	if _, closed := h.s.endTask(child.ID, func(task *protocol.TeamTask) {
		task.Status = protocol.TaskDone
		task.Report = &protocol.TeamReport{Title: "Committed", Body: result, At: h.clock.now()}
	}); !closed {
		t.Fatal("could not commit the exact crash-window child state")
	}
	// Simulate the process loss after endTask persisted but before afterEnd.
	h.s.mu.Lock()
	for _, run := range h.s.runs {
		if run.timer != nil {
			run.timer.Stop()
		}
	}
	h.s.runs = make(map[string]*run)
	h.s.mu.Unlock()
	h.s.recoverRuns()
	waitFor(t, "recovery to deliver the committed child report", func() bool {
		h.sessions.mu.Lock()
		defer h.sessions.mu.Unlock()
		for _, continuation := range h.sessions.continued {
			if continuation.session == rootRef.Session && strings.Contains(continuation.text, result) {
				return true
			}
		}
		return false
	})
	if current := h.task(t, root.ID); current.Waiting != "" || current.Session != rootRef.Session || current.Goal != root.Goal || current.Status != protocol.TaskWorking {
		t.Fatalf("recovery lost the original parent execution or kept its stale hold: %+v", current)
	}
	if current := h.task(t, child.ID); current.Status != protocol.TaskDone || current.Report == nil || current.Report.Body != result {
		t.Fatal("recovery duplicated or altered already committed child work")
	}
}
