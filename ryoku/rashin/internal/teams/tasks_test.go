package teams

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

func TestATaskRunsAsASessionAndEndsWithItsReport(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Research", nil)
	created := h.newTask(t, team, "Find out why the build is slow.\nLook at the last week.")
	if created.Status != protocol.TaskWorking {
		t.Fatalf("a task with a free slot is %q, want working", created.Status)
	}
	ref := h.agent(t, created.ID)
	made, ok := h.sessions.madeFor(created.ID)
	if !ok || made.Kind != protocol.KindAgent || made.Cwd != team.Folder || made.Team != team.ID || made.Origin != protocol.OriginTeam {
		t.Fatalf("the task's session was made with %+v", made)
	}
	if !h.sessions.wasStarted(ref.Session) {
		t.Fatal("the task's session was never started")
	}
	goal, ok := h.sessions.goal(ref.Session, protocol.GoalSet)
	if !ok || !strings.Contains(goal.Objective, "Look at the last week.") || goal.MaxTurns != team.Budget.TaskTurns {
		t.Fatalf("the session's goal is %+v, want the brief within %d turns", goal, team.Budget.TaskTurns)
	}

	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{
		AgentRef: ref, Title: "Slow tests", Body: "The integration tests take nine minutes.",
	})
	done := h.task(t, created.ID)
	if done.Status != protocol.TaskDone || done.Ended == nil {
		t.Fatalf("a reported task is %q, want done", done.Status)
	}
	if done.Report == nil || done.Report.Body != "The integration tests take nine minutes." || done.Read {
		t.Fatalf("the report is %+v, read %v; want it kept unread", done.Report, done.Read)
	}
	if _, ok := h.sessions.goal(ref.Session, protocol.GoalComplete); !ok {
		t.Fatal("the report did not complete the session's goal")
	}
	read := do[protocol.TeamTask](t, h, protocol.MTeamsTaskRead, protocol.TeamTaskRef{ID: created.ID})
	if !read.Read || !h.task(t, created.ID).Read {
		t.Fatal("reading the report did not mark it read")
	}
}

func TestAFinishedTaskKeepsItsSteps(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Research", nil)
	created := h.newTask(t, team, "Count the files.")
	ref := h.agent(t, created.ID)
	steps := func(count int) protocol.SessionInfo {
		h.sessions.mu.Lock()
		defer h.sessions.mu.Unlock()
		info := h.sessions.infos[ref.Session]
		info.Steps = count
		h.sessions.infos[ref.Session] = info
		return info
	}
	steps(4)
	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Title: "Files", Body: "Twelve."})
	if done := h.task(t, created.ID); done.Steps != 4 {
		t.Fatalf("the reported task kept %d steps, want the session's 4", done.Steps)
	}
	// the turn that reported goes on to its last words, and those steps count too
	h.s.AfterTurn(context.Background(), steps(5), protocol.TurnEndData{})
	if done := h.task(t, created.ID); done.Steps != 5 {
		t.Fatalf("after its last turn the task kept %d steps, want 5", done.Steps)
	}
	// a restart reads the task back from its file
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	if stored, _ := reopened.task(created.ID); stored.Steps != 5 {
		t.Fatalf("the stored task keeps %d steps, want 5", stored.Steps)
	}
}

