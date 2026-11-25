package engine

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

type scriptedReply struct {
	deltas []llm.Delta
	final  llm.Final
	fail   error
}
type scriptedGateway struct {
	core.Gateway
	mu       sync.Mutex
	replies  []scriptedReply
	requests []llm.Request
}

func (g *scriptedGateway) Models(context.Context, protocol.ModelsParams) (protocol.ModelsResult, error) {
	return protocol.ModelsResult{Models: []protocol.ModelInfo{{ID: "openrouter/test", Name: "Test", Tools: true, Available: true, Reasoning: true, Params: 10_000_000_000}}, Routes: []protocol.Choice{{ID: protocol.RouteAuto, Name: "Auto"}}}, nil
}
func (g *scriptedGateway) Window(context.Context, llm.Request) int { return 32768 }
func (g *scriptedGateway) Stream(_ context.Context, req llm.Request) (llm.Stream, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests = append(g.requests, req)
	if len(g.replies) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	next := g.replies[0]
	g.replies = g.replies[1:]
	return &scriptedStream{script: next}, nil
}

type scriptedStream struct {
	script scriptedReply
	at     int
}

func (s *scriptedStream) Next() (llm.Delta, error) {
	if s.at >= len(s.script.deltas) {
		if s.script.fail != nil {
			return llm.Delta{}, s.script.fail
		}
		return llm.Delta{}, io.EOF
	}
	d := s.script.deltas[s.at]
	s.at++
	return d, nil
}
func (s *scriptedStream) Final() llm.Final { return s.script.final }
func (s *scriptedStream) Close() error     { return nil }

type engineSink struct {
	mu     sync.Mutex
	events []struct {
		kind string
		data any
	}
	cards chan protocol.PermissionData
}

func (s *engineSink) Emit(kind string, data any) {
	s.mu.Lock()
	s.events = append(s.events, struct {
		kind string
		data any
	}{kind, data})
	s.mu.Unlock()
	if kind == protocol.EvPermission && s.cards != nil {
		s.cards <- data.(protocol.PermissionData)
	}
}
func (s *engineSink) Update(fn func(*protocol.SessionInfo)) { var info protocol.SessionInfo; fn(&info) }
func (s *engineSink) has(kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events {
		if e.kind == kind {
			return true
		}
	}
	return false
}

type scriptedTool struct {
	name     string
	readonly bool
	run      func(context.Context, tool.Env) tool.Result
}

func (t scriptedTool) Name() string { return t.name }
func (t scriptedTool) Spec(tool.Profile) llm.Tool {
	return llm.Tool{Name: t.name, Parameters: json.RawMessage(`{"type":"object"}`)}
}
func (t scriptedTool) Kind() tool.Kind { return protocol.ToolOther }
func (t scriptedTool) ReadOnly() bool  { return t.readonly }
func (t scriptedTool) Present(json.RawMessage) tool.Presentation {
	return tool.Presentation{Title: t.name}
}
func (t scriptedTool) Run(ctx context.Context, _ tool.Call, e tool.Env) tool.Result {
	return t.run(ctx, e)
}

