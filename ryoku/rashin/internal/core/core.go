// Package core holds the interfaces between the daemon and its subsystems, so
// the gateway, Ask, harness and index packages build and test on their own and
// the daemon wires concrete implementations together.
package core

import (
	"context"
	"encoding/json"
	"net/http"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// Broadcaster sends a global event to every subscribed client.
type Broadcaster interface {
	Broadcast(event string, data any)
}

// Sink is how a runner reports a turn. The session manager implements it: Emit
// appends to the session log, coalescing text, thought and terminal deltas, and
// streams the result to attached clients.
type Sink interface {
	// Emit records one event. data is the protocol *Data type for kind.
	Emit(kind string, data any)
	// Update changes SessionInfo fields (model, mode, title, alive, state,
	// error) and broadcasts session.info.
	Update(func(*protocol.SessionInfo))
}

// PromptInput is one user turn.
type PromptInput struct {
	Turn     int
	Text     string
	Images   []protocol.Attachment
	Terminal *protocol.TerminalContext
	Handoff  *protocol.Handoff
	Origin   string
	// Run is a command the user runs in place of a question.
	Run string
	// History is the session log before this turn.
	History []protocol.SessionEvent
}

// TurnResult is how a turn ended.
type TurnResult struct {
	Stop  string
	Usage *protocol.TokenUsage
}

// Runner drives one session. The session manager emits user, turn_start and
// turn_end; the runner emits everything in between. Prompt blocks until the
// turn ends. Calls on one runner are serialised by the session manager except
// Cancel and Permission, which may arrive while Prompt runs.
type Runner interface {
	Prompt(ctx context.Context, in PromptInput, sink Sink) (TurnResult, error)
	// Cancel ends the running turn; open permissions resolve as cancelled.
	Cancel()
	// Permission answers an open permission request.
	Permission(request, option string) error
	SetModel(ctx context.Context, model string, sink Sink) error
	SetMode(ctx context.Context, mode string, sink Sink) error
	// SetOption sets one option a harness offers beside its model and mode.
	SetOption(ctx context.Context, option, value string, sink Sink) error
	// Start brings the session's process up without a prompt, so it reports
	// its model, modes and options. A runner with no process does nothing.
	Start(ctx context.Context, sink Sink) error
	// Alive reports whether an external process runs for this session.
	Alive() bool
	// Stop ends the external process; the session stays resumable.
	Stop() error
	// State is runner state persisted in the session's meta.json and handed
	// back on the next start (a harness's own session id).
	State() json.RawMessage
}

// AskOptions creates an Ask runner.
type AskOptions struct {
	Session  protocol.SessionInfo
	Settings func() protocol.Settings
}

// Asker creates Ask runners.
type Asker interface {
	NewAsk(opts AskOptions) Runner
}

// HarnessOptions creates an Agent runner.
type HarnessOptions struct {
	Session   protocol.SessionInfo
	Harness   string
	Cwd       string
	Approvals func() string
	// Brief is what the agent is told about where it runs: Ryoku, the vault,
	// Rashin and this session. It is sent once, ahead of the first prompt.
	Brief func() string
	// Resume is the State a previous runner of this session returned.
	Resume json.RawMessage
	// Tools are the MCP servers handed to the agent with its session.
	Tools []MCPServer
	// Sandbox, set for a team session, is the sandbox its agent runs in, less
	// the harness's own state, which the harness adds as writable. Such an
	// agent gets no file or terminal access through Rashin. Nil runs the agent
	// as the user.
	Sandbox *sandbox.Spec
}

// MCPServer is a stdio MCP server an agent starts for its session.
type MCPServer struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// Harnesses discovers external harnesses and starts their runners.
type Harnesses interface {
	List(ctx context.Context, refresh bool) ([]protocol.HarnessInfo, error)
	NewAgent(ctx context.Context, opts HarnessOptions) (Runner, error)
	// Probe starts a harness once in base, a team sandbox without a folder,
	// with no prompt: it reads the harness's models and options and picks the
	// model and options given, then stops it. A harness that does not start
	// or refuses a pick is a result with OK false, not an error.
	Probe(ctx context.Context, params protocol.HarnessProbeParams, base sandbox.Spec) (protocol.HarnessProbeResult, error)
	// OnLearned sets what runs each time a harness's capabilities are
	// recorded, outside any lock.
	OnLearned(fn func())
}

// ImportReport says what a migration step brought over.
type ImportReport struct {
	Imported int
	Skipped  []string
	Notes    []string
}

// Gateway is the model router: chat completions plus the provider and key
// management behind the providers.*, models.list and gateway.usage methods.
type Gateway interface {
	llm.Client
	Providers(ctx context.Context) ([]protocol.ProviderInfo, error)
	AddKey(ctx context.Context, p protocol.AddKeyParams) (protocol.ProviderInfo, error)
	RemoveKey(ctx context.Context, p protocol.RemoveKeyParams) (protocol.ProviderInfo, error)
	EnableProvider(ctx context.Context, p protocol.EnableProviderParams) (protocol.ProviderInfo, error)
	Test(ctx context.Context, provider string) (protocol.ProviderTestResult, error)
	Models(ctx context.Context, provider string) (protocol.ModelsResult, error)
	Usage(ctx context.Context, days int) (protocol.GatewayUsage, error)
	Status(ctx context.Context) protocol.GatewayStatus
	// API serves the router to other programs in OpenAI's shape, behind the
	// key APIKey returns (and replaces on rotate).
	API() http.Handler
	APIKey(ctx context.Context, rotate bool) (string, error)
	// ImportProwl copies keys for catalogue providers out of Prowl's state
	// directory. It is safe to call again: duplicates are skipped.
	ImportProwl(ctx context.Context, prowlDir string) (ImportReport, error)
	Close() error
}

// Indexer is the index engine behind the index.* methods and Ask's search
// tools.
type Indexer interface {
	List(ctx context.Context) ([]protocol.IndexInfo, error)
	Add(ctx context.Context, p protocol.IndexAddParams) (protocol.IndexInfo, error)
	Remove(ctx context.Context, ref protocol.IndexRef) error
	Refresh(ctx context.Context, ref protocol.IndexRef) (protocol.IndexInfo, error)
	Status(ctx context.Context, ref protocol.IndexRef) (protocol.IndexInfo, error)
	Search(ctx context.Context, p protocol.SearchParams) (protocol.SearchResult, error)
	Find(ctx context.Context, p protocol.NameParams) (protocol.SymbolsResult, error)
	Def(ctx context.Context, p protocol.NameParams) (protocol.DefResult, error)
	Outline(ctx context.Context, p protocol.PathParams) (protocol.SymbolsResult, error)
	References(ctx context.Context, p protocol.NameParams) (protocol.ReferencesResult, error)
	Peek(ctx context.Context, p protocol.PeekParams) (protocol.PeekResult, error)
	Overview(ctx context.Context, ref protocol.IndexRef) (protocol.Overview, error)
	Impact(ctx context.Context, p protocol.PathParams) (protocol.Impact, error)
	Wip(ctx context.Context, p protocol.WipParams) (protocol.WipResult, error)
	Changed(ctx context.Context, p protocol.WipParams) (protocol.WipResult, error)
	// Busy reports a running refresh or embedding job.
	Busy() bool
	// Release drops loaded indexes and the embedding model.
	Release()
	Close() error
}

// MemoryCaller says who calls a memory.* method: the Rashin app, an agent
// session through its MCP tools (Session), a team task's agent through its
// own socket (Team and Task), or anything else on the socket, such as the
// CLI. The daemon fills it from the connection; a caller never claims it.
type MemoryCaller struct {
	App     bool
	Session string
	Team    string
	Task    string
}

// Recall is what Ask is given from memory for one turn. System goes into the
// system prompt: the card and how to use memory. Context goes into this
// turn's user message only and is never stored. Used is shown under the
// answer. Tools offers the memory tools and Sessions the search of past
// conversations.
type Recall struct {
	System   string
	Context  string
	Used     *protocol.MemoryEventData
	Tools    bool
	Sessions bool
}

// MemorySessions is what memory needs from the session manager: a session's
// info and stored events, every stored session of a kind, and a way to put a
// memory line into a session's transcript.
type MemorySessions interface {
	Info(id string) (protocol.SessionInfo, bool)
	Events(id string) ([]protocol.SessionEvent, error)
	List(kind protocol.SessionKind) []protocol.SessionInfo
	EmitMemory(id string, data protocol.MemoryEventData)
}

// Memory is what Rashin remembers between conversations: the memory.*
// methods, what Ask, agents and teams are told, and the hooks the session
// manager calls. Nothing of it runs while memory is off.
type Memory interface {
	// Bind hands memory the session manager; the daemon calls it once at
	// start.
	Bind(sessions MemorySessions)
	// Handle serves one memory.* method for from.
	Handle(ctx context.Context, from MemoryCaller, method string, params json.RawMessage) (any, error)
	// Status fills the snapshot and every memory.changed event.
	Status(ctx context.Context) protocol.MemoryStatus
	// Recall is memory for one Ask turn; zero when the session or the
	// settings use none.
	Recall(ctx context.Context, info protocol.SessionInfo, question string) Recall
	// Brief is the memory part of an agent session's brief, "" for none.
	Brief(info protocol.SessionInfo) string
	// TeamBrief is what a team's members are told the team remembers, ""
	// when it keeps nothing.
	TeamBrief(team protocol.Team) string
	// TurnEnded lets memory learn from a session once it goes quiet;
	// SessionDeleted drops what memory kept of a deleted session.
	TurnEnded(info protocol.SessionInfo)
	SessionDeleted(id string)
	// Settle learns from every conversation still waiting, ahead of the idle
	// exit. It returns at once; Busy reports the work until it is done.
	Settle()
	Busy() bool
	// Release drops loaded models and caches at rest.
	Release()
	Close() error
}

// SessionControl is the daemon's session manager as Work and Teams see it:
// they create and prompt Agent sessions for tasks and review notes.
type SessionControl interface {
	Info(id string) (protocol.SessionInfo, bool)
	List(kind protocol.SessionKind) []protocol.SessionInfo
	New(ctx context.Context, p protocol.SessionNewParams) (protocol.SessionInfo, error)
	Prompt(ctx context.Context, p protocol.PromptParams) (protocol.PromptResult, error)
}

// TeamSessions is what Teams needs beyond SessionControl to run a task as an
// Agent session: bring its agent up, pick the member's engine, set the task
// as the session's goal, stop it, continue it with a note when what held it
// between turns is settled, and delete it when the task is pruned.
type TeamSessions interface {
	SessionControl
	// Start brings the session's agent up without a prompt.
	Start(ctx context.Context, id string) error
	// SetModel picks the session's model among the ones its harness offers.
	SetModel(ctx context.Context, id, model string) error
	// SetOption sets one of the options the session's harness offers to
	// value, one of that option's choices.
	SetOption(ctx context.Context, id, option, value string) error
	// Goal sets, ends or reports the session's goal; Set starts its first
	// turn on an idle session.
	Goal(ctx context.Context, p protocol.GoalParams) (protocol.SessionInfo, error)
	// Continue starts a turn under the session's goal that says text, drawn
	// as a note from Rashin rather than as the user's words.
	Continue(ctx context.Context, id, text string) error
	// Cancel ends the running turn; Stop also stops the agent process.
	Cancel(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	// Delete stops the session and removes it with its log.
	Delete(ctx context.Context, id string) error
	// Watched reports whether a Rashin window is connected, so what happens
	// unattended is recorded as such.
	Watched() bool
}

// Teams is the control plane behind the teams.* and vault.* methods: teams of
// agents with their tasks, routines, approvals, spend and credentials, plus
// the hooks the session manager calls for sessions that work a team task.
type Teams interface {
	// Bind hands Teams the session manager; the daemon calls it once at start.
	Bind(sessions TeamSessions)
	// Handle serves one teams.* or vault.* method.
	Handle(ctx context.Context, method string, params json.RawMessage) (any, error)
	// Snapshot fills the Teams part of the subscribe snapshot.
	Snapshot(ctx context.Context, into *protocol.Snapshot)
	// ToolEnv is what Rashin's MCP server is told for a team session: its
	// team, task and run token. Nil for any other session.
	ToolEnv(info protocol.SessionInfo) map[string]string
	// Brief is the team part of what a team session's agent is told once,
	// ahead of its first prompt. Empty for any other session.
	Brief(info protocol.SessionInfo) string
	// AfterTurn counts a team session's turn and its tokens.
	AfterTurn(ctx context.Context, info protocol.SessionInfo, end protocol.TurnEndData)
	// Hold reports whether a team session must not start its next goal turn
	// yet, because it waits on the user or on its members; Teams continues it
	// once that settles.
	Hold(info protocol.SessionInfo) bool
	// ScreenGranted reports whether a session may have its own screen without
	// asking the user, because its team was given one.
	ScreenGranted(info protocol.SessionInfo) bool
	// Access is what a team session's sandbox adds for its team: folders it
	// may change and paths it sees as empty. Zero for any other session.
	Access(info protocol.SessionInfo) protocol.TeamAccess
	// GoalEnded tells Teams that a team session's goal ended: done, blocked or
	// stopped, with the agent's summary.
	GoalEnded(info protocol.SessionInfo, goal protocol.Goal)
	// SessionChanged lets Teams follow a team session's state.
	SessionChanged(info protocol.SessionInfo)
	// SettingsChanged lets Teams follow a change of the settings: a global
	// pause stops working tasks, and the limits and the wake unit apply again.
	SettingsChanged(settings protocol.Settings)
	// RestoreHome puts team's home for a harness back as the user signed it
	// in, before a run there, and tells the user what it moved: in the
	// ledger and as a notice in the Inbox under that team.
	RestoreHome(harness, team string) error
	// Working reports whether a task of team works on harness now.
	Working(harness, team string) bool
	// TeamName is a team's name, false for a team that does not exist.
	TeamName(team string) (string, bool)
	// Busy reports team tasks at work; Armed reports routines that are on, so
	// the daemon stays up to fire them.
	Busy() bool
	Armed() bool
	Close() error
}

// Work is the coding control plane behind the projects.*, worktrees.*, git.*,
// review.*, tasks.* and activity.list methods, plus the hooks the session
// manager calls around Agent turns.
type Work interface {
	// Bind hands Work the session manager; the daemon calls it once at start.
	Bind(sessions SessionControl)
	// Handle serves one Work method (protocol name, raw params) and returns the
	// result value to encode.
	Handle(ctx context.Context, method string, params json.RawMessage) (any, error)
	// Watch adds (on) or drops one interest in git.changed for a project; the
	// daemon counts interests per connection and drops them on disconnect.
	Watch(project string, on bool)
	// Projects and Tasks feed the subscribe snapshot.
	Projects(ctx context.Context) []protocol.ProjectInfo
	Tasks(ctx context.Context) []protocol.TaskInfo
	// PrepareSession resolves where a new Agent session runs: Cwd, the project
	// root, an existing worktree, or a new worktree (Worktree "new"). It fills
	// p.Cwd, p.Worktree and p.Branch and returns the updated params.
	PrepareSession(ctx context.Context, p protocol.SessionNewParams) (protocol.SessionNewParams, error)
	// BeforeTurn checkpoints the session's worktree before a turn; it reports
	// false outside Git.
	BeforeTurn(ctx context.Context, s protocol.SessionInfo, turn int) (bool, error)
	// AfterTurn refreshes the worktree, updates task status and products, and
	// emits git.changed and worktrees.changed.
	AfterTurn(ctx context.Context, s protocol.SessionInfo, turn int)
	// SessionChanged lets Work move a task with its session's state.
	SessionChanged(s protocol.SessionInfo)
	// RevertTurn restores the session's worktree files to the checkpoint taken
	// before turn. It refuses while another session runs in that worktree.
	RevertTurn(ctx context.Context, s protocol.SessionInfo, turn int) (protocol.GitOpResult, error)
	// Busy reports running Git operations, setups or cleanups.
	Busy() bool
	Close() error
}
