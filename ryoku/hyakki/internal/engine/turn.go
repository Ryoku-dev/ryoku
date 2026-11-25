package engine

// Derived from oh-my-pi (MIT), packages/agent/src/agent-loop.ts@c7fff67

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

type turnState struct {
	sink     core.Sink
	turn     int
	prefix   string
	attempts int
	ask      bool
	loop     thinkingLoop
	guards   []StreamGuard
	tools    tool.Set
	env      tool.Env
	context  TurnContext
	calls    []CallRecord
	usage    llm.Usage
	last     llm.Final
	plan     *protocol.PlanData
	proposal *protocol.CommandPlan
	lastText string
	visible  string
	end      bool
}

func (ts *turnState) kindOf(name string) string {
	if t, ok := ts.tools.Lookup(name); ok {
		return t.Kind()
	}
	return protocol.ToolOther
}

func (r *runner) runTurn(ctx context.Context, in core.PromptInput, sink core.Sink, started time.Time) (core.TurnResult, error) {
	model, strict := r.currentModel()
	if reason := r.budget().exceeded(started); reason != "" {
		return r.limited(sink, reason, llm.Usage{})
	}
	r.mu.Lock()
	initialMode := r.mode
	sessionInfo := r.opts.Session
	r.mu.Unlock()
	cwd := r.opts.Cwd
	if isAsk(initialMode) && in.Terminal != nil && in.Terminal.Cwd != "" {
		cwd = in.Terminal.Cwd
	}
	var msg llm.Message
	var err error
	if in.Run != "" && isAsk(initialMode) && r.f.deps.Ask != nil {
		var ok bool
		x := r.environment(sink, cwd, sessionInfo.Project, tool.Profile{Mode: initialMode, Class: tool.ClassFull}).Exec
		msg, ok, err = r.f.deps.Ask.Run(ctx, x, in, cwd, sink)
		if err != nil {
			return r.failedTurn(llm.Usage{}, err)
		}
		if !ok {
			return core.TurnResult{Stop: protocol.StopCancelled}, nil
		}
	} else if r.f.deps.Ask != nil {
		msg, err = r.f.deps.Ask.Message(in.Text, in.Images)
	} else {
		msg = llm.Message{Role: llm.RoleUser, Content: in.Text}
	}
	if err != nil {
		return r.failedTurn(llm.Usage{}, err)
	}
	if msg.Role == "" {
		msg.Role = llm.RoleUser
	}
	if err = r.transcript.BeginTurn(in.Turn); err != nil {
		return r.failedTurn(llm.Usage{}, err)
	}
	if r.transcript.Len() == 0 && isAsk(initialMode) && r.f.deps.Ask != nil {
		if history := r.f.deps.Ask.History(in.History); len(history) > 0 {
			if err = r.transcript.Append(history...); err != nil {
				return r.failedTurn(llm.Usage{}, err)
			}
		}
	}
	reminder := Reminder(time.Now(), cwd, in.Terminal)
	if reminder != r.reminder {
		msg.Content = reminder + "\n\n" + msg.Content
		r.reminder = reminder
	}
	if err = r.transcript.Append(msg); err != nil {
		return r.failedTurn(llm.Usage{}, err)
	}
	project := sessionInfo.Project
	ts := &turnState{sink: sink, turn: in.Turn, prefix: string(initialMode), ask: isAsk(initialMode)}
	r.mu.Lock()
	mode := r.mode
	effort := r.options[protocol.OptionEffort]
	r.mu.Unlock()
	mp := ProfileFor(mode)
	var guard loopGuard
	var rc recovery
	var blocks []Block
	models := protocol.ModelsResult{}
	var lastReq llm.Request
	if r.f.deps.Gateway != nil {
		models, _ = r.f.deps.Gateway.Models(ctx, protocol.ModelsParams{})
	}
	for round := 0; round < mp.Rounds; round++ {
		if err := ctx.Err(); err != nil {
			return core.TurnResult{Stop: protocol.StopCancelled, Usage: usageResult(ts.usage)}, nil
		}
		if reason := r.budget().exceeded(started); reason != "" {
			return r.limited(sink, reason, ts.usage)
		}
		r.mu.Lock()
		mode = r.mode
		r.mu.Unlock()
		mp = ProfileFor(mode)
		probe := llm.Request{Model: model, Strict: strict}
		window := Window(ctx, r.f.deps.Gateway, probe, ts.last)
		class := tool.ClassFull
		selected := model
		if protocol.IsRoute(model) && ts.last.Model != "" {
			selected = via(ts.last)
		}
		for _, m := range models.Models {
			if m.ID == selected {
				class = tool.ClassForModel(m)
				break
			}
		}
		if protocol.IsRoute(model) && ts.last.Model == "" && !r.opts.Settings().Providers.Models.RemoteFirst {
			anyLocal, allCompact := false, true
			for _, m := range models.Models {
				if m.Tier == protocol.TierLocal && m.Available && m.Tools {
					anyLocal = true
					allCompact = allCompact && tool.ClassForModel(m) == tool.ClassCompact
				}
			}
			if anyLocal && allCompact {
				class = tool.ClassCompact
			}
		}
		profile := tool.Profile{Mode: mode, Class: class, Window: window, ReadOnly: mp.ReadOnly || r.readOnly}
		env := r.environment(sink, cwd, project, profile)
		ts.env = env
		if r.f.deps.Tools != nil {
			ts.tools = r.f.deps.Tools.For(ctx, env)
		}
		ts.context = TurnContext{Session: env.Session, Cwd: cwd, Project: project, Profile: profile, Prompt: in.Text, Goal: in.Goal}
		if round == 0 {
			for _, guard := range r.f.deps.Guards {
				if g := guard.Guard(ts.context); g != nil {
					ts.guards = append(ts.guards, g)
				}
			}
			for _, provider := range r.f.deps.Context {
				blocks = append(blocks, provider.Blocks(ctx, ts.context)...)
			}
		}
		habits := ""
		if ts.ask && r.f.deps.Ask != nil {
			habits = r.f.deps.Ask.Habits(r.opts.Settings())
		}
		brief := r.sessionBrief()
		role := r.role
		if profile.Compact() && r.compactRole != "" {
			role = r.compactRole
		}
		session := env.Session
		if ts.ask {
			session.Mode = r.gate.approvalMode()
		}
		system := SystemPrompt(PromptInput{Profile: profile, Template: mp.Prompt, Session: session, Cwd: cwd, Project: project, Sandbox: r.opts.Sandbox, Settings: r.opts.Settings(), Brief: brief, Blocks: blocks, Tools: ts.tools.Names(), Habits: habits, Role: role, Context: r.sharedContext})
		req := llm.Request{Model: model, Strict: strict, Messages: append([]llm.Message{{Role: llm.RoleSystem, Content: system}}, r.transcript.Messages()...), Tools: ts.tools.Specs(), Temperature: temperatureFor(profile), Reasoning: effort, MaxTokens: mp.MaxOutput}
		window = Window(ctx, r.f.deps.Gateway, req, ts.last)
		if _, err = r.transcript.Compact(ctx, r.f.deps.Gateway, req, window, sink); err != nil {
			return r.failedTurn(ts.usage, err)
		}
		req.Messages = append([]llm.Message{{Role: llm.RoleSystem, Content: system}}, r.transcript.Messages()...)
		if PromptTokens(req, r.transcript.Anchor())+MinOutputTokens+OutputHeadwayTokens > window {
			if _, err = r.transcript.ForceCompact(ctx, r.f.deps.Gateway, req, window, sink); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			req.Messages = append([]llm.Message{{Role: llm.RoleSystem, Content: system}}, r.transcript.Messages()...)
		}
		promptTokens := PromptTokens(req, r.transcript.Anchor())
		if promptTokens+MinOutputTokens+OutputHeadwayTokens > window {
			return r.failedTurn(ts.usage, fmt.Errorf("model context window %d cannot fit the system prompt and tools", window))
		}
		req.MaxTokens = FitOutput(window, promptTokens, mp.MaxOutput)
		ceiling := r.budget()
		if ceiling.Tokens > 0 {
			remaining := ceiling.Tokens - ceiling.SpentTokens - promptTokens
			if remaining < MinOutputTokens {
				return r.limited(sink, "tokens", ts.usage)
			}
			req.MaxTokens = min(req.MaxTokens, remaining)
		}
		for _, g := range ts.guards {
			g.Reset()
		}
		lastReq = req
		rep := r.streamReply(ctx, ts, req)
		for retry := 0; rep.err != nil && retry < retryMax && retryable(ctx, rep.err) && !rep.shown; retry++ {
			if !sleep(ctx, backoff(retry+1)) {
				return core.TurnResult{Stop: protocol.StopCancelled, Usage: usageResult(ts.usage)}, nil
			}
			for _, g := range ts.guards {
				g.Reset()
			}
			rep = r.streamReply(ctx, ts, req)
		}
		if rep.err != nil && contextTooLong(rep.err) && !rep.shown {
			if _, err = r.transcript.ForceCompact(ctx, r.f.deps.Gateway, req, window, sink); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			req.Messages = append([]llm.Message{{Role: llm.RoleSystem, Content: system}}, r.transcript.Messages()...)
			retryPrompt := PromptTokens(req, r.transcript.Anchor())
			if retryPrompt+MinOutputTokens+OutputHeadwayTokens > window {
				return r.failedTurn(ts.usage, rep.err)
			}
			req.MaxTokens = min(req.MaxTokens, FitOutput(window, retryPrompt, mp.MaxOutput))
			for _, g := range ts.guards {
				g.Reset()
			}
			rep = r.streamReply(ctx, ts, req)
		}
		if err = r.appendNotes(rep.notices); err != nil {
			return r.failedTurn(ts.usage, err)
		}
		if rep.err != nil {
			if ctx.Err() != nil {
				return core.TurnResult{Stop: protocol.StopCancelled, Usage: usageResult(ts.usage)}, nil
			}
			if malformedCall(rep.err) && rc.malformed < nudgeMax {
				rc.malformed++
				if e := r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf(malformedNote, rc.malformed, nudgeMax)}); e != nil {
					return r.failedTurn(ts.usage, e)
				}
				continue
			}
			if retryable(ctx, rep.err) && rep.shown && rc.stall < nudgeMax {
				rc.stall++
				if err = r.transcript.Append(llm.Message{Role: llm.RoleAssistant, Content: rep.text}, llm.Message{Role: llm.RoleUser, Content: stallNote}); err != nil {
					return r.failedTurn(ts.usage, err)
				}
				continue
			}
			return r.failedTurn(ts.usage, rep.err)
		}
		if rep.interrupt != nil {
			rc.interrupts++
			if rc.interrupts > interruptMax {
				return r.failedTurn(ts.usage, errors.New("stream rule interrupted too many attempts"))
			}
			if err = r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: rep.interrupt.Message}); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			if !sleep(ctx, interruptDelay) {
				return core.TurnResult{Stop: protocol.StopCancelled, Usage: usageResult(ts.usage)}, nil
			}
			continue
		}
		if rep.looped {
			rc.thinking++
			if rc.thinking > nudgeMax {
				if r.closingAnswer(ctx, ts, req) {
					return r.endTurn(ts, protocol.StopEnd, nil)
				}
				sink.Emit(protocol.EvNotice, protocol.NoticeData{Kind: protocol.NoticeLoop, Text: i18n.T("The model repeated itself too many times, so the turn stopped.")})
				return r.endTurn(ts, protocol.StopEnd, nil)
			}
			if err = r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: thinkingLoopRedirect}); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			continue
		}
		ts.last = rep.final
		ts.lastText = rep.text
		ts.visible = rep.visible
		ts.usage.Input += rep.final.Usage.Input
		ts.usage.Output += rep.final.Usage.Output
		ts.usage.CacheRead += rep.final.Usage.CacheRead
		ts.usage.CacheWrite += rep.final.Usage.CacheWrite
		ts.usage.Estimated = ts.usage.Estimated || rep.final.Usage.Estimated
		ts.usage.Unmetered = ts.usage.Unmetered || rep.final.Usage.Unmetered
		r.spend(rep.final.Usage)
		if len(rep.calls) == 0 && strings.TrimSpace(rep.text) == "" {
			rc.empty++
			if rc.empty > nudgeMax {
				return r.failedTurn(ts.usage, errors.New("model repeatedly returned an empty answer"))
			}
			if err = r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf(emptyStopNote, rc.empty, nudgeMax)}); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			continue
		}
		if rep.final.Finish == llm.FinishLength && rc.length < nudgeMax {
			rc.length++
			if err = r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf(lengthStopNote, req.MaxTokens, rc.length, nudgeMax)}); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			continue
		}
		if rep.final.Finish != llm.FinishLength && malformedArguments(rep.calls) {
			rc.malformed++
			if rc.malformed > nudgeMax {
				return r.failedTurn(ts.usage, errors.New("model repeatedly returned malformed tool calls"))
			}
			if err = r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf(malformedNote, rc.malformed, nudgeMax)}); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			continue
		}
		fillCallIDs(rep.calls, ts.prefix, ts.turn, ts.attempts)
		if err = r.transcript.Append(llm.Message{Role: llm.RoleAssistant, Content: rep.text, Thinking: rep.thinking, ToolCalls: rep.calls}); err != nil {
			return r.failedTurn(ts.usage, err)
		}
		r.transcript.SetUsage(rep.final.Usage)
		if len(rep.calls) > 0 && rep.final.Finish != llm.FinishLength {
			count := guard.observe(rep.calls)
			if count >= loopThreshold {
				if guard.redirected {
					for _, call := range rep.calls {
						if err = r.transcript.Append(llm.Message{Role: llm.RoleTool, Name: call.Name, ToolCallID: call.ID, Content: loopSkipped, IsError: true}); err != nil {
							return r.failedTurn(ts.usage, err)
						}
					}
					if r.closingAnswer(ctx, ts, req) {
						return r.endTurn(ts, protocol.StopEnd, nil)
					}
					sink.Emit(protocol.EvNotice, protocol.NoticeData{Kind: protocol.NoticeLoop, Text: i18n.T("The same tool call kept repeating, so the turn stopped.")})
					return r.endTurn(ts, protocol.StopEnd, nil)
				}
				guard.redirected = true
				for _, call := range rep.calls {
					if err = r.transcript.Append(llm.Message{Role: llm.RoleTool, Name: call.Name, ToolCallID: call.ID, Content: loopSkipped, IsError: true}); err != nil {
						return r.failedTurn(ts.usage, err)
					}
				}
				if err = r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: loopNote(rep.calls[0], count, lastCallResult(ts.calls, rep.calls[0]))}); err != nil {
					return r.failedTurn(ts.usage, err)
				}
				continue
			}
			rc.progressed()
			if err = r.executeCalls(ctx, ts, rep.calls); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			if r.transcript.Rollover() {
				r.reminder = ""
			}
			if ts.end {
				return r.endTurn(ts, protocol.StopEnd, nil)
			}
			if queued := r.steer.drain(); len(queued) > 0 {
				if err = r.transcript.Append(queued...); err != nil {
					return r.failedTurn(ts.usage, err)
				}
			}
			continue
		}
		if queued := r.steer.drain(); len(queued) > 0 {
			if err = r.transcript.Append(queued...); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			continue
		}
		if rep.final.Finish == llm.FinishLength {
			for _, call := range rep.calls {
				if err = r.transcript.Append(llm.Message{Role: llm.RoleTool, Name: call.Name, ToolCallID: call.ID, Content: lengthCallNote, IsError: true}); err != nil {
					return r.failedTurn(ts.usage, err)
				}
			}
			return r.endTurn(ts, protocol.StopMaxTokens, nil)
		}
		if mode == tool.ModePlan && ts.plan != nil {
			approved, e := r.gate.approvePlan(ctx, planText(ts.plan))
			if e != nil {
				return core.TurnResult{Stop: protocol.StopCancelled, Usage: usageResult(ts.usage)}, nil
			}
			if approved {
				r.mu.Lock()
				r.mode = tool.ModeAgent
				r.options[protocol.OptionPlan] = protocol.OptionOff
				r.mu.Unlock()
				sink.Emit(protocol.EvNotice, protocol.NoticeData{Kind: protocol.NoticePlan, Text: i18n.T("Plan approved. Building started.")})
				if err = r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: PlanApproved(planText(ts.plan))}); err != nil {
					return r.failedTurn(ts.usage, err)
				}
				continue
			}
			return r.endTurn(ts, protocol.StopEnd, nil)
		}
		if detectsUnexpectedStop(mode) && rc.unexpected < nudgeMax && stoppedShort(ctx, r.f.deps.Judge, in.Text, rep.text) {
			rc.unexpected++
			if err = r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: fmt.Sprintf(unexpectedStopNote, rc.unexpected, nudgeMax)}); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			continue
		}
		if queued := r.steer.finish(); len(queued) > 0 {
			if err = r.transcript.Append(queued...); err != nil {
				return r.failedTurn(ts.usage, err)
			}
			continue
		}
		return r.endTurn(ts, finishStop(rep.final.Finish), nil)
	}
	if r.closingAnswer(ctx, ts, lastReq) {
		return r.endTurn(ts, protocol.StopEnd, nil)
	}
	return r.endTurn(ts, protocol.StopMaxRounds, nil)
}

