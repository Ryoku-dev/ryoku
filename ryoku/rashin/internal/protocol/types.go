package protocol

import (
	"encoding/json"
	"time"
)

// Empty is the params or result of a method that carries nothing.
type Empty struct{}

// Client kinds sent in hello.
const (
	ClientApp = "app"
	ClientBar = "bar"
	ClientCLI = "cli"
	ClientHub = "hub"
)

// HelloParams opens a connection.
type HelloParams struct {
	Client  string `json:"client"`
	Version int    `json:"version"`
	PID     int    `json:"pid,omitempty"`
}

// HelloResult identifies the daemon.
type HelloResult struct {
	Daemon   string `json:"daemon"`
	Version  string `json:"version"`
	Protocol int    `json:"protocol"`
	Enabled  bool   `json:"enabled"`
}

// SessionCounts summarises the session store.
type SessionCounts struct {
	Ask     int `json:"ask"`
	Agent   int `json:"agent"`
	Working int `json:"working"`
	Alive   int `json:"alive"`
}

// GatewayStatus summarises the model router.
type GatewayStatus struct {
	// Providers counts enabled providers with a usable key or no key needed.
	Providers int `json:"providers"`
	// Models counts models available right now.
	Models  int `json:"models"`
	Keyless int `json:"keyless"`
}

// Status is the daemon's state at a glance.
type Status struct {
	Version   string        `json:"version"`
	Enabled   bool          `json:"enabled"`
	Running   bool          `json:"running"`
	PID       int           `json:"pid,omitempty"`
	UptimeSec int64         `json:"uptimeSec,omitempty"`
	Clients   int           `json:"clients"`
	Sessions  SessionCounts `json:"sessions"`
	Gateway   GatewayStatus `json:"gateway"`
	Harnesses int           `json:"harnesses"`
	Indexes   int           `json:"indexes"`
}

// Snapshot is the subscribe result: everything a client renders at start.
type Snapshot struct {
	Status    Status         `json:"status"`
	Sessions  []SessionInfo  `json:"sessions"`
	Settings  Settings       `json:"settings"`
	Providers []ProviderInfo `json:"providers"`
	Harnesses []HarnessInfo  `json:"harnesses"`
	Indexes   []IndexInfo    `json:"indexes"`
	Projects  []ProjectInfo  `json:"projects"`
	Tasks     []TaskInfo     `json:"tasks"`
}

// SessionListParams filters session.list. Before is a session id: only older
// sessions are returned.
type SessionListParams struct {
	Kind   SessionKind `json:"kind,omitempty"`
	Limit  int         `json:"limit,omitempty"`
	Before string      `json:"before,omitempty"`
}

// SessionListResult is newest first.
type SessionListResult struct {
	Sessions []SessionInfo `json:"sessions"`
}

// SessionNewParams creates a session.
type SessionNewParams struct {
	Kind    SessionKind `json:"kind"`
	Harness string      `json:"harness,omitempty"`
	Cwd     string      `json:"cwd,omitempty"`
	Model   string      `json:"model,omitempty"`
	Title   string      `json:"title,omitempty"`
	Origin  string      `json:"origin,omitempty"`
	// Project binds an Agent session to a project. Worktree is "" (the project
	// root, or Cwd without a project), "new" (create one from Base, named Name,
	// on Branch or the project's prefix plus Name), or an existing worktree path.
	Project  string `json:"project,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Base     string `json:"base,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Name     string `json:"name,omitempty"`
	Task     string `json:"task,omitempty"`
}

// SessionRef names one session.
type SessionRef struct {
	ID string `json:"id"`
}

// SessionAttachParams replays events after Since, then streams.
type SessionAttachParams struct {
	ID    string `json:"id"`
	Since int64  `json:"since"`
	// Limit caps the replayed events; 0 lets the daemon choose a page that
	// fits one frame.
	Limit int `json:"limit,omitempty"`
}

