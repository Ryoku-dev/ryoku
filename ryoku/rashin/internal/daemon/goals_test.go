package daemon

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// goalManager runs an Agent session whose runner calls act on each turn,
// with the text that turn was sent
func goalManager(t *testing.T, act func(turn int, text string, sink core.Sink, manager *sessionManager, id string) (core.TurnResult, error)) (*sessionManager, protocol.SessionInfo, func() []string) {
	t.Helper()
	isolateDaemon(t)
	var mu sync.Mutex
	var sent []string
	var manager *sessionManager
	var id string
	runner := &fakeRunner{alive: true}
	runner.prompt = func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		mu.Lock()
		sent = append(sent, in.Text)
		turn := len(sent)
		mu.Unlock()
		return act(turn, in.Text, sink, manager, id)
	}
	settings := func() protocol.Settings {
		return protocol.Settings{Agent: protocol.AgentSettings{Approvals: protocol.ApprovalsReadOnly}}
	}
	manager = newSessionManager(nil, &fakeAgents{runner: runner}, nil, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	info, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	id = info.ID
	return manager, info, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), sent...)
	}
}

func waitGoal(t *testing.T, manager *sessionManager, id string, done func(*protocol.Goal) bool) protocol.SessionInfo {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info, _ := manager.Info(id)
		if info.Goal != nil && done(info.Goal) && info.State != protocol.StateWorking {
			return info
		}
		time.Sleep(5 * time.Millisecond)
	}
	info, _ := manager.Info(id)
	t.Fatalf("goal never settled: %+v", info.Goal)
	return info
}

func stepTurn(turn int, sink core.Sink) {
	sink.Emit(protocol.EvTool, protocol.ToolData{Call: "step-" + strconv.Itoa(turn), Title: "Step", Status: protocol.ToolDone})
}

// A goal keeps the agent going turn after turn on the whole objective until
// the agent reports it done; the first turn carries the objective, the
// ones after are continuations the transcript marks as Rashin's.
func TestAGoalRunsUntilTheAgentCompletesIt(t *testing.T) {
	manager, info, sent := goalManager(t, func(turn int, _ string, sink core.Sink, manager *sessionManager, id string) (core.TurnResult, error) {
		stepTurn(turn, sink)
		if turn == 3 {
			if _, err := manager.goal(context.Background(), protocol.GoalParams{ID: id, Op: protocol.GoalComplete, Summary: "Built and checked."}); err != nil {
				t.Error(err)
			}
		}
		return core.TurnResult{}, nil
	})
	runner := manager.agents.(*fakeAgents).runner.(*fakeRunner)
	original := runner.prompt
	var flagsMu sync.Mutex
	var flags []bool
	runner.prompt = func(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
		flagsMu.Lock()
		flags = append(flags, in.Goal)
		flagsMu.Unlock()
		return original(ctx, in, sink)
	}
	if handled, err := manager.goalCommand(context.Background(), info.ID, "/goal Make the parser total"); !handled || err != nil {
		t.Fatalf("handled %v err %v", handled, err)
	}
	settled := waitGoal(t, manager, info.ID, func(goal *protocol.Goal) bool { return goal.Status != protocol.GoalActive })
	goal := settled.Goal
	if goal.Status != protocol.GoalDone || goal.Reason != "Built and checked." || goal.Turns != 3 || goal.Ended == nil {
		t.Fatalf("goal = %+v", goal)
	}
	texts := sent()
	if len(texts) != 3 || !strings.Contains(texts[0], "<objective>\nMake the parser total\n</objective>") || !strings.Contains(texts[1], "Continue the goal") {
		t.Fatalf("sent %q", texts)
	}
	flagsMu.Lock()
	gotFlags := slices.Clone(flags)
	flagsMu.Unlock()
	if !slices.Equal(gotFlags, []bool{false, true, true}) {
		t.Fatalf("goal turns reached the runner with flags %v", gotFlags)
	}
	events, err := manager.items[info.ID].log.Since(0)
	if err != nil {
		t.Fatal(err)
	}
	var shown []protocol.UserData
	for _, event := range events {
		if event.Kind == protocol.EvUser {
			var user protocol.UserData
			_ = decode(event.Data, &user)
			shown = append(shown, user)
		}
	}
	if len(shown) != 3 || shown[0].Text != "Make the parser total" || shown[0].Goal || !shown[1].Goal {
		t.Fatalf("transcript shows %+v", shown)
	}
}

