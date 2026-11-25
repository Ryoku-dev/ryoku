package teams

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func memberBudgetHarness(t *testing.T, cap int64) (*harness, protocol.Team) {
	t.Helper()
	// Queued work stays deterministic; no asynchronous engine or wall-clock
	// timer is needed to exercise monthly and deadline transitions.
	h := newHarnessWith(t, func(settings *protocol.Settings) { settings.Teams.Paused = true })
	team := h.newTeam(t, "Seat budgets", func(p *protocol.TeamCreateParams) {
		p.Budget = protocol.TeamBudget{MonthTokens: 1_000_000, TaskTurns: 10, TaskMinutes: 20}
	})
	team.Members[0].Budget = &protocol.TeamBudget{MonthTokens: cap}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	team, _ = h.s.state.team(team.ID)
	return h, team
}

func memberBudgetSpend(t *testing.T, h *harness, team protocol.Team, member string, tokens int) {
	t.Helper()
	if err := h.s.addSpend(team.ID, member, "", "", h.clock.now(), protocol.TokenUsage{Input: tokens}); err != nil {
		t.Fatal(err)
	}
}

func memberBudgetTask(t *testing.T, h *harness, team protocol.Team, member, parent string) protocol.TeamTask {
	t.Helper()
	now := h.clock.now()
	task := protocol.TeamTask{ID: newID("task"), Team: team.ID, Member: member, Parent: parent, Title: "Budget boundary", Status: protocol.TaskReady, Created: now, Updated: now}
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	return task
}

func TestMemberBudgetWarningsPersistAndResetOnlyWithMonthOrCap(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	member := team.Members[0]
	memberBudgetSpend(t, h, team, member.ID, 79)
	h.s.checkMemberBudget(team.ID, member.ID, 79)
	if got := h.ledgerCount(t, "member.budget.warn", ""); got != 0 {
		t.Fatalf("warned before 80%%: %d", got)
	}
	memberBudgetSpend(t, h, team, member.ID, 1)
	for range 5 {
		h.s.checkMemberBudget(team.ID, member.ID, 80)
	}
	if got := h.ledgerCount(t, "member.budget.warn", ""); got != 1 {
		t.Fatalf("duplicate callbacks produced %d warnings", got)
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &Service{state: reopened, now: h.clock.now}
	warn, err := restarted.markMemberBudgetWarning(team.ID, member.ID, h.clock.now(), 100, 80)
	if err != nil || warn {
		t.Fatalf("restart forgot warning: warn=%v err=%v", warn, err)
	}
	if used, err := restarted.budgetTokens(team.ID, member.ID, h.clock.now()); err != nil || used != 80 {
		t.Fatalf("restart lost durable member totals: used=%d err=%v", used, err)
	}
	h.clock.set(time.Date(2026, time.December, 1, 0, 0, 0, 0, time.Local))
	if !h.s.memberBudgetReady(team, member) {
		t.Fatal("an unpaused seat inherited last month's usage")
	}
	memberBudgetSpend(t, h, team, member.ID, 80)
	h.s.checkMemberBudget(team.ID, member.ID, 80)
	if got := h.ledgerCount(t, "member.budget.warn", ""); got != 2 {
		t.Fatalf("new month did not get exactly one new warning: %d", got)
	}
	current, _ := h.s.state.team(team.ID)
	budget := *current.Members[0].Budget
	budget.MonthTokens = 110
	current.Members[0].Budget = &budget
	if err := h.s.state.putTeam(current); err != nil {
		t.Fatal(err)
	}
	memberBudgetSpend(t, h, team, member.ID, 8)
	h.s.checkMemberBudget(team.ID, member.ID, 8)
	if got := h.ledgerCount(t, "member.budget.warn", ""); got != 3 {
		t.Fatalf("new cap's exact 80%% boundary did not warn once: %d", got)
	}
}

func TestConcurrentMemberCapsPauseOnlyThatSeatAndSurviveRestart(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	seat, peer := team.Members[0], team.Members[1]
	first := memberBudgetTask(t, h, team, seat.ID, "")
	second := memberBudgetTask(t, h, team, seat.ID, "")
	// The capped seat's delegation must not recursively stop a peer's work.
	delegated := memberBudgetTask(t, h, team, peer.ID, first.ID)
	memberBudgetSpend(t, h, team, seat.ID, 99)
	h.s.checkMemberBudget(team.ID, seat.ID, 99)
	if !h.s.memberBudgetReady(team, seat) {
		t.Fatal("a seat one token below its cap cannot run")
	}
	memberBudgetSpend(t, h, team, seat.ID, 1)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.s.checkMemberBudget(team.ID, seat.ID, 1)
		}()
	}
	wg.Wait()
	current, _ := h.s.state.team(team.ID)
	if current.Paused || current.Budget != team.Budget || !current.Members[0].Paused || current.Members[0].PauseReason != protocol.PausedByBudget || current.Members[1].Paused {
		t.Fatalf("cap escaped its seat: %+v", current)
	}
	for _, task := range []protocol.TeamTask{first, second} {
		if got := h.task(t, task.ID).Status; got != protocol.TaskStopped {
			t.Fatalf("capped seat task is %q", got)
		}
	}
	if got := h.task(t, delegated.ID).Status; got != protocol.TaskReady {
		t.Fatalf("another seat's delegated task was affected: %s", got)
	}
	approval := h.pending(t)
	if approval.Kind != protocol.ApprovalMemberBudget || approval.Member != seat.ID || approval.Task != "" || !approval.Expires.IsZero() {
		t.Fatalf("wrong member budget approval: %+v", approval)
	}
	if got := h.ledgerCount(t, "member.budget.stop", ""); got != 1 {
		t.Fatalf("concurrent cap emitted %d stops", got)
	}
	if h.s.memberBudgetReady(team, seat) || !h.s.memberBudgetReady(team, peer) {
		t.Fatal("readiness used a stale roster or blocked the wrong seat")
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &Service{state: reopened, now: h.clock.now}
	if restarted.memberBudgetReady(team, seat) || len(reopened.approvals(func(rec approvalRecord) bool { return rec.Kind == protocol.ApprovalMemberBudget })) != 1 {
		t.Fatal("restart lost the seat pause or its approval")
	}
	h.clock.set(time.Date(2026, time.December, 1, 0, 0, 0, 0, time.Local))
	if h.s.memberBudgetReady(team, seat) {
		t.Fatal("month rollover silently lifted an explicit budget pause")
	}
}

