package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/ask/command"
	"ryoku-rashin/internal/core"
	gitcore "ryoku-rashin/internal/git"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
	"ryoku-rashin/internal/sandbox"
	store "ryoku-rashin/internal/session"
)

type eventTarget interface {
	sendEvent(name string, data any) bool
}

type managedSession struct {
	mu           sync.Mutex
	log          *store.Log
	runner       core.Runner
	coalescer    *store.Coalescer
	attached     map[eventTarget]struct{}
	running      bool
	finishing    bool
	finishDone   chan struct{}
	stopping     bool
	starting     bool
	startDone    chan struct{}
	reserveDone  chan struct{}
	steering     bool
	cancel       context.CancelFunc
	turnDone     chan struct{}
	permissions  map[string]struct{}
	tools        map[string]string
	forkHandoff  *protocol.Handoff
	lastActivity time.Time
	// calls are the tool calls of the running turn; turnCalls counts the
	// last finished turn's, and goalIdle the goal turns in a row with none
	calls     map[string]struct{}
	turnCalls int
	goalIdle  int
	// goalTurn is set while a turn that started under an active goal runs
	goalTurn bool
	// asks are Rashin's own open questions, by request, waiting for an option
	asks map[string]chan string
}

// promptTurn is how a turn starts: reserved when the caller already holds
// the session, goal when Rashin sends it on toward the goal, say when the
// agent is sent other text than the transcript shows
type promptTurn struct {
	handoff  *protocol.Handoff
	reserved bool
	goal     bool
	say      string
}

type sessionManager struct {
	mu        sync.Mutex
	items     map[string]*managedSession
	asker     core.Asker
	harnesses core.Harnesses
	work      core.Work
	settings  func() protocol.Settings
	broadcast func(string, any)
	changed   func()
	warn      func(string, ...any)
	// brief is what an agent session is told ahead of its first prompt,
	// askBrief what a harness answering an Ask is, and prepare readies an
	// agent's folder; the daemon sets them, tests leave them nil
	brief    func(protocol.SessionInfo) string
	askBrief func(protocol.SessionInfo) string
	prepare  func(protocol.SessionInfo)
	// tools are the MCP servers an agent session is handed
	tools func(protocol.SessionInfo) []core.MCPServer
	// ended hears that a session was stopped or deleted, so what it held ends too
	ended func(id string)
	// teams runs team tasks as Agent sessions and hears how their turns and
	// goals go, and watched says a Rashin window is connected; the daemon
	// sets both, tests leave them nil
	teams   core.Teams
	watched func() bool
	// sandbox is the sandbox a team session's agent runs in, and askSandbox
	// the one a harness that answers an Ask runs in while it is read-only;
	// the daemon sets them, tests leave them nil
	sandbox    func(protocol.SessionInfo) (*sandbox.Spec, error)
	askSandbox func(protocol.SessionInfo) (*sandbox.Spec, error)
	// memory hears every finished turn and every deleted session; the
	// daemon sets it when memory is built in, tests leave it nil
	memory core.Memory
}

func newSessionManager(asker core.Asker, harnesses core.Harnesses, work core.Work, settings func() protocol.Settings, broadcast func(string, any), changed func(), warn func(string, ...any)) *sessionManager {
	return &sessionManager{items: make(map[string]*managedSession), asker: asker, harnesses: harnesses, work: work, settings: settings, broadcast: broadcast, changed: changed, warn: warn}
}

// Info is a session's info. A session the manager does not hold is read
// from disk without being opened, so asking about many sessions holds none
// of them open.
func (m *sessionManager) Info(id string) (protocol.SessionInfo, bool) {
	m.mu.Lock()
	s := m.items[id]
	m.mu.Unlock()
	if s != nil {
		return s.log.Info(), true
	}
	info, err := store.ReadInfo(id)
	return info, err == nil
}

// List is every stored session of kind. A temporary session is never
// stored, so it is never here.
func (m *sessionManager) List(kind protocol.SessionKind) []protocol.SessionInfo {
	list, err := store.List(kind)
	if err != nil {
		return []protocol.SessionInfo{}
	}
	return list
}

func (m *sessionManager) New(ctx context.Context, p protocol.SessionNewParams) (protocol.SessionInfo, error) {
	return m.newSession(ctx, p)
}

func (m *sessionManager) Prompt(ctx context.Context, p protocol.PromptParams) (protocol.PromptResult, error) {
	result, _, err := m.prompt(ctx, p, nil)
	return result, err
}

func (m *sessionManager) newSession(ctx context.Context, p protocol.SessionNewParams) (protocol.SessionInfo, error) {
	if p.Kind == protocol.KindAgent && m.work != nil && (p.Project != "" || p.Worktree != "") {
		prepared, err := m.work.PrepareSession(ctx, p)
		if err != nil {
			return protocol.SessionInfo{}, err
		}
		p = prepared
	}
	return m.create(p)
}

func (m *sessionManager) get(id string) (*managedSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.items[id]; s != nil {
		return s, nil
	}
	log, err := store.Open(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No session %s.", id))
		}
		return nil, err
	}
	s := &managedSession{log: log, attached: make(map[eventTarget]struct{}), permissions: make(map[string]struct{}), lastActivity: log.Info().Updated}
	s.coalescer = store.NewCoalescer(func(kind string, data any) error { return m.persist(s, kind, data) })
	m.restoreForkHandoff(s)
	m.items[id] = s
	return s, nil
}

func (m *sessionManager) restoreForkHandoff(s *managedSession) {
	info := s.log.Info()
	if info.Kind != protocol.KindAgent || info.Parent == "" {
		return
	}
	events, err := s.log.Since(0)
	if err != nil {
		return
	}
	for _, event := range events {
		if event.Kind != protocol.EvUser {
			continue
		}
		var user protocol.UserData
		if json.Unmarshal(event.Data, &user) == nil && user.Handoff != nil && user.Handoff.From == info.Parent {
			return
		}
	}
	s.forkHandoff = &protocol.Handoff{From: info.Parent, Title: info.Title, Question: "Forked session context", Answer: forkTranscript(events)}
}