func scriptedTurn(finish, text string, calls ...llm.ToolCall) scriptedReply {
	d := llm.Delta{Text: text, ToolCalls: calls}
	return scriptedReply{deltas: []llm.Delta{d}, final: llm.Final{Model: "test", Provider: "openrouter", Finish: finish, Context: 32768, Usage: llm.Usage{Input: 50, Output: 10}}}
}
func engineFixture(t *testing.T, g *scriptedGateway, tools ...tool.Tool) *runner {
	t.Helper()
	f := New(Deps{Gateway: g, Tools: tool.NewRegistry(tools...)})
	t.Cleanup(func() { f.Close() })
	got, err := f.NewAgent(context.Background(), core.AgentOptions{Session: protocol.SessionInfo{ID: t.Name(), Kind: protocol.KindAgent, Temporary: true, Model: "openrouter/test", Cwd: t.TempDir()}, Settings: func() protocol.Settings { return protocol.Settings{} }, Approvals: func() string { return protocol.ApprovalsAuto }})
	if err != nil {
		t.Fatal(err)
	}
	return got.(*runner)
}
func invokeTurn(t *testing.T, r *runner, s *engineSink) core.TurnResult {
	t.Helper()
	result, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Finish this task."}, s)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestEngineExecutesToolAndReplaysResult(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "c1", Name: tool.Read, Arguments: `{"path":"a"}`}), scriptedTurn(llm.FinishStop, "Done.")}}
	ran := 0
	r := engineFixture(t, g, scriptedTool{name: tool.Read, readonly: true, run: func(_ context.Context, _ tool.Env) tool.Result { ran++; return tool.Result{Text: "file contents"} }})
	sink := &engineSink{}
	result := invokeTurn(t, r, sink)
	if result.Stop != protocol.StopEnd || ran != 1 || len(g.requests) != 2 {
		t.Fatalf("stop=%s ran=%d requests=%d", result.Stop, ran, len(g.requests))
	}
	replayed := false
	for _, m := range g.requests[1].Messages {
		if m.Role == llm.RoleTool && m.ToolCallID == "c1" && m.Content == "file contents" {
			replayed = true
		}
	}
	if !replayed || !sink.has(protocol.EvText) || !sink.has(protocol.EvTool) {
		t.Fatal("tool result or streamed events missing")
	}
}
func TestEngineLengthNeverRunsPartialTool(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishLength, "partial", llm.ToolCall{ID: "c", Name: tool.Write, Arguments: `{"path":"a"}`}), scriptedTurn(llm.FinishLength, "again"), scriptedTurn(llm.FinishLength, "again"), scriptedTurn(llm.FinishLength, "again")}}
	ran := 0
	r := engineFixture(t, g, scriptedTool{name: tool.Write, run: func(_ context.Context, _ tool.Env) tool.Result { ran++; return tool.Result{} }})
	result := invokeTurn(t, r, &engineSink{})
	if ran != 0 || result.Stop != protocol.StopMaxTokens {
		t.Fatalf("ran %d tools, stop %s", ran, result.Stop)
	}
}
func TestEngineLoopGuardStopsIdenticalCalls(t *testing.T) {
	replies := make([]scriptedReply, 7)
	for i := range replies {
		replies[i] = scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "c", Name: tool.Read, Arguments: `{"path":"a"}`})
	}
	g := &scriptedGateway{replies: replies}
	ran := 0
	r := engineFixture(t, g, scriptedTool{name: tool.Read, readonly: true, run: func(_ context.Context, _ tool.Env) tool.Result { ran++; return tool.Result{Text: "same"} }})
	sink := &engineSink{}
	result := invokeTurn(t, r, sink)
	if result.Stop != protocol.StopEnd || ran != 4 || !sink.has(protocol.EvNotice) {
		t.Fatalf("stop=%s ran=%d notice=%v", result.Stop, ran, sink.has(protocol.EvNotice))
	}
	foundResult := false
	for _, m := range g.requests[len(g.requests)-1].Messages {
		foundResult = foundResult || strings.Contains(m.Content, "Last result: `same`")
	}
	if !foundResult {
		t.Fatal("loop redirect reported the assistant's text, not the tool result")
	}
}
func TestEnginePlanRequiresApprovalBeforeWrite(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "todo", Name: tool.Todo, Arguments: "{}"}), scriptedTurn(llm.FinishStop, "Plan ready."), scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "write", Name: tool.Write, Arguments: "{}"}), scriptedTurn(llm.FinishStop, "Done.")}}
	written := 0
	r := engineFixture(t, g, scriptedTool{name: tool.Todo, readonly: true, run: func(_ context.Context, _ tool.Env) tool.Result {
		return tool.Result{Plan: &protocol.PlanData{Entries: []protocol.PlanEntry{{Content: "Write the file", Status: "pending"}}}}
	}}, scriptedTool{name: tool.Write, run: func(_ context.Context, _ tool.Env) tool.Result { written++; return tool.Result{Text: "written"} }})
	r.mode = tool.ModePlan
	sink := &engineSink{cards: make(chan protocol.PermissionData, 1)}
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Make a plan, then write."}, sink)
		done <- err
	}()
	select {
	case card := <-sink.cards:
		if card.Kind != protocol.PermissionPlan || written != 0 {
			t.Errorf("card=%s written=%d", card.Kind, written)
		}
		if err := r.Permission(card.Request, optionBuild); err != nil {
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("plan card did not appear")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("plan did not resume")
	}
	if written != 1 || r.mode != tool.ModeAgent {
		t.Fatalf("written=%d mode=%s", written, r.mode)
	}
}
func TestEngineCancelResolvesPermission(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "write", Name: tool.Write, Arguments: "{}"})}}
	ran := 0
	r := engineFixture(t, g, scriptedTool{name: tool.Write, run: func(ctx context.Context, e tool.Env) tool.Result {
		decision, err := e.Permissions.Allow(ctx, tool.Request{Call: tool.Call{ID: "write", Name: tool.Write}, Tier: protocol.TierWrite})
		if err != nil || !decision.Allowed {
			return tool.Failed("cancelled")
		}
		ran++
		return tool.Result{Text: "written"}
	}})
	r.opts.Approvals = func() string { return protocol.ApprovalsAsk }
	r.gate.approvals = r.opts.Approvals
	sink := &engineSink{cards: make(chan protocol.PermissionData, 1)}
	done := make(chan core.TurnResult, 1)
	go func() {
		res, _ := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Write."}, sink)
		done <- res
	}()
	select {
	case <-sink.cards:
		r.Cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("permission card did not appear")
	}
	select {
	case res := <-done:
		if res.Stop != protocol.StopCancelled || ran != 0 {
			t.Fatalf("stop=%s ran=%d", res.Stop, ran)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not end permission wait")
	}
	if !sink.has(protocol.EvPermissionResolved) {
		t.Fatal("pending permission was not resolved")
	}
}

type interruptOnce struct{ fired bool }

func (g *interruptOnce) Reset() {}
func (g *interruptOnce) Observe(d llm.Delta) *Interrupt {
	if !g.fired && strings.Contains(d.Text, "wrong") {
		g.fired = true
		return &Interrupt{Message: "<system-interrupt>Correct the answer.</system-interrupt>"}
	}
	return nil
}

type guardSource struct{ g *interruptOnce }

func (s guardSource) Guard(TurnContext) StreamGuard { return s.g }
func TestEngineStreamGuardDiscardsAttempt(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishStop, "wrong"), scriptedTurn(llm.FinishStop, "Correct.")}}
	r := engineFixture(t, g)
	r.f.deps.Guards = []StreamGuards{guardSource{g: &interruptOnce{}}}
	sink := &engineSink{}
	result := invokeTurn(t, r, sink)
	if result.Stop != protocol.StopEnd || len(g.requests) != 2 {
		t.Fatalf("stop=%s calls=%d", result.Stop, len(g.requests))
	}
	for _, event := range sink.events {
		if event.kind == protocol.EvText && strings.Contains(event.data.(protocol.TextData).Delta, "wrong") {
			t.Fatal("aborted attempt leaked into transcript")
		}
	}
	found := false
	for _, m := range g.requests[1].Messages {
		if strings.Contains(m.Content, "<system-interrupt>") {
			found = true
		}
	}
	if !found {
		t.Fatal("interrupt not passed to retry")
	}
}
