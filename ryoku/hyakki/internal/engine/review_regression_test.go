package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/engine/tools/agents"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

type parallelScriptedTool struct{ scriptedTool }

func (parallelScriptedTool) Parallel() bool { return true }

func TestProviderAnchorIncludesRealToolResults(t *testing.T) {
	tr := NewMemoryTranscript()
	tr.Append(llm.Message{Role: llm.RoleUser, Content: "read both"}, llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "one", Name: tool.Read}, {ID: "two", Name: tool.Read}}})
	tr.SetUsage(llm.Usage{Input: 1000, Output: 50})
	for _, id := range []string{"one", "two"} {
		tr.Append(llm.Message{Role: llm.RoleTool, ToolCallID: id, Name: tool.Read, Content: strings.Repeat("x", 24<<10)})
	}
	req := requestFor(tr)
	if got := PromptTokens(req, tr.Anchor()); got < llm.EstimateRequest(req) || tr.Anchor().Extra < 10000 {
		t.Fatalf("tool results escaped budget: anchor=%+v prompt=%d estimate=%d", tr.Anchor(), got, llm.EstimateRequest(req))
	}
	if p := PromptTokens(req, tr.Anchor()); !NewBudget(16384).Over(p) {
		t.Fatal("tool results should trigger compaction before another model request")
	}
}

func TestPersistedPlanWinsBothDirections(t *testing.T) {
	for _, tc := range []struct {
		stored, configured tool.Mode
		wantPlan           string
	}{
		{tool.ModePlan, tool.ModeAgent, protocol.OptionOn},
		{tool.ModeAgent, tool.ModePlan, protocol.OptionOff},
	} {
		f := New(Deps{})
		state, _ := json.Marshal(map[string]any{"version": TranscriptVersion, "mode": tc.stored, "options": map[string]string{protocol.OptionPlan: tc.wantPlan}})
		defer f.Close()
		r := f.newRunner(Options{Session: protocol.SessionInfo{ID: t.Name() + string(tc.stored), Temporary: true}, Mode: tc.configured, Options: map[string]string{protocol.OptionPlan: string(map[tool.Mode]string{tool.ModePlan: protocol.OptionOn, tool.ModeAgent: protocol.OptionOff}[tc.configured])}, Resume: state})
		if r.mode != tc.stored || r.options[protocol.OptionPlan] != tc.wantPlan {
			t.Errorf("stored=%s configured=%s restored=%s plan=%s", tc.stored, tc.configured, r.mode, r.options[protocol.OptionPlan])
		}
		r.Stop()
	}
}

func TestSteeringBeforeYieldSurvivesToNextTurn(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "end", Name: tool.Todo, Arguments: "{}"}), scriptedTurn(llm.FinishStop, "Next turn.")}}
	started, release := make(chan struct{}), make(chan struct{})
	r := engineFixture(t, g, scriptedTool{name: tool.Todo, readonly: true, run: func(context.Context, tool.Env) tool.Result {
		close(started)
		<-release
		return tool.Result{Text: "done", End: true}
	}})
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "first"}, &engineSink{})
		done <- err
	}()
	<-started
	if !r.Steer(core.PromptInput{Text: "do not lose this"}) {
		t.Fatal("steer rejected")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "next"}, &engineSink{}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range g.requests[1].Messages {
		found = found || strings.Contains(m.Content, "do not lose this")
	}
	if !found {
		t.Fatal("accepted steering disappeared on early turn end")
	}
}

func TestReadOnlyCallsAreNotAutomaticallyParallel(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "a", Name: tool.Read, Arguments: "{}"}, llm.ToolCall{ID: "b", Name: tool.Grep, Arguments: "{}"}), scriptedTurn(llm.FinishStop, "Done.")}}
	var active atomic.Int32
	var overlapped atomic.Bool
	run := func(context.Context, tool.Env) tool.Result {
		if active.Add(1) > 1 {
			overlapped.Store(true)
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
		return tool.Result{Text: "read"}
	}
	r := engineFixture(t, g, scriptedTool{name: tool.Read, readonly: true, run: run}, scriptedTool{name: tool.Grep, readonly: true, run: run})
	invokeTurn(t, r, &engineSink{})
	if overlapped.Load() {
		t.Fatal("read-only tools without Parallel opt-in overlapped")
	}
}