func (m *sessionManager) create(p protocol.SessionNewParams) (protocol.SessionInfo, error) {
	if p.Kind != protocol.KindAsk && p.Kind != protocol.KindAgent {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Session kind must be ask or agent."))
	}
	// a team task's conversation is its audit trail, so it is always kept
	if p.Temporary && (p.Team != "" || p.TeamTask != "" || p.Member != "") {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A team task's conversation can't be temporary."))
	}
	memory, err := startingMemory(p.Memory, p.Temporary)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	if p.Kind == protocol.KindAgent {
		if p.Cwd == "" {
			p.Cwd = paths.Home()
		}
		if !filepath.IsAbs(p.Cwd) {
			return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("An Agent session directory must be an absolute path."))
		}
	}
	now := time.Now()
	id, err := store.NewID(p.Kind, now)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	origin := p.Origin
	if origin == "" {
		origin = protocol.OriginApp
	}
	state := protocol.StateIdle
	if p.Kind == protocol.KindAgent {
		state = protocol.StateStopped
	}
	info := protocol.SessionInfo{ID: id, Kind: p.Kind, Title: p.Title, Harness: strings.TrimSpace(p.Harness), Model: p.Model, Cwd: p.Cwd, Origin: origin, State: state, Created: now, Updated: now, Project: p.Project, Worktree: p.Worktree, Branch: p.Branch, Task: p.Task, Temporary: p.Temporary, Memory: memory}
	// an Ask a harness answers runs it from the home folder and starts on the
	// harness's own model, which Rashin's routes do not name
	if info.Kind == protocol.KindAsk && info.Harness != "" {
		info.Cwd = paths.Home()
	} else if info.Model == "" && info.Kind == protocol.KindAsk {
		info.Model = m.settings().Ask.Model
	}
	// only the client says an Ask may change the machine: the bar and the
	// terminal never do, so theirs stay read-only whatever the default is
	if info.Kind == protocol.KindAsk && protocol.KnownApprovals(p.Mode) {
		info.Mode = p.Mode
	}
	// a team task's session works under its team's approvals, not the agent default
	if info.Kind == protocol.KindAgent {
		info.Team, info.TeamTask, info.Member = p.Team, p.TeamTask, p.Member
		if protocol.KnownApprovals(p.Approvals) {
			info.Approvals = p.Approvals
		}
	}
	log, err := store.Create(info, nil)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	s := &managedSession{log: log, attached: make(map[eventTarget]struct{}), permissions: make(map[string]struct{}), lastActivity: now}
	s.coalescer = store.NewCoalescer(func(kind string, data any) error { return m.persist(s, kind, data) })
	m.mu.Lock()
	m.items[id] = s
	m.mu.Unlock()
	m.broadcast(protocol.EvtSessionInfo, info)
	m.sessionChanged(info)
	m.changed()
	return info, nil
}

func (m *sessionManager) persist(s *managedSession, kind string, data any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.log.Append(kind, data)
	if err != nil {
		return err
	}
	// the log keeps no secret: the user's prompt and a memory line are
	// scrubbed as they land and the whole turn, its streamed text merged,
	// once it ends
	scrub := s.log.ScrubLast
	if kind == protocol.EvTurnEnd {
		scrub = s.log.ScrubTurn
	}
	if kind == protocol.EvUser || kind == protocol.EvMemory || kind == protocol.EvTurnEnd {
		if _, err := scrub(redact.Exact); err != nil {
			m.warn("scrub session %s: %v", s.log.Info().ID, err)
		}
	}
	infoChanged := false
	switch kind {
	case protocol.EvText:
		var text protocol.TextData
		if decode(data, &text) {
			info := s.log.Info()
			preview := info.Preview + text.Delta
			preview = lastRunes(preview, 140)
			_ = s.log.Update(func(i *protocol.SessionInfo) { i.Preview = preview }, nil)
			infoChanged = true
		}
	case protocol.EvTool:
		var tool protocol.ToolData
		if decode(data, &tool) {
			if s.tools == nil {
				s.tools = make(map[string]string)
			}
			if s.calls == nil {
				s.calls = make(map[string]struct{})
			}
			s.calls[tool.Call] = struct{}{}
			if tool.Title != "" {
				s.tools[tool.Call] = tool.Title
			}
			title := tool.Title
			if title == "" {
				title = s.tools[tool.Call]
			}
			steps := len(s.calls)
			if title != "" || steps != s.log.Info().Steps {
				_ = s.log.Update(func(i *protocol.SessionInfo) {
					if title != "" {
						i.Activity = toolStatusWord(tool.Status) + ": " + title
					}
					i.Steps = steps
				}, nil)
				infoChanged = true
			}
		}
	case protocol.EvPermission:
		var p protocol.PermissionData
		if decode(data, &p) && p.Request != "" {
			s.permissions[p.Request] = struct{}{}
			_ = s.log.Update(func(i *protocol.SessionInfo) { i.Pending = len(s.permissions); i.State = protocol.StateWaiting }, nil)
			infoChanged = true
		}
	case protocol.EvPermissionResolved:
		var p protocol.PermissionResolvedData
		if decode(data, &p) {
			delete(s.permissions, p.Request)
			_ = s.log.Update(func(i *protocol.SessionInfo) {
				i.Pending = len(s.permissions)
				if s.running && i.Pending == 0 {
					i.State = protocol.StateWorking
				}
			}, nil)
			infoChanged = true
		}
	case protocol.EvTitle:
		var p protocol.TitleData
		if decode(data, &p) && p.Title != "" {
			_ = s.log.Update(func(i *protocol.SessionInfo) { i.Title = p.Title }, nil)
			infoChanged = true
		}
	case protocol.EvTurnEnd:
		s.tools = nil
		s.turnCalls, s.calls = len(s.calls), nil
		// the steps stay, so a finished turn still says how much it did
		_ = s.log.Update(func(i *protocol.SessionInfo) { i.Unread = true; i.Activity = ""; i.TurnStarted = nil }, nil)
		infoChanged = true
	}
	payload := protocol.SessionEventData{Session: s.log.Info().ID, SessionEvent: ev}
	for target := range s.attached {
		target.sendEvent(protocol.EvtSessionEvent, payload)
	}
	if infoChanged {
		m.emitInfoLocked(s)
	}
	m.changed()
	return nil
}

func decode(v any, out any) bool {
	b, err := json.Marshal(v)
	return err == nil && json.Unmarshal(b, out) == nil
}

func runnerState(r core.Runner) json.RawMessage {
	if r == nil {
		return nil
	}
	return r.State()
}

// lastRunes keeps the last n runes of s. When that cuts a word in two, the
// tail starts at the next word instead, so a preview never opens on a
// fragment such as "'ve asked" for "I've asked".
func lastRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	tail := r[len(r)-n:]
	if unicode.IsSpace(r[len(r)-n-1]) {
		return string(tail)
	}
	for index, letter := range tail {
		if unicode.IsSpace(letter) {
			return strings.TrimLeftFunc(string(tail[index:]), unicode.IsSpace)
		}
	}
	return string(tail)
}

func toolStatusWord(status string) string {
	switch status {
	case protocol.ToolPending:
		return i18n.T("Pending")
	case protocol.ToolDone:
		return i18n.T("Done")
	case protocol.ToolFailed:
		return i18n.T("Failed")
	default:
		return i18n.T("Running")
	}
}

