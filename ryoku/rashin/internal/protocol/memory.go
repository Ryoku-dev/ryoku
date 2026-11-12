package protocol

import "time"

// Memory scopes. A scope is MemoryYou, MemoryProjectPrefix plus a project's
// key, or MemoryTeamPrefix plus a team id. A project's key follows its
// repository (its first commit), not the folder it sits in.
const (
	MemoryYou           = "you"
	MemoryProjectPrefix = "project:"
	MemoryTeamPrefix    = "team:"
)

// Groups of scopes a list may ask for besides one exact scope.
const (
	MemoryProjects = "projects"
	MemoryTeams    = "teams"
)

// Memory states. An active memory is used. A suggested one waits for the user
// to keep or dismiss it. A replaced one was superseded by a newer fact and is
// kept for MemoryReplacedDays. A blocked one was written by hand into
// something that reads as an instruction or a secret, and is never used.
const (
	MemoryActive    = "active"
	MemorySuggested = "suggested"
	MemoryReplaced  = "replaced"
	MemoryBlocked   = "blocked"
)

// MemoryReplacedDays is how long a replaced memory waits before it is
// forgotten.
const MemoryReplacedDays = 90

// MemoryTextMax is the longest a memory may be, in characters.
const MemoryTextMax = 280

// MemoryCardMax is the longest the card about the user may be, in characters.
const MemoryCardMax = 1375

// How long a memory is meant to last: Memory.Kind.
const (
	MemoryPermanent = "permanent"
	MemoryDurable   = "durable"
	MemoryEphemeral = "ephemeral"
)

// Where a memory came from: MemorySource.Kind.
const (
	MemoryFromAsk    = "ask"
	MemoryFromAgent  = "agent"
	MemoryFromTeam   = "team"
	MemoryFromYou    = "you"
	MemoryFromFile   = "file"
	MemoryFromImport = "import"
)

// What a conversation does with memory: SessionInfo.Memory. Empty remembers:
// it uses memory and is learned from. MemoryUseOnly uses memory and is never
// learned from; MemoryNone does neither.
const (
	MemoryRemember = ""
	MemoryUseOnly  = "use"
	MemoryNone     = "off"
)

// Which model learns: MemorySettings.Model. LearnLocal waits for a local
// model; LearnLocalFirst falls back to the model that already answered the
// conversation; LearnAny uses Ask's own route.
const (
	LearnLocal      = "local"
	LearnLocalFirst = "localFirst"
	LearnAny        = "any"
)

// What keeps memory: MemorySettings.Engine.
const (
	MemoryBuiltin   = "builtin"
	MemoryHindsight = "hindsight"
	MemoryHoncho    = "honcho"
)

// Engine states: MemoryStatus.EngineState.
const (
	EngineReady       = "ready"
	EngineStarting    = "starting"
	EngineStopped     = "stopped"
	EngineSetup       = "setup"
	EngineUnreachable = "unreachable"
)

// Learner states: MemoryLearner.State.
const (
	LearnerReady   = "ready"
	LearnerWaiting = "waiting"
	LearnerOff     = "off"
)

// What teams may do to the user's memory: MemorySettings.TeamSuggestions.
const (
	TeamSuggestAsk   = "ask"
	TeamSuggestNever = "never"
)

// What a project shares: ProjectSettings.Memory. Empty shares.
const (
	ProjectMemoryShared = ""
	ProjectMemoryKept   = "kept"
)

// MemorySettings are memory's settings; only the Rashin app may change them.
// Until Asked is set by the first-run card, memory is off whatever Use says.
type MemorySettings struct {
	Asked bool `json:"asked"`
	// Use lets answers draw on memory; off keeps what is stored and uses none
	// of it, and nothing is learned.
	Use bool `json:"use"`
	// Learn reads quiet conversations and remembers what will help later.
	Learn bool `json:"learn"`
	// Sessions lets Ask and agents search past conversations.
	Sessions bool `json:"sessions"`
	// Agents gives harness sessions the card and the memory tools.
	Agents bool `json:"agents"`
	// TeamSuggestions is TeamSuggestAsk or TeamSuggestNever.
	TeamSuggestions string `json:"teamSuggestions"`
	// Sensitive keeps health, beliefs, money, relationships and similar
	// topics. Secrets are never kept.
	Sensitive bool `json:"sensitive"`
	// Model is LearnLocal, LearnLocalFirst or LearnAny.
	Model string `json:"model"`
	// FadeMonths forgets a memory nothing used for that long; 0 never does.
	FadeMonths int `json:"fadeMonths"`
	// QuietMinutes is how long a conversation rests before it is learned
	// from.
	QuietMinutes int `json:"quietMinutes"`
	// Engine is MemoryBuiltin, MemoryHindsight or MemoryHoncho.
	Engine    string            `json:"engine"`
	Hindsight HindsightSettings `json:"hindsight"`
	Honcho    HonchoSettings    `json:"honcho"`
}