func TestStopRetryAndDeleteATask(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Ops", nil)
	task := h.newTask(t, team, "Rotate the logs.")
	first := h.agent(t, task.ID)

	stopped := do[protocol.TeamTask](t, h, protocol.MTeamsTaskStop, protocol.TeamTaskRef{ID: task.ID})
	if stopped.Status != protocol.TaskStopped || stopped.Ended == nil {
		t.Fatalf("a stopped task is %q, want stopped", stopped.Status)
	}
	waitFor(t, "the stopped task's session to stop", func() bool { return h.sessions.wasStopped(first.Session) })
	refused(t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: first, Body: "Done anyway."}, protocol.CodeConflict)

	retried := do[protocol.TeamTask](t, h, protocol.MTeamsTaskRetry, protocol.TeamTaskRetryParams{ID: task.ID, Note: "Keep a week of logs."})
	if retried.Status != protocol.TaskWorking || retried.Report != nil || retried.Reason != "" || retried.Ended != nil {
		t.Fatalf("a retried task is %+v, want working afresh", retried)
	}
	second := h.agent(t, task.ID)
	if second.Session == first.Session {
		t.Fatal("a retried task kept its old session")
	}
	again := h.task(t, task.ID)
	if !strings.Contains(again.Brief, "Keep a week of logs.") || len(again.Sessions) != 2 {
		t.Fatalf("the retried task's brief is %q with sessions %v", again.Brief, again.Sessions)
	}
	refused(t, h, protocol.MTeamsTaskRetry, protocol.TeamTaskRetryParams{ID: task.ID}, protocol.CodeConflict)

	if _, err := h.call(protocol.MTeamsTaskDelete, protocol.TeamTaskRef{ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.s.state.task(task.ID); ok {
		t.Fatal("a deleted task is still kept")
	}
	listed := do[protocol.TeamTasksResult](t, h, protocol.MTeamsTasks, protocol.TeamTasksParams{Team: team.ID})
	if len(listed.Tasks) != 0 {
		t.Fatalf("the team still lists %d tasks", len(listed.Tasks))
	}
	waitFor(t, "the deleted task's session to stop", func() bool { return h.sessions.wasStopped(second.Session) })
	refused(t, h, protocol.MTeamsTaskStop, protocol.TeamTaskRef{ID: task.ID}, protocol.CodeNotFound)
}

func TestTasksWaitForAFreeSlotAcrossTeamsAndWithinOne(t *testing.T) {
	h := newHarness(t, 3)
	alpha := h.newTeam(t, "Alpha", nil)
	beta := h.newTeam(t, "Beta", nil)
	a1 := h.newTask(t, alpha, "First.")
	a2 := h.newTask(t, alpha, "Second.")
	a3 := h.newTask(t, alpha, "Third.")
	b1 := h.newTask(t, beta, "Fourth.")
	b2 := h.newTask(t, beta, "Fifth.")
	expect := func(step string, want map[string]string) {
		t.Helper()
		for id, status := range want {
			if got := h.task(t, id).Status; got != status {
				t.Fatalf("%s: task %s is %q, want %q", step, h.task(t, id).Brief, got, status)
			}
		}
	}
	expect("at first", map[string]string{
		a1.ID: protocol.TaskWorking, a2.ID: protocol.TaskWorking, b1.ID: protocol.TaskWorking,
		// two of a team at once, three across teams
		a3.ID: protocol.TaskReady, b2.ID: protocol.TaskReady,
	})
	h.agent(t, a2.ID)

	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: h.agent(t, a1.ID), Body: "First is done."})
	// the oldest queued task takes the freed slot
	expect("after the first report", map[string]string{a3.ID: protocol.TaskWorking, b2.ID: protocol.TaskReady})
	h.agent(t, a3.ID)

	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: h.agent(t, b1.ID), Body: "Fourth is done."})
	expect("after the second report", map[string]string{b2.ID: protocol.TaskWorking})
	h.agent(t, b2.ID)
}

func TestAgentCallsAnswerOnlyTheRunsOwnSession(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Inbox", nil)
	task := h.newTask(t, team, "Sort the inbox.")
	ref := h.agent(t, task.ID)
	cases := []struct {
		name string
		ref  protocol.AgentRef
		code string
	}{
		{"a wrong token", protocol.AgentRef{Session: ref.Session, Token: strings.Repeat("0", len(ref.Token))}, protocol.CodeBadRequest},
		{"no token", protocol.AgentRef{Session: ref.Session}, protocol.CodeBadRequest},
		{"a session on no team task", protocol.AgentRef{Session: "session-elsewhere", Token: ref.Token}, protocol.CodeNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			refused(t, h, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: c.ref, Title: "Archive everything"}, c.code)
		})
	}
	t.Run("the right token from another session's socket", func(t *testing.T) {
		raw, err := json.Marshal(protocol.AgentAskParams{AgentRef: ref, Title: "Archive everything"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = h.s.Handle(WithAgentSession(context.Background(), "session-elsewhere"), protocol.MTeamsAgentAsk, raw)
		var failure *protocol.Error
		if !errors.As(err, &failure) || failure.Code != protocol.CodeForbidden {
			t.Fatalf("a call on another session's socket: got %v, want forbidden", err)
		}
	})
	if waiting := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(waiting) != 0 {
		t.Fatalf("a refused call left %d approvals", len(waiting))
	}

	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Body: "Sorted."})
	ended := refused(t, h, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: ref, Title: "One more thing"}, protocol.CodeConflict)
	if ended.Message != i18n.T("This task already ended.") {
		t.Fatalf("a call after the task ended says %q", ended.Message)
	}
}

// The app makes a task's title from the first line of what was typed, so
// that line must not reach the agent twice.
func TestATasksGoalSaysItsTitleOnce(t *testing.T) {
	long := strings.Repeat("word ", 30)
	for _, test := range []struct {
		name, title, brief, want string
	}{
		{"typed in the app", "Find the release date", "Find the release date\nCheck the tracker.", "Find the release date\nCheck the tracker."},
		{"a long first line cut short", strings.TrimSpace(long[:80]), long + "\nThen report.", long + "\nThen report."},
		{"a title of its own", "Weekly digest", "Read the feeds and sum them up.", "Weekly digest\n\nRead the feeds and sum them up."},
		{"no brief", "Tidy the folder", "", "Tidy the folder"},
	} {
		if got := objective(protocol.TeamTask{Title: test.title, Brief: test.brief}); got != test.want {
			t.Errorf("%s: the goal is %q, want %q", test.name, got, test.want)
		}
	}
}
