package protocol

import (
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Team is a standing group of agents with one purpose on this computer. Its
// work arrives as tasks, from the user, from its routines or from its lead,
// and every task runs as an ordinary Agent session in the team's folder.
type Team struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	// Template names the template the team started from: research, mail,
	// repo, system or blank.
	Template string `json:"template,omitempty"`
	// Folder is where the team works and keeps what it makes.
	Folder  string    `json:"folder"`
	Members []Member  `json:"members"`
	Tools   TeamTools `json:"tools"`
	// Approvals is how far its agents go before asking: an Approvals value.
	Approvals string     `json:"approvals"`
	Budget    TeamBudget `json:"budget"`
	// Notify is what reaches the desktop: all (reports and needs), needs (only
	// when it needs the user) or none.
	Notify string `json:"notify"`
	// Paused teams start no task and fire no routine; PauseReason is "you" or
	// "budget".
	Paused      bool      `json:"paused,omitempty"`
	PauseReason string    `json:"pauseReason,omitempty"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	// Running is how many of its tasks run at once, 1 to TeamsPerTeamMax; 0
	// takes TeamsSettings.PerTeam.
	Running int `json:"running,omitempty"`
	// Access widens or narrows what its agents' sandbox lets them reach.
	Access TeamAccess `json:"access"`
	// Stats is filled by the daemon from the team's tasks and spend.
	Stats TeamStats `json:"stats"`
}

// TeamAccess is what a team's sandbox adds to the defaults: folders its
// agents may change besides the team's own, and paths they see as empty
// besides TeamsSettings.Hidden. Paths are written as given, "~/" or
// absolute.
type TeamAccess struct {
	// Writable are existing folders, at most TeamWritableMax, none of them
	// "/", the home folder or inside another team's folder.
	Writable []string `json:"writable,omitempty"`
	// Hidden follows the rules of TeamsSettings.Hidden.
	Hidden []string `json:"hidden,omitempty"`
}

// TeamWritableMax is the most folders a team may change besides its own.
const TeamWritableMax = 16

// Team notification levels.
const (
	NotifyAll   = "all"
	NotifyNeeds = "needs"
	NotifyNone  = "none"
)

// Team pause reasons.
const (
	PausedByYou    = "you"
	PausedByBudget = "budget"
)

// Member is one agent of a team. The lead receives the team's tasks and may
// hand pieces to the others; members do not delegate.
type Member struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
	Lead  bool   `json:"lead,omitempty"`
	// Harness is a HarnessInfo ID, empty to follow the teams defaults; Model
	// is that harness's model id, empty for the harness's own default.
	Harness string `json:"harness"`
	Model   string `json:"model,omitempty"`
	// Options are harness option values by SessionOption ID, such as how
	// hard to think, set after the model when a task starts; an option the
	// harness refuses is left at its default.
	Options map[string]string `json:"options,omitempty"`
	// Instructions are the member's standing role: what it does and how.
	Instructions string `json:"instructions,omitempty"`
}

// Engine limits for the model and options a member, a task or the teams
// defaults name.
const (
	EngineModelMax       = 200
	EngineOptionsMax     = 8
	EngineOptionIDMax    = 64
	EngineOptionValueMax = 200
)

// ValidEngineModel reports whether model may name a harness's model: empty
// for the harness's default, else at most EngineModelMax runes without white
// space. Whether the harness offers it is only known when a task starts.
func ValidEngineModel(model string) bool {
	return utf8.RuneCountInString(model) <= EngineModelMax && strings.IndexFunc(model, unicode.IsSpace) < 0
}

// ValidEngineOptions reports whether options fit the engine limits: at most
// EngineOptionsMax entries, each id and value present and within its length.
func ValidEngineOptions(options map[string]string) bool {
	if len(options) > EngineOptionsMax {
		return false
	}
	for id, value := range options {
		if id == "" || value == "" || utf8.RuneCountInString(id) > EngineOptionIDMax || utf8.RuneCountInString(value) > EngineOptionValueMax {
			return false
		}
	}
	return true
}

// ResolveEngine is the harness, model and options a member's task starts
// with. A member that names no harness follows the teams defaults: their
// Harness, else the Agents harness, else fallback, the registry's default
// harness; its model and options, where it names none, are the defaults'
// too. A member with a harness of its own takes only its own model and
// options, empty for that harness's defaults. The daemon starts tasks and the
// app describes them through this one chain.
func ResolveEngine(member Member, settings Settings, fallback string) (harness, model string, options map[string]string) {
	if member.Harness != "" {
		return member.Harness, member.Model, member.Options
	}
	defaults := settings.Teams.Defaults
	harness = defaults.Harness
	if harness == "" {
		harness = settings.Agent.Harness
	}
	if harness == "" {
		harness = fallback
	}
	model, options = member.Model, member.Options
	if model == "" {
		model = defaults.Model
	}
	if len(options) == 0 {
		options = defaults.Options
	}
	return harness, model, options
}

// TeamTools are the abilities Rashin grants a team beyond its harness.
type TeamTools struct {
	// Screen lets its agents use a hidden screen of their own without being
	// asked for it each time.
	Screen bool `json:"screen,omitempty"`
	// Mail lets its agents send mail through the mail account; every send is
	// approved by the user unless the recipient is trusted for the team.
	Mail bool `json:"mail,omitempty"`
}

// TeamBudget limits what a team spends.
type TeamBudget struct {
	// MonthTokens caps the tokens a team's tasks use per calendar month; 0
	// is no limit. At WarnPercent the user is told; at the cap the team
	// pauses and asks.
	MonthTokens int64 `json:"monthTokens"`
	WarnPercent int   `json:"warnPercent,omitempty"`
	// TaskTurns caps the turns one task may take, 1 to TeamTaskTurnsMax.
	TaskTurns int `json:"taskTurns"`
	// TaskMinutes caps how long one task may work, 0 for no limit and at
	// most TeamTaskMinutesMax. It is always written, so a default of no
	// limit survives in the teams settings.
	TaskMinutes int `json:"taskMinutes"`
}

// Team budget limits.
const (
	TeamTaskTurnsMax   = 200
	TeamTaskMinutesMax = 24 * 60
)

// TeamStats summarise a team for lists and boards.
type TeamStats struct {
	Working     int        `json:"working"`
	Queued      int        `json:"queued"`
	NeedsYou    int        `json:"needsYou"`
	Done        int        `json:"done"`
	MonthTokens int64      `json:"monthTokens"`
	LastReport  *time.Time `json:"lastReport,omitempty"`
	NextRoutine *time.Time `json:"nextRoutine,omitempty"`
	// Doing is what each working member is on, by member id.
	Doing map[string]string `json:"doing,omitempty"`
	// Tasks counts every task the team keeps, finished or not, so a team
	// that never ran reads as such.
	Tasks int `json:"tasks"`
}

// TeamTemplate is a ready-made team a new one can start from.
type TeamTemplate struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Purpose     string         `json:"purpose"`
	Description string         `json:"description"`
	Members     []Member       `json:"members"`
	Tools       TeamTools      `json:"tools"`
	Approvals   string         `json:"approvals"`
	Budget      TeamBudget     `json:"budget"`
	Routines    []RoutineDraft `json:"routines,omitempty"`
}

// TeamTemplatesResult lists the templates in the order they are offered.
type TeamTemplatesResult struct {
	Templates []TeamTemplate `json:"templates"`
}

// TeamsResult lists teams by name.
type TeamsResult struct {
	Teams []Team `json:"teams"`
}

// TeamCreateParams creates a team. Folder defaults to ~/Teams/<name>;
// Routines are created with it, on only when Enabled says so.
type TeamCreateParams struct {
	Name      string         `json:"name"`
	Purpose   string         `json:"purpose"`
	Template  string         `json:"template,omitempty"`
	Folder    string         `json:"folder,omitempty"`
	Members   []Member       `json:"members"`
	Tools     TeamTools      `json:"tools"`
	Approvals string         `json:"approvals,omitempty"`
	Budget    TeamBudget     `json:"budget"`
	Notify    string         `json:"notify,omitempty"`
	Routines  []RoutineDraft `json:"routines,omitempty"`
	// Running is Team.Running; 0 takes TeamsSettings.PerTeam.
	Running int `json:"running,omitempty"`
	// Access is Team.Access.
	Access TeamAccess `json:"access"`
}

// TeamUpdateParams applies a JSON merge patch to a team: name, purpose,
// folder, members (the whole list), tools, approvals, budget, notify,
// running, access.
type TeamUpdateParams struct {
	ID    string          `json:"id"`
	Patch json.RawMessage `json:"patch"`
}

// TeamRef names a team.
type TeamRef struct {
	ID string `json:"id"`
}

// TeamPauseParams pauses a team or lets it work again. Pausing stops its
// running tasks; resuming a team paused by its budget needs Raise.
type TeamPauseParams struct {
	ID     string `json:"id"`
	Paused bool   `json:"paused"`
	// Raise is the new monthly token cap when a budget pause is lifted.
	Raise int64 `json:"raise,omitempty"`
}

// TeamExport is a team's definition as a portable document: no history, no
// credential values, only the names of the credentials it used.
type TeamExport struct {
	Version     int            `json:"version"`
	Name        string         `json:"name"`
	Purpose     string         `json:"purpose"`
	Template    string         `json:"template,omitempty"`
	Members     []Member       `json:"members"`
	Tools       TeamTools      `json:"tools"`
	Approvals   string         `json:"approvals"`
	Budget      TeamBudget     `json:"budget"`
	Notify      string         `json:"notify"`
	Routines    []RoutineDraft `json:"routines,omitempty"`
	Credentials []string       `json:"credentials,omitempty"`
	// Running is Team.Running.
	Running int `json:"running,omitempty"`
	// Access is Team.Access, carried as written; an import checks it again.
	Access TeamAccess `json:"access"`
}

// TeamExportResult carries the exported document and where it was written.
type TeamExportResult struct {
	Export TeamExport `json:"export"`
	Path   string     `json:"path"`
}

// TeamImportParams creates a team from an exported document read from Path.
// The team arrives paused with its routines off.
type TeamImportParams struct {
	Path string `json:"path"`
}

// Team task statuses use the Work board's vocabulary: TaskReady (queued for
// a free slot), TaskWorking, TaskNeedsYou (an approval or a credential waits
// on the user), TaskDone (reported), TaskBlocked (the agent could not finish)
// and TaskStopped.
const TaskStopped = "stopped"

// Where a team task came from.
const (
	TaskFromYou     = "you"
	TaskFromRoutine = "routine"
	TaskFromLead    = "lead"
)

// What a working team task waits on while its agent is held between turns.
const (
	WaitingOnYou     = "you"
	WaitingOnMembers = "members"
)

// TeamTask is one piece of a team's work. It runs as an Agent session; its
// result is the report its agent hands back.
type TeamTask struct {
	ID    string `json:"id"`
	Team  string `json:"team"`
	Title string `json:"title"`
	Brief string `json:"brief"`
	// Status is TaskReady, TaskWorking, TaskNeedsYou, TaskDone, TaskBlocked
	// or TaskStopped.
	Status string `json:"status"`
	// Waiting is WaitingOnYou or WaitingOnMembers while the agent is held.
	Waiting string `json:"waiting,omitempty"`
	Member  string `json:"member"`
	// Parent is the lead's task this one was delegated from.
	Parent  string `json:"parent,omitempty"`
	Origin  string `json:"origin"`
	Routine string `json:"routine,omitempty"`
	// Session is the Agent session working the task; Sessions every one that
	// did, oldest first.
	Session  string      `json:"session,omitempty"`
	Sessions []string    `json:"sessions,omitempty"`
	Report   *TeamReport `json:"report,omitempty"`
	// Reason says why a task is blocked or stopped.
	Reason   string     `json:"reason,omitempty"`
	Usage    TokenUsage `json:"usage"`
	Turns    int        `json:"turns"`
	MaxTurns int        `json:"maxTurns"`
	Created  time.Time  `json:"created"`
	Updated  time.Time  `json:"updated"`
	Started  *time.Time `json:"started,omitempty"`
	Ended    *time.Time `json:"ended,omitempty"`
	// Read is set once the user has opened the report.
	Read bool `json:"read,omitempty"`
	// Harness, Model and Options are the engine the task runs on. Before it
	// starts, Model and Options hold only what was asked for this task and
	// Harness is empty; at its start all three are set to what was resolved
	// (ResolveEngine, then this task's own Model and Options), and Model
	// follows a switch made in its session. A finished task keeps them.
	Harness string            `json:"harness,omitempty"`
	Model   string            `json:"model,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

// TeamReport is what an agent hands back when its task is done.
type TeamReport struct {
	Title string `json:"title"`
	// Body is Markdown.
	Body string `json:"body"`
	// Files are paths, inside the team's folder, of what the task made.
	Files []string  `json:"files,omitempty"`
	At    time.Time `json:"at"`
	// Quiet says nothing changed since the last run: the task ends done
	// without marking the Inbox unread or notifying.
	Quiet bool `json:"quiet,omitempty"`
}

// TeamTasksParams lists a team's tasks, or every team's when Team is empty,
// newest first. Open keeps only tasks that have not finished.
type TeamTasksParams struct {
	Team   string `json:"team,omitempty"`
	Open   bool   `json:"open,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Before string `json:"before,omitempty"`
}

// TeamTasksResult lists tasks, newest first.
type TeamTasksResult struct {
	Tasks []TeamTask `json:"tasks"`
}

// TeamTaskCreateParams gives a team a task. Member empty hands it to the
// lead. Title empty takes the brief's first line.
type TeamTaskCreateParams struct {
	Team   string `json:"team"`
	Member string `json:"member,omitempty"`
	Title  string `json:"title,omitempty"`
	Brief  string `json:"brief"`
	// Model and Options are an engine for this task only, on its member's
	// harness; empty keeps the member's.
	Model   string            `json:"model,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

// TeamTaskRef names a task.
type TeamTaskRef struct {
	ID string `json:"id"`
}

// TeamTaskRetryParams runs a finished, blocked or stopped task again in a
// new session; Note is added to its brief.
type TeamTaskRetryParams struct {
	ID   string `json:"id"`
	Note string `json:"note,omitempty"`
}

// What a routine does with a run it missed while the computer was off or
// asleep, and with a run that falls due while the last one still works.
const (
	MissedSkip   = "skip"
	MissedOnce   = "once"
	OverlapSkip  = "skip"
	OverlapQueue = "queue"
)

// Routine limits.
const (
	RoutineMinInterval = 15 * time.Minute
	RoutinesEnabledMax = 20
	RoutineFailuresMax = 5
)

// Routine is a task a team takes on by itself on a schedule.
type Routine struct {
	ID    string `json:"id"`
	Team  string `json:"team"`
	Title string `json:"title"`
	Brief string `json:"brief"`
	// Member empty hands each run to the lead.
	Member string `json:"member,omitempty"`
	// Schedule is a five-field cron expression in the computer's time zone:
	// minute hour day-of-month month day-of-week.
	Schedule string `json:"schedule"`
	Enabled  bool   `json:"enabled"`
	Missed   string `json:"missed"`
	Overlap  string `json:"overlap"`
	// Failures counts runs in a row that did not finish; at
	// RoutineFailuresMax the routine turns itself off.
	Failures   int        `json:"failures,omitempty"`
	LastRun    *time.Time `json:"lastRun,omitempty"`
	LastTask   string     `json:"lastTask,omitempty"`
	LastStatus string     `json:"lastStatus,omitempty"`
	// QuietStreak counts runs in a row that ended with a quiet report; any
	// other ending sets it back to 0.
	QuietStreak int `json:"quietStreak,omitempty"`
	// NextRun and Sentence are filled by the daemon.
	NextRun  *time.Time `json:"nextRun,omitempty"`
	Sentence string     `json:"sentence"`
	Created  time.Time  `json:"created"`
	Updated  time.Time  `json:"updated"`
}

// RoutineDraft is a routine before it belongs to a team.
type RoutineDraft struct {
	Title    string `json:"title"`
	Brief    string `json:"brief"`
	Member   string `json:"member,omitempty"`
	Schedule string `json:"schedule"`
	Enabled  bool   `json:"enabled,omitempty"`
}

// RoutinesResult lists routines by team, then title.
type RoutinesResult struct {
	Routines []Routine `json:"routines"`
}

// RoutineCreateParams creates a routine; RunNow also starts one run at once,
// so the user sees what it does.
type RoutineCreateParams struct {
	Team     string `json:"team"`
	Title    string `json:"title"`
	Brief    string `json:"brief"`
	Member   string `json:"member,omitempty"`
	Schedule string `json:"schedule"`
	Missed   string `json:"missed,omitempty"`
	Overlap  string `json:"overlap,omitempty"`
	Enabled  bool   `json:"enabled"`
	RunNow   bool   `json:"runNow,omitempty"`
}

// RoutineUpdateParams applies a JSON merge patch to a routine: title, brief,
// member, schedule, enabled, missed, overlap.
type RoutineUpdateParams struct {
	ID    string          `json:"id"`
	Patch json.RawMessage `json:"patch"`
}

// RoutineRef names a routine.
type RoutineRef struct {
	ID string `json:"id"`
}

// RoutinePreviewParams asks what a schedule means.
type RoutinePreviewParams struct {
	Schedule string `json:"schedule"`
	Count    int    `json:"count,omitempty"`
}

// RoutinePreview says a schedule in words and when it next fires, or why it
// is not a schedule.
type RoutinePreview struct {
	Sentence string      `json:"sentence,omitempty"`
	Next     []time.Time `json:"next,omitempty"`
	Error    string      `json:"error,omitempty"`
}

// Approval kinds.
const (
	ApprovalMail       = "mail"
	ApprovalCredential = "credential"
	ApprovalRequest    = "request"
	ApprovalAction     = "action"
	ApprovalBudget     = "budget"
)

// Approval statuses.
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalDenied   = "denied"
	ApprovalExpired  = "expired"
)

// Approval decisions.
const (
	DecideOnce    = "once"
	DecideAlways  = "always"
	DecideDeny    = "deny"
	DecideProvide = "provide"
	// DecideChoose answers a choice question with one of its Options.
	DecideChoose = "choose"
)

// Choice question limits.
const (
	ApprovalOptionsMax     = 6
	ApprovalOptionRunesMax = 80
)

// ValidApprovalOptions reports whether options can make a choice question:
// none, or 2 to ApprovalOptionsMax distinct options, each with text and at
// most ApprovalOptionRunesMax runes.
func ValidApprovalOptions(options []string) bool {
	if len(options) == 0 {
		return true
	}
	if len(options) < 2 || len(options) > ApprovalOptionsMax {
		return false
	}
	seen := make(map[string]bool, len(options))
	for _, option := range options {
		if strings.TrimSpace(option) == "" || utf8.RuneCountInString(option) > ApprovalOptionRunesMax || seen[option] {
			return false
		}
		seen[option] = true
	}
	return true
}

// Approval is something a team waits on the user for: an exact action it may
// take once approved, a credential it asks for, or a limit it reached.
type Approval struct {
	ID      string `json:"id"`
	Team    string `json:"team"`
	Task    string `json:"task,omitempty"`
	Session string `json:"session,omitempty"`
	Member  string `json:"member,omitempty"`
	Kind    string `json:"kind"`
	// Title says in one line what is asked; Detail shows exactly what would
	// happen, with any credential value replaced by its name.
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	// Always says what DecideAlways would allow from then on; empty when the
	// approval can only be given once.
	Always string `json:"always,omitempty"`
	Status string `json:"status"`
	// Decision and Note are the user's answer; the agent hears the note.
	Decision string `json:"decision,omitempty"`
	Note     string `json:"note,omitempty"`
	// Options make the approval a choice question: the user picks one with
	// DecideChoose, or denies them all.
	Options []string `json:"options,omitempty"`
	// Choice is the option the user picked, kept for the record.
	Choice string `json:"choice,omitempty"`
	// Result is what came of an approved action, for the record.
	Result  string     `json:"result,omitempty"`
	Created time.Time  `json:"created"`
	Decided *time.Time `json:"decided,omitempty"`
	Expires time.Time  `json:"expires"`
	// Unwatched marks an approval asked while no Rashin window was open.
	Unwatched bool `json:"unwatched,omitempty"`
}

// ApprovalsParams lists approvals, newest first; Pending keeps only those
// still waiting.
type ApprovalsParams struct {
	Team    string `json:"team,omitempty"`
	Pending bool   `json:"pending,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// ApprovalsResult lists approvals, newest first.
type ApprovalsResult struct {
	Approvals []Approval `json:"approvals"`
}

// ApprovalDecideParams answers an approval. Decision is DecideOnce,
// DecideAlways, DecideDeny, DecideProvide with Value for a credential
// request, or DecideChoose with Choice, one of the approval's Options. The
// agent is then told "The user chose: <option>" and any note. Value never
// comes back from any method.
type ApprovalDecideParams struct {
	ID       string `json:"id"`
	Decision string `json:"decision"`
	Note     string `json:"note,omitempty"`
	Value    string `json:"value,omitempty"`
	Choice   string `json:"choice,omitempty"`
}

// Who started what an activity entry records.
const (
	InitiatorYou     = "you"
	InitiatorRoutine = "routine"
	InitiatorAgent   = "agent"
	InitiatorRashin  = "rashin"
)

// Activity filters for teams.activity.
const (
	ActivityAll         = "all"
	ActivityUnwatched   = "unwatched"
	ActivityApprovals   = "approvals"
	ActivityCredentials = "credentials"
	ActivityFailures    = "failures"
)

// TeamActivityParams pages the teams' ledger, newest first.
type TeamActivityParams struct {
	Team   string `json:"team,omitempty"`
	Filter string `json:"filter,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Before string `json:"before,omitempty"`
}

// TeamSpendParams asks for a team's spend in a month (YYYY-MM, empty for
// this one), or every team's when Team is empty.
type TeamSpendParams struct {
	Team  string `json:"team,omitempty"`
	Month string `json:"month,omitempty"`
}

// DaySpend is one day's tokens.
type DaySpend struct {
	Day    string `json:"day"`
	Input  int64  `json:"input"`
	Output int64  `json:"output"`
}

// NamedSpend is the tokens and runs of one member, routine, team or engine.
type NamedSpend struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Tokens int64  `json:"tokens"`
	Tasks  int    `json:"tasks"`
}

