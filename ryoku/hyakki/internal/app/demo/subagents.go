package demo

import (
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// demoGoal is the goal the working demo session follows
func demoGoal(now time.Time) *protocol.Goal {
	return &protocol.Goal{Objective: i18n.T("Map the IPC server, then make the help output match the registered commands."), Status: protocol.GoalActive, Turns: 1, MaxTurns: 30, Started: now.Add(-6 * time.Minute)}
}

// subagentEvents shows native task delegation and tool activity inside its child work
func subagentEvents(after int64, now time.Time) []protocol.SessionEvent {
	seq := after
	next := func(kind string, data any) protocol.SessionEvent {
		seq++
		return event(seq, kind, data)
	}
	set := *demoGoal(now)
	set.Turns = 0
	return []protocol.SessionEvent{
		next(protocol.EvGoal, protocol.GoalData{Goal: set}),
		next(protocol.EvUser, protocol.UserData{Text: set.Objective}),
		next(protocol.EvTurnStart, protocol.TurnStartData{Turn: 2}),
		next(protocol.EvText, protocol.TextData{Message: "goal-1", Delta: i18n.T("I read the IPC service and the help builder. Next I map every registered command.")}),
		next(protocol.EvTurnEnd, protocol.TurnEndData{Turn: 2, Stop: protocol.StopEnd}),
		next(protocol.EvUser, protocol.UserData{Text: i18n.Tf("Continuing the goal, turn %d of %d", 2, 30), Goal: true}),
		next(protocol.EvTurnStart, protocol.TurnStartData{Turn: 3}),
		next(protocol.EvTool, protocol.ToolData{
			Call: "task-ipc", Title: "task", Kind: protocol.ToolOther, Status: protocol.ToolRunning,
			Agents: []protocol.AgentWork{
				{
					ID: "ipc", Name: "explore", Task: i18n.T("Map how the IPC server reads a command"), Status: protocol.ToolRunning,
					Doing: i18n.T("Read src/app/application.cpp"), StepCount: 3, Tokens: 4800, Model: demoLocalID, Millis: 12400,
					Steps: []protocol.AgentStep{{Title: i18n.T("Read src/ipc/ipc_service.h")}, {Title: i18n.T("Read src/ipc/ipc_service.cpp")}, {Title: i18n.T("Search for registerHandler")}},
				},
				{
					ID: "paths", Name: "explore", Task: i18n.T("Find where the socket path is chosen"), Status: protocol.ToolDone,
					StepCount: 3, Tokens: 3600, Model: demoLocalID, Millis: 8100,
					Steps:  []protocol.AgentStep{{Title: i18n.T("Search for resolveSocketPath")}, {Title: i18n.T("Read src/ipc/ipc_service.cpp")}, {Title: i18n.T("Read src/core/paths.cpp")}},
					Output: i18n.T("resolveSocketPath() prefers $XDG_RUNTIME_DIR/noctalia.sock and falls back to /tmp/noctalia-<uid>.sock."),
				},
				{
					ID: "tests", Name: "reviewer", Task: i18n.T("Check the IPC tests still cover help output"), Status: protocol.ToolFailed,
					StepCount: 2, Tokens: 2400, Model: demoFreeID, Millis: 5300,
					Steps:  []protocol.AgentStep{{Title: i18n.T("Search tests for buildHelp")}, {Title: i18n.T("Run ctest -R ipc"), Failed: true}},
					Output: i18n.T("ctest exited 8: the ipc_help test expects the old usage line."),
				},
			},
		}),
		next(protocol.EvTool, protocol.ToolData{
			Call: "agent-ipc", Title: "task", Kind: protocol.ToolOther, Status: protocol.ToolRunning,
			Agents: []protocol.AgentWork{{
				ID: "agent-ipc", Name: "explore", Task: i18n.T("Review the help text against the registered commands"), Status: protocol.ToolRunning,
				Doing: i18n.T("Read src/ipc/ipc_service.cpp"), StepCount: 2, Model: demoLocalID, Millis: 6200,
				Steps: []protocol.AgentStep{{Title: i18n.T("Search for buildHelp")}, {Title: i18n.T("Read src/ipc/ipc_service.cpp")}},
			}},
		}),
		next(protocol.EvTool, protocol.ToolData{
			Call: "agent-ipc-1", Parent: "agent-ipc", Title: i18n.T("Search for buildHelp"), Kind: protocol.ToolSearch,
			Status: protocol.ToolDone, Input: "buildHelp", Output: "src/ipc/ipc_service.cpp:158",
		}),
		next(protocol.EvTool, protocol.ToolData{
			Call: "agent-ipc-2", Parent: "agent-ipc", Title: i18n.T("Read src/ipc/ipc_service.cpp"), Kind: protocol.ToolRead,
			Status: protocol.ToolRunning, Input: "src/ipc/ipc_service.cpp",
		}),
	}
}
