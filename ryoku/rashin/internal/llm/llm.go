// Package llm holds the chat-completion types shared by the gateway, which
// serves them, and the Ask runner, which consumes them.
package llm

import (
	"context"
	"encoding/json"
)

// Role is a chat message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Image is an image sent with a user message.
type Image struct {
	Mime string
	Data []byte
}

// ToolCall is one function call the model asked for. Arguments is the raw
// JSON object the model produced.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// Message is one chat message.
type Message struct {
	Role    Role
	Content string
	// Images ride on user messages only.
	Images []Image
	// ToolCalls ride on assistant messages.
	ToolCalls []ToolCall
	// ToolCallID and Name identify the call a tool message answers.
	ToolCallID string
	Name       string
}

// Tool is a function the model may call. Parameters is a JSON Schema object.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// Request is one chat completion. Model is a route (auto, auto:fast,
// auto:smart) or a pinned provider/model. A pinned model fails over to auto
// unless Strict is set, which a session pin chosen by the user does: a model
// the user picked for the conversation must not silently change.
type Request struct {
	Model       string
	Strict      bool
	Messages    []Message
	Tools       []Tool
	Temperature *float64
	MaxTokens   int
	// Extra are further OpenAI chat parameters a caller of the local API set
	// (top_p, stop, tool_choice and the like), passed upstream as they came.
	Extra map[string]json.RawMessage
}

// Delta is one streamed increment. Tool calls arrive complete, once each, when
// the model has finished them.
type Delta struct {
	Text      string
	Reasoning string
	ToolCalls []ToolCall
}

// Usage counts tokens; Estimated marks counts the provider did not report.
type Usage struct {
	Input     int
	Output    int
	Estimated bool
}

// Finish reasons.
const (
	FinishStop      = "stop"
	FinishToolCalls = "tool_calls"
	FinishLength    = "length"
	FinishFilter    = "content_filter"
)

// Final describes a finished stream.
type Final struct {
	// Model is the provider/model that answered.
	Model    string
	Provider string
	Finish   string
	Usage    Usage
	// Attempts counts the upstream calls made, failovers included.
	Attempts int
}

// Stream yields deltas. Next returns io.EOF after the last delta; Final is
// valid from then on.
type Stream interface {
	Next() (Delta, error)
	Final() Final
	Close() error
}

// Client starts chat completions.
type Client interface {
	Stream(ctx context.Context, req Request) (Stream, error)
}
