package engine

import (
	"context"
	askmode "ryoku-rashin/internal/engine/ask"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// AskHooks are the pieces only Ask has: Theia's one-shot answer, the
// command a user runs in place of a question, attachments, the replay of an
// Ask from before the native engine, the habits section, and the filter and
// answer event around what the model says. internal/engine/ask implements
// them (ask.New); the daemon hands them over in Deps.Ask. A runner calls them
// only in ask-read and ask-act, except Message, which every mode uses for
// attachments when it is set.
type AskHooks interface {
	// Glance answers a Theia capture (PromptInput.Glance set, or any turn of
	// a session whose Origin is protocol.OriginSight) in one shot, in place
	// of the round loop. model is the session's own model, "" when neither
	// it nor the glance names one: then Glance sends llm.Request{Light:
	// true} (the glance's Model, when set, still wins). strict is the pin.
	Glance(ctx context.Context, in core.PromptInput, model string, strict bool, sink core.Sink) (core.TurnResult, error)
	// Message is the user message of a turn: text with its attachments
	// (images inline, text files read in, anything else named).
	Message(text string, attachments []protocol.Attachment) (llm.Message, error)
	// History replays an Ask session's earlier turns from its log, within
	// Ask's history budget. The runner uses it only while the session has no
	// engine transcript yet (an Ask from before the native engine).
	History(events []protocol.SessionEvent) []llm.Message
	// Run runs the command the user typed in place of a question
	// (PromptInput.Run) in cwd with a live terminal, and returns the user
	// message that tells the model what it printed. ok is false when the
	// turn was cancelled while it ran.
	Run(ctx context.Context, x exec.Executor, in core.PromptInput, cwd string, sink core.Sink) (told llm.Message, ok bool, err error)
	// Habits is the user's habits section of the Ask prompt (shell, editor,
	// most used commands), "" when off.
	Habits(settings protocol.Settings) string
	// Filter starts the filter for one model reply: text is what the user
	// may see now (NEEDS_AGENT lines and recalled-memory fences held back),
	// thought the same for reasoning.
	Filter(text, thought func(delta string)) askmode.Filter
	// Answer is the answer event that ends an Ask turn. raw is the final
	// reply's whole text, visible what the user was shown of it, plan what
	// propose returned this turn, via the model that answered.
	Answer(raw, visible string, plan *protocol.CommandPlan, via string) protocol.AnswerData
}

// AskFilter is the Ask mode's stream filter.
type AskFilter = askmode.Filter
