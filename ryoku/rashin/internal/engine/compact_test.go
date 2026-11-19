package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// summariser answers every request with a fixed summary and records them.
type summariser struct {
	window   int
	requests []llm.Request
}

func (s *summariser) Window(context.Context, llm.Request) int { return s.window }

func (s *summariser) Stream(_ context.Context, req llm.Request) (llm.Stream, error) {
	s.requests = append(s.requests, req)
	return &oneDelta{text: "Here is the summary.\n## Goal\n- keep going"}, nil
}

type oneDelta struct {
	text string
	sent bool
}

func (o *oneDelta) Next() (llm.Delta, error) {
	if o.sent {
		return llm.Delta{}, io.EOF
	}
	o.sent = true
	return llm.Delta{Text: o.text}, nil
}
func (o *oneDelta) Final() llm.Final { return llm.Final{Finish: llm.FinishStop} }
func (o *oneDelta) Close() error     { return nil }

type events struct{ tools []protocol.ToolData }

func (e *events) Emit(kind string, data any) {
	if d, ok := data.(protocol.ToolData); ok && kind == protocol.EvTool {
		e.tools = append(e.tools, d)
	}
}
func (e *events) Update(func(*protocol.SessionInfo)) {}

func requestFor(tr *Transcript) llm.Request {
	return llm.Request{Model: protocol.RouteAuto, Messages: append([]llm.Message{{Role: llm.RoleSystem, Content: "system"}}, tr.Messages()...)}
}

func TestCompactPrunesOldToolOutputBeforeSummarising(t *testing.T) {
	tr := NewMemoryTranscript()
	tr.BeginTurn(1)
	tr.Append(llm.Message{Role: llm.RoleUser, Content: "read the package"})
	for i := range 6 {
		id := fmt.Sprintf("c%d", i)
		tr.Append(
			llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: id, Name: "read", Arguments: fmt.Sprintf(`{"path":"f%d.go"}`, i)}}},
			llm.Message{Role: llm.RoleTool, ToolCallID: id, Name: "read", Content: strings.Repeat("x", 8000)},
		)
	}
	tr.Append(
		llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "now", Name: "read", Arguments: `{"path":"last.go"}`}}},
		llm.Message{Role: llm.RoleTool, ToolCallID: "now", Name: "read", Content: "package last"},
	)
	model := &summariser{window: 16384}
	sink := &events{}
	changed, err := tr.Compact(context.Background(), model, requestFor(tr), 16384, sink)
	if err != nil || !changed {
		t.Fatalf("Compact = %v, %v; want changed", changed, err)
	}
	if len(model.requests) != 0 {
		t.Fatalf("pruning was enough, yet %d summary calls were made", len(model.requests))
	}
	got := tr.Messages()
	if !strings.HasPrefix(got[2].Content, "[output of read f0.go pruned") {
		t.Errorf("oldest output not pruned to a stub: %.60q", got[2].Content)
	}
	if got[len(got)-1].Content != "package last" || len(got[len(got)-3].Content) != 8000 {
		t.Error("the current round and the newest outputs must stay whole")
	}
	if len(sink.tools) != 2 || !strings.HasPrefix(sink.tools[0].Call, CompactCallPrefix) || sink.tools[1].Status != protocol.ToolDone {
		t.Errorf("compaction line: %+v", sink.tools)
	}

	model.requests = nil
	if changed, _ := tr.Compact(context.Background(), model, requestFor(tr), 16384, nil); changed {
		t.Error("a conversation under the trigger must be left alone")
	}
}