// SessionAttachResult carries the replay. More is set when the replay was cut
// at a page boundary: the client attaches again with Since set to the last
// event's seq until More is false. The connection joins the session's live
// events only with the last page, so nothing arrives between pages.
type SessionAttachResult struct {
	Info   SessionInfo    `json:"info"`
	Events []SessionEvent `json:"events"`
	More   bool           `json:"more,omitempty"`
}

// PromptParams starts a turn.
type PromptParams struct {
	ID       string           `json:"id"`
	Text     string           `json:"text"`
	Images   []Attachment     `json:"images,omitempty"`
	Terminal *TerminalContext `json:"terminal,omitempty"`
}

// PromptResult names the turn that started.
type PromptResult struct {
	Turn int `json:"turn"`
}

// PermissionParams answers a permission request.
type PermissionParams struct {
	ID      string `json:"id"`
	Request string `json:"request"`
	Option  string `json:"option"`
}

// SetModelParams picks a model for a session.
type SetModelParams struct {
	ID    string `json:"id"`
	Model string `json:"model"`
}

// SetModeParams picks a harness mode for a session.
type SetModeParams struct {
	ID   string `json:"id"`
	Mode string `json:"mode"`
}

// RenameParams renames a session.
type RenameParams struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// HandoffParams moves an Ask session's work to a new Agent session. With Run
// the new session is prompted at once; Text adds the user's own words.
type HandoffParams struct {
	ID      string `json:"id"`
	Harness string `json:"harness,omitempty"`
	Cwd     string `json:"cwd,omitempty"`
	Run     bool   `json:"run,omitempty"`
	Text    string `json:"text,omitempty"`
	// Project, Worktree, Base, Branch and Name place the new Agent session as
	// in SessionNewParams.
	Project  string `json:"project,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Base     string `json:"base,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Name     string `json:"name,omitempty"`
}

// AskParams creates (or continues) an Ask session, attaches the caller and
// prompts, in one call.
type AskParams struct {
	Question string           `json:"question"`
	Session  string           `json:"session,omitempty"`
	Origin   string           `json:"origin,omitempty"`
	Model    string           `json:"model,omitempty"`
	Images   []Attachment     `json:"images,omitempty"`
	Terminal *TerminalContext `json:"terminal,omitempty"`
}

// AskResult names the session and turn; session.event frames with a seq above
// Since follow on the same connection.
type AskResult struct {
	Session SessionInfo `json:"session"`
	Turn    int         `json:"turn"`
	Since   int64       `json:"since"`
}

// Key health.
const (
	KeyUnknown = "unknown"
	KeyHealthy = "healthy"
	KeyError   = "error"
)

// KeyInfo describes a stored provider key; the secret never leaves the daemon.
type KeyInfo struct {
	ID        string     `json:"id"`
	Label     string     `json:"label,omitempty"`
	Status    string     `json:"status"`
	Masked    string     `json:"masked"`
	LastError string     `json:"lastError,omitempty"`
	Checked   *time.Time `json:"checked,omitempty"`
}

// ProviderInfo describes one catalogue provider and its state here.
type ProviderInfo struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Tier          string     `json:"tier"`
	Keyless       bool       `json:"keyless,omitempty"`
	Enabled       bool       `json:"enabled"`
	Configured    bool       `json:"configured"`
	Keys          []KeyInfo  `json:"keys,omitempty"`
	Models        int        `json:"models"`
	Available     int        `json:"available"`
	Signup        string     `json:"signup,omitempty"`
	KeyHint       string     `json:"keyHint,omitempty"`
	Note          string     `json:"note,omitempty"`
	CooldownUntil *time.Time `json:"cooldownUntil,omitempty"`
}

// ProvidersResult lists providers.
type ProvidersResult struct {
	Providers []ProviderInfo `json:"providers"`
}

// AddKeyParams stores a key for a provider.
type AddKeyParams struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
	Label    string `json:"label,omitempty"`
}

// RemoveKeyParams removes a stored key by id.
type RemoveKeyParams struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
}

// ProviderParams names one provider.
type ProviderParams struct {
	Provider string `json:"provider"`
}

// EnableProviderParams turns a provider on or off for routing.
type EnableProviderParams struct {
	Provider string `json:"provider"`
	Enabled  bool   `json:"enabled"`
}