// closingAnswer gives a turn that is about to stop on a loop or the round cap
// one last request without tools, so the user gets an answer from what was
// already read instead of a bare stop notice. It reports whether that answer
// came.
func (r *runner) closingAnswer(ctx context.Context, ts *turnState, req llm.Request) bool {
	if ctx.Err() != nil || len(ts.calls) == 0 || len(req.Messages) == 0 {
		return false
	}
	if err := r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: closingNote}); err != nil {
		return false
	}
	req.Messages = append([]llm.Message{req.Messages[0]}, r.transcript.Messages()...)
	req.Tools = nil
	window := Window(ctx, r.f.deps.Gateway, req, ts.last)
	req.MaxTokens = FitOutput(window, PromptTokens(req, r.transcript.Anchor()), 0)
	rep := r.streamReply(ctx, ts, req)
	if rep.err != nil || rep.looped || len(rep.calls) > 0 || strings.TrimSpace(rep.text) == "" {
		return false
	}
	if err := r.transcript.Append(llm.Message{Role: llm.RoleAssistant, Content: rep.text}); err != nil {
		return false
	}
	ts.last, ts.lastText, ts.visible = rep.final, rep.text, rep.visible
	ts.usage.Input += rep.final.Usage.Input
	ts.usage.Output += rep.final.Usage.Output
	r.spend(rep.final.Usage)
	return true
}