func (m *sessionManager) emitInfoLocked(s *managedSession) {
	info := s.log.Info()
	m.broadcast(protocol.EvtSessionInfo, info)
	for target := range s.attached {
		target.sendEvent(protocol.EvtSessionInfo, info)
	}
	m.sessionChanged(info)
}

// sessionChanged tells Work about a project's session and Teams about a team
// task's session.
func (m *sessionManager) sessionChanged(info protocol.SessionInfo) {
	if m.work != nil && info.Project != "" {
		m.work.SessionChanged(info)
	}
	if m.teams != nil && info.TeamTask != "" {
		m.teams.SessionChanged(info)
	}
}

func (m *sessionManager) attach(id string, since int64, target eventTarget) (protocol.SessionAttachResult, error) {
	s, err := m.get(id)
	if err != nil {
		return protocol.SessionAttachResult{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := attachPage(s, since, 0)
	if err != nil {
		return protocol.SessionAttachResult{}, err
	}
	if !result.More {
		s.attached[target] = struct{}{}
	}
	return result, nil
}

func (m *sessionManager) attachQueued(id string, since int64, limit int, target eventTarget, respond func(protocol.SessionAttachResult, error) bool) {
	s, err := m.get(id)
	if err != nil {
		respond(protocol.SessionAttachResult{}, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := attachPage(s, since, limit)
	if !respond(result, err) || err != nil {
		return
	}
	if !result.More {
		s.attached[target] = struct{}{}
	}
}

func attachPage(s *managedSession, since int64, limit int) (protocol.SessionAttachResult, error) {
	events, err := s.log.Since(since)
	if err != nil {
		return protocol.SessionAttachResult{}, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	result := protocol.SessionAttachResult{Info: s.log.Info()}
	for _, event := range events {
		if len(result.Events) >= limit {
			result.More = true
			break
		}
		candidate := append(result.Events, event)
		probe := protocol.SessionAttachResult{Info: result.Info, Events: candidate}
		encoded, marshalErr := json.Marshal(probe)
		if marshalErr != nil {
			return protocol.SessionAttachResult{}, marshalErr
		}
		if len(encoded) > protocol.MaxLine-1024 && len(result.Events) > 0 {
			result.More = true
			break
		}
		result.Events = candidate
	}
	if len(result.Events) < len(events) {
		result.More = true
	}
	return result, nil
}

func (m *sessionManager) detach(id string, target eventTarget) {
	s, err := m.get(id)
	if err != nil {
		return
	}
	s.mu.Lock()
	delete(s.attached, target)
	s.mu.Unlock()
	m.changed()
}

func (m *sessionManager) detachAll(target eventTarget) {
	m.mu.Lock()
	items := make([]*managedSession, 0, len(m.items))
	for _, s := range m.items {
		items = append(items, s)
	}
	m.mu.Unlock()
	for _, s := range items {
		s.mu.Lock()
		delete(s.attached, target)
		s.mu.Unlock()
	}
}

func (m *sessionManager) ensureRunner(ctx context.Context, s *managedSession) (core.Runner, error) {
	for {
		s.mu.Lock()
		if s.stopping {
			s.mu.Unlock()
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The session is stopping."))
		}
		if s.runner != nil {
			runner := s.runner
			s.mu.Unlock()
			return runner, nil
		}
		if s.starting {
			done := s.startDone
			s.mu.Unlock()
			select {
			case <-done:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		s.starting = true
		s.startDone = make(chan struct{})
		info := s.log.Info()
		resume := s.log.Runner()
		harness := info.Harness
		if harness == "" && info.Kind == protocol.KindAgent {
			harness = m.settings().Agent.Harness
		}
		s.mu.Unlock()

		var runner core.Runner
		var err error
		switch {
		case info.Kind == protocol.KindAsk && harness == "":
			runner = m.asker.NewAsk(core.AskOptions{Session: info, Settings: m.settings})
		case info.Kind == protocol.KindAsk:
			runner, err = m.askHarness(ctx, s, info, resume)
		default:
			// a team task's session keeps its team's approvals; others follow the setting
			approvals := func() string {
				if own := s.log.Info().Approvals; own != "" {
					return own
				}
				return m.settings().Agent.Approvals
			}
			options := core.HarnessOptions{Session: info, Harness: harness, Cwd: info.Cwd, Approvals: approvals, Resume: resume}
			if m.brief != nil {
				options.Brief = func() string { return m.brief(s.log.Info()) }
			}
			if m.tools != nil {
				options.Tools = m.tools(info)
			}
			if info.Team != "" && m.sandbox != nil {
				options.Sandbox, err = m.sandbox(info)
			}
			if err == nil {
				runner, err = m.harnesses.NewAgent(ctx, options)
			}
			if err == nil && m.prepare != nil {
				go m.prepare(info)
			}
		}
		var alive bool
		var state json.RawMessage
		if err == nil && runner != nil {
			alive = runner.Alive()
			state = runner.State()
		}
		s.mu.Lock()
		s.starting = false
		close(s.startDone)
		s.startDone = nil
		if err != nil {
			s.mu.Unlock()
			return nil, err
		}
		if s.stopping {
			s.mu.Unlock()
			if runner != nil {
				_ = runner.Stop()
			}
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The session is stopping."))
		}
		s.runner = runner
		s.lastActivity = time.Now()
		if info.Kind == protocol.KindAgent {
			_ = s.log.Update(func(i *protocol.SessionInfo) { i.Harness = harness; i.Alive = alive; i.State = protocol.StateIdle }, state)
			m.emitInfoLocked(s)
		}
		s.mu.Unlock()
		return runner, nil
	}
}

func (m *sessionManager) prompt(ctx context.Context, p protocol.PromptParams, handoff *protocol.Handoff) (protocol.PromptResult, int64, error) {
	if handled, err := m.goalCommand(ctx, p.ID, p.Text); handled {
		return protocol.PromptResult{}, 0, err
	}
	s, err := m.get(p.ID)
	if err != nil {
		return protocol.PromptResult{}, 0, err
	}
	if p.Run = strings.TrimSpace(p.Run); p.Run != "" {
		// refused before the turn exists, so the client can still send the
		// command to a terminal
		if info := s.log.Info(); info.Kind != protocol.KindAsk || info.Harness != "" {
			return protocol.PromptResult{}, 0, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.T("Only an Ask conversation on Rashin's providers runs a command for you. Run it in a terminal."))
		}
		if reason, needs := command.NeedsTerminal(p.Run); needs {
			return protocol.PromptResult{}, 0, protocol.Errorf(protocol.CodeUnsupported, "%s", i18n.Tf("This command needs a terminal: %s.", reason))
		}
		p.Text, p.Images = "", nil
	}
	for {
		s.mu.Lock()
		if !s.finishing {
			s.mu.Unlock()
			break
		}
		done := s.finishDone
		s.mu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
			return protocol.PromptResult{}, 0, ctx.Err()
		}
	}
	return m.startPrompt(ctx, s, p, promptTurn{handoff: handoff})
}

func (m *sessionManager) startPrompt(ctx context.Context, s *managedSession, p protocol.PromptParams, turnOptions promptTurn) (protocol.PromptResult, int64, error) {
	handoff, reserved := turnOptions.handoff, turnOptions.reserved
	if reserved {
		defer func() {
			s.mu.Lock()
			if s.reserveDone != nil {
				close(s.reserveDone)
				s.reserveDone = nil
			}
			s.mu.Unlock()
		}()
	}
	runner, err := m.ensureRunner(ctx, s)
	if err != nil {
		return protocol.PromptResult{}, 0, err
	}
	s.mu.Lock()
	if s.stopping {
		if reserved {
			s.running = false
		}
		s.mu.Unlock()
		return protocol.PromptResult{}, 0, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The session is stopping."))
	}
	if !reserved {
		if s.running {
			s.mu.Unlock()
			return protocol.PromptResult{}, 0, protocol.Errorf(protocol.CodeConflict, "%s", i18n.Tf("A turn is already running in session %s.", p.ID))
		}
		s.running = true
	}
	turnCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.cancel = cancel
	s.turnDone = done
	info := s.log.Info()
	history, err := s.log.Since(0)
	if err != nil {
		s.running = false
		s.cancel = nil
		close(done)
		if s.turnDone == done {
			s.turnDone = nil
		}
		s.mu.Unlock()
		return protocol.PromptResult{}, 0, err
	}
	if handoff == nil && s.forkHandoff != nil {
		handoff = s.forkHandoff
		s.forkHandoff = nil
	}
	turn := info.Turns + 1
	s.goalTurn = info.Goal != nil && info.Goal.Status == protocol.GoalActive
	s.mu.Unlock()
	// a temporary conversation leaves nothing behind, checkpoint refs in the
	// user's repository included
	if info.Kind == protocol.KindAgent && m.work != nil && !info.Temporary {
		checkpointed, checkpointErr := m.work.BeforeTurn(ctx, info, turn)
		if checkpointErr != nil {
			if m.warn != nil {
				m.warn("checkpoint session %s turn %d: %v", info.ID, turn, checkpointErr)
			}
		} else if checkpointed {
			state := runner.State()
			s.mu.Lock()
			_ = s.log.Update(func(i *protocol.SessionInfo) { i.Checkpoints = true }, state)
			m.emitInfoLocked(s)
			s.mu.Unlock()
		}
	}

	before := info.LastSeq
	if err := s.coalescer.Emit(protocol.EvUser, protocol.UserData{Text: p.Text, Images: p.Images, Terminal: p.Terminal, Handoff: handoff, Run: p.Run, Goal: turnOptions.goal}); err != nil {
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		close(done)
		if s.turnDone == done {
			s.turnDone = nil
		}
		s.mu.Unlock()
		return protocol.PromptResult{}, before, err
	}
	alive := runner.Alive()
	state := runner.State()
	started := time.Now()
	s.mu.Lock()
	s.calls = nil
	_ = s.log.Update(func(i *protocol.SessionInfo) {
		i.Turns = turn
		i.State = protocol.StateWorking
		i.Error = ""
		i.TurnStarted = &started
		i.Steps = 0
		// A harness may send a better title later; until it does, a session is
		// named after what it was first asked, or every row reads the same. A
		// prompt that is only an attachment is named after that.
		if i.Title == "" {
			i.Title = questionTitle(p.Text)
		}
		if i.Title == "" && len(p.Images) > 0 {
			i.Title = questionTitle(filepath.Base(p.Images[0].Path))
		}
		i.Alive = alive
	}, state)
	s.lastActivity = started
	m.emitInfoLocked(s)
	s.mu.Unlock()
	if err := s.coalescer.Emit(protocol.EvTurnStart, protocol.TurnStartData{Turn: turn}); err != nil {
		s.mu.Lock()
		_ = s.log.Update(func(i *protocol.SessionInfo) { i.TurnStarted = nil }, nil)
		s.running = false
		s.cancel = nil
		close(done)
		if s.turnDone == done {
			s.turnDone = nil
		}
		s.mu.Unlock()
		return protocol.PromptResult{}, before, err
	}
	input := core.PromptInput{Turn: turn, Text: p.Text, Images: p.Images, Terminal: p.Terminal, Handoff: handoff, Origin: info.Origin, Run: p.Run, History: history}
	if turnOptions.say != "" {
		input.Text = turnOptions.say
	}
	go m.runTurn(turnCtx, s, runner, input, done)
	m.changed()
	return protocol.PromptResult{Turn: turn}, before, nil
}

// questionTitle names a session after its first question: every line joined,
// clipped at a word as a task's title is.
func questionTitle(question string) string {
	return protocol.ClipWords(strings.Join(strings.Fields(question), " "), 60)
}

func (m *sessionManager) runTurn(ctx context.Context, s *managedSession, runner core.Runner, input core.PromptInput, done chan struct{}) {
	result, err := runner.Prompt(ctx, input, &runnerSink{manager: m, session: s})
	stop := result.Stop
	if stop == "" {
		stop = protocol.StopEnd
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			stop = protocol.StopCancelled
		} else {
			stop = protocol.StopError
			_ = s.coalescer.Emit(protocol.EvError, protocol.ErrorData{Message: err.Error(), Code: protocol.CodeInternal})
		}
	} else if ctx.Err() != nil {
		stop = protocol.StopCancelled
	}
	_ = s.coalescer.Flush()
	s.mu.Lock()
	s.cancel = nil
	s.lastActivity = time.Now()
	requests := make([]string, 0, len(s.permissions))
	for request := range s.permissions {
		requests = append(requests, request)
	}
	s.permissions = make(map[string]struct{})
	s.mu.Unlock()
	for _, request := range requests {
		_ = s.coalescer.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: request, Outcome: "cancelled"})
	}
	state := protocol.StateIdle
	if stop == protocol.StopError {
		state = protocol.StateFailed
	}
	stateData := runner.State()
	alive := runner.Alive()
	s.mu.Lock()
	_ = s.log.Update(func(i *protocol.SessionInfo) {
		i.State = state
		i.Pending = 0
		i.Alive = alive
		if stop == protocol.StopError {
			i.Error = i18n.T("The turn failed.")
		}
	}, stateData)
	s.running = false
	s.finishing = true
	s.finishDone = make(chan struct{})
	info := s.log.Info()
	s.mu.Unlock()
	_ = s.coalescer.Emit(protocol.EvTurnEnd, protocol.TurnEndData{Turn: input.Turn, Stop: stop, Usage: result.Usage})
	s.mu.Lock()
	s.finishing = false
	close(s.finishDone)
	s.finishDone = nil
	m.emitInfoLocked(s)
	ended := s.log.Info()
	s.mu.Unlock()

	if m.work != nil && info.Kind == protocol.KindAgent {
		m.work.AfterTurn(context.Background(), info, input.Turn)
	}
	if m.teams != nil && info.TeamTask != "" {
		m.teams.AfterTurn(context.Background(), info, protocol.TurnEndData{Turn: input.Turn, Stop: stop, Usage: result.Usage})
	}
	// a temporary conversation is never learned from, so memory never hears of it
	if m.memory != nil && !ended.Temporary {
		m.memory.TurnEnded(ended)
	}
	s.mu.Lock()
	calls := s.turnCalls
	s.mu.Unlock()
	onward := m.goalAfterTurn(s, stop, calls)

	var queued *protocol.QueuedPrompt
	var goal *protocol.Goal
	s.mu.Lock()
	if !s.steering && !s.stopping && !s.running {
		current := s.log.Info()
		if len(current.Queue) > 0 {
			next := current.Queue[0]
			queued = &next
			s.running = true
			s.reserveDone = make(chan struct{})
			_ = s.log.Update(func(i *protocol.SessionInfo) {
				i.Queue = append([]protocol.QueuedPrompt(nil), i.Queue[1:]...)
			}, nil)
		} else if onward && current.Goal != nil && current.Goal.Status == protocol.GoalActive && !m.held(current) {
			next := *current.Goal
			goal = &next
			s.running = true
			s.reserveDone = make(chan struct{})
		}
	}
	close(done)
	if s.turnDone == done {
		s.turnDone = nil
	}
	info = s.log.Info()
	s.mu.Unlock()
	if queued != nil {
		if _, _, promptErr := m.startPrompt(context.Background(), s, protocol.PromptParams{ID: info.ID, Text: queued.Text, Images: queued.Images}, promptTurn{reserved: true}); promptErr != nil && m.warn != nil {
			m.warn("queued prompt session %s: %v", info.ID, promptErr)
		}
	}
	if goal != nil {
		shown, say := goalTurn(*goal)
		if _, _, promptErr := m.startPrompt(context.Background(), s, protocol.PromptParams{ID: info.ID, Text: shown}, promptTurn{reserved: true, goal: goal.Turns > 0, say: say}); promptErr != nil && m.warn != nil {
			m.warn("goal turn session %s: %v", info.ID, promptErr)
		}
	}
	m.changed()
}

type runnerSink struct {
	manager *sessionManager
	session *managedSession
}

func (s *runnerSink) Emit(kind string, data any) { _ = s.session.coalescer.Emit(kind, data) }
func (s *runnerSink) Update(fn func(*protocol.SessionInfo)) {
	s.session.mu.Lock()
	runner := s.session.runner
	s.session.mu.Unlock()
	state := runnerState(runner)
	s.session.mu.Lock()
	_ = s.session.log.Update(fn, state)
	s.manager.emitInfoLocked(s.session)
	s.session.mu.Unlock()
	s.manager.changed()
}

func (m *sessionManager) cancel(id string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	cancel := s.cancel
	runner := s.runner
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if runner != nil {
		runner.Cancel()
	}
	return nil
}

func (m *sessionManager) queue(ctx context.Context, p protocol.QueueParams) (protocol.SessionInfo, error) {
	if handled, err := m.goalCommand(ctx, p.ID, p.Text); handled {
		if err != nil {
			return protocol.SessionInfo{}, err
		}
		info, _ := m.Info(p.ID)
		return info, nil
	}
	s, err := m.get(p.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		if _, _, err := m.prompt(ctx, protocol.PromptParams{ID: p.ID, Text: p.Text, Images: p.Images}, nil); err != nil {
			return protocol.SessionInfo{}, err
		}
		return s.log.Info(), nil
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		s.mu.Unlock()
		return protocol.SessionInfo{}, err
	}
	queued := protocol.QueuedPrompt{ID: "queue-" + hex.EncodeToString(random[:]), Text: p.Text, Images: p.Images}
	if err := s.log.Update(func(i *protocol.SessionInfo) { i.Queue = append(i.Queue, queued) }, nil); err != nil {
		s.mu.Unlock()
		return protocol.SessionInfo{}, err
	}
	m.emitInfoLocked(s)
	info := s.log.Info()
	s.mu.Unlock()
	m.changed()
	return info, nil
}

func (m *sessionManager) unqueue(p protocol.UnqueueParams) (protocol.SessionInfo, error) {
	s, err := m.get(p.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	info := s.log.Info()
	index := -1
	for i := range info.Queue {
		if info.Queue[i].ID == p.Queue {
			index = i
			break
		}
	}
	if index < 0 {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No queued prompt %s.", p.Queue))
	}
	if err := s.log.Update(func(i *protocol.SessionInfo) {
		i.Queue = append(i.Queue[:index:index], i.Queue[index+1:]...)
	}, nil); err != nil {
		return protocol.SessionInfo{}, err
	}
	m.emitInfoLocked(s)
	m.changed()
	return s.log.Info(), nil
}

func (m *sessionManager) steer(ctx context.Context, p protocol.PromptParams) (protocol.PromptResult, error) {
	s, err := m.get(p.ID)
	if err != nil {
		return protocol.PromptResult{}, err
	}
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		result, _, err := m.prompt(ctx, p, nil)
		return result, err
	}
	s.steering = true
	done := s.turnDone
	cancel := s.cancel
	runner := s.runner
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if runner != nil {
		runner.Cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			s.mu.Lock()
			s.steering = false
			s.mu.Unlock()
			return protocol.PromptResult{}, ctx.Err()
		}
	}
	s.mu.Lock()
	s.steering = false
	s.mu.Unlock()
	result, _, err := m.prompt(ctx, p, nil)
	return result, err
}

func (m *sessionManager) read(id string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.log.Update(func(i *protocol.SessionInfo) { i.Unread = false }, nil); err != nil {
		return err
	}
	m.emitInfoLocked(s)
	m.changed()
	return nil
}

func (m *sessionManager) revertTurn(ctx context.Context, p protocol.TurnParams) (protocol.GitOpResult, error) {
	if m.work == nil {
		return protocol.GitOpResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Work is not available."))
	}
	s, err := m.get(p.ID)
	if err != nil {
		return protocol.GitOpResult{}, err
	}
	info := s.log.Info()
	if info.Temporary {
		return protocol.GitOpResult{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Turn revert is not available in a temporary conversation."))
	}
	if info.Worktree != "" && m.worktreeRunning(info.Worktree) {
		return protocol.GitOpResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("A session is still running in this worktree."))
	}
	return m.work.RevertTurn(ctx, info, p.Turn)
}

// checkpointRefs is where Work keeps each session's turn checkpoints, one
// namespace per session id.
const checkpointRefs = "refs/rashin/checkpoints/"

// sweepCheckpoints deletes, in every project Work knows, the checkpoint refs
// of sessions that are not stored: a deleted session whose refs a failure
// kept, or one that was never stored at all. A session the manager holds
// keeps its refs.
func (m *sessionManager) sweepCheckpoints(ctx context.Context) {
	if m.work == nil {
		return
	}
	for _, project := range m.work.Projects(ctx) {
		out, err := gitcore.Run(ctx, project.Root, "for-each-ref", "--format=%(refname)", checkpointRefs)
		if err != nil {
			continue
		}
		swept := map[string]bool{}
		for _, ref := range strings.Split(string(out), "\n") {
			id, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(ref), checkpointRefs), "/")
			if id == "" || swept[id] || m.holds(id) {
				continue
			}
			if _, err := store.ReadInfo(id); !errors.Is(err, os.ErrNotExist) {
				continue
			}
			swept[id] = true
			if err := gitcore.DeleteRefs(ctx, project.Root, checkpointRefs+id+"/"); err != nil && m.warn != nil {
				m.warn("sweep checkpoints of %s in %s: %v", id, project.Root, err)
			}
		}
	}
}

