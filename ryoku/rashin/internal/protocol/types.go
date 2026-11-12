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

// APIInfo is the state of the local API: the loopback address other programs
// send OpenAI-style chat completions to. It answers only while the daemon
// runs. Error says why it is on and not listening, as when the port is taken.
type APIInfo struct {
	Enabled   bool   `json:"enabled"`
	Listening bool   `json:"listening"`
	URL       string `json:"url"`
	Port      int    `json:"port"`
	Error     string `json:"error,omitempty"`
}

// APIKeyParams asks for the local API's key. Rotate replaces it first, which
// cuts off every program that holds the old one.
type APIKeyParams struct {
	Rotate bool `json:"rotate,omitempty"`
}

// APIKey is the key other programs present to the local API.
type APIKey struct {
	Key string `json:"key"`
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
	API       APIInfo       `json:"api"`
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
	// Screens are the agent screens that are up.
	Screens []ScreenState `json:"screens,omitempty"`
	// Teams, their open and recent tasks, pending and recent approvals,
	// routines and the vault without any value.
	Teams     []Team     `json:"teams,omitempty"`
	TeamTasks []TeamTask `json:"teamTasks,omitempty"`
	Approvals []Approval `json:"approvals,omitempty"`
	Routines  []Routine  `json:"routines,omitempty"`
	Vault     *VaultInfo `json:"vault,omitempty"`
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
	// Mode is what a new Ask session may do: an Approvals value. Empty is
	// read-only.
	Mode   string `json:"mode,omitempty"`
	Title  string `json:"title,omitempty"`
	Origin string `json:"origin,omitempty"`
	// Project binds an Agent session to a project. Worktree is "" (the project
	// root, or Cwd without a project), "new" (create one from Base, named Name,
	// on Branch or the project's prefix plus Name), or an existing worktree path.
	Project  string `json:"project,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Base     string `json:"base,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Name     string `json:"name,omitempty"`
	Task     string `json:"task,omitempty"`
	// Team, TeamTask and Member bind an Agent session to the team task it
	// works; Approvals overrides the agent approvals setting for it.
	Team      string `json:"team,omitempty"`
	TeamTask  string `json:"teamTask,omitempty"`
	Member    string `json:"member,omitempty"`
	Approvals string `json:"approvals,omitempty"`
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
	// Run is a command the user runs in an Ask conversation, in place of a
	// question: it runs in the background, its output shows as it comes, and
	// Ask answers from it.
	Run string `json:"run,omitempty"`
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

// SetModeParams picks a mode for a session: one the harness offers, or for
// Ask an Approvals value.
type SetModeParams struct {
	ID   string `json:"id"`
	Mode string `json:"mode"`
}

// SetOptionParams sets one option an agent session's harness offers.
type SetOptionParams struct {
	ID     string `json:"id"`
	Option string `json:"option"`
	Value  string `json:"value"`
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

// ProviderInfo describes one provider and its state here. Tier is the
// provider's own (free, credits or paid; local for a server on this computer
// or the local network); a model may carry another, as ModelInfo.Tier says.
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
	// Local marks a model server on this computer or the local network.
	// Address is where Rashin reaches its OpenAI-compatible API, and Running
	// says it answered the last time Rashin asked for its models.
	Local   bool   `json:"local,omitempty"`
	Address string `json:"address,omitempty"`
	Running bool   `json:"running,omitempty"`
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

// Model tiers. Auto, Fast and Smart route among free, credits and local
// models only; a paid model answers only when it is the model asked for.
const (
	// TierFree is a model free to use, with or without a key.
	TierFree = "free"
	// TierCredits is a model paid for with credits a provider renews for free.
	TierCredits = "credits"
	// TierPaid is a model the key's owner pays for.
	TierPaid = "paid"
	// TierLocal is a model served on this computer or the local network.
	TierLocal = "local"
)

// ModelInfo describes one chat model. Tier is one of the model tiers.
type ModelInfo struct {
	ID            string     `json:"id"`
	Provider      string     `json:"provider"`
	Name          string     `json:"name"`
	Tier          string     `json:"tier"`
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
	// Capabilities are learned from the agent's initialize answer each time
	// it runs in the user's own home, and kept per binary. A team's runs keep
	// their own in TeamHome.Capabilities and never change these.
	Capabilities *HarnessCapabilities `json:"capabilities,omitempty"`
	// Setup is what the harness's own settings say it will use, read from
	// its files and never written.
	Setup []HarnessFact `json:"setup,omitempty"`
	// State are the paths under HOME the harness keeps its own state in,
	// written "~/.omp". Every team sandbox hides them, for every harness.
	State []string `json:"state,omitempty"`
	// Teams is how teams run the harness, nil for one that is not installed.
	Teams *HarnessTeams `json:"teams,omitempty"`
}

// HarnessTeams is how teams run a harness. Each team has a home of its own
// for each harness it uses: team sessions and Check run the harness with HOME
// there, so every team signs in apart from the user's own setup, which every
// team sandbox hides, and apart from every other team.
type HarnessTeams struct {
	// Login is the command a sign-in runs in a team's home; empty when Rashin
	// knows none, which leaves the setup.
	Login []string `json:"login,omitempty"`
	// Guarded are the paths under a team home, relative to it, that Rashin
	// puts back as they were signed in before every team task and shows
	// read-only while one runs.
	Guarded []string `json:"guarded,omitempty"`
	// Homes are the team homes that exist, by team. A team without one is
	// not signed in.
	Homes []TeamHome `json:"homes,omitempty"`
}

// TeamHome is one team's home for one harness.
type TeamHome struct {
	Team string `json:"team"`
	Home string `json:"home"`
	// SignedIn is when the user last finished a sign-in or the setup there;
	// nil until then and while one is open. A team task on a harness its
	// team is not signed in to ends blocked.
	SignedIn *time.Time `json:"signedIn,omitempty"`
	// Opening is set while a sign-in or setup terminal is open.
	Opening bool `json:"opening,omitempty"`
	// Capabilities are what the harness reported when it last ran in this
	// home, kept per binary; nil until a task or Check ran it there.
	Capabilities *HarnessCapabilities `json:"capabilities,omitempty"`
}

// TeamCapabilities is what a harness reported in team's home, nil until it
// ran there. It never falls back to the user's own: a team's account may
// offer other models.
func (h HarnessInfo) TeamCapabilities(team string) *HarnessCapabilities {
	home, _ := h.Teams.HomeOf(team)
	return home.Capabilities
}

// HomeOf is team's home for the harness, false when it has none.
func (t *HarnessTeams) HomeOf(team string) (TeamHome, bool) {
	if t == nil {
		return TeamHome{}, false
	}
	for _, home := range t.Homes {
		if home.Team == team {
			return home, true
		}
	}
	return TeamHome{}, false
}

// HarnessTeamsOpenParams opens a terminal in a team's home for a harness:
// its sign-in, or with Setup a shell there.
type HarnessTeamsOpenParams struct {
	// ID is a HarnessInfo ID; Team the team whose home it is.
	ID    string `json:"id"`
	Team  string `json:"team"`
	Setup bool   `json:"setup,omitempty"`
	// CopyFrom, with Setup, names another team signed in to the same
	// harness: the guarded config it signed in with (never its login) is
	// copied into this team's home first, for setups that keep a key in
	// config.
	CopyFrom string `json:"copyFrom,omitempty"`
}

// HarnessTeamsOpenResult is what the app runs in a terminal: Argv ends by
// calling harnesses.teams_done with the token it carries.
type HarnessTeamsOpenResult struct {
	Argv []string `json:"argv"`
	Home string   `json:"home"`
}

// HarnessTeamsDoneParams finishes a sign-in or setup: the token its terminal
// was given and the exit status of what ran there.
type HarnessTeamsDoneParams struct {
	Token  string `json:"token"`
	Status int    `json:"status"`
}

// HarnessTeamsDoneResult says whether the harness is signed in for teams now.
type HarnessTeamsDoneResult struct {
	SignedIn bool   `json:"signedIn"`
	Text     string `json:"text"`
}

// HarnessFact is one line of a harness's own setup: a label and its value.
type HarnessFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// HarnessCapabilities is what an agent advertised at initialize.
type HarnessCapabilities struct {
	LoadSession bool     `json:"loadSession,omitempty"`
	Images      bool     `json:"images,omitempty"`
	Audio       bool     `json:"audio,omitempty"`
	Context     bool     `json:"context,omitempty"`
	Modes       []Choice `json:"modes,omitempty"`
	Models      bool     `json:"models,omitempty"`
	// Started is the model and the option values the agent came up with the
	// last time it ran.
	Started []HarnessFact `json:"started,omitempty"`
	// ModelList is every model the agent offered at its last start, in its
	// own order.
	ModelList []Choice `json:"modelList,omitempty"`
	// DefaultModel is the model id a new session of the agent starts on. It
	// is learned only from a new session, never from a resumed one, which
	// reports the model it was left on; empty until then.
	DefaultModel string `json:"defaultModel,omitempty"`
	// Options are the settings the agent offers beside its model and mode,
	// each with its choices and the value a new session starts with.
	Options []SessionOption `json:"options,omitempty"`
	// Learned is when these capabilities were recorded.
	Learned time.Time `json:"learned,omitzero"`
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

// HarnessProbeParams starts a harness once in a team's sandbox, without a
// prompt, to read its models and check that Model and Options are taken.
type HarnessProbeParams struct {
	// ID is a HarnessInfo ID.
	ID string `json:"id"`
	// Team is the team whose home the harness runs in; a Check without one
	// says the team has to be saved and signed in first.
	Team string `json:"team,omitempty"`
	// Model and Options, when set, are picked as a team task would pick
	// them.
	Model   string            `json:"model,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

// HarnessProbeResult is the harness as the probe left it, with the
// capabilities it just reported. OK is false when it did not start or
// refused the model or an option; Message then says why in the harness's
// own words.
type HarnessProbeResult struct {
	Harness HarnessInfo `json:"harness"`
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
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
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Root    string   `json:"root,omitempty"`
	Roots   []string `json:"roots,omitempty"`
	Kind    string   `json:"kind"`
	State   string   `json:"state"`
	Files   int      `json:"files"`
	Chunks  int      `json:"chunks"`
	Symbols int      `json:"symbols"`
	Vectors int      `json:"vectors"`
	// Languages counts the indexed files of each language, most first.
	Languages []Count    `json:"languages,omitempty"`
	Updated   *time.Time `json:"updated,omitempty"`
	Error     string     `json:"error,omitempty"`
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

// KnownApprovals reports whether value is one of the three approval policies.
func KnownApprovals(value string) bool {
	return value == ApprovalsReadOnly || value == ApprovalsAsk || value == ApprovalsAuto
}

// AskSettings configures Ask. Approvals is what a new Ask in the app may do;
// asks from the bar and the terminal stay read-only.
type AskSettings struct {
	Model     string `json:"model"`
	Habits    bool   `json:"habits"`
	Approvals string `json:"approvals"`
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

// APISettings configures the local API. It is off until asked for. While it
// is on and listening, the daemon does not exit at idle.
type APISettings struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

// DefaultAPIPort is where the local API listens unless the setting says otherwise.
const DefaultAPIPort = 3600

// TeamsSettings are the defaults and limits every team shares.
type TeamsSettings struct {
	// MaxRunning is how many team tasks run at once across all teams, 1 to
	// TeamsMaxRunningMax.
	MaxRunning int `json:"maxRunning"`
	// PerTeam is how many tasks one team runs at once unless the team sets
	// its own Running, 1 to TeamsPerTeamMax and never above MaxRunning.
	PerTeam int `json:"perTeam"`
	// Paused holds every team: no task starts and no routine fires while it
	// is set, whatever each team's own pause says.
	Paused bool `json:"paused,omitempty"`
	// RoutinesWake lets a timer start Rashin at login and while a routine is
	// on. It is true unless the user turned it off, so it is always written.
	RoutinesWake bool `json:"routinesWake"`
	// Defaults are what a new team starts with when it names nothing else.
	// A member left on the default harness keeps following Defaults.Harness,
	// Model and Options.
	Defaults TeamDefaults `json:"defaults"`
	// ApprovalHours is how long a question waits for the user before it
	// counts as no, 1 to TeamsApprovalHoursMax.
	ApprovalHours int `json:"approvalHours"`
	// QuietHours, when set, holds desktop notifications back between From
	// and To; what needs the user still waits in the Inbox.
	QuietHours *QuietHours `json:"quietHours,omitempty"`
	// Hidden are the user's own paths every team agent sees as empty, written
	// as given ("~/Private" or absolute), beside what Rashin always hides:
	// TeamsAlwaysHidden and every harness's own state.
	Hidden []string `json:"hidden"`
	// KeepDays is how long finished tasks and their sessions are kept: one
	// of TeamsKeepDays, 0 keeping them forever.
	KeepDays int `json:"keepDays"`
}

// TeamDefaults are what a new team starts with, and the engine of every
// member left on the default harness.
type TeamDefaults struct {
	// Harness is a HarnessInfo ID; empty runs the Agents harness
	// (AgentSettings.Harness), else the registry's default.
	Harness string `json:"harness,omitempty"`
	// Model and Options belong to the harness Harness resolves to; empty
	// takes that harness's own defaults.
	Model   string            `json:"model,omitempty"`
	Options map[string]string `json:"options,omitempty"`
	// Approvals is an Approvals value.
	Approvals string     `json:"approvals"`
	Budget    TeamBudget `json:"budget"`
	// Notify is NotifyAll, NotifyNeeds or NotifyNone.
	Notify string `json:"notify"`
}

// QuietHours is a daily window in local time, "HH:MM" each way. To before
// From crosses midnight.
type QuietHours struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Covers reports whether t falls inside the window: From inclusive, To
// exclusive. A window whose times do not parse or are equal covers nothing.
func (q QuietHours) Covers(t time.Time) bool {
	from, okFrom := ParseClock(q.From)
	to, okTo := ParseClock(q.To)
	if !okFrom || !okTo || from == to {
		return false
	}
	now := t.Hour()*60 + t.Minute()
	if from < to {
		return now >= from && now < to
	}
	return now >= from || now < to
}

// ParseClock reads "HH:MM" on a 24-hour clock, two digits each, as minutes
// after midnight.
func ParseClock(value string) (int, bool) {
	if len(value) != len("15:04") {
		return 0, false
	}
	clock, err := time.Parse("15:04", value)
	if err != nil {
		return 0, false
	}
	return clock.Hour()*60 + clock.Minute(), true
}

// Teams settings defaults and limits.
const (
	// DefaultTeamsMaxRunning is how many team tasks run at once by default.
	DefaultTeamsMaxRunning = 3
	// TeamsMaxRunningMax keeps team tasks from starving the desktop they
	// share.
	TeamsMaxRunningMax = 6
	// DefaultTeamsPerTeam is how many tasks one team runs at once by default.
	DefaultTeamsPerTeam = 2
	// TeamsPerTeamMax is the most tasks one team may run at once.
	TeamsPerTeamMax = 4
	// DefaultApprovalHours is how long a question waits by default.
	DefaultApprovalHours = 24
	// TeamsApprovalHoursMax is a week.
	TeamsApprovalHoursMax = 168
	// TeamsHiddenMax is the most paths the teams settings or one team may
	// hide.
	TeamsHiddenMax = 64
)

// TeamsKeepDays are the values KeepDays may take; 0 keeps every finished
// task.
var TeamsKeepDays = []int{0, 30, 90, 365}

// Kinds of secrets every team sandbox hides.
const (
	HiddenKeys      = "keys"
	HiddenGit       = "git"
	HiddenCloud     = "cloud"
	HiddenPackages  = "packages"
	HiddenBrowsers  = "browsers"
	HiddenMail      = "mail"
	HiddenChat      = "chat"
	HiddenHistories = "histories"
	HiddenRyoku     = "ryoku"
)

// HiddenGroup is one kind of secret and the paths that hold it.
type HiddenGroup struct {
	Kind  string   `json:"kind"`
	Paths []string `json:"paths"`
}

// TeamsAlwaysHidden are the places credentials and private state live, hidden
// from every team agent and every harness Check whatever the settings say,
// as each sandbox shares the network and anything it reads can leave. Paths
// are under the user's home, written "~/". Every harness's own state is
// hidden as well (HarnessInfo.State).
func TeamsAlwaysHidden() []HiddenGroup {
	return []HiddenGroup{
		{Kind: HiddenKeys, Paths: []string{
			"~/.ssh", "~/.gnupg", "~/.password-store", "~/.local/share/keyrings", "~/.local/share/kwalletd",
			"~/.pki", "~/.config/Bitwarden", "~/.config/Bitwarden CLI", "~/.config/op", "~/.config/1Password",
			"~/.1password", "~/.config/keepassxc", "~/.local/share/gopass", "~/.config/gopass", "~/.config/sops",
		}},
		{Kind: HiddenGit, Paths: []string{
			"~/.git-credentials", "~/.config/git/credentials", "~/.netrc", "~/.config/gh", "~/.config/hub",
			"~/.config/glab-cli",
		}},
		{Kind: HiddenCloud, Paths: []string{
			"~/.aws", "~/.config/gcloud", "~/.azure", "~/.kube", "~/.docker", "~/.terraform.d", "~/.terraformrc",
			"~/.vault-token", "~/.config/rclone", "~/.config/doctl", "~/.config/hcloud", "~/.oci", "~/.fly",
			"~/.wrangler", "~/.config/.wrangler", "~/.supabase", "~/.local/share/com.vercel.cli",
		}},
		{Kind: HiddenPackages, Paths: []string{
			"~/.npmrc", "~/.yarnrc", "~/.yarnrc.yml", "~/.pypirc", "~/.cargo/credentials", "~/.cargo/credentials.toml",
			"~/.gem/credentials", "~/.m2/settings.xml", "~/.m2/settings-security.xml", "~/.gradle/gradle.properties",
			"~/.config/composer/auth.json", "~/.pub-cache/credentials.json", "~/.config/dart/pub-credentials.json",
			"~/.nuget/NuGet/NuGet.Config",
		}},
		{Kind: HiddenBrowsers, Paths: []string{
			"~/.mozilla", "~/.zen", "~/.librewolf", "~/.waterfox", "~/.floorp", "~/.config/chromium",
			"~/.config/google-chrome", "~/.config/google-chrome-beta", "~/.config/google-chrome-unstable",
			"~/.config/BraveSoftware", "~/.config/vivaldi", "~/.config/vivaldi-snapshot", "~/.config/microsoft-edge",
			"~/.config/microsoft-edge-beta", "~/.config/microsoft-edge-dev", "~/.config/opera", "~/.config/thorium",
			"~/.local/share/epiphany", "~/.var/app",
		}},
		{Kind: HiddenMail, Paths: []string{
			"~/.thunderbird", "~/.local/share/evolution", "~/.config/evolution", "~/.local/share/geary",
			"~/.config/geary", "~/.config/aerc", "~/.msmtprc", "~/.config/msmtp", "~/.mbsyncrc", "~/.muttrc",
			"~/.config/mutt", "~/.config/neomutt",
		}},
		{Kind: HiddenChat, Paths: []string{
			"~/.config/discord", "~/.config/discordcanary", "~/.config/discordptb", "~/.config/vesktop",
			"~/.config/Vencord", "~/.config/equibop", "~/.config/legcord", "~/.config/WebCord", "~/.config/Slack",
			"~/.config/Signal", "~/.local/share/TelegramDesktop", "~/.config/Element", "~/.config/Element-Nightly",
		}},
		{Kind: HiddenHistories, Paths: []string{
			"~/.bash_history", "~/.zsh_history", "~/.zhistory", "~/.histfile", "~/.local/share/fish/fish_history",
			"~/.local/share/atuin", "~/.python_history", "~/.node_repl_history", "~/.psql_history",
			"~/.mysql_history", "~/.sqlite_history", "~/.lesshst",
		}},
		{Kind: HiddenRyoku, Paths: []string{
			"~/.local/state/ryoku", "~/.local/share/ryoku/rashin", "~/.local/share/ryoku/vault", "~/.cache/ryoku/rashin",
			"~/Teams", "~/.config/dev.ryoku.ryotunes", "~/.local/share/dev.ryoku.ryotunes", "~/.local/state/ryotunes",
		}},
	}
}

// TeamsAlwaysHiddenPaths are every path of TeamsAlwaysHidden, in order.
func TeamsAlwaysHiddenPaths() []string {
	var out []string
	for _, group := range TeamsAlwaysHidden() {
		out = append(out, group.Paths...)
	}
	return out
}

// Settings is rashin.json. Enabled is changed only by rashin on and off.
type Settings struct {
	Enabled   bool              `json:"enabled"`
	Ask       AskSettings       `json:"ask"`
	Agent     AgentSettings     `json:"agent"`
	Daemon    DaemonSettings    `json:"daemon"`
	Index     IndexSettings     `json:"index"`
	API       APISettings       `json:"api"`
	Teams     TeamsSettings     `json:"teams"`
	Providers ProvidersSettings `json:"providers"`
}

// ProvidersSettings are the providers' settings that are not keys. Local maps
// a local provider's id to the address of its OpenAI-compatible API; only the
// Rashin app may change one, since the gateway sends requests there.
type ProvidersSettings struct {
	Local map[string]string `json:"local,omitempty"`
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

	// Teams mode
	TabInbox    = "inbox"
	TabTeams    = "teams"
	TabWork     = "work"
	TabRoutines = "routines"
	TabVault    = "vault"
	TabActivity = "activity"
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
	// Team, TeamTask, Routine and Credential open Teams mode on one of them;
	// View then names a team page tab.
	Team       string `json:"team,omitempty"`
	TeamTask   string `json:"teamTask,omitempty"`
	Routine    string `json:"routine,omitempty"`
	Credential string `json:"credential,omitempty"`
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
	// Team and TeamTask name what a Teams notice is about.
	Team     string `json:"team,omitempty"`
	TeamTask string `json:"teamTask,omitempty"`
}
