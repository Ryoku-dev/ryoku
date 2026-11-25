package engine

import (
	"strings"
	"testing"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/engine/tools/agents"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func TestEngineSubagentYieldReturnsToParent(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{
		scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "task-1", Name: tool.Task, Arguments: `{"tasks":[{"name":"Helper","agent":"worker","task":"Find the answer."}]}`}),
		scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "yield-1", Name: tool.Yield, Arguments: `{"data":"The answer is 42."}`}),
		scriptedTurn(llm.FinishStop, "It is 42."),
	}}
	r := engineFixture(t, g, agents.Tools()...)
	if r.Slots("rashin-models/test") != 1 || r.Slots("openrouter/test") != 4 {
		t.Fatal("incorrect local/remote child concurrency")
	}
	sink := &engineSink{}
	result := invokeTurn(t, r, sink)
	if result.Stop != protocol.StopEnd || len(g.requests) != 3 {
		t.Fatalf("stop=%s requests=%d", result.Stop, len(g.requests))
	}
	found := false
	for _, msg := range g.requests[2].Messages {
		if msg.Role == llm.RoleTool && msg.Name == tool.Task && strings.Contains(msg.Content, "The answer is 42.") {
			found = true
		}
	}
	if !found {
		t.Fatal("yield result did not reach parent")
	}
	stepped := false
	for _, event := range sink.events {
		if event.kind != protocol.EvTool {
			continue
		}
		data := event.data.(protocol.ToolData)
		if data.Parent == "task-1" && data.Call == "yield-1" && data.Status == protocol.ToolDone {
			stepped = true
		}
		if data.Call == "task-1" && len(data.Agents) > 0 && data.Agents[0].Status == protocol.ToolDone {
			work := data.Agents[0]
			if work.StepCount < 1 || len(work.Steps) == 0 || work.Steps[0].Title == "" {
				t.Fatalf("child work has no step title: %+v", work)
			}
		}
	}
	if !stepped {
		t.Fatal("child tool event did not forward under parent task")
	}
}