// holds reports whether the manager holds session id.
func (m *sessionManager) holds(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.items[id] != nil
}

func (m *sessionManager) worktreeRunning(worktree string) bool {
	m.mu.Lock()
	items := make([]*managedSession, 0, len(m.items))
	for _, item := range m.items {
		items = append(items, item)
	}
	m.mu.Unlock()
	for _, item := range items {
		item.mu.Lock()
		running := item.running && item.log.Info().Worktree == worktree
		item.mu.Unlock()
		if running {
			return true
		}
	}
	return false
}

func (m *sessionManager) fork(ctx context.Context, p protocol.ForkParams) (protocol.SessionInfo, error) {
	source, err := m.get(p.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	sourceInfo := source.log.Info()
	if p.Seq < 0 || p.Seq > sourceInfo.LastSeq {
		return protocol.SessionInfo{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Fork sequence %d is outside the session.", p.Seq))
	}
	events, err := source.log.Since(0)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	copied := events[:0]
	openTurn := -1
	nextUser := -1
	for _, event := range events {
		if event.Seq > p.Seq {
			break
		}
		switch event.Kind {
		case protocol.EvUser:
			nextUser = len(copied)
		case protocol.EvTurnStart:
			openTurn = len(copied)
			if nextUser >= 0 {
				openTurn = nextUser
			}
		case protocol.EvTurnEnd:
			openTurn = -1
			nextUser = -1
		}
		copied = append(copied, event)
	}
	if openTurn >= 0 {
		copied = copied[:openTurn]
	}
	info, err := m.newSession(ctx, protocol.SessionNewParams{
		Kind: sourceInfo.Kind, Harness: sourceInfo.Harness, Cwd: sourceInfo.Cwd,
		Model: sourceInfo.Model, Title: sourceInfo.Title, Origin: sourceInfo.Origin,
		Project: sourceInfo.Project, Worktree: sourceInfo.Worktree, Branch: sourceInfo.Branch, Task: sourceInfo.Task,
		Memory: sourceInfo.Memory,
	})
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	target, err := m.get(info.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	for _, event := range copied {
		if err := target.log.AppendCopy(event); err != nil {
			return protocol.SessionInfo{}, err
		}
	}
	turns := 0
	preview := ""
	for _, event := range copied {
		switch event.Kind {
		case protocol.EvTurnStart:
			var data protocol.TurnStartData
			if json.Unmarshal(event.Data, &data) == nil && data.Turn > turns {
				turns = data.Turn
			}
		case protocol.EvText:
			var data protocol.TextData
			if json.Unmarshal(event.Data, &data) == nil {
				preview = lastRunes(preview+data.Delta, 140)
			}
		}
	}
	target.mu.Lock()
	state := protocol.StateIdle
	if sourceInfo.Kind == protocol.KindAgent {
		state = protocol.StateStopped
	}
	if err := target.log.Update(func(i *protocol.SessionInfo) {
		i.Parent = sourceInfo.ID
		i.Pinned = sourceInfo.Pinned
		i.Turns = turns
		i.Preview = preview
		i.State = state
		i.Alive = false
		i.Pending = 0
		i.Unread = false
		i.Activity = ""
		i.Queue = nil
	}, nil); err != nil {
		target.mu.Unlock()
		return protocol.SessionInfo{}, err
	}
	if sourceInfo.Kind == protocol.KindAgent {
		target.forkHandoff = &protocol.Handoff{From: sourceInfo.ID, Title: sourceInfo.Title, Question: "Forked session context", Answer: forkTranscript(copied)}
	}
	m.emitInfoLocked(target)
	info = target.log.Info()
	target.mu.Unlock()
	m.changed()
	return info, nil
}

func forkTranscript(events []protocol.SessionEvent) string {
	var transcript strings.Builder
	for _, event := range events {
		switch event.Kind {
		case protocol.EvUser:
			var data protocol.UserData
			if json.Unmarshal(event.Data, &data) == nil && data.Text != "" {
				fmt.Fprintf(&transcript, "User: %s\n", data.Text)
			}
		case protocol.EvText:
			var data protocol.TextData
			if json.Unmarshal(event.Data, &data) == nil {
				transcript.WriteString(data.Delta)
			}
		case protocol.EvTurnEnd:
			if transcript.Len() > 0 {
				transcript.WriteByte('\n')
			}
		}
	}
	return strings.TrimSpace(transcript.String())
}

func (m *sessionManager) permission(p protocol.PermissionParams) error {
	s, err := m.get(p.ID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if answer, ok := s.asks[p.Request]; ok {
		delete(s.asks, p.Request)
		s.mu.Unlock()
		answer <- p.Option
		return nil
	}
	if _, ok := s.permissions[p.Request]; !ok {
		s.mu.Unlock()
		return protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No open permission %s.", p.Request))
	}
	runner := s.runner
	s.mu.Unlock()
	if runner == nil {
		return protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("The session runner is not available."))
	}
	return runner.Permission(p.Request, p.Option)
}

func (m *sessionManager) setModel(ctx context.Context, p protocol.SetModelParams) error {
	s, err := m.get(p.ID)
	if err != nil {
		return err
	}
	r, err := m.ensureRunner(ctx, s)
	if err != nil {
		return err
	}
	if err := r.SetModel(ctx, p.Model, &runnerSink{manager: m, session: s}); err != nil {
		return err
	}
	state := r.State()
	s.mu.Lock()
	s.lastActivity = time.Now()
	_ = s.log.Update(func(i *protocol.SessionInfo) { i.Model = p.Model }, state)
	m.emitInfoLocked(s)
	s.mu.Unlock()
	m.changed()
	return nil
}

func (m *sessionManager) setMode(ctx context.Context, p protocol.SetModeParams) error {
	s, err := m.get(p.ID)
	if err != nil {
		return err
	}
	r, err := m.ensureRunner(ctx, s)
	if err != nil {
		return err
	}
	if err := r.SetMode(ctx, p.Mode, &runnerSink{manager: m, session: s}); err != nil {
		return err
	}
	state := r.State()
	s.mu.Lock()
	s.lastActivity = time.Now()
	_ = s.log.Update(func(i *protocol.SessionInfo) { i.Mode = p.Mode }, state)
	m.emitInfoLocked(s)
	s.mu.Unlock()
	m.changed()
	return nil
}

func (m *sessionManager) setOption(ctx context.Context, p protocol.SetOptionParams) error {
	s, err := m.get(p.ID)
	if err != nil {
		return err
	}
	r, err := m.ensureRunner(ctx, s)
	if err != nil {
		return err
	}
	if err := r.SetOption(ctx, p.Option, p.Value, &runnerSink{manager: m, session: s}); err != nil {
		return err
	}
	m.touch(s, r)
	return nil
}

// start brings an agent session's process up before its first prompt, so the
// model and options it will use are on screen while the user still decides
// what to ask. The idle stop takes it down again if nothing follows.
func (m *sessionManager) start(ctx context.Context, id string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	r, err := m.ensureRunner(ctx, s)
	if err != nil {
		return err
	}
	if err := r.Start(ctx, &runnerSink{manager: m, session: s}); err != nil {
		return err
	}
	m.touch(s, r)
	return nil
}

// Start, SetModel, SetOption, Goal, Cancel, Stop and Delete are the session
// controls Teams runs a task's session with.
func (m *sessionManager) Start(ctx context.Context, id string) error { return m.start(ctx, id) }

func (m *sessionManager) SetModel(ctx context.Context, id, model string) error {
	return m.setModel(ctx, protocol.SetModelParams{ID: id, Model: model})
}

func (m *sessionManager) SetOption(ctx context.Context, id, option, value string) error {
	return m.setOption(ctx, protocol.SetOptionParams{ID: id, Option: option, Value: value})
}

func (m *sessionManager) Goal(ctx context.Context, p protocol.GoalParams) (protocol.SessionInfo, error) {
	return m.goal(ctx, p)
}

func (m *sessionManager) Cancel(_ context.Context, id string) error { return m.cancel(id) }

func (m *sessionManager) Stop(_ context.Context, id string) error { return m.stop(id) }

func (m *sessionManager) Delete(_ context.Context, id string) error { return m.delete(id) }

// Continue starts a turn under the session's goal that says text, drawn as a
// note from Rashin. A turn still running, or about to start, is waited for,
// so the note always reaches the agent as a turn of its own.
func (m *sessionManager) Continue(ctx context.Context, id, text string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	for {
		s.mu.Lock()
		if s.stopping {
			s.mu.Unlock()
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The session is stopping."))
		}
		if !s.running && !s.finishing {
			info := s.log.Info()
			s.running = true
			s.reserveDone = make(chan struct{})
			s.mu.Unlock()
			goal := info.Goal != nil && info.Goal.Status == protocol.GoalActive
			_, _, err := m.startPrompt(context.Background(), s, protocol.PromptParams{ID: id, Text: text}, promptTurn{reserved: true, goal: goal})
			return err
		}
		wait := s.turnDone
		if wait == nil {
			wait = s.reserveDone
		}
		if wait == nil {
			wait = s.finishDone
		}
		s.mu.Unlock()
		var retry <-chan time.Time
		if wait == nil {
			retry = time.After(50 * time.Millisecond)
		}
		select {
		case <-wait:
		case <-retry:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Watched reports whether a Rashin window is connected.
func (m *sessionManager) Watched() bool { return m.watched != nil && m.watched() }

// touch records that the session's runner was just used outside a turn.
func (m *sessionManager) touch(s *managedSession, r core.Runner) {
	state := r.State()
	alive := r.Alive()
	s.mu.Lock()
	s.lastActivity = time.Now()
	_ = s.log.Update(func(i *protocol.SessionInfo) { i.Alive = alive }, state)
	m.emitInfoLocked(s)
	s.mu.Unlock()
	m.changed()
}

func (m *sessionManager) rename(p protocol.RenameParams) (protocol.SessionInfo, error) {
	s, err := m.get(p.ID)
	if err != nil {
		return protocol.SessionInfo{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.log.Update(func(i *protocol.SessionInfo) { i.Title = strings.TrimSpace(p.Title) }, nil); err != nil {
		return protocol.SessionInfo{}, err
	}
	m.emitInfoLocked(s)
	return s.log.Info(), nil
}

func (m *sessionManager) stop(id string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.stopping {
		done := s.turnDone
		startDone := s.startDone
		reserveDone := s.reserveDone
		s.mu.Unlock()
		if startDone != nil {
			<-startDone
		}
		if reserveDone != nil {
			<-reserveDone
		}
		if done != nil {
			<-done
		}
		return nil
	}
	s.stopping = true
	startDone := s.startDone
	reserveDone := s.reserveDone
	s.mu.Unlock()
	if startDone != nil {
		<-startDone
	}
	if reserveDone != nil {
		<-reserveDone
	}
	s.mu.Lock()
	cancel := s.cancel
	runner := s.runner
	done := s.turnDone
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if runner != nil {
		runner.Cancel()
	}
	if m.ended != nil {
		m.ended(id)
	}
	var stopErr error
	if runner != nil {
		stopErr = runner.Stop()
	}
	if done != nil {
		<-done
	}
	_ = s.coalescer.Flush()
	stateData := runnerState(runner)
	s.mu.Lock()
	state := protocol.StateStopped
	if s.log.Info().Kind == protocol.KindAsk {
		state = protocol.StateIdle
	}
	_ = s.log.Update(func(i *protocol.SessionInfo) { i.State = state; i.Alive = false }, stateData)
	if s.runner == runner {
		s.runner = nil
	}
	s.stopping = false
	s.lastActivity = time.Now()
	m.emitInfoLocked(s)
	s.mu.Unlock()
	m.changed()
	return stopErr
}

func (m *sessionManager) delete(id string) error {
	s, err := m.get(id)
	if err != nil {
		return err
	}
	if err := m.stop(id); err != nil {
		return err
	}
	info := s.log.Info()
	m.discard(info)
	if err := s.coalescer.Close(); err != nil {
		return err
	}
	if err := s.log.Close(); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.items, id)
	m.mu.Unlock()
	if !info.Temporary {
		if err := store.Delete(id); err != nil {
			return err
		}
	}
	m.broadcast(protocol.EvtSessionsChanged, protocol.SessionsChanged{Removed: []string{id}})
	m.changed()
	return nil
}

// discard ends what a deleted session leaves outside its log: its turns'
// checkpoints and what memory kept of it.
func (m *sessionManager) discard(info protocol.SessionInfo) {
	if info.Worktree != "" {
		_ = gitcore.DeleteRefs(context.Background(), info.Worktree, checkpointRefs+info.ID+"/")
	}
	if m.memory != nil {
		m.memory.SessionDeleted(info.ID)
	}
}

// withTemporary adds the temporary sessions the manager holds, of kind or
// of every kind when kind is empty, to stored, newest first.
func (m *sessionManager) withTemporary(stored []protocol.SessionInfo, kind protocol.SessionKind) []protocol.SessionInfo {
	m.mu.Lock()
	items := make([]*managedSession, 0, len(m.items))
	for _, s := range m.items {
		items = append(items, s)
	}
	m.mu.Unlock()
	added := false
	for _, s := range items {
		if info := s.log.Info(); info.Temporary && (kind == "" || info.Kind == kind) {
			stored, added = append(stored, info), true
		}
	}
	if added {
		store.SortNewest(stored)
	}
	return stored
}

func (m *sessionManager) list(p protocol.SessionListParams) (protocol.SessionListResult, error) {
	all, err := store.List(p.Kind)
	if err != nil {
		return protocol.SessionListResult{}, err
	}
	all = m.withTemporary(all, p.Kind)
	if p.Before != "" {
		found := false
		for index, info := range all {
			if info.ID == p.Before {
				all = all[index+1:]
				found = true
				break
			}
		}
		if !found {
			out := all[:0]
			for _, info := range all {
				if info.ID < p.Before {
					out = append(out, info)
				}
			}
			all = out
		}
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return protocol.SessionListResult{Sessions: all}, nil
}

// recent is the newest limit sessions a client lists at once. Team task
// sessions are left out: their tasks show them, and a busy team would push
// the user's own conversations off the list.
func (m *sessionManager) recent(limit int) ([]protocol.SessionInfo, error) {
	all, err := store.List("")
	if err != nil {
		return nil, err
	}
	all = m.withTemporary(all, "")
	out := make([]protocol.SessionInfo, 0, min(limit, len(all)))
	for _, info := range all {
		if len(out) == limit {
			break
		}
		if info.Team == "" {
			out = append(out, info)
		}
	}
	return out, nil
}

func (m *sessionManager) counts() protocol.SessionCounts {
	all, _ := store.List("")
	all = m.withTemporary(all, "")
	var c protocol.SessionCounts
	for _, info := range all {
		if info.Kind == protocol.KindAsk {
			c.Ask++
		} else {
			c.Agent++
		}
		if info.State == protocol.StateWorking || info.State == protocol.StateWaiting {
			c.Working++
		}
		if info.Alive {
			c.Alive++
		}
	}
	return c
}

func (m *sessionManager) busy() bool {
	m.mu.Lock()
	items := make([]*managedSession, 0, len(m.items))
	for _, s := range m.items {
		items = append(items, s)
	}
	m.mu.Unlock()
	for _, s := range items {
		s.mu.Lock()
		running, runner := s.running, s.runner
		s.mu.Unlock()
		if running || runner != nil && runner.Alive() {
			return true
		}
	}
	return false
}

func (m *sessionManager) shutdown(deadline time.Duration) {
	m.mu.Lock()
	items := make([]*managedSession, 0, len(m.items))
	for _, s := range m.items {
		items = append(items, s)
	}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, s := range items {
		wg.Add(1)
		go func(s *managedSession) {
			defer wg.Done()
			info := s.log.Info()
			_ = m.stop(info.ID)
			_ = s.coalescer.Close()
			_ = s.log.Close()
			// a temporary session ends with the daemon
			if info.Temporary {
				m.discard(info)
			}
		}(s)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	timer := time.NewTimer(deadline)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}

func (m *sessionManager) idleStop(now time.Time) {
	minutes := m.settings().Agent.IdleStopMinutes
	if minutes <= 0 {
		return
	}
	m.mu.Lock()
	items := make([]*managedSession, 0, len(m.items))
	for _, s := range m.items {
		items = append(items, s)
	}
	m.mu.Unlock()
	for _, s := range items {
		info := s.log.Info()
		s.mu.Lock()
		running, pending, runner, lastActivity := s.running, len(s.permissions), s.runner, s.lastActivity
		s.mu.Unlock()
		// an Ask a harness answers holds its process up just as an agent does
		due := !running && pending == 0 && runner != nil && runner.Alive() && now.Sub(lastActivity) >= time.Duration(minutes)*time.Minute
		if due {
			go func(id string) { _ = m.stop(id) }(info.ID)
		}
	}
}

// exchange is the last question of a session with what was attached to it,
// and what it was answered.
type exchange struct {
	question, answer, reason string
	attached                 []protocol.Attachment
}

func (m *sessionManager) lastExchange(id string) (exchange, error) {
	s, err := m.get(id)
	if err != nil {
		return exchange{}, err
	}
	events, err := s.log.Since(0)
	if err != nil {
		return exchange{}, err
	}
	var last exchange
	for _, ev := range events {
		switch ev.Kind {
		case protocol.EvUser:
			var d protocol.UserData
			if json.Unmarshal(ev.Data, &d) == nil {
				last = exchange{question: d.Text, attached: d.Images}
			}
		case protocol.EvText:
			var d protocol.TextData
			if json.Unmarshal(ev.Data, &d) == nil {
				last.answer += d.Delta
			}
		case protocol.EvAnswer:
			var d protocol.AnswerData
			if json.Unmarshal(ev.Data, &d) == nil {
				last.reason = d.Reason
			}
		}
	}
	return last, nil
}

func (m *sessionManager) latestAsk(origin string) string {
	all, _ := store.List(protocol.KindAsk)
	sort.SliceStable(all, func(i, j int) bool { return all[i].Updated.After(all[j].Updated) })
	for _, info := range all {
		if origin == "" || info.Origin == origin {
			return info.ID
		}
	}
	return ""
}
