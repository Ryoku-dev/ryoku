package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/gateway/textcalls"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func TestOpenAIReplaysReasoningAndToolResultPictures(t *testing.T) {
	p := &provider{ID: "deepseek", Tier: protocol.TierFree}
	m := &model{ID: "reasoner", Vision: true, Tools: true, Reasoning: true, ReplayReasoning: true}
	messages := []llm.Message{
		{Role: llm.RoleAssistant, Content: "", Thinking: []llm.Thinking{{Text: "compute"}}, ToolCalls: []llm.ToolCall{{ID: "call1", Name: "read", Arguments: `{"path":"photo"}`}}},
		{Role: llm.RoleTool, Name: "read", ToolCallID: "call1", Content: "a screenshot", Images: []llm.Image{{Mime: "image/png", Data: []byte("png")}}},
	}
	wire, err := renderOpenAI(llm.Request{Model: "deepseek/reasoner", Messages: messages, Reasoning: llm.EffortHigh, Stop: []string{"STOP"}, Tools: []llm.Tool{{Name: "read", Parameters: json.RawMessage(`{"type":"object"}`)}}}, p, m)
	if err != nil || wire.ReasoningEffort != "high" || len(wire.Messages) != 3 || wire.Messages[0].Reasoning != "compute" || wire.Messages[1].ToolCallID != "call1" || wire.Messages[2].Role != "user" || len(wire.Stop) != 1 {
		t.Fatalf("render = %#v, %v", wire, err)
	}
	encoded, err := json.Marshal(wire)
	if err != nil || !strings.Contains(string(encoded), "data:image/png;base64,") || !strings.Contains(string(encoded), `"tool_choice":"auto"`) {
		t.Fatalf("wire = %s, %v", encoded, err)
	}
	m.Vision = false
	m.ReplayReasoning = false
	wire, err = renderOpenAI(llm.Request{Messages: messages}, p, m)
	if err != nil || len(wire.Messages) != 2 || wire.Messages[0].Reasoning != "" || !strings.Contains(wire.Messages[1].Content.(string), imageOmitted) {
		t.Fatalf("blind render = %#v, %v", wire, err)
	}
}

func TestTextDialectSendsSchemasAsInstructions(t *testing.T) {
	tool := llm.Tool{Name: "read", Description: "Read a file", Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)}
	request := llm.Request{Messages: []llm.Message{{Role: llm.RoleSystem, Content: "You assist."}, {Role: llm.RoleUser, Content: "read"}}, Tools: []llm.Tool{tool}}
	wire, err := renderOpenAI(request, &provider{Tier: protocol.TierLocal}, &model{ID: "lfm", ToolDialect: textcalls.XMLFn})
	if err != nil || len(wire.Tools) != 0 || len(wire.Stop) == 0 || len(wire.Messages) != 2 || !strings.Contains(wire.Messages[0].Content.(string), "<function=NAME>") {
		t.Fatalf("text dialect request = %#v, %v", wire, err)
	}
}

func TestOpenAICacheAccounting(t *testing.T) {
	var openai wireUsage
	if err := json.Unmarshal([]byte(`{"prompt_tokens":240,"completion_tokens":16,"prompt_tokens_details":{"cached_tokens":150}}`), &openai); err != nil {
		t.Fatal(err)
	}
	usage := openai.usage()
	if usage.Input != 240 || usage.Output != 16 || usage.CacheRead != 150 {
		t.Fatalf("OpenAI cache usage = %#v", usage)
	}
	var deepseek wireUsage
	if err := json.Unmarshal([]byte(`{"prompt_tokens":240,"completion_tokens":16,"prompt_cache_hit_tokens":150}`), &deepseek); err != nil {
		t.Fatal(err)
	}
	if got := deepseek.usage(); got.CacheRead != 150 || got.Input != 240 {
		t.Fatalf("DeepSeek cache usage = %#v", got)
	}
}
