package engine

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func (r *runner) Cancel() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if r.parent == "" {
		r.gate.cancelAll()
	}
}

func (r *runner) Permission(request, option string) error { return r.gate.resolve(request, option) }

func (r *runner) Alive() bool {
	r.mu.Lock()
	e := r.executor
	r.mu.Unlock()
	return e != nil && exec.Alive(e)
}

func (r *runner) Stop() error {
	r.Cancel()
	r.mu.Lock()
	e := r.executor
	r.executor = nil
	r.closed = true
	r.mu.Unlock()
	releaseTranscript(r.opts.Session.ID, r.transcript)
	if e != nil && r.parent == "" {
		return e.Close()
	}
	return nil
}

func (r *runner) State() json.RawMessage {
	r.mu.Lock()
	state := struct {
		Version    int               `json:"version"`
		Transcript json.RawMessage   `json:"transcript"`
		Allowed    bool              `json:"allowed"`
		Ceiling    ceilings          `json:"ceiling"`
		Mode       tool.Mode         `json:"mode"`
		Options    map[string]string `json:"options"`
		Brief      string            `json:"brief"`
		BriefReady bool              `json:"briefReady"`
	}{TranscriptVersion, r.transcript.State(), r.gate.allowedAll(), r.ceiling, r.mode, r.options, r.brief, r.briefReady}
	b, _ := json.Marshal(state)
	r.mu.Unlock()
	return b
}

// Announce publishes the initial model, approvals and options for a new session.
func (r *runner) Announce(ctx context.Context, sink core.Sink) error {
	r.emitChoices(ctx, sink)
	return ctx.Err()
}

func (r *runner) emitChoices(ctx context.Context, sink core.Sink) {
	if sink == nil {
		return
	}
	model, _ := r.currentModel()
	models := protocol.ModelsResult{}
	if r.f.deps.Gateway != nil {
		models, _ = r.f.deps.Gateway.Models(ctx, protocol.ModelsParams{})
	}
	choices := append([]protocol.Choice(nil), models.Routes...)
	var selected protocol.ModelInfo
	for _, m := range models.Models {
		choices = append(choices, protocol.Choice{ID: m.ID, Name: m.Name})
		if m.ID == model {
			selected = m
		}
	}
	sink.Emit(protocol.EvModel, protocol.ChoiceData{Current: model, Options: choices, Local: selected.Tier == protocol.TierLocal})
	mode := r.gate.approvalMode()
	sink.Emit(protocol.EvMode, protocol.ChoiceData{Current: mode, Options: []protocol.Choice{{ID: protocol.ApprovalsReadOnly, Name: i18n.T("Read only")}, {ID: protocol.ApprovalsAsk, Name: i18n.T("Ask first")}, {ID: protocol.ApprovalsAuto, Name: i18n.T("Auto")}}})
	r.mu.Lock()
	plan := r.options[protocol.OptionPlan]
	effort := r.options[protocol.OptionEffort]
	ask := isAsk(r.mode)
	r.mu.Unlock()
	opts := []protocol.SessionOption{}
	if !ask {
		opts = append(opts, protocol.SessionOption{ID: protocol.OptionPlan, Name: i18n.T("Plan first"), Current: plan, Choices: []protocol.Choice{{ID: protocol.OptionOff, Name: i18n.T("Off")}, {ID: protocol.OptionOn, Name: i18n.T("On")}}})
	}
	if selected.Reasoning {
		opts = append(opts, protocol.SessionOption{ID: protocol.OptionEffort, Name: i18n.T("Reasoning effort"), Current: effort, Choices: []protocol.Choice{{ID: llm.EffortOff, Name: i18n.T("Off")}, {ID: llm.EffortLow, Name: i18n.T("Low")}, {ID: llm.EffortMedium, Name: i18n.T("Medium")}, {ID: llm.EffortHigh, Name: i18n.T("High")}}})
	}
	sink.Emit(protocol.EvOptions, protocol.OptionsData{Options: opts})
}

func (r *runner) SetModel(ctx context.Context, model string, sink core.Sink) error {
	if !protocol.ValidModel(model) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a valid model."))
	}
	if r.f.deps.Gateway != nil && !protocol.IsRoute(model) {
		list, err := r.f.deps.Gateway.Models(ctx, protocol.ModelsParams{})
		if err != nil {
			return err
		}
		found := false
		for _, m := range list.Models {
			if m.ID == model {
				found = true
				break
			}
		}
		if !found {
			return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That model is not available."))
		}
	}
	r.mu.Lock()
	r.model = model
	r.pinned = !protocol.IsRoute(model)
	r.mu.Unlock()
	if sink != nil {
		sink.Update(func(info *protocol.SessionInfo) { info.Model = model; info.Pinned = r.pinned })
	}
	r.emitChoices(ctx, sink)
	return nil
}

