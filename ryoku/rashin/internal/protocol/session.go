package protocol

import (
	"encoding/json"
	"time"
)

// SessionKind tells Ask sessions from Agent sessions.
type SessionKind string

const (
	KindAsk   SessionKind = "ask"
	KindAgent SessionKind = "agent"
)

// KindOfSession reads the kind out of a session id. Ids are
// <kind>-<yyyymmdd>-<hhmmss>-<4 hex>, so the kind is known before anything
// else about the session is.
func KindOfSession(id string) (SessionKind, bool) {
	for _, kind := range []SessionKind{KindAsk, KindAgent} {
		if len(id) > len(kind) && id[:len(kind)] == string(kind) && id[len(kind)] == '-' {
			return kind, true
		}
	}
	return "", false
}

// SessionState is where a session stands right now.
type SessionState string

const (
	StateIdle    SessionState = "idle"
	StateWorking SessionState = "working"
	StateWaiting SessionState = "waiting" // an open permission waits for the user
	StateStopped SessionState = "stopped" // the agent process is not running
	StateFailed  SessionState = "failed"
)

// Origins name the surface that created a session.
const (
	OriginApp      = "app"
	OriginBar      = "bar"
	OriginCLI      = "cli"
	OriginTerminal = "terminal"
	OriginFix      = "fix"
	OriginImport   = "import"
)

// SessionInfo describes one session. The daemon owns every field.
type SessionInfo struct {
	ID      string       `json:"id"`
	Kind    SessionKind  `json:"kind"`
	Title   string       `json:"title"`
	Harness string       `json:"harness,omitempty"`
	Model   string       `json:"model,omitempty"`
	Mode    string       `json:"mode,omitempty"`
	Cwd     string       `json:"cwd,omitempty"`
	Origin  string       `json:"origin"`
	State   SessionState `json:"state"`
	Created time.Time    `json:"created"`
	Updated time.Time    `json:"updated"`
	Turns   int          `json:"turns"`
	LastSeq int64        `json:"lastSeq"`
	// Preview is the latest visible text, at most 140 runes.
	Preview string `json:"preview,omitempty"`
	// Pending counts open permission requests.
	Pending int `json:"pending,omitempty"`
	// Alive is true while the agent process runs.
	Alive bool   `json:"alive,omitempty"`
	Error string `json:"error,omitempty"`
	// Project, Worktree, Branch and Task bind an Agent session to Work.
	Project  string `json:"project,omitempty"`
	Worktree string `json:"worktree,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Task     string `json:"task,omitempty"`
	// Parent is the session this one was forked or handed off from.
	Parent string `json:"parent,omitempty"`
	// Activity says what the agent is doing right now, from its latest tool.
	Activity string `json:"activity,omitempty"`
	// Unread is set when a turn ends and cleared by session.read.
	Unread bool `json:"unread,omitempty"`
	// Queue holds follow-ups sent when the running turn ends.
	Queue []QueuedPrompt `json:"queue,omitempty"`
	// Checkpoints is true when the session's turns have file checkpoints.
	Checkpoints bool `json:"checkpoints,omitempty"`
}

// SessionEvent is one line of a session log and one session.event on the wire.
type SessionEvent struct {
	Seq  int64           `json:"seq"`
	TS   time.Time       `json:"ts"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data,omitempty"`
}

// SessionEventData is the data of a session.event wire event.
type SessionEventData struct {
	Session string `json:"session"`
	SessionEvent
}

// Session event kinds.
const (
	EvUser               = "user"
	EvText               = "text"
	EvThought            = "thought"
	EvTool               = "tool"
	EvPermission         = "permission"
	EvPermissionResolved = "permission_resolved"
	EvPlan               = "plan"
	EvTerminal           = "terminal"
	EvTurnStart          = "turn_start"
	EvTurnEnd            = "turn_end"
	EvError              = "error"
	EvModel              = "model"
	EvMode               = "mode"
	EvOptions            = "options"
	EvCommands           = "commands"
	EvAnswer             = "answer"
	EvTitle              = "title"
)

// Attachment is a file on this machine sent with a prompt.
type Attachment struct {
	Path string `json:"path"`
	Mime string `json:"mime,omitempty"`
}