func TestMemberBudgetApprovalRaisesOnlyTheSeatAndDenialKeepsItPaused(t *testing.T) {
	for _, deny := range []bool{false, true} {
		t.Run(map[bool]string{false: "raise", true: "deny"}[deny], func(t *testing.T) {
			h, team := memberBudgetHarness(t, 100)
			member := team.Members[0]
			memberBudgetSpend(t, h, team, member.ID, 100)
			h.s.checkMemberBudget(team.ID, member.ID, 100)
			asked := h.pending(t)
			decision := protocol.DecideOnce
			if deny {
				decision = protocol.DecideDeny
			}
			do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: decision})
			current, _ := h.s.state.team(team.ID)
			if current.Budget != team.Budget || current.Paused || current.Members[1].Paused {
				t.Fatal("member approval changed team or peer limits")
			}
			if deny {
				if !current.Members[0].Paused || current.Members[0].Budget.MonthTokens != 100 {
					t.Fatal("denial lifted the cap pause")
				}
			} else if current.Members[0].Paused || current.Members[0].Budget.MonthTokens != 100_000 || !h.s.memberBudgetReady(current, current.Members[0]) {
				t.Fatalf("raise did not resume only the seat: %+v", current.Members[0])
			}
			refused(t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce}, protocol.CodeConflict)
			h.s.checkMemberBudget(team.ID, member.ID, 100)
			if got := len(h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals); got != 0 {
				t.Fatalf("duplicate usage recreated %d approval cards", got)
			}
		})
	}
}

