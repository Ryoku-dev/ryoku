package teams

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// askingTask starts a task whose agent asks the user before it acts, and
// returns the task, its agent and the approval it waits on.
func askingTask(t *testing.T, h *harness, title string) (protocol.TeamTask, protocol.AgentRef, protocol.Approval) {
	t.Helper()
	team := h.newTeam(t, "Repo", nil)
	task := h.newTask(t, team, "Tidy the repository.")
	ref := h.agent(t, task.ID)
	do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: ref, Title: title})
	return task, ref, h.pending(t)
}

func TestAnApprovalHoldsItsTaskAndIsCarriedOutOnce(t *testing.T) {
	h := newHarness(t, 0)
	task, ref, asked := askingTask(t, h, "Delete the stale branches")
	if asked.Kind != protocol.ApprovalAction || asked.Task != task.ID || !asked.Expires.Equal(asked.Created.Add(protocol.DefaultApprovalHours*time.Hour)) {
		t.Fatalf("the approval is %+v", asked)
	}
	held := h.task(t, task.ID)
	if held.Status != protocol.TaskNeedsYou || held.Waiting != protocol.WaitingOnYou {
		t.Fatalf("an asking task is %q waiting on %q, want held on the user", held.Status, held.Waiting)
	}
	if !h.s.Hold(h.sessions.info(ref.Session)) {
		t.Fatal("the session of a held task may start its next turn")
	}

	decided := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	if decided.Status != protocol.ApprovalApproved || decided.Decided == nil {
		t.Fatalf("an approved approval is %q", decided.Status)
	}
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if working := h.task(t, task.ID); working.Status != protocol.TaskWorking || working.Waiting != "" {
		t.Fatalf("an answered task is %q waiting on %q, want working", working.Status, working.Waiting)
	}
	if h.s.Hold(h.sessions.info(ref.Session)) {
		t.Fatal("an answered task still holds its session")
	}

	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideDeny}, protocol.CodeConflict)
	if notes := h.sessions.notes(ref.Session); len(notes) != 1 {
		t.Fatalf("a second answer reached the agent: %q", notes)
	}
	if kept, _ := h.s.state.approval(asked.ID); kept.Status != protocol.ApprovalApproved {
		t.Fatalf("a second answer turned the approval %q", kept.Status)
	}
}

func TestDenyingEndsTheHoldWithTheNote(t *testing.T) {
	h := newHarness(t, 0)
	task, ref, asked := askingTask(t, h, "Force-push main")
	decided := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{
		ID: asked.ID, Decision: protocol.DecideDeny, Note: "Not before the release.",
	})
	if decided.Status != protocol.ApprovalDenied || decided.Note != "Not before the release." {
		t.Fatalf("a denied approval is %q with note %q", decided.Status, decided.Note)
	}
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if note := h.sessions.notes(ref.Session)[0]; !strings.Contains(note, "Not before the release.") {
		t.Fatalf("the agent was told %q, without the note", note)
	}
	if working := h.task(t, task.ID); working.Status != protocol.TaskWorking || working.Waiting != "" {
		t.Fatalf("a denied task is %q waiting on %q, want working", working.Status, working.Waiting)
	}
}

func TestAnUnansweredApprovalExpiresAsANo(t *testing.T) {
	h := newHarness(t, 0)
	task, ref, asked := askingTask(t, h, "Publish the draft")

	h.clock.advance(protocol.DefaultApprovalHours*time.Hour - time.Minute)
	h.s.armExpiry()
	if kept, _ := h.s.state.approval(asked.ID); kept.Status != protocol.ApprovalPending {
		t.Fatalf("an approval a minute before its time is %q, want pending", kept.Status)
	}

	h.clock.advance(2 * time.Minute)
	h.s.armExpiry()
	expired, _ := h.s.state.approval(asked.ID)
	if expired.Status != protocol.ApprovalExpired || expired.Decision != protocol.DecideDeny {
		t.Fatalf("an approval past its time is %q decided %q, want expired as a no", expired.Status, expired.Decision)
	}
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if working := h.task(t, task.ID); working.Status != protocol.TaskWorking {
		t.Fatalf("the task of an expired approval is %q, want working", working.Status)
	}
	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce}, protocol.CodeConflict)
}

