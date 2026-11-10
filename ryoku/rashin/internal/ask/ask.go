// Package ask runs Rashin's read-only system agent on the built-in model router.
package ask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const maxRounds = 8

// Factory creates independent Ask session runners.
type Factory struct {
	client llm.Client
	index  core.Indexer
	habits habitCache
	doctor doctorCache
}

// New creates the native Ask runner factory.
func New(client llm.Client, idx core.Indexer) *Factory {
	return &Factory{client: client, index: idx}
}

// NewAsk creates one stateful Ask session.
func (f *Factory) NewAsk(opts core.AskOptions) core.Runner {
	return &runner{factory: f, opts: opts, model: opts.Session.Model}
}

type runner struct {
	factory *Factory
	opts    core.AskOptions

	mu     sync.Mutex
	model  string
	strict bool
	cancel context.CancelFunc
}

func (r *runner) Prompt(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.cancel = cancel
	model := r.model
	strict := r.strict
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		r.cancel = nil
		r.mu.Unlock()
	}()

	settings := protocol.Settings{}
	if r.opts.Settings != nil {
		settings = r.opts.Settings()
	}
	if model == "" {
		model = settings.Ask.Model
	}
	if model == "" {
		model = protocol.RouteAuto
	}
	messages := []llm.Message{{Role: llm.RoleSystem, Content: r.factory.systemPrompt(settings, in.Terminal)}}
	messages = append(messages, historyMessages(in.History, historyBudgetBytes)...)
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: in.Text})

	var usage protocol.TokenUsage
	var last llm.Final
	var plan *protocol.CommandPlan
	for round := range maxRounds {
		stream, err := r.factory.client.Stream(ctx, llm.Request{
			Model: model, Strict: strict, Messages: messages, Tools: askTools(), MaxTokens: 4096,
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				return core.TurnResult{Stop: protocol.StopCancelled, Usage: usagePtr(usage)}, nil
			}
			return core.TurnResult{Stop: protocol.StopError, Usage: usagePtr(usage)}, modelError(err)
		}

		messageID := fmt.Sprintf("ask-%d-%d", in.Turn, round+1)
		var raw strings.Builder
		var roundVisible strings.Builder
		var calls []llm.ToolCall
		filter := newNeedsFilter(func(delta string) {
			if delta == "" {
				return
			}
			roundVisible.WriteString(delta)
			sink.Emit(protocol.EvText, protocol.TextData{Message: messageID, Delta: delta})
		})
		for {
			delta, nextErr := stream.Next()
			if nextErr == io.EOF {
				break
			}
			if nextErr != nil {
				_ = stream.Close()
				if errors.Is(nextErr, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
					return core.TurnResult{Stop: protocol.StopCancelled, Usage: usagePtr(usage)}, nil
				}
				return core.TurnResult{Stop: protocol.StopError, Usage: usagePtr(usage)}, modelError(nextErr)
			}
			if delta.Text != "" {
				raw.WriteString(delta.Text)
				filter.Write(delta.Text)
			}
			if delta.Reasoning != "" {
				sink.Emit(protocol.EvThought, protocol.TextData{Message: messageID, Delta: delta.Reasoning})
			}
			calls = append(calls, delta.ToolCalls...)
		}
		filter.Close()
		last = stream.Final()
		_ = stream.Close()
		usage.Input += last.Usage.Input
		usage.Output += last.Usage.Output
		usage.Estimated = usage.Estimated || last.Usage.Estimated
		for i := range calls {
			if calls[i].ID == "" {
				calls[i].ID = fmt.Sprintf("ask-%d-%d-tool-%d", in.Turn, round+1, i+1)
			}
		}

		assistant := llm.Message{Role: llm.RoleAssistant, Content: raw.String(), ToolCalls: calls}
		messages = append(messages, assistant)
		if len(calls) == 0 {
			_, needsAgent, reason := stripNeedsAgent(raw.String())
			sink.Emit(protocol.EvAnswer, protocol.AnswerData{
				Actions: extractActions(roundVisible.String()), Plan: plan,
				NeedsAgent: needsAgent, Reason: reason, Via: via(last),
			})
			return core.TurnResult{Stop: finishStop(last.Finish), Usage: usagePtr(usage)}, nil
		}

		for _, call := range calls {
			title, kind := toolPresentation(call)
			input := truncateRunes(compactInput(call.Arguments), 400)
			sink.Emit(protocol.EvTool, protocol.ToolData{
				Call: call.ID, Title: title, Kind: kind, Status: protocol.ToolRunning, Input: input,
			})
			output, nextPlan, toolErr := r.factory.runTool(ctx, call, in.Terminal)
			if nextPlan != nil {
				plan = nextPlan
			}
			status := protocol.ToolDone
			if toolErr != nil {
				status = protocol.ToolFailed
				if output == "" {
					output = toolErr.Error()
				} else {
					output += "\n" + toolErr.Error()
				}
			}
			output = truncateRunes(output, 6000)
			sink.Emit(protocol.EvTool, protocol.ToolData{
				Call: call.ID, Title: title, Kind: kind, Status: status, Output: output,
			})
			messages = append(messages, llm.Message{
				Role: llm.RoleTool, ToolCallID: call.ID, Name: call.Name, Content: output,
			})
			if ctx.Err() != nil {
				return core.TurnResult{Stop: protocol.StopCancelled, Usage: usagePtr(usage)}, nil
			}
		}
	}

	note := i18n.T("I stopped after eight read-only tool rounds. Narrow the question or hand it to an Agent.")
	messageID := fmt.Sprintf("ask-%d-limit", in.Turn)
	sink.Emit(protocol.EvText, protocol.TextData{Message: messageID, Delta: note})
	sink.Emit(protocol.EvAnswer, protocol.AnswerData{Actions: extractActions(note), Plan: plan, Via: via(last)})
	return core.TurnResult{Stop: protocol.StopMaxRounds, Usage: usagePtr(usage)}, nil
}

