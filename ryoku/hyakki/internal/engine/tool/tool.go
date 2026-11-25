// Package tool is the contract between the engine and its tools: what a tool
// is, what it is given for one call (Env), what it returns (Result), how
// tools are registered and picked per mode and model class (Registry,
// Profile, the canonical name table in names.go), and the caps that scale
// tool output to the context window (Limits).
//
// A tool never touches files or starts processes itself: it goes through
// Env.Exec, which is the user's own machine for a normal session and the
// task's sandbox for a team task, so one implementation serves both.
package tool

import (
	"context"
	"encoding/json"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// Kind is a protocol tool kind (protocol.ToolRead ... protocol.ToolOther):
// the icon and folding of the tool line in the app.
type Kind = string

// Tool is one function the model may call.
type Tool interface {
	// Name is the canonical name (names.go); Spec().Name must equal it.
	Name() string
	// Spec is the schema the model sees for this profile: a compact profile
	// gets one-line descriptions and flat schemas, a full one everything.
	Spec(p Profile) llm.Tool
	Kind() Kind
	// ReadOnly reports that the tool changes nothing of the user's outside
	// Rashin's own state (memory, skills, todos, notes and teams count as
	// Rashin's), or that the daemon service it calls owns the consent (the
	// screen asks before it acts). It never asks through Permissions and is
	// the only kind a ReadOnly profile offers. Only write, edit, bash,
	// run_command and write_file are not ReadOnly; such a tool must call
	// Env.Permissions.Allow before it changes anything.
	ReadOnly() bool
	// Present makes the tool line from the call's arguments, before the tool
	// runs: a short title, the input shown under it, and the files it
	// touches. It must not fail; on bad arguments it shows them raw.
	Present(args json.RawMessage) Presentation
	// Run performs the call. Failures the model can act on are a Result with
	// IsError, never a panic; ctx ends when the turn is cancelled.
	Run(ctx context.Context, call Call, env Env) Result
}

// Parallel is an opt-in for tools whose calls are independent when the model
// emits several in one round. ReadOnly alone does not imply independence.
type Parallel interface {
	Parallel() bool
}

// Presentation is the tool line before a call runs.
type Presentation struct {
	Title     string
	Input     string
	Locations []protocol.Location
}

// Conditional is a Tool offered only to some sessions: the memory tools when
// memory is on for the session, team tools in team tasks, learn when the
// autolearn feature is on, and so on. A tool without it is always offered
// where the mode's table names it.
type Conditional interface {
	Enabled(env Env) bool
}

// Provider supplies tools that are not known when the registry is built:
// the user's MCP servers (feature mcp). Their names are the provider's own,
// never one of the canonical names; a clash drops the provider's tool.
type Provider interface {
	Tools(ctx context.Context, env Env) []Tool
}

// Call is one tool call of the model.
type Call struct {
	ID   string
	Name string
	// Args is the JSON object the model produced, already repaired by the
	// gateway's text-call parser when the model wrote calls as text.
	Args json.RawMessage
	// Parent is the task call whose subagent made this call, "" otherwise.
	Parent string
}

// Result is what a call returned.
type Result struct {
	// Text is told to the model. The engine cuts it at Env.Limits.Told as a
	// last resort; shape it below that (Limits.Head/Tail) yourself.
	Text string
	// Shown is the tool line's output in the app; "" shows Text. Cut at
	// Limits.Shown runes.
	Shown string
	// Images go back to the model with the tool result (screen_look, an
	// image read); the gateway strips them for a model without vision.
	Images  []llm.Image
	IsError bool
	// Diff is a file change made or proposed, shown inline.
	Diff      *protocol.Diff
	Locations []protocol.Location
	// Plan replaces the session's plan: emitted as a plan event (todo).
	Plan *protocol.PlanData
	// Proposal is the commands Ask proposes (propose); it becomes
	// AnswerData.Plan at the end of the turn.
	Proposal *protocol.CommandPlan
	// Agents are the subagents this call ran, final state (task); progress
	// goes out through Env.Emit while it runs.
	Agents []protocol.AgentWork
	// Usage is model use the call spent itself (subagents), added to the
	// turn's usage.
	Usage llm.Usage
	// End stops the turn after this round: the model is not called again
	// (propose handing back in Ask, yield in a subagent).
	End bool
}

// Failed is a Result the model reads as an error.
func Failed(text string) Result { return Result{Text: text, IsError: true} }

// Env is everything one call is given. The engine builds it per call; a
// tool must not keep it past Run.
type Env struct {
	Session protocol.SessionInfo
	// Cwd is the session's working folder: relative paths resolve against
	// it.
	Cwd string
	// Project is the Git top level that holds Cwd, "" outside Git.
	Project string
	Profile Profile
	// Limits are Profile.Limits(), computed once per turn.
	Limits Limits
	// Exec does every file and process operation.
	Exec exec.Executor
	// Sandbox is the team task's sandbox Exec runs in; nil runs as the user.
	Sandbox *sandbox.Spec
	// Context says which in-process Rashin tools the session has.
	Context core.ToolContext
	// Sink reports events beside the tool line: terminal output, memory
	// lines.
	Sink core.Sink
	// Emit updates this call's tool line; the engine fills Call and Parent.
	Emit func(protocol.ToolData)
	// Permissions asks the user, under the session's approvals.
	Permissions Permissions
	Index       core.Indexer
	// Host calls daemon methods as this session.
	Host     Host
	Settings func() protocol.Settings
	// Backup keeps what a file held before a tool changed it, in the
	// session's before/ folder, and returns where.
	Backup func(path string, previous []byte) (string, error)
	// Spawner runs subagents (task); nil where the mode has none.
	Spawner Spawner
}

// Feature reports whether an optional feature (protocol.Feature*) is on.
func (e Env) Feature(id string) bool {
	if e.Settings == nil {
		return false
	}
	return e.Settings().Features[id]
}

// Host calls one daemon method with the session's identity: memory as
// MemoryCaller{Session}, teams.agent_* with the run token, the team gate
// applied to a team session. params and result are encoded as JSON.
type Host interface {
	Call(ctx context.Context, method string, params, result any) error
}

// Permissions is the engine's approval gate for one session.
type Permissions interface {
	// Allow decides whether a change may run: by the session's approvals
	// alone (reads never wait, danger always does, auto runs, ask waits
	// unless the user allowed the conversation) or by asking the user with a
	// permission card and waiting. It returns an error only when ctx ends.
	Allow(ctx context.Context, req Request) (Decision, error)
	// Choose shows a question card with options and waits for the user's
	// pick (the ask tool). It returns the chosen Option.ID.
	Choose(ctx context.Context, q Question) (string, error)
}

// Request is one change a tool asks to make.
type Request struct {
	Call Call
	// Title is the card's line, Input what it shows under it.
	Title string
	Kind  Kind
	Input string
	// Tier is the command classifier's tier (protocol.Tier*): a write tool
	// says protocol.TierWrite, a command its classified tier.
	Tier string
	// Diff, shown on the card before the user decides.
	Diff *protocol.Diff
}

// Decision is the gate's answer. Reason, when refused, is told to the model
// as final. By is "user" or "policy".
type Decision struct {
	Allowed bool
	Reason  string
	By      string
}

// Question is a card that asks the user to pick one option. It rides on the
// permission card (one option picked, no free text), so multi-select and
// typed answers are not offered.
type Question struct {
	Call     Call
	Question string
	Options  []protocol.Choice
	// Recommended is the ID of the option the model suggests, "" for none:
	// the engine lists it first and marks its label as recommended.
	Recommended string
}

// Spawner runs subagents in process, inside the parent's turn.
type Spawner interface {
	// Spawn runs one child to its end. Its tool calls are emitted with
	// Parent = req.Parent; progress reports its AgentWork after each step.
	Spawn(ctx context.Context, req SpawnRequest, progress func(protocol.AgentWork)) (SpawnResult, error)
}

// SpawnRequest describes one subagent.
type SpawnRequest struct {
	// ID is the AgentWork id, unique within the parent call.
	ID string
	// Name is the kind of agent, such as explore or reviewer.
	Name string
	// Task is what it is asked to do; Context is extra background.
	Task    string
	Context string
	// Write lets it change files, when the parent's profile is not ReadOnly;
	// its changes still pass the parent's Permissions.
	Write bool
	// Model is "" for the parent's model.
	Model string
	// Parent is the task call's ID.
	Parent string
}

// SpawnResult is what a subagent returned through yield, or its last text.
type SpawnResult struct {
	Output string
	Failed bool
	Work   protocol.AgentWork
	Usage  llm.Usage
}