// HindsightSettings say how Hindsight keeps memory: run on this computer by
// Rashin (Local), or reached at URL with the vault credential named Key.
type HindsightSettings struct {
	Local bool   `json:"local"`
	URL   string `json:"url,omitempty"`
	Key   string `json:"key,omitempty"`
}

// HonchoSettings point memory at a Honcho server: its address, the workspace
// Rashin keeps to itself, and the name of the vault credential holding its
// key. A key never travels in settings.
type HonchoSettings struct {
	URL       string `json:"url,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Key       string `json:"key,omitempty"`
}

// MemorySource says where a memory came from: a conversation and its turn, a
// team task, the user, a hand edit of the file, or an import from another
// assistant (From). Model is the model that learned it.
type MemorySource struct {
	Kind    string `json:"kind"`
	Session string `json:"session,omitempty"`
	Turn    int    `json:"turn,omitempty"`
	Title   string `json:"title,omitempty"`
	Team    string `json:"team,omitempty"`
	Task    string `json:"task,omitempty"`
	From    string `json:"from,omitempty"`
	Model   string `json:"model,omitempty"`
}

// MemoryRef names a conversation memory was used in or read from.
type MemoryRef struct {
	Session string      `json:"session"`
	Kind    SessionKind `json:"kind,omitempty"`
	Title   string      `json:"title,omitempty"`
}

// MemoryVersion is an earlier text of a memory, newest first. By is "you",
// "learning" or "file".
type MemoryVersion struct {
	Text string    `json:"text"`
	At   time.Time `json:"at"`
	By   string    `json:"by"`
}

// Memory is one fact Rashin remembers. Text may hold Markdown code spans for
// paths and commands; nothing else in it is formatted.
type Memory struct {
	ID string `json:"id"`
	// Scope is the exact scope; ScopeName says it in words: "About you", a
	// project's or a team's name.
	Scope     string       `json:"scope"`
	ScopeName string       `json:"scopeName"`
	Text      string       `json:"text"`
	Status    string       `json:"status"`
	Kind      string       `json:"kind,omitempty"`
	Pinned    bool         `json:"pinned,omitempty"`
	Sensitive bool         `json:"sensitive,omitempty"`
	Source    MemorySource `json:"source"`
	Created   time.Time    `json:"created"`
	Updated   time.Time    `json:"updated"`
	// Uses counts answers that drew on it; LastUsedIn is the latest.
	Uses       int        `json:"uses,omitempty"`
	LastUsed   *time.Time `json:"lastUsed,omitempty"`
	LastUsedIn *MemoryRef `json:"lastUsedIn,omitempty"`
	// Replaced, ReplacedBy and ReplacedByText describe a replaced memory.
	Replaced       *time.Time      `json:"replaced,omitempty"`
	ReplacedBy     string          `json:"replacedBy,omitempty"`
	ReplacedByText string          `json:"replacedByText,omitempty"`
	Versions       []MemoryVersion `json:"versions,omitempty"`
	// Blocked says why a hand-edited line is not used.
	Blocked string `json:"blocked,omitempty"`
}

// MemoryCaps are what the engine in use can do; the app greys out what it
// cannot and says why.
type MemoryCaps struct {
	Edit          bool `json:"edit"`
	Replace       bool `json:"replace"`
	Pin           bool `json:"pin"`
	Versions      bool `json:"versions"`
	ForgetSession bool `json:"forgetSession"`
	// LeavesMachine is set when what is remembered is sent off this
	// computer.
	LeavesMachine bool `json:"leavesMachine"`
}

// MemoryCounts count memories by scope group and state.
type MemoryCounts struct {
	All       int `json:"all"`
	You       int `json:"you"`
	Projects  int `json:"projects"`
	Teams     int `json:"teams"`
	Suggested int `json:"suggested"`
	Replaced  int `json:"replaced"`
	Blocked   int `json:"blocked"`
	// LearnedMonth and ForgotMonth count the last 30 days.
	LearnedMonth int `json:"learnedMonth"`
	ForgotMonth  int `json:"forgotMonth"`
}

// MemoryScope is one scope with memories in it. Mode is a project's
// ProjectSettings.Memory.
type MemoryScope struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
	Mode  string `json:"mode,omitempty"`
}

// MemoryDay is one day of the memory chart: active memories at its end, and
// how many were learned and forgotten on it. Day is YYYY-MM-DD, local time.
type MemoryDay struct {
	Day     string `json:"day"`
	InUse   int    `json:"inUse"`
	Learned int    `json:"learned"`
	Forgot  int    `json:"forgot"`
}

// MemoryLearner is the learning model's state. Waiting counts conversations
// waiting to be learned from; Model is the one last used or about to be.
type MemoryLearner struct {
	State   string     `json:"state"`
	Model   string     `json:"model,omitempty"`
	Local   bool       `json:"local,omitempty"`
	Waiting int        `json:"waiting,omitempty"`
	Last    *time.Time `json:"last,omitempty"`
	Detail  string     `json:"detail,omitempty"`
}

// MemoryCard is the card about the user that Ask and agents are given. It is
// built from the list; From counts the memories in it.
type MemoryCard struct {
	Text  string    `json:"text"`
	Built time.Time `json:"built"`
	From  int       `json:"from"`
	Limit int       `json:"limit"`
}

// MemoryStatus is memory as a whole: the snapshot carries it and every
// memory.changed event brings it again.
type MemoryStatus struct {
	Engine       string        `json:"engine"`
	EngineState  string        `json:"engineState"`
	EngineDetail string        `json:"engineDetail,omitempty"`
	Caps         MemoryCaps    `json:"caps"`
	Counts       MemoryCounts  `json:"counts"`
	Scopes       []MemoryScope `json:"scopes,omitempty"`
	Days         []MemoryDay   `json:"days,omitempty"`
	Learner      MemoryLearner `json:"learner"`
	Card         MemoryCard    `json:"card"`
	// Path is the folder holding the memory files, Bytes their size.
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// What one MemoryItem of a change did.
const (
	MemoryOpRemembered = "remembered"
	MemoryOpReplaced   = "replaced"
	MemoryOpForgot     = "forgot"
	MemoryOpEdited     = "edited"
	MemoryOpSuggested  = "suggested"
	MemoryOpKept       = "kept"
	MemoryOpDismissed  = "dismissed"
	MemoryOpRestored   = "restored"
	MemoryOpPinned     = "pinned"
	MemoryOpUnpinned   = "unpinned"
)

// What started a change: MemoryChange.Origin.
const (
	MemoryByLearning = "learning"
	MemoryByTool     = "tool"
	MemoryByYou      = "you"
	MemoryByFile     = "file"
	MemoryByImport   = "import"
	MemoryByTeam     = "team"
	MemoryByFading   = "fading"
)

// MemoryItem is one memory a change touched. Replaces is the text a
// remembered memory replaced.
type MemoryItem struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Scope     string `json:"scope"`
	ScopeName string `json:"scopeName"`
	Op        string `json:"op"`
	Replaces  string `json:"replaces,omitempty"`
}

// MemoryChange is one write to memory, which Undo reverses whole: what a
// learning pass, a remember, an edit or a forget did, and where.
type MemoryChange struct {
	ID      string       `json:"id"`
	At      time.Time    `json:"at"`
	Origin  string       `json:"origin"`
	Session string       `json:"session,omitempty"`
	Turn    int          `json:"turn,omitempty"`
	Team    string       `json:"team,omitempty"`
	Task    string       `json:"task,omitempty"`
	Model   string       `json:"model,omitempty"`
	Items   []MemoryItem `json:"items"`
	Undone  bool         `json:"undone,omitempty"`
}

// MemoryChanged is the memory.changed event: the status after a change, and
// the change when one write caused it.
type MemoryChanged struct {
	Status MemoryStatus  `json:"status"`
	Change *MemoryChange `json:"change,omitempty"`
}

// What a memory session event reports: MemoryEventData.Kind.
const (
	MemoryUpdated  = "updated"
	MemoryUsed     = "used"
	MemoryLooked   = "looked"
	MemoryRefused  = "refused"
	MemoryNarrowed = "narrowed"
)

// Why a memory was not kept: MemoryEventData.Reason.
const (
	MemoryRefusedSecret      = "secret"
	MemoryRefusedSensitive   = "sensitive"
	MemoryRefusedInstruction = "instruction"
	MemoryRefusedOff         = "off"
)

// MemoryEventData is the data of a memory session event. Updated carries the
// change that was written, Used what the answer drew on (Card is set when the
// card was in the prompt), Looked the past conversations that were opened,
// Refused why Text was not kept, and Narrowed the conversation's new Memory
// mode.
type MemoryEventData struct {
	Kind     string        `json:"kind"`
	Change   *MemoryChange `json:"change,omitempty"`
	Used     []MemoryItem  `json:"used,omitempty"`
	Card     bool          `json:"card,omitempty"`
	Sessions []MemoryRef   `json:"sessions,omitempty"`
	Reason   string        `json:"reason,omitempty"`
	Text     string        `json:"text,omitempty"`
	Memory   string        `json:"memory,omitempty"`
}

// Sorts of memory.list.
const (
	MemorySortNewest = "newest"
	MemorySortOldest = "oldest"
	MemorySortUsed   = "used"
	MemorySortUnused = "unused"
)

// MemoryListParams filters memory.list. Scope is empty for every scope,
// MemoryYou, MemoryProjects, MemoryTeams or one exact scope; Status is empty
// for active memories; Session keeps memories learned from that conversation.
type MemoryListParams struct {
	Scope   string `json:"scope,omitempty"`
	Status  string `json:"status,omitempty"`
	Query   string `json:"query,omitempty"`
	Sort    string `json:"sort,omitempty"`
	Session string `json:"session,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Offset  int    `json:"offset,omitempty"`
}

