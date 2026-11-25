package teams

import (
	"context"
	"errors"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func TestUnmeteredMemberAccountingFailsClosedWithoutBlockingOtherSeats(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	team.Budget.MonthTokens = 0
	team.Members[1].Budget = &protocol.TeamBudget{MonthTokens: 100}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	seat, peer := team.Members[0], team.Members[1]
	mine := memberBudgetTask(t, h, team, seat.ID, "")
	other := memberBudgetTask(t, h, team, peer.ID, "")
	if err := h.s.addSpend(team.ID, seat.ID, "", "", h.clock.now(), protocol.TokenUsage{Input: 17, Output: 3, Unmetered: true}); err != nil {
		t.Fatal(err)
	}
	if used, err := h.s.budgetTokens(team.ID, seat.ID, h.clock.now()); used != 20 || !errors.Is(err, errUnmeteredBudget) {
		t.Fatalf("incomplete seat account lost known tokens or uncertainty: %d, %v", used, err)
	}
	if used, err := h.s.budgetTokens(team.ID, peer.ID, h.clock.now()); used != 0 || err != nil {
		t.Fatalf("another seat inherited unknown usage: %d, %v", used, err)
	}
	if h.s.memberBudgetReady(team, seat) || !h.s.memberBudgetReady(team, peer) {
		t.Fatal("unmetered member usage did not respect independent seat caps")
	}
	// A later measured turn cannot retroactively meter an earlier unknown one.
	memberBudgetSpend(t, h, team, seat.ID, 5)
	h.s.checkMemberBudget(team.ID, seat.ID, 5)
	if h.task(t, mine.ID).Status != protocol.TaskStopped || h.task(t, other.ID).Status != protocol.TaskReady {
		t.Fatal("unknown accounting stopped the wrong seat's task")
	}
	current, _ := h.s.state.team(team.ID)
	if current.Paused || !current.Members[0].Paused || current.Members[1].Paused {
		t.Fatalf("member uncertainty incorrectly paused the team or its peer: %+v", current)
	}
	asked := h.pending(t)
	rec, _ := h.s.state.approval(asked.ID)
	if _, err := h.s.effectMemberBudget(context.Background(), &rec, protocol.ApprovalDecideParams{Decision: protocol.DecideOnce}); !errors.Is(err, errUnmeteredBudget) {
		t.Fatalf("raising a token count invented a measurement: %v", err)
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &Service{state: reopened, now: h.clock.now}
	if used, err := restarted.budgetTokens(team.ID, seat.ID, h.clock.now()); used != 25 || !errors.Is(err, errUnmeteredBudget) {
		t.Fatalf("restart forgot durable unmetered usage: %d, %v", used, err)
	}
	if !restarted.memberBudgetReady(team, peer) {
		t.Fatal("restarted accounting globally poisoned an independent peer")
	}
}

func TestUnmeteredZeroUsageIsDurableAndLabeledAtEverySpendLevel(t *testing.T) {
	h, team := memberBudgetHarness(t, 0)
	team.Budget.MonthTokens = 0
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	at := h.clock.now()
	if err := h.s.addSpend(team.ID, team.Members[0].ID, "unmetered-routine", "test/big", at, protocol.TokenUsage{Unmetered: true}); err != nil {
		t.Fatal(err)
	}
	if !h.s.memberBudgetReady(team, team.Members[0]) {
		t.Fatal("unmetered work without any token cap was prohibited")
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &Service{state: reopened, now: h.clock.now}
	shown, err := restarted.spend(protocol.TeamSpendParams{Team: team.ID})
	if err != nil {
		t.Fatal(err)
	}
	if shown.Tokens != 0 || !shown.Unmetered || shown.Estimated || !restarted.monthUnmetered(team.ID, at) {
		t.Fatalf("unknown usage was presented as measured or estimated free work: %+v", shown)
	}
	if len(shown.Days) != 1 || !shown.Days[0].Unmetered || shown.Days[0].Input != 0 || shown.Days[0].Output != 0 {
		t.Fatalf("daily unknown usage was lost: %+v", shown.Days)
	}
	for name, rows := range map[string][]protocol.NamedSpend{"members": shown.Members, "routines": shown.Routines, "engines": shown.Engines} {
		if len(rows) != 1 || !rows[0].Unmetered || rows[0].Tokens != 0 {
			t.Errorf("%s showed unmetered usage as a measured zero: %+v", name, rows)
		}
	}
	all, err := restarted.spend(protocol.TeamSpendParams{})
	if err != nil {
		t.Fatal(err)
	}
	if !all.Unmetered || len(all.Teams) != 1 || !all.Teams[0].Unmetered {
		t.Fatalf("aggregate team accounting concealed unknown usage: %+v", all)
	}
	// The maps returned for presentation must not mutate budget authority.
	month, err := restarted.spendOfMonth(at.Format(monthLayout))
	if err != nil {
		t.Fatal(err)
	}
	month[team.ID].UnmeteredMembers[team.Members[0].ID] = false
	month[team.ID].UnmeteredRoutines["unmetered-routine"] = false
	month[team.ID].UnmeteredEngines["test/big"] = false
	if _, err := restarted.budgetTokens(team.ID, team.Members[0].ID, at); !errors.Is(err, errUnmeteredBudget) {
		t.Fatal("presentation clone modified the durable member uncertainty")
	}
	shown, err = restarted.spend(protocol.TeamSpendParams{Team: team.ID})
	if err != nil || !shown.Routines[0].Unmetered || !shown.Engines[0].Unmetered {
		t.Fatalf("presentation clone modified routine/engine markers: %+v, %v", shown, err)
	}
}

func TestSharedTokenCapFailsClosedOnUnmeteredUsageAcrossRestart(t *testing.T) {
	h, team := memberBudgetHarness(t, 0)
	first := memberBudgetTask(t, h, team, team.Members[0].ID, "")
	second := memberBudgetTask(t, h, team, team.Members[1].ID, "")
	if err := h.s.addSpend(team.ID, team.Members[0].ID, "", "", h.clock.now(), protocol.TokenUsage{Unmetered: true}); err != nil {
		t.Fatal(err)
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &Service{state: reopened, now: h.clock.now}
	if used, err := restarted.budgetTokens(team.ID, "", h.clock.now()); used != 0 || !errors.Is(err, errUnmeteredBudget) {
		t.Fatalf("restart treated incomplete team accounting as free: %d, %v", used, err)
	}
	for _, member := range team.Members {
		if restarted.memberBudgetReady(team, member) {
			t.Fatal("shared cap allowed work with unknown team usage")
		}
	}
	h.s.checkBudget(team.ID, 0)
	current, _ := h.s.state.team(team.ID)
	if !current.Paused || current.PauseReason != protocol.PausedByBudget {
		t.Fatal("shared token cap ignored unmetered usage")
	}
	if h.task(t, first.ID).Status != protocol.TaskStopped || h.task(t, second.ID).Status != protocol.TaskStopped {
		t.Fatal("shared cap left one of its seats running")
	}
	if asked := h.pending(t); asked.Kind != protocol.ApprovalBudget {
		t.Fatalf("shared uncertainty requested the wrong approval: %+v", asked)
	}
	h.clock.set(time.Date(2026, time.December, 1, 0, 0, 0, 0, time.Local))
	if used, err := restarted.budgetTokens(team.ID, "", h.clock.now()); used != 0 || err != nil || restarted.monthUnmetered(team.ID, h.clock.now()) {
		t.Fatalf("unknown usage leaked into the next month: %d, %v", used, err)
	}
}

func TestUnattributedUnmeteredUsageCannotBeAssignedToAFreeSeat(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	team.Budget.MonthTokens = 0
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	if err := h.s.addSpend(team.ID, "", "", "", h.clock.now(), protocol.TokenUsage{Unmetered: true}); err != nil {
		t.Fatal(err)
	}
	if h.s.memberBudgetReady(team, team.Members[0]) {
		t.Fatal("unattributed usage bypassed a configured seat cap")
	}
	if !h.s.memberBudgetReady(team, team.Members[1]) {
		t.Fatal("unattributed usage prohibited a seat with no token cap")
	}
}

func TestNativeMeasuredUsageRemainsMeasured(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	if err := h.s.addSpend(team.ID, team.Members[0].ID, "", "test/big", h.clock.now(), protocol.TokenUsage{Input: 17, Output: 3}); err != nil {
		t.Fatal(err)
	}
	shown, err := h.s.spend(protocol.TeamSpendParams{Team: team.ID})
	if err != nil {
		t.Fatal(err)
	}
	if shown.Tokens != 20 || shown.Unmetered || h.s.monthUnmetered(team.ID, h.clock.now()) || !h.s.memberBudgetReady(team, team.Members[0]) {
		t.Fatalf("ordinary native accounting changed: %+v", shown)
	}
	if len(shown.Members) != 1 || shown.Members[0].Unmetered || len(shown.Engines) != 1 || shown.Engines[0].Unmetered || len(shown.Days) != 1 || shown.Days[0].Unmetered {
		t.Fatal("measured accounting acquired unknown-usage markers")
	}
}