func (r *runner) sessionBrief() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.briefReady {
		if r.opts.Brief != nil {
			r.brief = r.opts.Brief()
		}
		r.briefReady = true
	}
	return r.brief
}

func lastCallResult(calls []CallRecord, call llm.ToolCall) string {
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].Name == call.Name && canonicalArgs(string(calls[i].Args)) == canonicalArgs(call.Arguments) {
			return calls[i].Result
		}
	}
	return ""
}
func (r *runner) appendNotes(notes []string) error {
	for _, note := range notes {
		if err := r.transcript.Append(llm.Message{Role: llm.RoleUser, Content: note}); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) endTurn(ts *turnState, stop string, err error) (core.TurnResult, error) {
	if ts.ask && r.f.deps.Ask != nil {
		ts.sink.Emit(protocol.EvAnswer, r.f.deps.Ask.Answer(ts.lastText, ts.visible, ts.proposal, via(ts.last)))
	}
	rec := TurnRecord{TurnContext: ts.context, Turn: ts.turn, Calls: ts.calls, Text: ts.lastText, Model: via(ts.last), Stop: stop, Usage: ts.usage}
	for _, observer := range r.f.deps.Observers {
		go observer.TurnEnded(r.observerCtx, rec)
	}
	return core.TurnResult{Stop: stop, Usage: usageResult(ts.usage)}, err
}

func (r *runner) environment(sink core.Sink, cwd, project string, p tool.Profile) tool.Env {
	r.mu.Lock()
	if r.executor == nil && r.f.deps.Executor != nil {
		r.executor = r.f.deps.Executor(r.opts.Session, r.opts.Sandbox)
	}
	e := r.executor
	session := r.opts.Session
	r.mu.Unlock()
	env := tool.Env{Session: session, Cwd: cwd, Project: project, Profile: p, Limits: p.Limits(), Exec: e, Sandbox: r.opts.Sandbox, Context: r.opts.Tools, Sink: sink, Permissions: r.gate, Index: r.f.deps.Index, Settings: r.opts.Settings, Spawner: r}
	if r.f.deps.Host != nil {
		env.Host = r.f.deps.Host(session)
	}
	return env
}

func (r *runner) executeCalls(ctx context.Context, ts *turnState, calls []llm.ToolCall) error {
	results := make([]tool.Result, len(calls))
	run := func(i int) {
		call := calls[i]
		defer func() {
			if p := recover(); p != nil {
				results[i] = tool.Failed(fmt.Sprintf("Tool %s failed unexpectedly: %v", call.Name, p))
			}
		}()
		t, ok := ts.tools.Lookup(call.Name)
		if !ok {
			results[i] = tool.Failed(i18n.T("This tool is not available in this mode."))
			return
		}
		pres := t.Present(json.RawMessage(call.Arguments))
		emit := func(data protocol.ToolData) {
			data.Call = call.ID
			data.Parent = r.parentCall()
			ts.sink.Emit(protocol.EvTool, data)
		}
		emit(protocol.ToolData{Title: pres.Title, Kind: t.Kind(), Status: protocol.ToolRunning, Input: pres.Input, Locations: pres.Locations})
		env := ts.env
		env.Emit = emit
		results[i] = t.Run(ctx, tool.Call{ID: call.ID, Name: call.Name, Args: json.RawMessage(call.Arguments), Parent: r.parentCall()}, env)
	}
	for start := 0; start < len(calls); {
		end := start
		for end < len(calls) {
			t, ok := ts.tools.Lookup(calls[end].Name)
			if !ok || !canParallel(calls[end].Name, t) {
				break
			}
			end++
		}
		if end > start {
			var wg sync.WaitGroup
			for i := start; i < end; i++ {
				wg.Add(1)
				go func(i int) { defer wg.Done(); run(i) }(i)
			}
			wg.Wait()
			start = end
			continue
		}
		run(start)
		if results[start].End {
			for i := start + 1; i < len(calls); i++ {
				results[i] = tool.Failed(i18n.T("Not run: the turn already handed back its result."))
			}
			break
		}
		start++
	}
	for i, call := range calls {
		result := results[i]
		text := result.Text
		if len(text) > ts.env.Limits.Told {
			text = tool.HeadTail(text, ts.env.Limits.Head, ts.env.Limits.Tail)
		}
		shown := result.Shown
		if shown == "" {
			shown = text
		}
		shown, _ = tool.ClipRunes(shown, ts.env.Limits.Shown)
		status := protocol.ToolDone
		if result.IsError {
			status = protocol.ToolFailed
		}
		ts.sink.Emit(protocol.EvTool, protocol.ToolData{Call: call.ID, Status: status, Output: shown, Diff: result.Diff, Locations: result.Locations, Agents: result.Agents, Parent: r.parentCall()})
		ts.calls = append(ts.calls, CallRecord{ID: call.ID, Name: call.Name, Args: json.RawMessage(call.Arguments), Result: firstBytes(text, 2048), IsError: result.IsError, Parent: r.parentCall()})
		ts.usage.Input += result.Usage.Input
		ts.usage.Output += result.Usage.Output
		if call.Name != tool.Task {
			r.spend(result.Usage)
		}
		if result.Plan != nil {
			ts.plan = result.Plan
			ts.sink.Emit(protocol.EvPlan, *result.Plan)
		}
		if result.Proposal != nil {
			ts.proposal = result.Proposal
		}
		if result.End {
			ts.end = true
			if call.Name == tool.Yield {
				r.childOutput = text
				r.childFailed = result.IsError
			}
		}
		if err := r.transcript.Append(llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Name: call.Name, Content: text, Images: result.Images, IsError: result.IsError}); err != nil {
			return err
		}
	}
	return nil
}

func canParallel(name string, t tool.Tool) bool {
	if name == tool.Task || name == tool.ContextNotes || name == tool.NewContext || name == tool.SystemQuery || name == tool.Yield || name == tool.Propose || strings.HasPrefix(name, "screen_") {
		return false
	}
	parallel, ok := t.(tool.Parallel)
	return ok && t.ReadOnly() && parallel.Parallel()
}

func firstBytes(text string, max int) string { clipped, _ := clipText(text, max); return clipped }
