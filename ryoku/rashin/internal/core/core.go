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
	// Resume is the State a previous runner of this session returned.
	Resume json.RawMessage
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

// SessionControl is the daemon's session manager as Work sees it: Work creates
// and prompts Agent sessions for tasks and review notes.
type SessionControl interface {
	Info(id string) (protocol.SessionInfo, bool)
	List(kind protocol.SessionKind) []protocol.SessionInfo
	New(ctx context.Context, p protocol.SessionNewParams) (protocol.SessionInfo, error)
	Prompt(ctx context.Context, p protocol.PromptParams) (protocol.PromptResult, error)
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
