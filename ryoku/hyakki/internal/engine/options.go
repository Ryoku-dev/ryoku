package engine

import (
	"encoding/json"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
	"ryoku-rashin/internal/skills"
)

// Deps is what the daemon hands the engine once at start. New(Deps)
// *Factory, in engine.go, keeps it; the Factory serves core.Asker (NewAsk)
// and core.Agents (NewAgent). Nil services leave their tools and hooks out.
type Deps struct {
	// Gateway answers every model call, gives the window of a route
	// (Window) and the model list for SetModel and the class pick.
	Gateway core.Gateway
	Index   core.Indexer
	Memory  core.Memory
	Teams   core.Teams
	Skills  *skills.Library

	// Tools holds every built-in tool and the MCP provider; the daemon
	// builds it from the internal/engine/tools packages.
	Tools *tool.Registry
	// Executor opens a session's executor: in-process as the user when spec
	// is nil, else the tool host client for that sandbox (started on the
	// first call). The runner closes it on Stop.
	Executor func(info protocol.SessionInfo, spec *sandbox.Spec) exec.Executor
	// Host is the daemon's method dispatch with info's identity.
	Host func(info protocol.SessionInfo) tool.Host

	// Context adds system-prompt blocks, in this order: skills, decisions,
	// rules, memory.
	Context []ContextProvider
	// Guards are the stream rules.
	Guards []StreamGuards
	// Observers learn from finished turns.
	Observers []TurnObserver
	// Judge gives light verdicts; nil limits unexpected-stop detection to
	// its mechanical checks.
	Judge Judge
	// Ask is the Ask mode pieces (internal/engine/ask, ask.New): the
	// finisher, user runs and Theia. AskHooks is declared in EngineCore's
	// askhooks.go. Nil runs Ask without them.
	Ask AskHooks
}

// Options is one runner's resolved configuration. NewAsk builds it from
// core.AskOptions, NewAgent from core.AgentOptions, and the task tool's
// Spawn from the parent's, so every runner, child or not, is built the
// same way.
type Options struct {
	Session protocol.SessionInfo
	// Mode is where the runner starts: ModeForAsk or ModeForAgent. An Agent
	// moves between agent and plan as Plan first is set and its plan
	// approved; the rest keep their mode.
	Mode     tool.Mode
	Cwd      string
	Settings func() protocol.Settings
	// Approvals is read at each request: the session's own for Ask and team
	// sessions, agent.approvals otherwise.
	Approvals func() string
	// Options are the session options by id (protocol.OptionPlan,
	// protocol.OptionEffort).
	Options map[string]string
	// Brief is the session part of the system prompt.
	Brief func() string
	// Resume is the State a previous runner of this session returned.
	Resume json.RawMessage
	Tools  core.ToolContext
	MCP    []core.MCPServer
	// Sandbox, set for a team task, is where every tool runs.
	Sandbox *sandbox.Spec
}

// ModeForAsk is an Ask's mode: ask-read unless its approvals let it act. A
// session with no known approvals value is read-only, as Ask always was.
func ModeForAsk(info protocol.SessionInfo) tool.Mode {
	switch info.Mode {
	case protocol.ApprovalsAsk, protocol.ApprovalsAuto:
		return tool.ModeAskAct
	default:
		return tool.ModeAskRead
	}
}

// ModeForAgent is an Agent session's starting mode: dream-manager for a
// dream's lead, team for any other team task, plan when Plan first is on,
// agent otherwise.
func ModeForAgent(opts core.AgentOptions) tool.Mode {
	switch team := opts.Tools.Team; {
	case team != nil && team.Dream && team.Lead:
		return tool.ModeDreamManager
	case team != nil:
		return tool.ModeTeam
	case opts.Options[protocol.OptionPlan] == protocol.OptionOn:
		return tool.ModePlan
	default:
		return tool.ModeAgent
	}
}

// ModeProfile is what a mode is beyond its tools: EngineCore's modes.go
// holds one per tool.Mode.
type ModeProfile struct {
	Mode tool.Mode
	// ReadOnly offers read-only tools only (ask-read, plan; a subagent of a
	// read-only parent gets ReadOnly set on its Profile instead).
	ReadOnly bool
	// Rounds caps model rounds per turn: 8 for ask-read and 14 for ask-act,
	// as Ask had; protocol.StopMaxRounds past it.
	Rounds int
	// MaxOutput caps max_tokens; 0 leaves it to the model and the output
	// budget (prompt + max_tokens never exceeds the window).
	MaxOutput int
	// Prompt names the system prompt template EnginePrompts renders for the
	// mode; the class picks its compact or full variant.
	Prompt string
}

// CompactTemperature is the most a compact-class request asks for, per the
// small-model rules.
const CompactTemperature = 0.3
