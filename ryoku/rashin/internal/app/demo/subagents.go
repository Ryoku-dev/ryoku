package demo

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// subagentEvents is a turn that hands work to helpers: three running side by
// side the way omp reports them and one whose own calls Claude Code reports
func subagentEvents(after int64) []protocol.SessionEvent {
	seq := after
	next := func(kind string, data any) protocol.SessionEvent {
		seq++
		return event(seq, kind, data)
	}
	return []protocol.SessionEvent{
		next(protocol.EvUser, protocol.UserData{Text: i18n.T("Map the IPC server, then make the help output match the registered commands.")}),
		next(protocol.EvTurnStart, protocol.TurnStartData{Turn: 2}),
		next(protocol.EvTool, protocol.ToolData{
			Call: "task-ipc", Title: "task", Kind: protocol.ToolOther, Status: protocol.ToolRunning,
			Agents: []protocol.AgentWork{
				{
					ID: "ipc", Name: "explore", Task: i18n.T("Map how the IPC server reads a command"), Status: protocol.ToolRunning,
					Doing: i18n.T("Read src/app/application.cpp"), StepCount: 3, Tokens: 4800, Model: "fast", Millis: 12400,
					Steps: []protocol.AgentStep{{Title: i18n.T("Read src/ipc/ipc_service.h")}, {Title: i18n.T("Read src/ipc/ipc_service.cpp")}, {Title: i18n.T("Search for registerHandler")}},
				},
				{
					ID: "paths", Name: "explore", Task: i18n.T("Find where the socket path is chosen"), Status: protocol.ToolDone,
					StepCount: 3, Tokens: 3600, Model: "fast", Millis: 8100,
					Steps:  []protocol.AgentStep{{Title: i18n.T("Search for resolveSocketPath")}, {Title: i18n.T("Read src/ipc/ipc_service.cpp")}, {Title: i18n.T("Read src/core/paths.cpp")}},
					Output: i18n.T("resolveSocketPath() prefers $XDG_RUNTIME_DIR/noctalia.sock and falls back to /tmp/noctalia-<uid>.sock."),
				},
				{
					ID: "tests", Name: "reviewer", Task: i18n.T("Check the IPC tests still cover help output"), Status: protocol.ToolFailed,
					StepCount: 2, Tokens: 2400, Model: "fast", Millis: 5300,
					Steps:  []protocol.AgentStep{{Title: i18n.T("Search tests for buildHelp")}, {Title: i18n.T("Run ctest -R ipc"), Failed: true}},
					Output: i18n.T("ctest exited 8: the ipc_help test expects the old usage line."),
				},
			},
		}),
		next(protocol.EvTool, protocol.ToolData{
			Call: "agent-ipc", Title: i18n.T("Review the help text"), Kind: protocol.ToolThink, Status: protocol.ToolRunning,
			Agents: []protocol.AgentWork{{
				ID: "agent-ipc", Name: "Explore", Task: i18n.T("Review the help text against the registered commands"), Status: protocol.ToolRunning,
				Doing: i18n.T("Read src/ipc/ipc_service.cpp"), StepCount: 2, Millis: 6200,
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
