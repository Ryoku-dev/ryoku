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

// SessionControl is the daemon's session manager as Work and Teams see it:
// they create and prompt Agent sessions for tasks and review notes.
type SessionControl interface {
	Info(id string) (protocol.SessionInfo, bool)
	List(kind protocol.SessionKind) []protocol.SessionInfo
	New(ctx context.Context, p protocol.SessionNewParams) (protocol.SessionInfo, error)
	Prompt(ctx context.Context, p protocol.PromptParams) (protocol.PromptResult, error)
}

// TeamSessions is what Teams needs beyond SessionControl to run a task as an
// Agent session: bring its agent up, pick the member's model, set the task as
// the session's goal, stop it, and continue it with a note when what held it
// between turns is settled.
type TeamSessions interface {
	SessionControl
	// Start brings the session's agent up without a prompt.
	Start(ctx context.Context, id string) error
	// SetModel picks the session's model among the ones its harness offers.
	SetModel(ctx context.Context, id, model string) error
	// Goal sets, ends or reports the session's goal; Set starts its first
	// turn on an idle session.
	Goal(ctx context.Context, p protocol.GoalParams) (protocol.SessionInfo, error)
	// Continue starts a turn under the session's goal that says text, drawn
	// as a note from Rashin rather than as the user's words.
	Continue(ctx context.Context, id, text string) error
	// Cancel ends the running turn; Stop also stops the agent process.
	Cancel(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
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
	// GoalEnded tells Teams that a team session's goal ended: done, blocked or
	// stopped, with the agent's summary.
	GoalEnded(info protocol.SessionInfo, goal protocol.Goal)
	// SessionChanged lets Teams follow a team session's state.
	SessionChanged(info protocol.SessionInfo)
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