func TestAnEndingTaskNeverOverwritesTheUsersAnswer(t *testing.T) {
	h := newHarness(t, 0)
	task, _, asked := askingTask(t, h, "Delete the stale branches")
	answered := make(chan struct{})
	var answer error
	h.clock.beforeNextRead(func() {
		// the user approves while the ending task answers no; a no that has
		// claimed the approval keeps the user waiting until it is written
		go func() {
			_, answer = h.call(protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
			close(answered)
		}()
		select {
		case <-answered:
		case <-time.After(300 * time.Millisecond):
		}
	})
	h.s.denyPending(task.ID, "The task ended.")
	select {
	case <-answered:
	case <-time.After(10 * time.Second):
		t.Fatal("the user's answer never came back")
	}
	kept, _ := h.s.state.approval(asked.ID)
	if answer == nil && kept.Status != protocol.ApprovalApproved {
		t.Fatalf("the user's approval was carried out, then the ending task turned it %q", kept.Status)
	}
	if answer != nil && kept.Status != protocol.ApprovalDenied {
		t.Fatalf("the user's answer was refused (%v), yet the approval is %q", answer, kept.Status)
	}
	if answers := h.ledgerCount(t, "approval.approve", task.ID) + h.ledgerCount(t, "approval.deny", task.ID); answers != 1 {
		t.Fatalf("the approval was answered %d times, want once", answers)
	}
}

func TestAQuestionWaitsTheSettingsHours(t *testing.T) {
	h := newHarnessWith(t, func(s *protocol.Settings) { s.Teams.ApprovalHours = 2 })
	_, ref, asked := askingTask(t, h, "Publish the draft")
	if !asked.Expires.Equal(asked.Created.Add(2 * time.Hour)) {
		t.Fatalf("a question waits until %v, want two hours after %v", asked.Expires, asked.Created)
	}
	h.clock.advance(2*time.Hour + time.Minute)
	h.s.armExpiry()
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if note := h.sessions.notes(ref.Session)[0]; !strings.Contains(note, "Nobody answered within 2 hours.") {
		t.Fatalf("the agent was told %q", note)
	}
	var detail string
	for _, a := range h.ledger(t) {
		if a.Action == "approval.expire" {
			detail = a.Detail
		}
	}
	if detail != "Nobody answered within 2 hours, so it counts as a no." {
		t.Fatalf("the ledger says %q", detail)
	}
}

// A spending limit card only keeps paused a team that already is, so it
// waits until the user answers.
func TestASpendingLimitCardNeverExpires(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Spender", func(p *protocol.TeamCreateParams) {
		p.Budget = protocol.TeamBudget{MonthTokens: 100, TaskTurns: 10}
	})
	ref := h.agent(t, h.newTask(t, team, "Spend.").ID)
	h.s.AfterTurn(context.Background(), h.sessions.info(ref.Session), protocol.TurnEndData{Usage: &protocol.TokenUsage{Input: 100, Output: 100}})
	card := h.pending(t)
	if card.Kind != protocol.ApprovalBudget || !card.Expires.IsZero() {
		t.Fatalf("the spending limit card is %+v, want one without an expiry", card)
	}
	h.clock.advance(protocol.TeamsApprovalHoursMax * time.Hour * 2)
	h.s.armExpiry()
	if kept, _ := h.s.state.approval(card.ID); kept.Status != protocol.ApprovalPending {
		t.Fatalf("a spending limit card left two weeks is %q, want pending", kept.Status)
	}
}

func TestAChoiceQuestionTakesOneOfItsOptions(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Repo", nil)
	task := h.newTask(t, team, "Merge the branch.")
	ref := h.agent(t, task.ID)
	for _, options := range [][]string{{"Only one"}, {"Same", "Same"}, {"A", "B", "C", "D", "E", "F", "G"}, {"Fine", " "}} {
		refused(t, h, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: ref, Title: "How to merge", Options: options}, protocol.CodeBadRequest)
	}
	answer := do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: ref, Title: "How to merge", Options: []string{"Squash", "Rebase", "Merge commit"}})
	if !strings.Contains(answer.Text, "choose") {
		t.Fatalf("the agent was told %q", answer.Text)
	}
	asked := h.pending(t)
	if !slices.Equal(asked.Options, []string{"Squash", "Rebase", "Merge commit"}) {
		t.Fatalf("the question keeps the options %v", asked.Options)
	}
	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce}, protocol.CodeBadRequest)
	refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideChoose, Choice: "Fast forward"}, protocol.CodeBadRequest)

	chosen := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{
		ID: asked.ID, Decision: protocol.DecideChoose, Choice: "Rebase", Note: "Keep the history tidy.",
	})
	if chosen.Status != protocol.ApprovalApproved || chosen.Choice != "Rebase" {
		t.Fatalf("a chosen question is %q with choice %q", chosen.Status, chosen.Choice)
	}
	waitFor(t, "the agent to hear the choice", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	if note := h.sessions.notes(ref.Session)[0]; note != "The user chose: Rebase\nTheir note: Keep the history tidy." {
		t.Fatalf("the agent was told %q", note)
	}
	var detail string
	for _, a := range h.ledger(t) {
		if a.Action == "approval.approve" {
			detail = a.Detail
		}
	}
	if detail != "Chose: Rebase" {
		t.Fatalf("the ledger says %q", detail)
	}
}

func TestQuietHoursHoldDesktopNotificationsBack(t *testing.T) {
	h := newHarnessWith(t, func(s *protocol.Settings) {
		// the harness's clock starts at 10:00
		s.Teams.QuietHours = &protocol.QuietHours{From: "09:00", To: "12:00"}
	})
	team := h.newTeam(t, "Night", nil)
	h.sessions.mu.Lock()
	h.sessions.away = true
	h.sessions.mu.Unlock()
	t.Cleanup(func() {
		h.sessions.mu.Lock()
		h.sessions.away = false
		h.sessions.mu.Unlock()
	})
	needs := notification{Team: team.ID, Title: "Night needs you", Needs: true}
	if _, ok := h.s.notifyRoute(needs); ok {
		t.Fatal("a notification inside the quiet hours reaches the desktop")
	}
	h.setTeams(func(s *protocol.TeamsSettings) { s.QuietHours = &protocol.QuietHours{From: "22:00", To: "07:00"} })
	if route, ok := h.s.notifyRoute(needs); !ok || route != "inbox" {
		t.Fatalf("a notification outside the quiet hours goes to %q (%v), want the Inbox", route, ok)
	}
}