func (r *runner) SetMode(ctx context.Context, mode string, sink core.Sink) error {
	switch mode {
	case protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto:
	default:
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a valid approval mode."))
	}
	r.mu.Lock()
	if isAsk(r.mode) {
		if mode == protocol.ApprovalsReadOnly {
			r.mode = tool.ModeAskRead
		} else {
			r.mode = tool.ModeAskAct
		}
		r.opts.Session.Mode = mode
	} else {
		r.opts.Session.Approvals = mode
	}
	r.gate.setApprovals(func() string { return mode })
	r.mu.Unlock()
	if mode == protocol.ApprovalsReadOnly {
		r.gate.forget()
	}
	if sink != nil {
		sink.Update(func(info *protocol.SessionInfo) {
			if info.Kind == protocol.KindAsk {
				info.Mode = mode
			} else {
				info.Approvals = mode
			}
		})
	}
	r.emitChoices(ctx, sink)
	return nil
}

func (r *runner) SetOption(ctx context.Context, option, value string, sink core.Sink) error {
	switch option {
	case protocol.OptionPlan:
		r.mu.Lock()
		ask := isAsk(r.mode)
		team := r.opts.Tools.Team != nil
		r.mu.Unlock()
		if ask || team {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Plan first is for Agent sessions."))
		}
		if value != protocol.OptionOn && value != protocol.OptionOff {
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose On or Off."))
		}
		r.mu.Lock()
		r.options[option] = value
		if value == protocol.OptionOn {
			r.mode = tool.ModePlan
		} else {
			r.mode = tool.ModeAgent
		}
		r.mu.Unlock()
	case protocol.OptionEffort:
		switch value {
		case "", llm.EffortOff, llm.EffortLow, llm.EffortMedium, llm.EffortHigh:
		default:
			return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a valid reasoning effort."))
		}
		r.mu.Lock()
		r.options[option] = value
		r.mu.Unlock()
	default:
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("This option is not available."))
	}
	r.emitChoices(ctx, sink)
	return nil
}

func (r *runner) Prompt(ctx context.Context, in core.PromptInput, sink core.Sink) (core.TurnResult, error) {
	if sink == nil {
		return core.TurnResult{Stop: protocol.StopError}, errors.New("engine needs a session sink")
	}
	r.mu.Lock()
	if r.closed {
		r.closed = false
	}
	if r.cancel != nil {
		r.mu.Unlock()
		return core.TurnResult{Stop: protocol.StopError}, errors.New("turn already running")
	}
	if r.opts.Session.TeamTask != "" {
		r.brief, r.briefReady = "", false
	}
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.mu.Unlock()
	r.steer.start()
	if r.parent == "" {
		r.gate.bind(sink)
	}
	defer func() {
		cancel()
		if r.parent == "" {
			r.gate.cancelAll()
			r.gate.bind(nil)
		}
		if queued := r.steer.abandon(); len(queued) > 0 {
			_ = r.transcript.Append(queued...)
		}
		r.mu.Lock()
		r.cancel = nil
		r.mu.Unlock()
	}()
	started := time.Now()
	if remain := r.budget().remaining(started); remain > 0 {
		var timeout context.CancelFunc
		ctx, timeout = context.WithTimeout(ctx, remain)
		defer timeout()
	}
	r.mu.Lock()
	ask := isAsk(r.mode)
	r.mu.Unlock()
	if ask && r.f.deps.Ask != nil && (in.Glance != nil || r.opts.Session.Origin == protocol.OriginSight) {
		r.mu.Lock()
		model, strict := r.model, r.pinned
		r.mu.Unlock()
		return r.f.deps.Ask.Glance(ctx, in, model, strict, sink)
	}
	result, err := r.runTurn(ctx, in, sink, started)
	r.mu.Lock()
	r.ceiling.endTurn(started)
	r.mu.Unlock()
	if ctx.Err() == context.DeadlineExceeded && r.budget().Wall > 0 {
		var spent llm.Usage
		if result.Usage != nil {
			spent = llm.Usage{Input: result.Usage.Input, Output: result.Usage.Output}
		}
		return r.limited(sink, "time", spent)
	}
	if err != nil && ctx.Err() != nil {
		return core.TurnResult{Stop: protocol.StopCancelled, Usage: result.Usage}, nil
	}
	return result, err
}
func (r *runner) budget() ceilings {
	owner := r
	if r.budgetOwner != nil {
		owner = r.budgetOwner
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.ceiling
}

func (r *runner) spend(u llm.Usage) {
	owner := r
	if r.budgetOwner != nil {
		owner = r.budgetOwner
	}
	owner.mu.Lock()
	owner.ceiling.spend(u)
	owner.mu.Unlock()
}

func usageResult(u llm.Usage) *protocol.TokenUsage {
	return &protocol.TokenUsage{Input: u.Input, Output: u.Output, Estimated: u.Estimated, Unmetered: u.Unmetered}
}
func (r *runner) failedTurn(u llm.Usage, err error) (core.TurnResult, error) {
	return core.TurnResult{Stop: protocol.StopError, Usage: usageResult(u)}, err
}
func (r *runner) limited(sink core.Sink, kind string, u llm.Usage) (core.TurnResult, error) {
	msg := i18n.T("This session reached its budget and stopped.")
	if kind == "time" {
		msg = i18n.T("This session reached its time limit and stopped.")
	}
	sink.Emit(protocol.EvNotice, protocol.NoticeData{Kind: protocol.NoticeLimit, Text: msg})
	return core.TurnResult{Stop: protocol.StopMaxRounds, Usage: usageResult(u)}, nil
}