// MemoryListResult is one page of memories and how many matched in all.
type MemoryListResult struct {
	Memories []Memory `json:"memories"`
	Total    int      `json:"total"`
}

// MemoryRefParams names one memory.
type MemoryRefParams struct {
	ID string `json:"id"`
}

// MemoryIDsParams names memories.
type MemoryIDsParams struct {
	IDs []string `json:"ids"`
}

// MemoryAddParams remembers Text in Scope. Session names the conversation the
// user asked in, so the change shows there; Sensitive says the text is about
// health, beliefs, money, relationships or the like.
type MemoryAddParams struct {
	Scope     string `json:"scope"`
	Text      string `json:"text"`
	Pinned    bool   `json:"pinned,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
	Session   string `json:"session,omitempty"`
}

// MemoryEditParams changes a memory's text, scope or pin. A nil field stays.
type MemoryEditParams struct {
	ID     string  `json:"id"`
	Text   *string `json:"text,omitempty"`
	Scope  *string `json:"scope,omitempty"`
	Pinned *bool   `json:"pinned,omitempty"`
}

// MemoryForgetParams forgets memories. Session names the conversation the
// user asked in, so the change shows there.
type MemoryForgetParams struct {
	IDs     []string `json:"ids"`
	Session string   `json:"session,omitempty"`
}

// MemoryRestoreParams brings back a replaced memory (Version 0) or makes an
// earlier text (Version n, counted from 1, newest first) the current one.
type MemoryRestoreParams struct {
	ID      string `json:"id"`
	Version int    `json:"version,omitempty"`
}

// MemoryChangeParams names one change, to undo or redo.
type MemoryChangeParams struct {
	Change string `json:"change"`
}

// MemoryChangesParams asks for changes by id.
type MemoryChangesParams struct {
	IDs []string `json:"ids"`
}

// MemoryChangesResult holds the changes asked for that still exist.
type MemoryChangesResult struct {
	Changes []MemoryChange `json:"changes"`
}

// MemoryForgetAllParams forgets every memory in Scope, or everything when
// Scope is empty, with earlier versions and suggestions.
type MemoryForgetAllParams struct {
	Scope string `json:"scope,omitempty"`
}

// MemorySearchParams searches active memories by meaning and words.
type MemorySearchParams struct {
	Query string `json:"query"`
	Scope string `json:"scope,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// MemorySearchResult is best first.
type MemorySearchResult struct {
	Memories []Memory `json:"memories"`
}

// MemorySessionsParams searches past conversations. Last limits them to a
// recent span: 6h, 7d, 2w or 1m.
type MemorySessionsParams struct {
	Query string `json:"query"`
	Last  string `json:"last,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// MemorySessionHit is one past conversation that matched: its summary and the
// lines that matched, already scrubbed.
type MemorySessionHit struct {
	Session string      `json:"session"`
	Kind    SessionKind `json:"kind"`
	Title   string      `json:"title"`
	Updated time.Time   `json:"updated"`
	Summary string      `json:"summary,omitempty"`
	Lines   []string    `json:"lines,omitempty"`
}

// MemorySessionsResult is best first.
type MemorySessionsResult struct {
	Sessions []MemorySessionHit `json:"sessions"`
}

// MemoryCardParams asks for the card; Rebuild builds it again first.
type MemoryCardParams struct {
	Rebuild bool `json:"rebuild,omitempty"`
}

// MemoryImportParams splits what another assistant (From) said it remembers
// into suggestions in Scope.
type MemoryImportParams struct {
	Text  string `json:"text"`
	From  string `json:"from,omitempty"`
	Scope string `json:"scope,omitempty"`
}

// MemorySuppressParams stops a memory being used in one conversation.
type MemorySuppressParams struct {
	Session string `json:"session"`
	ID      string `json:"id"`
}

// MemoryLearnParams learns from a conversation now instead of when it goes
// quiet.
type MemoryLearnParams struct {
	Session string `json:"session"`
}

// SetMemoryParams sets what a conversation does with memory. Before its first
// message any mode may be picked; after it, only a narrower one.
type SetMemoryParams struct {
	ID     string `json:"id"`
	Memory string `json:"memory"`
}

// MemoryEngineParams names an engine: MemoryBuiltin, MemoryHindsight or
// MemoryHoncho.
type MemoryEngineParams struct {
	Engine string `json:"engine"`
}

// MemoryEngineTest says whether an engine answered and, if not, why.
type MemoryEngineTest struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// MemorySetupProgress is the memory.setup event: how far setting an engine
// up has come. Done is set on the last one, with Error when it failed.
type MemorySetupProgress struct {
	Engine string `json:"engine"`
	Step   string `json:"step"`
	Part   int    `json:"part"`
	Parts  int    `json:"parts"`
	Done   bool   `json:"done,omitempty"`
	Error  string `json:"error,omitempty"`
}

// TeamMemory is what a team keeps between runs and what it may know of the
// user. Remember gives members the team's memory and its tools; CarryReport
// starts a routine's run with the last run's report; ReadsYou adds the card
// about the user to members' briefs; Suggest lets members suggest memories
// about the user or a project, each waiting for the user.
type TeamMemory struct {
	Remember    bool `json:"remember"`
	CarryReport bool `json:"carryReport"`
	ReadsYou    bool `json:"readsYou"`
	Suggest     bool `json:"suggest"`
}

// DefaultTeamMemory is what a team keeps when nothing says otherwise: it
// remembers between runs, carries the last report and may suggest, and knows
// nothing of the user.
var DefaultTeamMemory = TeamMemory{Remember: true, CarryReport: true, Suggest: true}

// TeamMemory returns what the team keeps, DefaultTeamMemory for a team made
// before memory.
func (t Team) TeamMemory() TeamMemory {
	if t.Memory == nil {
		return DefaultTeamMemory
	}
	return *t.Memory
}

// ValidHonchoID reports whether id may name a Honcho workspace, peer or
// session: letters, digits, - and _, at most 512 of them.
func ValidHonchoID(id string) bool {
	if id == "" || len(id) > 512 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