func TestCompactSummarisesAndKeepsUserTurnAndRules(t *testing.T) {
	tr := NewMemoryTranscript()
	rule := `<system-interrupt reason="rule_violation" rule="no-sudo">Do not run sudo.</system-interrupt>`
	long := strings.Repeat("words ", 700)
	for turn := 1; turn <= 9; turn++ {
		tr.BeginTurn(turn)
		tr.Append(llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("request %d %s", turn, long)})
		if turn == 2 {
			tr.Append(llm.Message{Role: llm.RoleUser, Content: rule})
		}
		tr.Append(llm.Message{Role: llm.RoleAssistant, Content: fmt.Sprintf("reply %d %s", turn, long)})
	}
	tr.BeginTurn(10)
	tr.Append(
		llm.Message{Role: llm.RoleUser, Content: "the last request"},
		llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "x", Name: "bash", Arguments: `{"command":"ls"}`}}},
		llm.Message{Role: llm.RoleTool, ToolCallID: "x", Name: "bash", Content: "a.go"},
	)

	model := &summariser{window: 131072}
	changed, err := tr.Compact(context.Background(), model, requestFor(tr), 16384, nil)
	if err != nil || !changed {
		t.Fatalf("Compact = %v, %v; want changed", changed, err)
	}
	if len(model.requests) != 1 {
		t.Fatalf("want one summary call, got %d", len(model.requests))
	}
	if r := model.requests[0]; !r.Light || r.Model != protocol.RouteAuto {
		t.Errorf("summary must be light work: %+v", r)
	}
	got := tr.Messages()
	head := got[0]
	if head.Role != llm.RoleUser || !strings.Contains(head.Content, "<summary>\n## Goal\n- keep going\n</summary>") {
		t.Fatalf("summary head: %.200q", head.Content)
	}
	if !strings.Contains(head.Content, rule) {
		t.Error("the rule injection must survive compaction verbatim")
	}
	if strings.Contains(head.Content, "request 1 ") {
		t.Error("the oldest turn must be summarised away")
	}
	all := ""
	for _, m := range got {
		all += m.Content + "\n"
	}
	if !strings.Contains(all, "the last request") || got[len(got)-1].Content != "a.go" {
		t.Error("the last user turn and the current round must stay verbatim")
	}

	// A later summary keeps the injection while using the session model
	// when its pin is strict.
	for turn := 11; turn <= 15; turn++ {
		tr.BeginTurn(turn)
		tr.Append(
			llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("follow-up %d %s", turn, long)},
			llm.Message{Role: llm.RoleAssistant, Content: fmt.Sprintf("reply %d %s", turn, long)},
		)
	}
	strict := requestFor(tr)
	strict.Model, strict.Strict = "openrouter/some-model", true
	model.requests = nil
	if changed, err := tr.ForceCompact(context.Background(), model, strict, 16384, nil); !changed || err != nil || len(model.requests) == 0 {
		t.Fatalf("later compaction failed: changed=%v calls=%d err=%v", changed, len(model.requests), err)
	}
	if !strings.Contains(tr.Messages()[0].Content, rule) {
		t.Error("the rule injection disappeared after repeated compaction")
	}
	for _, r := range model.requests {
		if r.Light || r.Model != "openrouter/some-model" || !r.Strict {
			t.Errorf("strict session summarised on %+v", r)
		}
	}
}

func TestSummaryUsesLightModelsWindowForEveryFold(t *testing.T) {
	tr := NewMemoryTranscript()
	for n := 1; n <= 22; n++ {
		tr.BeginTurn(n)
		tr.Append(
			llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("question %d: %s", n, strings.Repeat("word ", 2000))},
			llm.Message{Role: llm.RoleAssistant, Content: fmt.Sprintf("answer %d: %s", n, strings.Repeat("word ", 2000))},
		)
	}
	model := &summariser{window: 16384}
	changed, err := tr.ForceCompact(context.Background(), model, requestFor(tr), 131072, nil)
	if err != nil || !changed || len(model.requests) < 2 {
		t.Fatalf("large conversation must fold across light windows: changed=%v calls=%d err=%v", changed, len(model.requests), err)
	}
	for n, req := range model.requests {
		if got := llm.EstimateRequest(req) + req.MaxTokens + OutputHeadwayTokens; got > model.window {
			t.Errorf("summary call %d needs %d tokens in %d-token window", n, got, model.window)
		}
	}
}

type failingSummariser struct{ calls int }

func (s *failingSummariser) Window(context.Context, llm.Request) int { return 8192 }
func (s *failingSummariser) Stream(context.Context, llm.Request) (llm.Stream, error) {
	s.calls++
	return nil, errors.New("summary model unavailable")
}

func TestFailedSummaryFallsBackToOlderRoundCut(t *testing.T) {
	tr := NewMemoryTranscript()
	for turn := 1; turn <= 10; turn++ {
		tr.BeginTurn(turn)
		tr.Append(llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf("request %d", turn)}, llm.Message{Role: llm.RoleAssistant, Content: strings.Repeat("word ", 1800)})
	}
	tr.BeginTurn(11)
	tr.Append(llm.Message{Role: llm.RoleUser, Content: "Keep this newest request."})
	model := &failingSummariser{}
	sink := &engineSink{}
	changed, err := tr.ForceCompact(context.Background(), model, requestFor(tr), 16384, sink)
	if err != nil || !changed || model.calls == 0 {
		t.Fatalf("summary failure blocked fallback: changed=%v calls=%d err=%v", changed, model.calls, err)
	}
	msgs := tr.Messages()
	if !strings.Contains(msgs[0].Content, "earlier messages left out") || !strings.Contains(msgs[len(msgs)-1].Content, "Keep this newest request.") || !sink.has(protocol.EvNotice) {
		t.Fatalf("fallback lost notice or newest request: %+v", msgs)
	}
	if got := PromptTokens(requestFor(tr), tr.Anchor()); got+MinOutputTokens+OutputHeadwayTokens > 16384 {
		t.Fatalf("fallback failed to free budget: %d", got)
	}
}