// TeamSpend is a month of a team's spend against its cap.
type TeamSpend struct {
	Team     string       `json:"team,omitempty"`
	Month    string       `json:"month"`
	Tokens   int64        `json:"tokens"`
	Cap      int64        `json:"cap"`
	Days     []DaySpend   `json:"days"`
	Members  []NamedSpend `json:"members,omitempty"`
	Routines []NamedSpend `json:"routines,omitempty"`
	Teams    []NamedSpend `json:"teams,omitempty"`
	// Engines is the spend by the engine tasks ran on: ID is the harness id
	// and the model id joined by EngineSpendSeparator, the model empty for
	// the harness's default; Name is left for the app to word.
	Engines []NamedSpend `json:"engines,omitempty"`
	// Estimated is set when a harness did not report some of its tokens.
	Estimated bool `json:"estimated,omitempty"`
}

// EngineSpendSeparator joins a harness id and a model id in an Engines ID.
const EngineSpendSeparator = "\x1f"

// TeamPruneParams deletes finished tasks with their sessions: those of Team,
// or of every team when it is empty, that ended more than KeepDays ago, or
// every finished one with All. What they made stays in the team's folder.
type TeamPruneParams struct {
	Team string `json:"team,omitempty"`
	All  bool   `json:"all,omitempty"`
}