// TerminalContext travels with an ask made from a shell.
type TerminalContext struct {
	Cwd         string `json:"cwd,omitempty"`
	LastCommand string `json:"lastCommand,omitempty"`
	LastStatus  *int   `json:"lastStatus,omitempty"`
	Shell       string `json:"shell,omitempty"`
	// Buffer asks for a command plan the shell will place on its prompt.
	Buffer bool `json:"buffer,omitempty"`
}

// Handoff carries an Ask exchange into a new Agent session.
type Handoff struct {
	From string `json:"from"`
	// Title is the source session's title when the handoff was made.
	Title    string `json:"title,omitempty"`
	Question string `json:"question"`
	Answer   string `json:"answer,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// UserData is the data of a user event.
type UserData struct {
	Text     string           `json:"text"`
	Images   []Attachment     `json:"images,omitempty"`
	Terminal *TerminalContext `json:"terminal,omitempty"`
	Handoff  *Handoff         `json:"handoff,omitempty"`
	// Run is the command the user ran in this turn in place of a question.
	Run string `json:"run,omitempty"`
}

// TextData is the data of text and thought events: a delta of one message.
type TextData struct {
	Message string `json:"message"`
	Delta   string `json:"delta"`
}

// Tool call kinds and statuses.
const (
	ToolRead    = "read"
	ToolEdit    = "edit"
	ToolDelete  = "delete"
	ToolMove    = "move"
	ToolSearch  = "search"
	ToolExecute = "execute"
	ToolThink   = "think"
	ToolFetch   = "fetch"
	ToolOther   = "other"

	ToolPending = "pending"
	ToolRunning = "running"
	ToolDone    = "done"
	ToolFailed  = "failed"
	// ToolStopped is a subagent that was cancelled before it finished.
	ToolStopped = "stopped"
)

// ToolData is the data of a tool event. Later events for the same Call
// overwrite only the fields they carry.
type ToolData struct {
	Call      string     `json:"call"`
	Title     string     `json:"title,omitempty"`
	Kind      string     `json:"kind,omitempty"`
	Status    string     `json:"status,omitempty"`
	Input     string     `json:"input,omitempty"`
	Output    string     `json:"output,omitempty"`
	Diff      *Diff      `json:"diff,omitempty"`
	Terminal  string     `json:"terminal,omitempty"`
	Locations []Location `json:"locations,omitempty"`
	// Agents are the subagents this call runs, replaced whole on each event
	// that carries them.
	Agents []AgentWork `json:"agents,omitempty"`
	// Parent is the call whose subagent made this call; such a call belongs
	// inside its parent's work, not in the transcript.
	Parent string `json:"parent,omitempty"`
}

// AgentWork is one subagent as its harness reports it.
type AgentWork struct {
	ID string `json:"id"`
	// Name is the kind of agent, such as explore or reviewer.
	Name string `json:"name,omitempty"`
	// Task is what it was asked to do, in one line.
	Task string `json:"task,omitempty"`
	// Status is pending, running, done, failed or stopped.
	Status string `json:"status"`
	// Doing is the step it is on while it runs.
	Doing string `json:"doing,omitempty"`
	// Steps are its latest steps, oldest first.
	Steps []AgentStep `json:"steps,omitempty"`
	// StepCount counts every step it took, Steps holds the latest few.
	StepCount int    `json:"stepCount,omitempty"`
	Tokens    int    `json:"tokens,omitempty"`
	Model     string `json:"model,omitempty"`
	// Millis is how long it has run, or ran.
	Millis int64 `json:"ms,omitempty"`
	// Output is what it returned, or why it failed.
	Output string `json:"output,omitempty"`
}

// AgentStep is one step a subagent took.
type AgentStep struct {
	Title  string `json:"title"`
	Failed bool   `json:"failed,omitempty"`
}

// Diff is a file change made or proposed by a tool call.
type Diff struct {
	Path      string `json:"path"`
	Old       string `json:"old"`
	New       string `json:"new"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Location points into a file.
type Location struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
}

// Permission option kinds.
const (
	OptAllowOnce    = "allow_once"
	OptAllowAlways  = "allow_always"
	OptRejectOnce   = "reject_once"
	OptRejectAlways = "reject_always"
)

// PermissionData is the data of a permission event.
type PermissionData struct {
	Request string             `json:"request"`
	Call    string             `json:"call,omitempty"`
	Title   string             `json:"title"`
	Kind    string             `json:"kind,omitempty"`
	Input   string             `json:"input,omitempty"`
	Options []PermissionOption `json:"options"`
}