func TestScreenTaskAndNotesNeverParallelEvenIfMarked(t *testing.T) {
	for _, name := range []string{tool.ScreenClick, tool.Task, tool.ContextNotes, tool.NewContext, tool.SystemQuery, tool.Yield, tool.Propose} {
		marked := parallelScriptedTool{scriptedTool{name: name, readonly: true}}
		if canParallel(name, marked) {
			t.Errorf("%s was marked parallel despite changing ordering or control flow", name)
		}
	}
	if !canParallel(tool.Read, parallelScriptedTool{scriptedTool{name: tool.Read, readonly: true}}) {
		t.Fatal("independent read lost parallel opt-in")
	}
}

type reviewHTTPError struct {
	code     int
	overflow bool
	message  string
}

func (e reviewHTTPError) Error() string        { return e.message }
func (e reviewHTTPError) StatusCode() int      { return e.code }
func (e reviewHTTPError) ContextTooLong() bool { return e.overflow }

func TestContextOverflowIsNotTransient(t *testing.T) {
	err := reviewHTTPError{400, true, "requested 8500 tokens of 8192, payload contains 500"}
	if retryable(context.Background(), err) || !contextTooLong(err) {
		t.Fatal("context rejection should compact, not retry unmodified")
	}
	if retryable(context.Background(), errors.New("400 bad request: requested 8500 tokens")) {
		t.Fatal("numbers inside an error body are not HTTP statuses")
	}
	if !retryable(context.Background(), reviewHTTPError{503, false, "temporary failure"}) {
		t.Fatal("typed 503 should retry")
	}
}

func TestStopReleasesPersistedTranscript(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	f := New(Deps{})
	defer f.Close()
	info := protocol.SessionInfo{ID: t.Name(), Kind: protocol.KindAgent}
	r := f.newRunner(Options{Session: info, Mode: tool.ModeAgent})
	if lookupTranscript(info.ID) != r.transcript {
		t.Fatal("open persisted transcript not available")
	}
	if err := r.Stop(); err != nil || lookupTranscript(info.ID) != nil {
		t.Fatalf("Stop retained persisted transcript: %v", err)
	}
}

func TestOldRunnerStopDoesNotReleaseReopenedTranscript(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	f := New(Deps{})
	defer f.Close()
	info := protocol.SessionInfo{ID: t.Name(), Kind: protocol.KindAgent}
	old := f.newRunner(Options{Session: info, Mode: tool.ModeAgent})
	current := f.newRunner(Options{Session: info, Mode: tool.ModeAgent})
	if err := old.Stop(); err != nil {
		t.Fatal(err)
	}
	if lookupTranscript(info.ID) != current.transcript {
		t.Fatal("old runner removed the current session's transcript")
	}
	if err := current.Stop(); err != nil || lookupTranscript(info.ID) != nil {
		t.Fatalf("current runner did not release its transcript: %v", err)
	}
}

func TestTornLastLineDoesNotCorruptNextAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engine.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"torn"`), 0o600); err != nil {
		t.Fatal(err)
	}
	tr := newTranscript("torn", path)
	if err := tr.Append(llm.Message{Role: llm.RoleUser, Content: "survived"}); err != nil {
		t.Fatal(err)
	}
	lines, _ := readLines(path)
	if len(lines) != 1 || lines[0].msg.Content != "survived" {
		t.Fatalf("valid append glued onto torn line: %+v", lines)
	}
}

func TestSubagentsHaveDistinctSessionAndSpendDuringWork(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{
		scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "read-one", Name: tool.Read, Arguments: "{}"}),
		scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "yield-one", Name: tool.Yield, Arguments: `{"data":"done one"}`}),
		scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "read-two", Name: tool.Read, Arguments: "{}"}),
		scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "yield-two", Name: tool.Yield, Arguments: `{"data":"done two"}`}),
	}}
	var ids []string
	r := engineFixture(t, g, append(agents.Tools(), scriptedTool{name: tool.Read, readonly: true, run: func(_ context.Context, e tool.Env) tool.Result {
		ids = append(ids, e.Session.ID)
		if e.Spawner.(*runner).budget().SpentTokens == 0 {
			t.Error("child model usage was not charged to parent before tool ran")
		}
		return tool.Result{Text: "read"}
	}})...)
	r.gate.bind(&engineSink{})
	for _, parent := range []string{"same-tool-call", "same-tool-call"} {
		result, err := r.Spawn(context.Background(), tool.SpawnRequest{ID: "SameName", Name: agents.Worker, Task: "Read once and yield.", Parent: parent}, nil)
		if err != nil || result.Failed || result.Usage.Input == 0 {
			t.Fatalf("Spawn(%s): %+v, %v", parent, result, err)
		}
	}
	if len(ids) != 2 || ids[0] == ids[1] || ids[0] == "" || ids[1] == "" {
		t.Fatalf("subagents did not get distinct nonempty session ids: %q", ids)
	}
	if spent := r.budget().SpentTokens; spent != 240 {
		t.Fatalf("child model spend not charged exactly once: %d", spent)
	}
}

func TestOverflowCompactsAndRetriesOnlyOnce(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{
		{fail: reviewHTTPError{400, true, "context length exceeded"}},
		scriptedTurn(llm.FinishStop, "## Goal\n- continue"),
		scriptedTurn(llm.FinishStop, "Recovered."),
	}}
	r := engineFixture(t, g)
	for i := 1; i <= 4; i++ {
		r.transcript.BeginTurn(i)
		r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("old %d", i)}, llm.Message{Role: llm.RoleAssistant, Content: strings.Repeat("word ", 1500)})
	}
	result, err := r.Prompt(context.Background(), core.PromptInput{Turn: 5, Text: "continue"}, &engineSink{})
	if err != nil || result.Stop != protocol.StopEnd || len(g.requests) != 3 {
		t.Fatalf("overflow recovery stopped: result=%+v err=%v requests=%d", result, err, len(g.requests))
	}
	if len(g.requests[2].Messages) >= len(g.requests[0].Messages) || llm.EstimateRequest(g.requests[2]) >= llm.EstimateRequest(g.requests[0]) {
		t.Fatalf("overflow retry did not use compacted context: initial=%d retry=%d", llm.EstimateRequest(g.requests[0]), llm.EstimateRequest(g.requests[2]))
	}
}

func TestRolloverKeepsAllSteeredUserMessages(t *testing.T) {
	tr := NewMemoryTranscript()
	tr.BeginTurn(1)
	tr.Append(llm.Message{Role: llm.RoleUser, Content: "original"}, llm.Message{Role: llm.RoleAssistant, Content: "working"}, llm.Message{Role: llm.RoleUser, Content: "steered later"})
	tr.SetNotes("important")
	tr.requestRollover()
	tr.Rollover()
	var all string
	for _, msg := range tr.Messages() {
		all += msg.Content + "\n"
	}
	if !strings.Contains(all, "original") || !strings.Contains(all, "steered later") {
		t.Fatalf("rollover lost steered request: %q", all)
	}
}

func TestToolPanicBecomesFailedResult(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "panic", Name: tool.Read, Arguments: "{}"}), scriptedTurn(llm.FinishStop, "Done.")}}
	r := engineFixture(t, g, parallelScriptedTool{scriptedTool{name: tool.Read, readonly: true, run: func(context.Context, tool.Env) tool.Result { panic("broken tool") }}})
	invokeTurn(t, r, &engineSink{})
	found := false
	for _, msg := range g.requests[1].Messages {
		found = found || msg.Role == llm.RoleTool && msg.IsError && strings.Contains(msg.Content, "broken tool")
	}
	if !found {
		t.Fatal("tool panic killed the turn instead of returning an error")
	}
}

func TestProgressResetsThinkingAndRuleInterruptCaps(t *testing.T) {
	rc := recovery{thinking: nudgeMax, interrupts: interruptMax}
	rc.progressed()
	if rc.thinking != 0 || rc.interrupts != 0 {
		t.Fatalf("progress did not reset unrelated loop counters: %+v", rc)
	}
}

func TestSetModeDuringRunningToolHasNoRace(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "read", Name: tool.Read, Arguments: "{}"}), scriptedTurn(llm.FinishStop, "Done.")}}
	started, release := make(chan struct{}), make(chan struct{})
	r := engineFixture(t, g, scriptedTool{name: tool.Read, readonly: true, run: func(_ context.Context, env tool.Env) tool.Result {
		close(started)
		<-release
		return tool.Result{Text: env.Session.Approvals}
	}})
	sink := &engineSink{}
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "read"}, sink)
		done <- err
	}()
	<-started
	for _, mode := range []string{protocol.ApprovalsAsk, protocol.ApprovalsAuto, protocol.ApprovalsReadOnly} {
		if err := r.SetMode(context.Background(), mode, sink); err != nil {
			t.Fatal(err)
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := r.gate.approvalMode(); got != protocol.ApprovalsReadOnly {
		t.Fatalf("mode setter lost last update: %s", got)
	}
}

func TestReadOnlyIgnoresPreviousAllowInConversation(t *testing.T) {
	g := newGate(func() string { return protocol.ApprovalsAuto }, false)
	g.setAllowed(true)
	g.setApprovals(func() string { return protocol.ApprovalsReadOnly })
	if !g.waits(protocol.TierWrite) || g.waits(protocol.TierRead) {
		t.Fatal("read-only mode inherited write approval")
	}
}

type noticeOnce struct{ fired bool }

func (g *noticeOnce) Reset() {}
func (g *noticeOnce) Observe(llm.Delta) *Interrupt {
	if g.fired {
		return nil
	}
	g.fired = true
	return &Interrupt{Notice: true, Message: "<system-reminder>Keep this notice.</system-reminder>"}
}

type noticeSource struct{ guard *noticeOnce }

func (s noticeSource) Guard(TurnContext) StreamGuard { return s.guard }

func TestNoticeSurvivesEmptyReplyNudge(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishStop, ""), scriptedTurn(llm.FinishStop, "Done.")}}
	r := engineFixture(t, g)
	r.f.deps.Guards = []StreamGuards{noticeSource{guard: &noticeOnce{}}}
	invokeTurn(t, r, &engineSink{})
	found := false
	for _, m := range g.requests[1].Messages {
		found = found || strings.Contains(m.Content, "Keep this notice.")
	}
	if !found {
		t.Fatal("notice vanished when the reply was nudged")
	}
}

func TestThinkingLoopExhaustionEmitsEndNotice(t *testing.T) {
	g := &scriptedGateway{}
	for i := 0; i <= nudgeMax; i++ {
		g.replies = append(g.replies, scriptedTurn(llm.FinishStop, strings.Repeat("loop", 80)))
	}
	r := engineFixture(t, g)
	sink := &engineSink{}
	result := invokeTurn(t, r, sink)
	if result.Stop != protocol.StopEnd || !sink.has(protocol.EvNotice) {
		t.Fatalf("thinking-loop stop was silent: stop=%s notice=%v", result.Stop, sink.has(protocol.EvNotice))
	}
}

func TestTeamContextRefreshesAfterHandoffAndSavedSessionReload(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{
		scriptedTurn(llm.FinishStop, "First result."),
		scriptedTurn(llm.FinishStop, "Second result."),
		scriptedTurn(llm.FinishStop, "Restored result."),
	}}
	r := engineFixture(t, g)
	r.opts.Session.TeamTask = "live-team-task"
	policy := "policy-state-before-accepted-handoff"
	r.opts.Brief = func() string { return policy }
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Inspect the task"}, &engineSink{}); err != nil {
		t.Fatal(err)
	}
	previous := policy
	policy = "policy-state-after-accepted-handoff"
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "Continue the approved task"}, &engineSink{}); err != nil {
		t.Fatal(err)
	}
	if sent := g.requests[1].Messages[0].Content; !strings.Contains(sent, policy) || strings.Contains(sent, previous) {
		t.Fatal("the model received superseded team context after the handoff")
	}
	previous = policy
	policy = "policy-state-after-session-reload"
	resumed := r.f.newRunner(Options{Session: r.opts.Session, Mode: tool.ModeAgent, Resume: r.State(), Brief: func() string { return policy }})
	if _, err := resumed.Prompt(context.Background(), core.PromptInput{Turn: 3, Text: "Continue the restored task"}, &engineSink{}); err != nil {
		t.Fatal(err)
	}
	if sent := g.requests[2].Messages[0].Content; !strings.Contains(sent, policy) || strings.Contains(sent, previous) {
		t.Fatal("the restored model received persisted stale team context")
	}
}