// TeamRemoved is the data of team.removed, routine.removed and
// team.task.removed: the id that is gone.
type TeamRemoved struct {
	ID string `json:"id"`
}

// The methods below are called by the rashin MCP server on behalf of the
// agent of a team task. Session and Token identify the run; the daemon
// refuses a token that does not belong to that session's live run.

// AgentRef identifies the calling run.
type AgentRef struct {
	Session string `json:"session"`
	Token   string `json:"token"`
}

// AgentReportParams hands the task's report back; it ends the task. Quiet
// says nothing changed since the last run.
type AgentReportParams struct {
	AgentRef
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Files []string `json:"files,omitempty"`
	Quiet bool     `json:"quiet,omitempty"`
}

// AgentDelegateParams gives a member of the lead's team one piece of the
// lead's task.
type AgentDelegateParams struct {
	AgentRef
	Member string `json:"member"`
	Title  string `json:"title"`
	Brief  string `json:"brief"`
}

// AgentAskParams asks the user to decide before the agent acts. With
// Options it is a choice question (ValidApprovalOptions): the agent is told
// "The user chose: <option>" and any note the user added.
type AgentAskParams struct {
	AgentRef
	Title   string   `json:"title"`
	Detail  string   `json:"detail,omitempty"`
	Options []string `json:"options,omitempty"`
}

