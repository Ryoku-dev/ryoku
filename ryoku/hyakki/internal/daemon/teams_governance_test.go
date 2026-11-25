package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/teams"
)

type supervisedTeamAgents func(context.Context, core.AgentOptions) (core.Runner, error)

func (factory supervisedTeamAgents) NewAgent(ctx context.Context, opts core.AgentOptions) (core.Runner, error) {
	return factory(ctx, opts)
}

func supervisedTeamCall[T any](crew *teams.Service, ctx context.Context, method string, params any) (T, error) {
	var empty T
	raw, err := json.Marshal(params)
	if err != nil {
		return empty, err
	}
	result, err := crew.Handle(ctx, method, raw)
	if err != nil {
		return empty, err
	}
	value, ok := result.(T)
	if !ok {
		return empty, fmt.Errorf("unexpected result for %s: %T", method, result)
	}
	return value, nil
}

// Only model execution is controlled. The policy, persistent team store,
// session loop, goal advancement, callbacks and Stop are the real components.
func supervisedTeam(t *testing.T, budget protocol.TeamBudget, act func(*teams.Service, *sessionManager, core.AgentOptions, core.PromptInput, core.Sink) (core.TurnResult, error)) (*teams.Service, *sessionManager, protocol.TeamTask) {
	t.Helper()
	isolateDaemon(t)
	// No installed desktop helper, service manager or session bus is available
	// to this test, even when the ordinary suite runs on a live desktop.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "absent"))
	if err := os.MkdirAll(os.Getenv("XDG_RUNTIME_DIR"), 0700); err != nil {
		t.Fatal(err)
	}
	settings := config.Defaults()
	settings.Teams.RoutinesWake = false
	settings.Agent.Approvals = protocol.ApprovalsAuto
	crew, err := teams.Open(NewBroadcaster(), func() protocol.Settings { return settings })
	if err != nil {
		t.Fatal(err)
	}
	crew.SetModels(func(context.Context, protocol.ModelsParams) (protocol.ModelsResult, error) {
		return protocol.ModelsResult{Models: []protocol.ModelInfo{{ID: "test/local", Provider: "test", Available: true, Tools: true, Tier: protocol.TierFree}}}, nil
	})
	var manager *sessionManager
	factory := supervisedTeamAgents(func(_ context.Context, opts core.AgentOptions) (core.Runner, error) {
		runner := &fakeRunner{alive: true}
		runner.prompt = func(_ context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
			return act(crew, manager, opts, in, sink)
		}
		return runner, nil
	})
	manager = newSessionManager(nil, factory, nil, func() protocol.Settings { return settings }, func(string, any) {}, func() {}, func(string, ...any) {})
	manager.teams = crew
	manager.tools = func(info protocol.SessionInfo) core.ToolContext {
		return core.ToolContext{Session: info.ID, Cwd: info.Cwd, Team: crew.ToolContext(info)}
	}
	crew.Bind(manager)
	t.Cleanup(func() {
		for _, info := range manager.List(protocol.KindAgent) {
			_ = manager.Stop(context.Background(), info.ID)
		}
		_ = crew.Close()
	})
	team, err := supervisedTeamCall[protocol.Team](crew, context.Background(), protocol.MTeamsCreate, protocol.TeamCreateParams{
		Name: "Supervised governance", Folder: filepath.Join(t.TempDir(), "work"), Budget: budget, Approvals: protocol.ApprovalsAuto,
		Members: []protocol.Member{{Name: "Coordinator", Lead: true, Model: "test/local"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := supervisedTeamCall[protocol.TeamTask](crew, context.Background(), protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: team.ID, Brief: "Perform bounded local work"})
	if err != nil {
		t.Fatal(err)
	}
	return crew, manager, task
}

func waitSupervisedTeam(t *testing.T, crew *teams.Service, id string, done func(protocol.TeamTask) bool) protocol.TeamTask {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		result, err := supervisedTeamCall[protocol.TeamTasksResult](crew, context.Background(), protocol.MTeamsTasks, protocol.TeamTasksParams{})
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range result.Tasks {
			if task.ID == id && done(task) {
				return task
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("supervised task did not reach its required transition")
	return protocol.TeamTask{}
}

func TestTeamMonthlyCapDoesNotDeadlockItsOwnAfterTurn(t *testing.T) {
	crew, manager, task := supervisedTeam(t, protocol.TeamBudget{MonthTokens: 10, TaskTurns: 8}, func(_ *teams.Service, _ *sessionManager, _ core.AgentOptions, _ core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		stepTurn(1, sink)
		return core.TurnResult{Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 10}}, nil
	})
	ended := waitSupervisedTeam(t, crew, task.ID, func(task protocol.TeamTask) bool { return task.Status == protocol.TaskStopped && task.Session != "" })
	info := waitGoal(t, manager, ended.Session, func(goal *protocol.Goal) bool { return goal.Status != protocol.GoalActive })
	if info.State != protocol.StateStopped {
		deadline := time.Now().Add(5 * time.Second)
		for info.State != protocol.StateStopped && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
			info, _ = manager.Info(ended.Session)
		}
	}
	if info.State != protocol.StateStopped {
		t.Fatal("the budget callback never returned to allow real Stop to complete")
	}
	approvals, err := supervisedTeamCall[protocol.ApprovalsResult](crew, context.Background(), protocol.MTeamsApprovals, protocol.ApprovalsParams{Pending: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(approvals.Approvals) != 1 || approvals.Approvals[0].Kind != protocol.ApprovalBudget {
		t.Fatal("the cap stopped its callback before publishing the real budget decision")
	}
}

func TestTeamDecisionDeliveryCannotBeOvertakenByAutomaticGoalTurns(t *testing.T) {
	asked := make(chan struct{})
	releaseFirst := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseFirst) }) }
	t.Cleanup(release)
	second := make(chan string, 1)
	var turns atomic.Int32
	crew, manager, task := supervisedTeam(t, protocol.TeamBudget{TaskTurns: 2}, func(crew *teams.Service, _ *sessionManager, opts core.AgentOptions, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		turn := turns.Add(1)
		ref := protocol.AgentRef{Session: opts.Session.ID, Token: opts.Tools.Team.Token}
		ctx := teams.WithAgentSession(context.Background(), ref.Session)
		if turn == 1 {
			if _, err := supervisedTeamCall[protocol.AgentAnswer](crew, ctx, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: ref, Title: "Choose the bounded approach"}); err != nil {
				return core.TurnResult{}, err
			}
			close(asked)
			<-releaseFirst
			stepTurn(1, sink)
			return core.TurnResult{Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 1}}, nil
		}
		second <- in.Text
		_, err := supervisedTeamCall[protocol.AgentAnswer](crew, ctx, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Title: "Applied decision", Body: "The Summoner's decision reached the bounded final turn."})
		return core.TurnResult{Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 1}}, err
	})
	// This cleanup must release the runner before the helper waits for Stop.
	t.Cleanup(release)
	select {
	case <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("the first supervised turn did not ask its real decision")
	}
	approvals, err := supervisedTeamCall[protocol.ApprovalsResult](crew, context.Background(), protocol.MTeamsApprovals, protocol.ApprovalsParams{Pending: true})
	if err != nil || len(approvals.Approvals) != 1 {
		t.Fatalf("decision lookup failed: %+v %v", approvals, err)
	}
	const marker = "opaque-owner-decision-47"
	if _, err := supervisedTeamCall[protocol.Approval](crew, context.Background(), protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: approvals.Approvals[0].ID, Decision: protocol.DecideOnce, Note: marker}); err != nil {
		t.Fatal(err)
	}
	ready := waitSupervisedTeam(t, crew, task.ID, func(task protocol.TeamTask) bool { return task.Waiting == "" && task.Session != "" })
	session, err := manager.get(ready.Session)
	if err != nil {
		t.Fatal(err)
	}
	session.mu.Lock()
	reserved := session.handoffs > 0
	session.mu.Unlock()
	if !reserved {
		t.Fatal("task hold was cleared before the delivery reservation existed")
	}
	release()
	select {
	case text := <-second:
		if !strings.Contains(text, marker) {
			t.Fatal("an automatic goal continuation consumed the final turn before the owner's decision")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the decision was starved behind automatic goal continuations")
	}
	waitSupervisedTeam(t, crew, task.ID, func(task protocol.TeamTask) bool { return task.Status == protocol.TaskDone })
	if turns.Load() != 2 {
		t.Fatal("the handoff spent an extra unrequested turn")
	}
}

func TestTeamGoalResetCannotRefreshTheTaskTurnCeiling(t *testing.T) {
	var turns atomic.Int32
	crew, _, task := supervisedTeam(t, protocol.TeamBudget{TaskTurns: 2}, func(_ *teams.Service, manager *sessionManager, opts core.AgentOptions, _ core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		turn := turns.Add(1)
		stepTurn(int(turn), sink)
		_, err := manager.Goal(context.Background(), protocol.GoalParams{ID: opts.Session.ID, Op: protocol.GoalSet, Objective: "Try resetting the model's local goal allowance", MaxTurns: 100})
		return core.TurnResult{Stop: protocol.StopEnd, Usage: &protocol.TokenUsage{Input: 1}}, err
	})
	ended := waitSupervisedTeam(t, crew, task.ID, func(task protocol.TeamTask) bool { return task.Status == protocol.TaskBlocked })
	if ended.Turns != 2 || turns.Load() != 2 {
		t.Fatalf("model goal resets refreshed the task's allowance: task=%d runner=%d", ended.Turns, turns.Load())
	}
}
