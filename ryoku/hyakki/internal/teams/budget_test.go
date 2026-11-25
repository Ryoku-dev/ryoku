package teams

import (
	"context"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestTheSpendingLimitWarnsOncePausesTheTeamAndResumesWhenRaised(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Spender", func(p *protocol.TeamCreateParams) {
		p.Budget = protocol.TeamBudget{MonthTokens: 1000, WarnPercent: 80, TaskTurns: 10}
	})
	first := h.newTask(t, team, "First.")
	second := h.newTask(t, team, "Second.")
	queued := h.newTask(t, team, "Third.")
	ref := h.agent(t, first.ID)
	h.agent(t, second.ID)
	if queued.Status != protocol.TaskReady {
		t.Fatalf("a third task of a team is %q, want queued", queued.Status)
	}
	turn := func(tokens int) {
		usage := &protocol.TokenUsage{Input: tokens / 2, Output: tokens - tokens/2}
		h.s.AfterTurn(context.Background(), h.sessions.info(ref.Session), protocol.TurnEndData{Usage: usage})
	}

	for _, step := range []struct {
		tokens, warnings int
	}{
		{500, 0},
		{350, 1}, // 850 crosses the warning at 800
		{50, 1},  // past the mark it does not warn again
	} {
		turn(step.tokens)
		if got := h.ledgerCount(t, "budget.warn", ""); got != step.warnings {
			t.Fatalf("after %d more tokens the team warned %d times, want %d", step.tokens, got, step.warnings)
		}
		if paused, _ := h.s.state.team(team.ID); paused.Paused {
			t.Fatal("a team under its limit paused")
		}
	}

	turn(200)
	paused, _ := h.s.state.team(team.ID)
	if !paused.Paused || paused.PauseReason != protocol.PausedByBudget {
		t.Fatalf("a team at its limit is paused %v for %q, want paused by its budget", paused.Paused, paused.PauseReason)
	}
	for _, task := range []protocol.TeamTask{first, second, queued} {
		if got := h.task(t, task.ID).Status; got != protocol.TaskStopped {
			t.Fatalf("task %s of a team at its limit is %q, want stopped", task.Brief, got)
		}
	}
	asked := h.pending(t)
	if asked.Kind != protocol.ApprovalBudget || asked.Team != team.ID {
		t.Fatalf("the team at its limit asks %+v", asked)
	}
	waiting := h.newTask(t, team, "After the raise.")
	if waiting.Status != protocol.TaskReady {
		t.Fatalf("a task of a paused team is %q, want queued", waiting.Status)
	}

	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	resumed, _ := h.s.state.team(team.ID)
	if resumed.Paused || resumed.Budget.MonthTokens <= 1100 {
		t.Fatalf("an approved raise left the team paused %v with a limit of %d", resumed.Paused, resumed.Budget.MonthTokens)
	}
	if got := h.task(t, waiting.ID).Status; got != protocol.TaskWorking {
		t.Fatalf("the queued task of a resumed team is %q, want working", got)
	}
	h.agent(t, waiting.ID)
	if got := h.ledgerCount(t, "budget.warn", ""); got != 1 {
		t.Fatalf("the team warned %d times in all, want once", got)
	}
}

// The month a team's clients show moves with every turn, not only when the
// team is next changed.
func TestATurnsTokensReachTheTeamsMonthAtOnce(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Counter", nil)
	ref := h.agent(t, h.newTask(t, team, "Count.").ID)
	usage := &protocol.TokenUsage{Input: 236, Output: 7096}
	h.s.AfterTurn(context.Background(), h.sessions.info(ref.Session), protocol.TurnEndData{Usage: usage})
	shown, ok := h.events.lastTeam(team.ID)
	if !ok || shown.Stats.MonthTokens != 7332 {
		t.Fatalf("after a turn of 7332 tokens clients were last told the team spent %d (told: %v)", shown.Stats.MonthTokens, ok)
	}
}