// AgentHTTPParams makes one HTTPS request with a credential the team may
// use. Rashin adds the credential the way the vault says, only for the
// credential's hosts, and never shows the value to the agent. SaveAs, a
// path relative to the team's folder, saves the whole answer there.
type AgentHTTPParams struct {
	AgentRef
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Credential string            `json:"credential"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	SaveAs     string            `json:"saveAs,omitempty"`
}

// AgentHTTPResult is the response, with any credential value removed.
// Waiting is set instead when the request waits on the user's approval.
type AgentHTTPResult struct {
	Status  int               `json:"status,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	// Truncated is set when the body was longer than the agent is shown.
	Truncated bool `json:"truncated,omitempty"`
	// Saved says how much of the answer went into the team's folder, and
	// where, when the request asked to save it.
	Saved   string `json:"saved,omitempty"`
	Waiting string `json:"waiting,omitempty"`
}

// AgentMailParams sends one mail through the mail account once the user
// approves it, or at once to a recipient trusted for the team.
type AgentMailParams struct {
	AgentRef
	To      []string `json:"to"`
	Cc      []string `json:"cc,omitempty"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

// AgentCredentialRequestParams asks the user to add a credential the team
// needs; the user types the value into Rashin, never into the agent.
type AgentCredentialRequestParams struct {
	AgentRef
	Name   string   `json:"name"`
	Reason string   `json:"reason"`
	Hosts  []string `json:"hosts"`
}

// AgentAnswer is the text the agent is shown for a call that does not return
// data.
type AgentAnswer struct {
	Text string `json:"text"`
}

// AgentStatusResult is the work the calling task delegated, with each
// member task's state and report.
type AgentStatusResult struct {
	Tasks []TeamTask `json:"tasks"`
}

// AgentCredential is what an agent may know about a credential: its name,
// what it is for and where it may be sent.
type AgentCredential struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Hosts       []string `json:"hosts"`
	// Trusted are the hosts the team may send it to without asking.
	Trusted []string `json:"trusted,omitempty"`
}

// AgentCredentialsResult lists the credentials the calling team may use.
type AgentCredentialsResult struct {
	Credentials []AgentCredential `json:"credentials"`
	Mail        bool              `json:"mail"`
}
