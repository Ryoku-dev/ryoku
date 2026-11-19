package engine

// Derived from oh-my-pi (MIT), packages/agent/src/agent-loop.ts@c7fff67

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/engine/tools/agents"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Factory creates native Ask and Agent runners with the same engine.
type Factory struct {
	deps        Deps
	observerCtx context.Context
	cancel      context.CancelFunc
}

var childSequence atomic.Uint64

func New(deps Deps) *Factory {
	ctx, cancel := context.WithCancel(context.Background())
	return &Factory{deps: deps, observerCtx: ctx, cancel: cancel}
}

// Close ends background observers when the daemon shuts down.
func (f *Factory) Close() error { f.cancel(); return nil }

func (f *Factory) NewAsk(opts core.AskOptions) core.Runner {
	info := opts.Session
	mode := ModeForAsk(info)
	return f.newRunner(Options{Session: info, Mode: mode, Cwd: paths.Home(), Settings: opts.Settings, Approvals: func() string { return info.Mode }})
}

func (f *Factory) NewAgent(ctx context.Context, opts core.AgentOptions) (core.Runner, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	options := make(map[string]string, len(opts.Options))
	for k, v := range opts.Options {
		options[k] = v
	}
	return f.newRunner(Options{Session: opts.Session, Mode: ModeForAgent(opts), Cwd: opts.Cwd, Settings: opts.Settings, Approvals: opts.Approvals, Options: options, Brief: opts.Brief, Resume: opts.Resume, Tools: opts.Tools, MCP: opts.MCP, Sandbox: opts.Sandbox}), nil
}

// runner owns one session's transcript, gate, model and executor.
type runner struct {
	f                                *Factory
	opts                             Options
	mu                               sync.Mutex
	cancel                           context.CancelFunc
	closed                           bool
	model                            string
	pinned                           bool
	mode                             tool.Mode
	childOutput                      string
	childFailed                      bool
	options                          map[string]string
	gate                             *gate
	transcript                       *Transcript
	executor                         exec.Executor
	steer                            steering
	ceiling                          ceilings
	reminder                         string
	parent                           string
	budgetOwner                      *runner
	role, compactRole, sharedContext string
	brief                            string
	briefReady                       bool
	readOnly                         bool
	observerCtx                      context.Context
}

func (f *Factory) newRunner(opts Options) *runner {
	if opts.Cwd == "" {
		opts.Cwd = opts.Session.Cwd
	}
	if opts.Settings == nil {
		opts.Settings = func() protocol.Settings { return protocol.Settings{} }
	}
	if opts.Approvals == nil {
		opts.Approvals = func() string { return opts.Session.Mode }
	}
	t, _ := OpenTranscript(opts.Session, opts.Resume)
	r := &runner{f: f, opts: opts, model: opts.Session.Model, pinned: opts.Session.Pinned, mode: opts.Mode, options: map[string]string{}, gate: newGate(opts.Approvals, isAsk(opts.Mode)), transcript: t, ceiling: ceilingsFrom(opts.Options), observerCtx: f.observerCtx}
	for k, v := range opts.Options {
		r.options[k] = v
	}
	if len(opts.Resume) > 0 {
		var state struct {
			Allowed    bool              `json:"allowed"`
			Ceiling    ceilings          `json:"ceiling"`
			Mode       tool.Mode         `json:"mode"`
			Options    map[string]string `json:"options"`
			Brief      string            `json:"brief"`
			BriefReady bool              `json:"briefReady"`
		}
		if json.Unmarshal(opts.Resume, &state) == nil {
			if state.Options != nil {
				r.options = make(map[string]string, len(state.Options))
				for k, v := range state.Options {
					r.options[k] = v
				}
				r.ceiling = ceilingsFrom(r.options)
			}
			r.gate.setAllowed(state.Allowed)
			r.ceiling.SpentWall = state.Ceiling.SpentWall
			r.ceiling.SpentTokens = state.Ceiling.SpentTokens
			r.brief, r.briefReady = state.Brief, state.BriefReady
			if state.Mode == tool.ModeAgent || state.Mode == tool.ModePlan {
				r.mode = state.Mode
				if state.Mode == tool.ModePlan {
					r.options[protocol.OptionPlan] = protocol.OptionOn
				} else {
					r.options[protocol.OptionPlan] = protocol.OptionOff
				}
			}
		}
	}
	if r.opts.Tools.Team == nil && !isAsk(r.mode) && (r.mode == tool.ModeAgent || r.mode == tool.ModePlan) {
		if r.options[protocol.OptionPlan] == protocol.OptionOn {
			r.mode = tool.ModePlan
		} else {
			r.mode = tool.ModeAgent
		}
	}
	return r
}

