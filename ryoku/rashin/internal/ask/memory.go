package ask

import (
	"context"
	"encoding/json"
	"errors"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// recall is what memory gives this turn: the card and rules for the system
// prompt, the facts for the question, and which memory tools to offer. A
// turn the user ran a command in asks nothing of memory beyond the card.
func (r *runner) recall(ctx context.Context, in core.PromptInput) core.Recall {
	if r.factory.memory == nil {
		return core.Recall{}
	}
	question := in.Text
	if in.Run != "" {
		question = ""
	}
	return r.factory.memory.Recall(ctx, r.opts.Session, question)
}

// withRecalled puts the recalled facts into this turn's user message only;
// the stored turn keeps what the user typed, so the next turn's history
// never carries them.
func withRecalled(asked llm.Message, recalled string) llm.Message {
	if recalled == "" {
		return asked
	}
	if asked.Content == "" {
		asked.Content = recalled
	} else {
		asked.Content += "\n\n" + recalled
	}
	return asked
}

// memoryTools are the memory tools recall offers: search, remember and
// forget when the session uses memory, and the past-conversation search
// when that is on.
func memoryTools(recall core.Recall) []llm.Tool {
	var tools []llm.Tool
	for _, tool := range brief.MemoryTools(brief.MemoryForAsk, recall.Sessions) {
		if tool.Name != brief.ToolSessionSearch && !recall.Tools {
			continue
		}
		parameters, _ := json.Marshal(tool.Schema)
		tools = append(tools, llm.Tool{Name: tool.Name, Description: tool.Description, Parameters: parameters})
	}
	return tools
}

// handed reports a tool among tools, so a model never runs a memory tool
// this turn did not give it.
func handed(tools []llm.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// runMemoryTool runs one memory tool for session and returns what the model
// is told. What a remember, forget or past-conversation search did shows in
// the conversation as memory's own line, so only a memory search shows as a
// tool.
func (f *Factory) runMemoryTool(ctx context.Context, call llm.ToolCall, session string, sink core.Sink) string {
	searching := call.Name == brief.ToolMemorySearch
	if searching {
		sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Title: i18n.T("Searching memory"), Kind: protocol.ToolSearch, Status: protocol.ToolRunning, Input: compactInput(call.Arguments)})
	}
	output, err := f.memoryCall(ctx, call, session)
	if err != nil {
		output = err.Error()
	}
	if searching {
		status := protocol.ToolDone
		if err != nil {
			status = protocol.ToolFailed
		}
		sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Title: i18n.T("Searching memory"), Kind: protocol.ToolSearch, Status: status, Output: truncateRunes(output, 6000)})
	}
	return output
}

// memoryCall serves one memory tool through memory as session.
func (f *Factory) memoryCall(ctx context.Context, call llm.ToolCall, session string) (string, error) {
	if f.memory == nil {
		return "", errors.New(i18n.T("Memory is not available."))
	}
	method, params, err := brief.MemoryParams(call.Name, json.RawMessage(call.Arguments), session)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	result, err := f.memory.Handle(ctx, core.MemoryCaller{Session: session}, method, raw)
	if err != nil {
		return "", err
	}
	answer, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return brief.MemoryAnswer(method, answer), nil
}