// Rashin ends a goal the agent cannot finish: at its turn budget, after two
// turns that did nothing, or when the user stops a turn.
func TestAGoalStopsAtItsBudgetAndPausesWhenItStalls(t *testing.T) {
	manager, info, _ := goalManager(t, func(turn int, _ string, sink core.Sink, _ *sessionManager, _ string) (core.TurnResult, error) {
		stepTurn(turn, sink)
		return core.TurnResult{}, nil
	})
	if _, err := manager.goal(context.Background(), protocol.GoalParams{ID: info.ID, Op: protocol.GoalSet, Objective: "Polish everything", MaxTurns: 2}); err != nil {
		t.Fatal(err)
	}
	stopped := waitGoal(t, manager, info.ID, func(goal *protocol.Goal) bool { return goal.Status != protocol.GoalActive })
	if stopped.Goal.Status != protocol.GoalStopped || stopped.Goal.Turns != 2 {
		t.Fatalf("budgeted goal = %+v", stopped.Goal)
	}

	idle, idleInfo, idleSent := goalManager(t, func(int, string, core.Sink, *sessionManager, string) (core.TurnResult, error) {
		return core.TurnResult{}, nil
	})
	if _, err := idle.goal(context.Background(), protocol.GoalParams{ID: idleInfo.ID, Op: protocol.GoalSet, Objective: "Think hard"}); err != nil {
		t.Fatal(err)
	}
	paused := waitGoal(t, idle, idleInfo.ID, func(goal *protocol.Goal) bool { return goal.Status != protocol.GoalActive })
	if paused.Goal.Status != protocol.GoalPaused || len(idleSent()) != 2 {
		t.Fatalf("a goal that made no progress = %+v after %d turns", paused.Goal, len(idleSent()))
	}

	cancelled, cancelledInfo, _ := goalManager(t, func(int, string, core.Sink, *sessionManager, string) (core.TurnResult, error) {
		return core.TurnResult{Stop: protocol.StopCancelled}, nil
	})
	if _, err := cancelled.goal(context.Background(), protocol.GoalParams{ID: cancelledInfo.ID, Op: protocol.GoalSet, Objective: "Ship it"}); err != nil {
		t.Fatal(err)
	}
	stoppedByUser := waitGoal(t, cancelled, cancelledInfo.ID, func(goal *protocol.Goal) bool { return goal.Status != protocol.GoalActive })
	if stoppedByUser.Goal.Status != protocol.GoalPaused || stoppedByUser.Goal.Reason != "You stopped the turn." {
		t.Fatalf("a stopped turn left the goal %+v", stoppedByUser.Goal)
	}
	if _, err := cancelled.goal(context.Background(), protocol.GoalParams{ID: cancelledInfo.ID, Op: protocol.GoalComplete, Summary: "x"}); err != nil {
		t.Fatalf("a paused goal cannot be reported done: %v", err)
	}
	if _, err := cancelled.goal(context.Background(), protocol.GoalParams{ID: cancelledInfo.ID, Op: protocol.GoalPause}); err == nil {
		t.Fatal("a finished goal was paused")
	}
}

// A token ceiling gives the agent one wrap-up turn and then stops without
// claiming that the user's objective was completed.
func TestGoalTokenBudgetWrapsUpThenStops(t *testing.T) {
	manager, info, sent := goalManager(t, func(turn int, _ string, sink core.Sink, _ *sessionManager, _ string) (core.TurnResult, error) {
		stepTurn(turn, sink)
		return core.TurnResult{Usage: &protocol.TokenUsage{Input: 60, Output: 40}}, nil
	})
	if _, err := manager.goal(context.Background(), protocol.GoalParams{ID: info.ID, Op: protocol.GoalSet, Objective: "Complete the parser", MaxTokens: 100}); err != nil {
		t.Fatal(err)
	}
	settled := waitGoal(t, manager, info.ID, func(goal *protocol.Goal) bool { return goal.Status != protocol.GoalActive })
	if goal := settled.Goal; goal.Status != protocol.GoalStopped || goal.Turns != 2 || goal.Tokens != 200 || goal.MaxTokens != 100 {
		t.Fatalf("token budget ended at %+v", goal)
	}
	texts := sent()
	if len(texts) != 2 || !strings.Contains(texts[1], "budget is spent") {
		t.Fatalf("wrap-up turn = %q", texts)
	}
}

// A control word is only a control word on its own: an objective that starts
// with one is still an objective.
func TestGoalCommandsTellControlWordsFromObjectives(t *testing.T) {
	manager, info, sent := goalManager(t, func(turn int, _ string, sink core.Sink, manager *sessionManager, id string) (core.TurnResult, error) {
		stepTurn(turn, sink)
		_, err := manager.goal(context.Background(), protocol.GoalParams{ID: id, Op: protocol.GoalComplete, Summary: "done"})
		return core.TurnResult{}, err
	})
	if handled, err := manager.goalCommand(context.Background(), info.ID, "/goal pause"); !handled || err == nil {
		t.Fatalf("pausing with no goal: handled %v err %v", handled, err)
	}
	if handled, err := manager.goalCommand(context.Background(), info.ID, "/goal Pause the build and fix its cache"); !handled || err != nil {
		t.Fatalf("an objective starting with pause: handled %v err %v", handled, err)
	}
	settled := waitGoal(t, manager, info.ID, func(goal *protocol.Goal) bool { return goal.Status == protocol.GoalDone })
	if settled.Goal.Objective != "Pause the build and fix its cache" || len(sent()) != 1 {
		t.Fatalf("goal %+v after %d turns", settled.Goal, len(sent()))
	}
	if handled, _ := manager.goalCommand(context.Background(), info.ID, "/goalkeeper"); handled {
		t.Fatal("a command that only starts with /goal was taken for one")
	}
}