func (r *runner) currentModel() (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	model := r.model
	if model == "" {
		if isAsk(r.mode) {
			model = r.opts.Settings().Ask.Model
		} else {
			model = r.opts.Settings().Agent.Model
		}
	}
	if model == "" {
		model = protocol.RouteAuto
	}
	return model, r.pinned || (!protocol.IsRoute(model) && strings.Contains(model, "/"))
}

func (r *runner) parentCall() string { return r.parent }

// Slots keeps a locally served model to one child and remote models to four.
func (r *runner) Slots(model string) int {
	if model == "" {
		model, _ = r.currentModel()
	}
	if protocol.IsRoute(model) && r.opts.Settings().Providers.Models.RemoteFirst {
		return 4
	}
	if protocol.IsRoute(model) || strings.HasPrefix(model, protocol.ProviderRashinModels+"/") || strings.HasPrefix(model, protocol.ProviderLlamaCpp+"/") || strings.HasPrefix(model, protocol.ProviderOllamaLocal+"/") {
		return 1
	}
	return 4
}

type childSink struct {
	parent   core.Sink
	parentID string
	mu       sync.Mutex
	progress func(protocol.AgentWork)
	titles   map[string]string
	work     protocol.AgentWork
	started  time.Time
}

func (s *childSink) Update(func(*protocol.SessionInfo)) {}
func (s *childSink) Emit(kind string, data any) {
	if kind != protocol.EvTool {
		return
	}
	d, ok := data.(protocol.ToolData)
	if !ok {
		return
	}
	d.Parent = s.parentID
	s.parent.Emit(kind, d)
	s.mu.Lock()
	if d.Title != "" {
		if s.titles == nil {
			s.titles = make(map[string]string)
		}
		s.titles[d.Call] = d.Title
	}
	if d.Status != protocol.ToolDone && d.Status != protocol.ToolFailed {
		if d.Status == protocol.ToolRunning {
			s.work.Doing = s.titles[d.Call]
			current := s.work
			current.Steps = append([]protocol.AgentStep(nil), s.work.Steps...)
			s.mu.Unlock()
			if s.progress != nil {
				s.progress(current)
			}
		} else {
			s.mu.Unlock()
		}
		return
	}
	title := s.titles[d.Call]
	delete(s.titles, d.Call)
	s.work.StepCount++
	s.work.Doing = title
	s.work.Steps = append(s.work.Steps, protocol.AgentStep{Title: title, Failed: d.Status == protocol.ToolFailed})
	if len(s.work.Steps) > agents.MaxSteps {
		s.work.Steps = s.work.Steps[len(s.work.Steps)-agents.MaxSteps:]
	}
	s.work.Millis = time.Since(s.started).Milliseconds()
	current := s.work
	current.Steps = append([]protocol.AgentStep(nil), s.work.Steps...)
	s.mu.Unlock()
	if s.progress != nil {
		s.progress(current)
	}
}