func TestMemberBudgetNeverResurrectsDeletedSeatsOrAcceptsNameFallback(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	seat := team.Members[0]
	memberBudgetSpend(t, h, team, seat.ID, 100)
	h.s.checkMemberBudget(team.ID, seat.ID, 100)
	asked := h.pending(t)
	current, _ := h.s.state.team(team.ID)
	current.Members = current.Members[1:]
	current.Members[0].Name = seat.ID
	if err := h.s.state.putTeam(current); err != nil {
		t.Fatal(err)
	}
	if h.s.memberBudgetReady(team, seat) {
		t.Fatal("deleted ID resolved through another seat's display name")
	}
	h.s.checkMemberBudget(team.ID, seat.ID, 100)
	rec, _ := h.s.state.approval(asked.ID)
	if _, err := h.s.effectMemberBudget(context.Background(), &rec, protocol.ApprovalDecideParams{Decision: protocol.DecideOnce}); err == nil {
		t.Fatal("an old approval raised a removed member")
	}
	current, _ = h.s.state.team(team.ID)
	if len(current.Members) != 1 || current.Members[0].ID == seat.ID || current.Members[0].Paused {
		t.Fatalf("deleted seat came back or changed its peer: %+v", current.Members)
	}
}

func TestMemberBudgetRaiseCannotLiftTeamCeilingOrEditedSeat(t *testing.T) {
	for _, edit := range []string{"team ceiling", "seat budget", "manual pause"} {
		t.Run(edit, func(t *testing.T) {
			h, team := memberBudgetHarness(t, 100)
			memberBudgetSpend(t, h, team, team.Members[0].ID, 100)
			h.s.checkMemberBudget(team.ID, team.Members[0].ID, 100)
			asked := h.pending(t)
			current, _ := h.s.state.team(team.ID)
			switch edit {
			case "team ceiling":
				current.Budget.MonthTokens = 100
			case "seat budget":
				budget := *current.Members[0].Budget
				budget.MonthTokens = 200
				current.Members[0].Budget = &budget
			case "manual pause":
				current.Members[0].PauseReason = protocol.PausedByYou
			}
			if err := h.s.state.putTeam(current); err != nil {
				t.Fatal(err)
			}
			rec, _ := h.s.state.approval(asked.ID)
			if _, err := h.s.effectMemberBudget(context.Background(), &rec, protocol.ApprovalDecideParams{Decision: protocol.DecideOnce}); err == nil {
				t.Fatal("stale or impossible raise was accepted")
			}
			after, _ := h.s.state.team(team.ID)
			if !after.Members[0].Paused || after.Budget != current.Budget || after.Members[0].Budget.MonthTokens != current.Members[0].Budget.MonthTokens {
				t.Fatal("failed approval changed the cap or unpaused work")
			}
		})
	}
}

