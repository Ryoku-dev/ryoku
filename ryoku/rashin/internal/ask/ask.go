// Package ask runs Rashin's system agent on the built-in model router. It
// reads the machine to answer, and changes it only in a session the user set
// to allow that.
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
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const (
	maxRounds = 8
	// a change is read, cleared, made and checked, which takes more rounds than a look
	maxActRounds = 14
)

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

// NewAsk creates one stateful Ask session. A session whose mode is not a known
// approvals value, which is every session made before modes and every one
// asked from the bar or a terminal, is read-only.
func (f *Factory) NewAsk(opts core.AskOptions) core.Runner {
	mode := opts.Session.Mode
	if !protocol.KnownApprovals(mode) {
		mode = protocol.ApprovalsReadOnly
	}
	return &runner{factory: f, opts: opts, model: opts.Session.Model, mode: mode}
}

type runner struct {
	factory *Factory
	opts    core.AskOptions

	mu     sync.Mutex
	model  string
	strict bool
	cancel context.CancelFunc

	// mode is an approvals value: what this session may do to the machine
	mode string
	// allowed is set once the user allows changes for the whole conversation
	allowed bool
	pending map[string]pendingRequest
	asked   int
	kept    int
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
	asked, err := userMessage(in.Text, in.Images)
	if err != nil {
		return core.TurnResult{Stop: protocol.StopError}, err
	}
	// the mode is read once a turn: what Ask was told it may do and the tools
	// it was handed have to agree for the whole of it
	mode := r.currentMode()
	rounds := maxRounds
	if acts(mode) {
		rounds = maxActRounds
	}
	cwd := paths.Home()
	if in.Terminal != nil && in.Terminal.Cwd != "" {
		cwd = in.Terminal.Cwd
	}
	messages := []llm.Message{{Role: llm.RoleSystem, Content: r.factory.systemPrompt(settings, in.Terminal, mode)}}
	messages = append(messages, historyMessages(in.History, historyBudgetBytes)...)
	messages = append(messages, asked)

	var usage protocol.TokenUsage
	var last llm.Final
	var plan *protocol.CommandPlan
	for round := range rounds {
		stream, err := r.factory.client.Stream(ctx, llm.Request{
			Model: model, Strict: strict, Messages: messages, Tools: askTools(mode), MaxTokens: 4096,
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
			if actTool(call.Name) {
				output, _ := r.act(ctx, call, sink, cwd)
				messages = append(messages, llm.Message{
					Role: llm.RoleTool, ToolCallID: call.ID, Name: call.Name, Content: output,
				})
				if ctx.Err() != nil {
					return core.TurnResult{Stop: protocol.StopCancelled, Usage: usagePtr(usage)}, nil
				}
				continue
			}
			title, kind, input := toolPresentation(call)
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

	note := i18n.Tf("I stopped after %d rounds of tools. Narrow the question or hand it to an Agent.", rounds)
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

func (r *runner) Permission(request, option string) error {
	r.mu.Lock()
	open, ok := r.pending[request]
	if !ok {
		r.mu.Unlock()
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This request is no longer open. Ask again."))
	}
	kind := ""
	for _, offered := range open.options {
		if offered.ID == option {
			kind = offered.Kind
		}
	}
	if kind == "" {
		r.mu.Unlock()
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("This answer is not one of the choices. Pick one of those shown."))
	}
	delete(r.pending, request)
	if kind == protocol.OptAllowAlways {
		r.allowed = true
	}
	r.mu.Unlock()
	open.sink.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: request, Option: option, Outcome: "selected", By: "user"})
	open.answer <- option
	return nil
}

func (r *runner) SetModel(_ context.Context, model string, sink core.Sink) error {
	r.mu.Lock()
	r.model = model
	r.strict = model != "" && model != protocol.RouteAuto && model != protocol.RouteFast && model != protocol.RouteSmart
	r.mu.Unlock()
	sink.Update(func(info *protocol.SessionInfo) { info.Model = model })
	return nil
}

// SetMode sets what this session may do. Going back to read-only also takes
// back an earlier "allow in this conversation".
func (r *runner) SetMode(_ context.Context, mode string, sink core.Sink) error {
	if !protocol.KnownApprovals(mode) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Ask can be read-only, ask first, or act without asking."))
	}
	r.mu.Lock()
	r.mode = mode
	if !acts(mode) {
		r.allowed = false
	}
	r.mu.Unlock()
	sink.Update(func(info *protocol.SessionInfo) { info.Mode = mode })
	return nil
}

func (r *runner) SetOption(_ context.Context, _, _ string, _ core.Sink) error {
	return protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.T("Ask has no options beside its model and what it may do."))
}

func (r *runner) Start(_ context.Context, _ core.Sink) error { return nil }

func (r *runner) Alive() bool            { return false }
func (r *runner) Stop() error            { return nil }
func (r *runner) State() json.RawMessage { return nil }

func modelError(err error) error {
	// the router has already said why nothing could take this and what to do
	var refused *llm.NoRoute
	if errors.As(err, &refused) {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", refused.Reason)
	}
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