// ProviderTestResult reports a real one-token completion through a provider.
type ProviderTestResult struct {
	Provider  string `json:"provider"`
	OK        bool   `json:"ok"`
	Model     string `json:"model,omitempty"`
	LatencyMs int64  `json:"latencyMs,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ModelInfo describes one free chat model.
type ModelInfo struct {
	ID            string     `json:"id"`
	Provider      string     `json:"provider"`
	Name          string     `json:"name"`
	Context       int        `json:"context,omitempty"`
	Tools         bool       `json:"tools,omitempty"`
	Reasoning     bool       `json:"reasoning,omitempty"`
	Vision        bool       `json:"vision,omitempty"`
	Rank          int        `json:"rank,omitempty"`
	Available     bool       `json:"available"`
	CooldownUntil *time.Time `json:"cooldownUntil,omitempty"`
}

// ModelsParams filters models.list by provider.
type ModelsParams struct {
	Provider string `json:"provider,omitempty"`
}

// Routes the gateway always accepts as a model.
const (
	RouteAuto  = "auto"
	RouteFast  = "auto:fast"
	RouteSmart = "auto:smart"
)

// ModelsResult lists models and the routes.
type ModelsResult struct {
	Models []ModelInfo `json:"models"`
	Routes []Choice    `json:"routes"`
}

// UsageParams asks for the last Days of usage (default 7).
type UsageParams struct {
	Days int `json:"days,omitempty"`
}

// UsageRow is one day of one model.
type UsageRow struct {
	Day          string `json:"day"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Requests     int    `json:"requests"`
	OK           int    `json:"ok"`
	Failed       int    `json:"failed"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	Estimated    bool   `json:"estimated,omitempty"`
}

// GatewayUsage is the gateway.usage result.
type GatewayUsage struct {
	Days int        `json:"days"`
	Rows []UsageRow `json:"rows"`
}

// HarnessInfo describes one external harness.
type HarnessInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Installed bool     `json:"installed"`
	Command   []string `json:"command,omitempty"`
	Path      string   `json:"path,omitempty"`
	Version   string   `json:"version,omitempty"`
	Default   bool     `json:"default,omitempty"`
	Install   string   `json:"install,omitempty"`
	// Custom marks a harness instance the user added in settings.
	Custom bool `json:"custom,omitempty"`
	// Capabilities are learned from the agent's initialize answer the first
	// time it runs, and kept per binary.
	Capabilities *HarnessCapabilities `json:"capabilities,omitempty"`
}

// HarnessCapabilities is what an agent advertised at initialize.
type HarnessCapabilities struct {
	LoadSession bool     `json:"loadSession,omitempty"`
	Images      bool     `json:"images,omitempty"`
	Audio       bool     `json:"audio,omitempty"`
	Context     bool     `json:"context,omitempty"`
	Modes       []Choice `json:"modes,omitempty"`
	Models      bool     `json:"models,omitempty"`
}

// HarnessInstance is a user-defined harness: any ACP agent command with its own
// environment (a second account, a different config dir, a wrapper).
type HarnessInstance struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Command []string          `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
}

// HarnessesListParams asks for a fresh PATH scan with Refresh.
type HarnessesListParams struct {
	Refresh bool `json:"refresh,omitempty"`
}

// HarnessesResult lists harnesses in table order.
type HarnessesResult struct {
	Harnesses []HarnessInfo `json:"harnesses"`
}

// Index kinds and states.
const (
	IndexProject = "project"
	IndexSystem  = "system"

	IndexEmpty    = "empty"
	IndexIndexing = "indexing"
	IndexReady    = "ready"
	IndexError    = "error"
)

// IndexInfo describes one index.
type IndexInfo struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Root    string     `json:"root,omitempty"`
	Roots   []string   `json:"roots,omitempty"`
	Kind    string     `json:"kind"`
	State   string     `json:"state"`
	Files   int        `json:"files"`
	Chunks  int        `json:"chunks"`
	Symbols int        `json:"symbols"`
	Vectors int        `json:"vectors"`
	Updated *time.Time `json:"updated,omitempty"`
	Error   string     `json:"error,omitempty"`
}