// PermissionOption is one answer to a permission request.
type PermissionOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

// PermissionResolvedData is the data of a permission_resolved event. Outcome is
// "selected" or "cancelled"; By is "user" or "policy".
type PermissionResolvedData struct {
	Request string `json:"request"`
	Option  string `json:"option,omitempty"`
	Outcome string `json:"outcome"`
	By      string `json:"by,omitempty"`
}

// PlanData is the data of a plan event: the full current plan.
type PlanData struct {
	Entries []PlanEntry `json:"entries"`
}

// PlanEntry is one step of an agent's plan. Status is pending, in_progress or
// completed.
type PlanEntry struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority string `json:"priority,omitempty"`
}

// TerminalData is the data of a terminal event.
type TerminalData struct {
	Terminal string `json:"terminal"`
	Command  string `json:"command,omitempty"`
	Delta    string `json:"delta,omitempty"`
	Exit     *int   `json:"exit,omitempty"`
	Done     bool   `json:"done,omitempty"`
}

// TurnStartData is the data of a turn_start event.
type TurnStartData struct {
	Turn int `json:"turn"`
}

// Stop reasons.
const (
	StopEnd       = "end_turn"
	StopCancelled = "cancelled"
	StopError     = "error"
	StopMaxTokens = "max_tokens"
	StopRefusal   = "refusal"
	StopMaxRounds = "max_turn_requests"
)

// TokenUsage counts tokens; Estimated marks counts the provider did not report.
type TokenUsage struct {
	Input     int  `json:"input"`
	Output    int  `json:"output"`
	Estimated bool `json:"estimated,omitempty"`
}

// TurnEndData is the data of a turn_end event.
type TurnEndData struct {
	Turn  int         `json:"turn"`
	Stop  string      `json:"stop"`
	Usage *TokenUsage `json:"usage,omitempty"`
}

// ErrorData is the data of an error event.
type ErrorData struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// Choice is one selectable option: a model, mode or route.
type Choice struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ChoiceData is the data of model and mode events.
type ChoiceData struct {
	Current string   `json:"current"`
	Options []Choice `json:"options,omitempty"`
}

// SessionOption is one setting a harness offers beside its model and mode,
// such as how hard to think. Category is the harness's own word for it.
type SessionOption struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Category string   `json:"category,omitempty"`
	Current  string   `json:"current"`
	Choices  []Choice `json:"choices,omitempty"`
}

// OptionsData is the data of an options event: every option the harness
// offers now.
type OptionsData struct {
	Options []SessionOption `json:"options"`
}

// Command is a slash command a harness offers.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Hint        string `json:"hint,omitempty"`
}

// CommandsData is the data of a commands event.
type CommandsData struct {
	Commands []Command `json:"commands"`
}

// Action kinds found in an answer.
const (
	ActCopy  = "copy"
	ActCmd   = "cmd"
	ActFile  = "file"
	ActDir   = "dir"
	ActURL   = "url"
	ActColor = "color"
)

// Action is something the user can do with an answer.
type Action struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// Command tiers, from the command classifier.
const (
	TierRead   = "read"
	TierWrite  = "write"
	TierSystem = "system"
	TierDanger = "danger"
)

// PlannedCommand is one command Ask proposes for the user to run.
type PlannedCommand struct {
	Command string `json:"command"`
	Why     string `json:"why,omitempty"`
	Tier    string `json:"tier"`
	// Terminal marks a command that cannot run in the conversation: it asks
	// for a password, or what it prints must not reach the model.
	Terminal bool `json:"terminal,omitempty"`
}

// CommandPlan is the commands Ask proposes, in order.
type CommandPlan struct {
	Summary  string           `json:"summary,omitempty"`
	Commands []PlannedCommand `json:"commands"`
}

// AnswerData is the data of an answer event (Ask sessions only).
type AnswerData struct {
	Actions    []Action     `json:"actions,omitempty"`
	Plan       *CommandPlan `json:"plan,omitempty"`
	NeedsAgent bool         `json:"needsAgent,omitempty"`
	Reason     string       `json:"reason,omitempty"`
	Via        string       `json:"via,omitempty"`
}

// TitleData is the data of a title event.
type TitleData struct {
	Title string `json:"title"`
}