func TestMemberBudgetFailsClosedOnUnreadableOrDamagedAccounting(t *testing.T) {
	for _, contents := range []string{"not JSON", "null", `{"2026-11":{"TEAM":{"days":{"2026-11-11":{"input":-1,"output":0}}}}}`} {
		t.Run(contents, func(t *testing.T) {
			h, team := memberBudgetHarness(t, 100)
			// Flush the cache to exercise the same read as a daemon restart.
			h.s.state.spendMu.Lock()
			h.s.state.spendData = nil
			h.s.state.spendMu.Unlock()
			if err := os.WriteFile(filepath.Join(h.dir, "spend.json"), []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if h.s.memberBudgetReady(team, team.Members[0]) {
				t.Fatal("damaged accounting gave a capped seat free work")
			}
			task := memberBudgetTask(t, h, team, team.Members[0].ID, "")
			h.s.checkMemberBudget(team.ID, team.Members[0].ID, 1)
			if got := h.task(t, task.ID).Status; got != protocol.TaskStopped {
				t.Fatalf("accounting failure left task %s", got)
			}
			asked := h.pending(t)
			rec, _ := h.s.state.approval(asked.ID)
			if _, err := h.s.effectMemberBudget(context.Background(), &rec, protocol.ApprovalDecideParams{Decision: protocol.DecideOnce}); err == nil {
				t.Fatal("raise ignored unavailable accounting")
			}
			if _, err := h.s.spend(protocol.TeamSpendParams{Team: team.ID}); err == nil {
				t.Fatal("spend API reported a free month after read failure")
			}
		})
	}
}

func TestSpendWriteFailureIsStickyForCappedSeats(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	path := filepath.Join(h.dir, "spend.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := h.s.addSpend(team.ID, team.Members[0].ID, "", "", h.clock.now(), protocol.TokenUsage{Input: 1}); err == nil {
		t.Fatal("write failure was discarded")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := h.s.addSpend(team.ID, team.Members[0].ID, "", "", h.clock.now(), protocol.TokenUsage{Input: 1}); err == nil {
		t.Fatal("a later readable path hid an earlier lost durable count")
	}
	if h.s.memberBudgetReady(team, team.Members[0]) || h.s.monthTokens(team.ID, h.clock.now()) != math.MaxInt64 {
		t.Fatal("write failure was treated as free spend")
	}
}

func TestSpendSaturatesBeforeIntegerAdditionCanWrap(t *testing.T) {
	h, team := memberBudgetHarness(t, math.MaxInt64)
	if err := h.s.addSpend(team.ID, team.Members[0].ID, "routine", "test/big", h.clock.now(), protocol.TokenUsage{Input: math.MaxInt, Output: math.MaxInt}); err != nil {
		t.Fatal(err)
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	reopened.spendMu.Lock()
	file, err := reopened.spendLocked()
	if err != nil {
		reopened.spendMu.Unlock()
		t.Fatal(err)
	}
	month := file[h.clock.now().Format(monthLayout)][team.ID]
	memberTokens := month.Members[team.Members[0].ID]
	routineTokens, engineTokens := month.Routines["routine"], month.Engines["test/big"]
	total := month.tokens()
	reopened.spendMu.Unlock()
	if memberTokens != math.MaxInt64 || routineTokens != math.MaxInt64 || engineTokens != math.MaxInt64 || total != math.MaxInt64 {
		t.Fatalf("overflow wrapped: member=%d routine=%d engine=%d total=%d", memberTokens, routineTokens, engineTokens, total)
	}
	if h.s.memberBudgetReady(team, team.Members[0]) {
		t.Fatal("overflow made a max-capped seat runnable")
	}
}

func TestMemberTaskLimitsOnlyNarrowIncludingDreams(t *testing.T) {
	for _, dream := range []bool{false, true} {
		team := protocol.Team{Budget: protocol.TeamBudget{TaskTurns: 10, TaskMinutes: 20}}
		if dream {
			team.Dream = &protocol.Dream{}
		}
		baseTurns, baseMinutes := 10, 20
		if dream {
			baseTurns, baseMinutes = protocol.TeamTaskTurnsMax, 0
		}
		for _, test := range []struct {
			name           string
			budget         *protocol.TeamBudget
			turns, minutes int
		}{
			{"inherit nil", nil, baseTurns, baseMinutes},
			{"inherit zero", &protocol.TeamBudget{}, baseTurns, baseMinutes},
			{"narrow", &protocol.TeamBudget{TaskTurns: 3, TaskMinutes: 2}, 3, 2},
		} {
			member := protocol.Member{Budget: test.budget}
			if got := effectiveTaskTurns(team, member); got != test.turns {
				t.Errorf("dream=%v %s turns=%d want %d", dream, test.name, got, test.turns)
			}
			if got := effectiveTaskMinutes(team, member); got != test.minutes {
				t.Errorf("dream=%v %s minutes=%d want %d", dream, test.name, got, test.minutes)
			}
		}
	}
	team := protocol.Team{Budget: protocol.TeamBudget{TaskTurns: 10, TaskMinutes: 20}}
	member := protocol.Member{Budget: &protocol.TeamBudget{TaskTurns: 100, TaskMinutes: 100}}
	if effectiveTaskTurns(team, member) != 10 || effectiveTaskMinutes(team, member) != 20 {
		t.Fatal("member task caps widened the team caps")
	}
}

func TestMemberDeadlineBlocksAtExactBoundaryWithoutStoppingOtherSeats(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	team.Budget.TaskMinutes = 0
	team.Dream = &protocol.Dream{}
	team.Members[0].Budget.TaskMinutes = 2
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	task := memberBudgetTask(t, h, team, team.Members[0].ID, "")
	peer := memberBudgetTask(t, h, team, team.Members[1].ID, task.ID)
	started := time.Date(2026, time.November, 11, 10, 0, 0, 0, time.Local)
	task.Started, task.Status = &started, protocol.TaskWorking
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	token := runToken()
	h.s.mu.Lock()
	h.s.runs[task.ID] = &run{Task: task.ID, Team: task.Team, Member: task.Member, Token: token, Started: started}
	h.s.mu.Unlock()
	h.clock.mu.Lock()
	h.clock.frozen = true
	h.clock.at = started.Add(2*time.Minute - time.Nanosecond)
	h.clock.mu.Unlock()
	h.s.overrun(task.ID, token)
	if got := h.task(t, task.ID).Status; got != protocol.TaskWorking {
		t.Fatalf("deadline fired early: %s", got)
	}
	h.clock.set(started.Add(2 * time.Minute))
	h.s.overrun(task.ID, token)
	if got := h.task(t, task.ID).Status; got != protocol.TaskBlocked {
		t.Fatalf("explicit Dream seat deadline did not fire: %s", got)
	}
	if got := h.task(t, peer.ID).Status; got != protocol.TaskReady {
		t.Fatalf("seat deadline stopped a delegated peer: %s", got)
	}
}

func TestBudgetArithmeticBoundaries(t *testing.T) {
	if mark := budgetWarnMark(math.MaxInt64, 80); mark != 7_378_697_629_483_820_646 {
		t.Fatalf("overflowed warning mark: %d", mark)
	}
	if budgetWarnMark(1, 80) != 1 || budgetWarnMark(101, 80) != 81 {
		t.Fatal("warning mark fired below its configured percentage")
	}
	for _, test := range []struct{ limit, used, want int64 }{
		{100, 100, 100_000},
		{200_000, 300_000, 400_000},
		{math.MaxInt64 - 1, math.MaxInt64 - 1, math.MaxInt64},
	} {
		got, err := checkedBudgetRaise(test.limit, test.used)
		if err != nil || got != test.want {
			t.Errorf("raise(%d,%d)=%d,%v want %d", test.limit, test.used, got, err, test.want)
		}
	}
	if _, err := checkedBudgetRaise(math.MaxInt64, math.MaxInt64); err == nil {
		t.Fatal("exhausted int64 cap wrapped into a new grant")
	}
}

func TestMemberRaiseStopsAtTeamCeilingAndNeverUnpausesTheTeam(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	memberBudgetSpend(t, h, team, team.Members[0].ID, 100)
	h.s.checkMemberBudget(team.ID, team.Members[0].ID, 100)
	asked := h.pending(t)
	current, _ := h.s.state.team(team.ID)
	current.Budget.MonthTokens = 500
	current.Paused, current.PauseReason = true, protocol.PausedByYou
	if err := h.s.state.putTeam(current); err != nil {
		t.Fatal(err)
	}
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	after, _ := h.s.state.team(team.ID)
	if after.Members[0].Budget.MonthTokens != 500 || after.Members[0].Paused || after.Budget.MonthTokens != 500 || !after.Paused || after.PauseReason != protocol.PausedByYou {
		t.Fatalf("seat raise widened or unpaused the team: %+v", after)
	}
	if h.s.memberBudgetReady(after, after.Members[0]) {
		t.Fatal("seat readiness bypassed a manually paused team")
	}
}

func TestMemberBudgetInheritsTeamCapAndNeverBypassesManualSeatPause(t *testing.T) {
	h, team := memberBudgetHarness(t, 0)
	memberBudgetSpend(t, h, team, team.Members[1].ID, 1_000_000)
	if h.s.memberBudgetReady(team, team.Members[0]) {
		t.Fatal("an uncapped seat bypassed the team's exhausted cap")
	}
	current, _ := h.s.state.team(team.ID)
	current.Budget.MonthTokens = 0
	current.Members[0].Paused, current.Members[0].PauseReason = true, protocol.PausedByYou
	if err := h.s.state.putTeam(current); err != nil {
		t.Fatal(err)
	}
	if h.s.memberBudgetReady(team, team.Members[0]) {
		t.Fatal("readiness bypassed a current seat pause using a stale roster")
	}
}

func TestNegativeUsageCannotCreditTheMemberAccount(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	memberBudgetSpend(t, h, team, team.Members[0].ID, 80)
	if err := h.s.addSpend(team.ID, team.Members[0].ID, "", "", h.clock.now(), protocol.TokenUsage{Input: -80, Output: 1}); err == nil {
		t.Fatal("negative usage was accepted as a refund")
	}
	if h.s.memberBudgetReady(team, team.Members[0]) {
		t.Fatal("invalid usage left a capped seat runnable")
	}
}

func TestMemberCapHaltsTheSessionEvenWhenStoppedTaskCannotPersist(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	task := memberBudgetTask(t, h, team, team.Members[0].ID, "")
	info, err := h.sessions.New(context.Background(), protocol.SessionNewParams{Team: team.ID, TeamTask: task.ID, Member: task.Member})
	if err != nil {
		t.Fatal(err)
	}
	task.Session, task.Status = info.ID, protocol.TaskWorking
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	// The monthly account and board remain writable, but this particular
	// task cannot be replaced. The fail-closed path must still stop its agent.
	path := h.s.state.taskPath(task.ID)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	memberBudgetSpend(t, h, team, task.Member, 100)
	h.s.checkMemberBudget(team.ID, task.Member, 100)
	waitFor(t, "the capped session to stop despite task-file failure", func() bool {
		return h.sessions.wasStopped(info.ID)
	})
	if h.s.memberBudgetReady(team, team.Members[0]) {
		t.Fatal("failed task status write bypassed the seat cap")
	}
	if asked := h.pending(t); asked.Member != task.Member {
		t.Fatalf("failed task write lost the member approval: %+v", asked)
	}
}

// delayedBudgetSessions pauses only the session read which precedes the
// locked end transition. State, spend, approvals and task lifecycle stay real.
type delayedBudgetSessions struct {
	*fakeSessions
	target  string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (sessions *delayedBudgetSessions) Info(id string) (protocol.SessionInfo, bool) {
	if id == sessions.target {
		sessions.once.Do(func() {
			close(sessions.entered)
			<-sessions.release
		})
	}
	return sessions.fakeSessions.Info(id)
}

func TestDelayedMemberCapStopCannotUndoConcurrentApprovedRaise(t *testing.T) {
	h, team := memberBudgetHarness(t, 100)
	task := memberBudgetTask(t, h, team, team.Members[0].ID, "")
	info, err := h.sessions.New(context.Background(), protocol.SessionNewParams{Team: team.ID, TeamTask: task.ID, Member: task.Member})
	if err != nil {
		t.Fatal(err)
	}
	task.Session, task.Status = info.ID, protocol.TaskWorking
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	delayed := &delayedBudgetSessions{fakeSessions: h.sessions, target: info.ID, entered: make(chan struct{}), release: make(chan struct{})}
	h.s.mu.Lock()
	h.s.sessions = delayed
	h.s.mu.Unlock()
	memberBudgetSpend(t, h, team, task.Member, 100)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.s.checkMemberBudget(team.ID, task.Member, 100)
	}()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(delayed.release) }) }
	t.Cleanup(func() {
		release()
		<-done
	})
	select {
	case <-delayed.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("cap callback did not reach the delayed task transition")
	}
	paused, _ := h.s.state.team(team.ID)
	observed, _ := budgetMember(paused, task.Member)
	if !observed.Paused {
		t.Fatal("stop delay occurred before the member pause persisted")
	}
	asked := h.pending(t)
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce})
	authorized := memberBudgetTask(t, h, team, task.Member, "")
	release()
	<-done
	// Exercise both a callback already inside endTaskIf and an old callback
	// which selects its tasks only after the raise completed.
	h.s.stopMemberBudgetTasks(team.ID, observed)
	if got := h.task(t, task.ID).Status; got != protocol.TaskWorking {
		t.Fatalf("delayed pre-raise stop ended resumed work: %s", got)
	}
	if got := h.task(t, authorized.ID).Status; got != protocol.TaskReady {
		t.Fatalf("stale cap generation stopped newly authorized work: %s", got)
	}
	if h.sessions.wasStopped(info.ID) {
		t.Fatal("stale stop halted the authorized session")
	}
	if got := h.ledgerCount(t, "task.stop", task.ID); got != 0 {
		t.Fatalf("stale generation recorded %d task stops", got)
	}
	current, _ := h.s.state.team(team.ID)
	if !h.s.memberBudgetReady(current, current.Members[0]) {
		t.Fatal("concurrent raise did not retain the authorized member budget")
	}
}