// IndexesResult lists indexes.
type IndexesResult struct {
	Indexes []IndexInfo `json:"indexes"`
}

// IndexAddParams registers a project folder.
type IndexAddParams struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
}

// IndexRef selects an index: ID wins; otherwise the index whose root holds
// Cwd, registering the enclosing git root on first use.
type IndexRef struct {
	ID  string `json:"id,omitempty"`
	Cwd string `json:"cwd,omitempty"`
}

// IndexProgress is the data of an index.progress event. Phase is walk, parse,
// embed, done or error.
type IndexProgress struct {
	ID      string `json:"id"`
	Phase   string `json:"phase"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Message string `json:"message,omitempty"`
}

// Search modes.
const (
	SearchHybrid   = "hybrid"
	SearchLexical  = "lexical"
	SearchSemantic = "semantic"
)

// SearchParams runs a search.
type SearchParams struct {
	IndexRef
	Query   string `json:"query"`
	Limit   int    `json:"limit,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Compact bool   `json:"compact,omitempty"`
	// Existing searches what an already registered index holds now: it never
	// registers a path and never waits for a refresh, starting one in the
	// background when files changed. Ask uses it so a question never builds
	// an index.
	Existing bool `json:"existing,omitempty"`
}

// SearchHit is one ranked chunk.
type SearchHit struct {
	Path     string  `json:"path"`
	Start    int     `json:"start"`
	End      int     `json:"end"`
	Score    float64 `json:"score"`
	Snippet  string  `json:"snippet,omitempty"`
	Symbol   string  `json:"symbol,omitempty"`
	Language string  `json:"language,omitempty"`
}

// SearchResult carries the hits; Semantic is false when vectors were not used.
type SearchResult struct {
	Index    string      `json:"index"`
	Hits     []SearchHit `json:"hits"`
	Semantic bool        `json:"semantic"`
}

// NameParams looks a symbol up by name or id.
type NameParams struct {
	IndexRef
	Name  string `json:"name"`
	Limit int    `json:"limit,omitempty"`
}

// Symbol is one definition. Paths are relative to the index root.
type Symbol struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Signature string `json:"signature,omitempty"`
	Path      string `json:"path"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	Parent    string `json:"parent,omitempty"`
	Doc       string `json:"doc,omitempty"`
	Depth     int    `json:"depth,omitempty"`
}

// SymbolsResult lists symbols.
type SymbolsResult struct {
	Index   string   `json:"index"`
	Symbols []Symbol `json:"symbols"`
}

// DefResult is one symbol with its source. Others lists other symbols with the
// same name.
type DefResult struct {
	Index     string   `json:"index"`
	Symbol    Symbol   `json:"symbol"`
	Body      string   `json:"body"`
	Truncated bool     `json:"truncated,omitempty"`
	Others    []Symbol `json:"others,omitempty"`
}

// PathParams names a file in an index.
type PathParams struct {
	IndexRef
	Path string `json:"path"`
	All  bool   `json:"all,omitempty"`
}

// Reference is one use of a name.
type Reference struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Text   string `json:"text"`
	Symbol string `json:"symbol,omitempty"`
}

// ReferencesResult lists references.
type ReferencesResult struct {
	Index      string      `json:"index"`
	Name       string      `json:"name"`
	References []Reference `json:"references"`
}

// PeekParams reads lines Start..End (1-based, inclusive) of a file.
type PeekParams struct {
	IndexRef
	Path  string `json:"path"`
	Start int    `json:"start"`
	End   int    `json:"end,omitempty"`
}

// PeekResult carries the lines read.
type PeekResult struct {
	Index     string `json:"index"`
	Path      string `json:"path"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Count is a named count.
type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// FileStat is a file with one number.
type FileStat struct {
	Path  string `json:"path"`
	Value int    `json:"value"`
}

