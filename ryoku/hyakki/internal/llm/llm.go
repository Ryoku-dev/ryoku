// Package llm holds the chat-completion types shared by the gateway, which
// serves them, and the engine and Rashin's other model callers, which consume
// them. Content stays the text body of every message, so callers that build
// plain system and user messages ignore the fields an engine turn adds.
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

// Image is an image sent with a user or tool message.
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

// Thinking is one block of an assistant's reasoning, replayed verbatim on the
// next request where the provider needs it. Signature is the provider's seal
// on it (an Anthropic signature, an OpenAI encrypted reasoning id); Redacted
// is reasoning the provider returned only sealed.
type Thinking struct {
	Text      string
	Signature string
	Redacted  []byte
}

// Message is one chat message.
type Message struct {
	Role    Role
	Content string
	// Images ride on user messages, and on tool messages for a tool that
	// returned pictures (a screenshot, an image read). The gateway sends a
	// tool message's images the way the routed model can take them, or says
	// they were left out.
	Images []Image
	// ToolCalls and Thinking ride on assistant messages.
	ToolCalls []ToolCall
	Thinking  []Thinking
	// ToolCallID and Name identify the call a tool message answers; IsError
	// marks a call that failed.
	ToolCallID string
	Name       string
	IsError    bool
	// Cache marks a prompt-cache breakpoint after this message, for shapes
	// with explicit caching; others ignore it.
	Cache bool
}

// Tool is a function the model may call. Parameters is a JSON Schema object.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// Reasoning effort: Request.Reasoning. Empty leaves it to the provider.
const (
	EffortOff    = "off"
	EffortLow    = "low"
	EffortMedium = "medium"
	EffortHigh   = "high"
)

// Tool choices beside a tool's name: Request.ToolChoice. Empty is auto when
// tools are sent.
const (
	ToolChoiceAuto     = "auto"
	ToolChoiceNone     = "none"
	ToolChoiceRequired = "required"
)

// Request is one chat completion. Model is a route (auto, auto:fast,
// auto:smart) or a pinned provider/model. A pinned model fails over to auto
// unless Strict is set, which a session pin chosen by the user does: a model
// the user picked for the conversation must not silently change.
type Request struct {
	Model     string
	Strict    bool
	LocalOnly bool
	// Light is internal work (titles, memory learning, writing, Theia,
	// compaction summaries, judges, classifiers): with no pin it goes to the
	// smallest local model that fits, never to a paid one. The local API
	// never sets it.
	Light       bool
	Messages    []Message
	Tools       []Tool
	Temperature *float64
	MaxTokens   int
	// Reasoning is how hard a model with reasoning thinks: one of the
	// Effort values, or empty for the provider's default.
	Reasoning string
	// ToolChoice is one of the ToolChoice values or a tool's name, which
	// forces that tool.
	ToolChoice string
	Stop       []string
	// Extra are further OpenAI chat parameters a caller of the local API set
	// (top_p, stop, tool_choice and the like), passed upstream as they came.
	Extra map[string]json.RawMessage
}

// Delta is one streamed increment. Tool calls arrive complete, once each, when
// the model has finished them; ToolStart names a call as soon as its id and
// name are known, so the pending tool line shows early. Signature closes the
// current reasoning block.
type Delta struct {
	Text      string
	Reasoning string
	Signature string
	ToolStart *ToolCall
	ToolCalls []ToolCall
}

// Usage counts tokens; Estimated marks counts the provider did not report.
// CacheRead and CacheWrite are the input tokens served from and written to
// the provider's prompt cache, part of Input.
type Usage struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
	Estimated  bool
	// Unmetered preserves unknown accounting from an external child runtime.
	Unmetered bool
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
	// Context is the context window in tokens of the model that answered, 0
	// when unknown.
	Context int
}

// Stream yields deltas. Next returns io.EOF after the last delta; Final is
// valid from then on.
type Stream interface {
	Next() (Delta, error)
	Final() Final
	Close() error
}

// NoRoute is the error for a request that nothing in use can take. Its text
// already says why and what to do, so a caller shows it as it is.
type NoRoute struct {
	Reason   string
	NoVision bool
}

func (e *NoRoute) Error() string { return e.Reason }

// Plan is the model a request would go to first, named before anything is
// sent so a caller can tell the user where it goes. Sending the request again
// with Model set to Route(), Strict set and LocalOnly set to Local goes to
// that model or fails; it never goes elsewhere. Local is a model on this
// computer, Vision one that reads pictures, Paid one that costs the user money
// or credits.
type Plan struct {
	Provider string
	Model    string
	Local    bool
	Vision   bool
	Paid     bool
}

// Route is the pinned provider/model route of the plan.
func (p Plan) Route() string { return p.Provider + "/" + p.Model }

// Client starts chat completions.
type Client interface {
	Stream(ctx context.Context, req Request) (Stream, error)
}