func (r *runner) Cancel() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (r *runner) Permission(_, _ string) error {
	return protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.T("Ask has no permissions because its tools are read only."))
}

func (r *runner) SetModel(_ context.Context, model string, sink core.Sink) error {
	r.mu.Lock()
	r.model = model
	r.strict = model != "" && model != protocol.RouteAuto && model != protocol.RouteFast && model != protocol.RouteSmart
	r.mu.Unlock()
	sink.Update(func(info *protocol.SessionInfo) { info.Model = model })
	return nil
}

func (r *runner) SetMode(_ context.Context, _ string, _ core.Sink) error {
	return protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.T("Ask does not have modes."))
}

func (r *runner) Alive() bool            { return false }
func (r *runner) Stop() error            { return nil }
func (r *runner) State() json.RawMessage { return nil }

func modelError(err error) error {
	return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.Tf("Ask could not start a free model response: %v. Add a free key in Rashin > Settings > Providers.", err))
}

func usagePtr(u protocol.TokenUsage) *protocol.TokenUsage {
	if u.Input == 0 && u.Output == 0 {
		return nil
	}
	return &u
}

func finishStop(finish string) string {
	switch finish {
	case llm.FinishLength:
		return protocol.StopMaxTokens
	case llm.FinishFilter:
		return protocol.StopRefusal
	default:
		return protocol.StopEnd
	}
}

func via(final llm.Final) string {
	model := strings.TrimSpace(final.Model)
	provider := strings.TrimSpace(final.Provider)
	if provider == "" || strings.HasPrefix(model, provider+"/") {
		return model
	}
	if model == "" {
		return provider
	}
	return provider + "/" + model
}

func compactInput(raw string) string {
	var value any
	if json.Unmarshal([]byte(raw), &value) == nil {
		if b, err := json.Marshal(value); err == nil {
			return string(b)
		}
	}
	return strings.TrimSpace(raw)
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	keep := n - 3
	if keep < 0 {
		keep = 0
	}
	count := 0
	for index := range s {
		if count == keep {
			return s[:index] + strings.Repeat(".", n-keep)
		}
		count++
	}
	return s
}

var _ core.Asker = (*Factory)(nil)
var _ core.Runner = (*runner)(nil)