// Overview maps an index.
type Overview struct {
	Index        string     `json:"index"`
	Root         string     `json:"root"`
	Files        int        `json:"files"`
	Symbols      int        `json:"symbols"`
	Chunks       int        `json:"chunks"`
	Languages    []Count    `json:"languages"`
	Entrypoints  []string   `json:"entrypoints"`
	Largest      []FileStat `json:"largest"`
	MostImported []FileStat `json:"mostImported"`
	Docs         []string   `json:"docs,omitempty"`
}

// Impact lists the files that depend on a path.
type Impact struct {
	Index      string   `json:"index"`
	Path       string   `json:"path"`
	Direct     []string `json:"direct"`
	Total      int      `json:"total"`
	All        []string `json:"all,omitempty"`
	Subsystems []Count  `json:"subsystems,omitempty"`
}

// WipParams compares the working tree with Base (changed) or HEAD (wip).
type WipParams struct {
	IndexRef
	Base string `json:"base,omitempty"`
}

// GitChange is one changed file. Status is modified, added, deleted,
// untracked, renamed or staged.
type GitChange struct {
	Path       string   `json:"path"`
	Status     string   `json:"status"`
	Indexed    bool     `json:"indexed"`
	Dependents int      `json:"dependents"`
	Markers    []string `json:"markers,omitempty"`
}

// WipResult lists changes.
type WipResult struct {
	Index   string      `json:"index"`
	Root    string      `json:"root"`
	Base    string      `json:"base,omitempty"`
	Changes []GitChange `json:"changes"`
}

// Agent approval policies.
const (
	ApprovalsReadOnly = "read-only"
	ApprovalsAsk      = "ask"
	ApprovalsAuto     = "auto"
)

// AskSettings configures Ask.
type AskSettings struct {
	Model  string `json:"model"`
	Habits bool   `json:"habits"`
}

// AgentSettings configures Agent sessions.
type AgentSettings struct {
	Harness         string            `json:"harness"`
	Approvals       string            `json:"approvals"`
	IdleStopMinutes int               `json:"idleStopMinutes"`
	Instances       []HarnessInstance `json:"instances,omitempty"`
}

// DaemonSettings configures the daemon's lifetime.
type DaemonSettings struct {
	IdleExitMinutes int `json:"idleExitMinutes"`
}

// IndexSettings configures indexing.
type IndexSettings struct {
	System   bool `json:"system"`
	Semantic bool `json:"semantic"`
}

// Settings is rashin.json. Enabled is changed only by rashin on and off.
type Settings struct {
	Enabled bool           `json:"enabled"`
	Ask     AskSettings    `json:"ask"`
	Agent   AgentSettings  `json:"agent"`
	Daemon  DaemonSettings `json:"daemon"`
	Index   IndexSettings  `json:"index"`
}

// SettingsSetParams carries a JSON merge patch (RFC 7396) over Settings.
type SettingsSetParams struct {
	Patch json.RawMessage `json:"patch"`
}

// App tabs.
const (
	TabAsk      = "ask"
	TabAgent    = "agent"
	TabProjects = "projects"
	TabIndex    = "index"
	TabSettings = "settings"
)

// UIOpenParams asks a running app to show a session or tab. It is also the
// data of the ui.focus event. With Project set the Projects tab opens that
// project at View (overview, graph, changes, tasks or activity), on Worktree
// when one is named.
type UIOpenParams struct {
	Session  string `json:"session,omitempty"`
	Tab      string `json:"tab,omitempty"`
	Project  string `json:"project,omitempty"`
	View     string `json:"view,omitempty"`
	Worktree string `json:"worktree,omitempty"`
}

// UIOpenResult reports whether a running app received the request.
type UIOpenResult struct {
	Delivered bool `json:"delivered"`
}

// SessionsChanged is the data of sessions.changed: sessions that are gone.
// New and changed sessions arrive as session.info.
type SessionsChanged struct {
	Removed []string `json:"removed,omitempty"`
}

// Notice levels.
const (
	NoticeInfo  = "info"
	NoticeWarn  = "warn"
	NoticeError = "error"
)

// Notice is the data of a notice event.
type Notice struct {
	Level   string `json:"level"`
	Text    string `json:"text"`
	Session string `json:"session,omitempty"`
}