// Spawn runs a child through the same engine while keeping its history separate.
func (r *runner) Spawn(ctx context.Context, req tool.SpawnRequest, progress func(protocol.AgentWork)) (tool.SpawnResult, error) {
	kind, ok := agents.Lookup(req.Name)
	if !ok {
		return tool.SpawnResult{}, errors.New("unknown subagent kind")
	}
	r.gate.mu.Lock()
	sink := r.gate.sink
	r.gate.mu.Unlock()
	if sink == nil {
		return tool.SpawnResult{}, errors.New("parent turn is not running")
	}
	model, _ := r.currentModel()
	if req.Model != "" {
		model = req.Model
	}
	compact := strings.HasPrefix(model, protocol.ProviderRashinModels+"/") || strings.HasPrefix(model, protocol.ProviderLlamaCpp+"/") || strings.HasPrefix(model, protocol.ProviderOllamaLocal+"/")
	if r.f.deps.Gateway != nil {
		if list, err := r.f.deps.Gateway.Models(ctx, protocol.ModelsParams{}); err == nil {
			for _, m := range list.Models {
				if m.ID == model {
					compact = tool.ClassForModel(m) == tool.ClassCompact
					break
				}
			}
		}
	}
	r.mu.Lock()
	info := r.opts.Session
	opts := r.opts
	r.mu.Unlock()
	seq := childSequence.Add(1)
	info.ID = info.ID + "#subagent-" + strconv.FormatUint(seq, 10)
	info.Temporary = true
	info.Model = model
	info.Kind = protocol.KindAgent
	opts.Session = info
	opts.Mode = tool.ModeSubagent
	opts.Resume = nil
	opts.Options = map[string]string{}
	child := &runner{f: r.f, opts: opts, model: model, pinned: !protocol.IsRoute(model), mode: tool.ModeSubagent, options: map[string]string{}, gate: r.gate, transcript: NewMemoryTranscript(), parent: req.Parent, readOnly: !req.Write || kind.ReadOnly, role: kind.Prompt, compactRole: kind.CompactPrompt, sharedContext: req.Context, observerCtx: r.observerCtx, budgetOwner: r}
	r.mu.Lock()
	if r.executor == nil && r.f.deps.Executor != nil {
		r.executor = r.f.deps.Executor(r.opts.Session, r.opts.Sandbox)
	}
	child.executor = r.executor
	r.mu.Unlock()
	start := time.Now()
	cs := &childSink{parent: sink, parentID: req.Parent, progress: progress, work: protocol.AgentWork{ID: req.ID, Name: kind.Name, Task: req.Task, Model: model, Status: protocol.ToolRunning}, started: start}
	if progress != nil {
		progress(cs.work)
	}
	var usage llm.Usage
	last := ""
	for attempt := 0; attempt <= agents.MaxYieldReminders; attempt++ {
		text := agents.Assignment(req)
		if attempt > 0 {
			text = agents.YieldReminder(attempt, agents.MaxYieldReminders, compact)
		}
		result, err := child.Prompt(ctx, core.PromptInput{Turn: attempt + 1, Text: text}, cs)
		if result.Usage != nil {
			usage.Input += result.Usage.Input
			usage.Output += result.Usage.Output
			usage.Estimated = usage.Estimated || result.Usage.Estimated
		}
		if err != nil {
			cs.work.Status = protocol.ToolFailed
			cs.work.Output = err.Error()
			cs.work.Millis = time.Since(start).Milliseconds()
			if progress != nil {
				progress(cs.work)
			}
			return tool.SpawnResult{Failed: true, Work: cs.work, Usage: usage}, err
		}
		if child.childOutput != "" || child.childFailed {
			cs.work.Status = protocol.ToolDone
			if child.childFailed {
				cs.work.Status = protocol.ToolFailed
			}
			cs.work.Output = child.childOutput
			cs.work.Tokens = usage.Input + usage.Output
			cs.work.Millis = time.Since(start).Milliseconds()
			if progress != nil {
				progress(cs.work)
			}
			return tool.SpawnResult{Output: child.childOutput, Failed: child.childFailed, Work: cs.work, Usage: usage}, nil
		}
		if result.Stop == protocol.StopCancelled {
			return tool.SpawnResult{Failed: true, Work: cs.work, Usage: usage}, ctx.Err()
		}
		messages := child.transcript.Messages()
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == llm.RoleAssistant {
				last = messages[i].Content
				break
			}
		}
	}
	failed := strings.TrimSpace(last) == ""
	if failed {
		last = i18n.T("The subagent ended without a result.")
	}
	last += agents.NoYieldWarning
	cs.work.Status = protocol.ToolDone
	if failed {
		cs.work.Status = protocol.ToolFailed
	}
	cs.work.Output = last
	cs.work.Tokens = usage.Input + usage.Output
	cs.work.Millis = time.Since(start).Milliseconds()
	if progress != nil {
		progress(cs.work)
	}
	return tool.SpawnResult{Output: last, Failed: failed, Work: cs.work, Usage: usage}, nil
}

var _ tool.Spawner = (*runner)(nil)
