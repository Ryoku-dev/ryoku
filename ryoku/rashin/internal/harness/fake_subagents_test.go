package harness

import (
	"os"
	"strconv"
	"time"
)

// fakeAgentPace stretches the scripted subagent runs so a person watching the
// app sees them work; tests leave it at zero
func fakeAgentPace() time.Duration {
	millis, _ := strconv.Atoi(os.Getenv("RASHIN_ACP_TEST_PACE_MS"))
	return time.Duration(millis) * time.Millisecond
}

type fakeAgentPlan struct {
	id, agent, task string
	steps           []string
	fail            bool
	output          string
}

var fakeAgentPlans = []fakeAgentPlan{
	{"ipc", "explore", "Map how the IPC server reads a command", []string{"Read src/ipc/ipc_service.h", "Read src/ipc/ipc_service.cpp", "Search for registerHandler", "Read src/app/application.cpp"}, false, "The server binds a Unix socket in start(), reads one line per connection in handleConnection() and runs it through the handler registry; each command is registered with registerHandler()."},
	{"paths", "explore", "Find where the socket path is chosen", []string{"Search for resolveSocketPath", "Read src/ipc/ipc_service.cpp", "Read src/core/paths.cpp"}, false, "resolveSocketPath() prefers $XDG_RUNTIME_DIR/noctalia.sock and falls back to /tmp/noctalia-<uid>.sock."},
	{"tests", "reviewer", "Check the IPC tests still cover help output", []string{"Search tests for buildHelp", "Run ctest -R ipc"}, true, "ctest exited 8: the ipc_help test expects the old usage line."},
}

// fakeSubagentsOmp streams a task tool the way omp reports it: progress in
// rawOutput.details while it runs, results at the end
func fakeSubagentsOmp(write func(any)) {
	pace := fakeAgentPace()
	tasks := make([]map[string]string, 0, len(fakeAgentPlans))
	for _, plan := range fakeAgentPlans {
		tasks = append(tasks, map[string]string{"id": plan.id, "description": plan.task, "task": plan.task})
	}
	writeUpdate(write, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "task-1", "title": "task", "kind": "other", "status": "pending", "rawInput": map[string]any{"agent": "explore", "tasks": tasks}})
	longest := 0
	for _, plan := range fakeAgentPlans {
		longest = max(longest, len(plan.steps))
	}
	for tick := 0; tick <= longest; tick++ {
		progress := make([]map[string]any, 0, len(fakeAgentPlans))
		for index, plan := range fakeAgentPlans {
			done := min(tick, len(plan.steps))
			status := "running"
			if tick == 0 && index > 0 {
				status = "pending"
			}
			if done == len(plan.steps) {
				status = "completed"
				if plan.fail {
					status = "failed"
				}
			}
			recent := make([]map[string]any, 0, done)
			for step := done - 1; step >= 0; step-- {
				recent = append(recent, map[string]any{"tool": "step", "args": "", "intent": plan.steps[step], "isError": plan.fail && step == len(plan.steps)-1, "endMs": 0})
			}
			entry := map[string]any{"index": index, "id": plan.id, "agent": plan.agent, "status": status, "task": plan.task, "description": plan.task, "recentTools": recent, "toolCount": done, "tokens": 1200 * (done + 1), "durationMs": 900 * (done + 1), "resolvedModel": "fast"}
			if status == "running" && done < len(plan.steps) {
				entry["currentToolIntent"] = plan.steps[done]
			}
			progress = append(progress, entry)
		}
		writeUpdate(write, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "task-1", "status": "in_progress", "rawOutput": map[string]any{
			"content": []map[string]string{{"type": "text", "text": "Running 3 agents"}},
			"details": map[string]any{"progress": progress},
		}})
		time.Sleep(pace)
	}
	results := make([]map[string]any, 0, len(fakeAgentPlans))
	for index, plan := range fakeAgentPlans {
		exit := 0
		if plan.fail {
			exit = 8
		}
		results = append(results, map[string]any{"index": index, "id": plan.id, "agent": plan.agent, "task": plan.task, "description": plan.task, "exitCode": exit, "output": plan.output, "stderr": "", "durationMs": 900 * (len(plan.steps) + 1), "tokens": 1200 * (len(plan.steps) + 1), "resolvedModel": "fast"})
	}
	writeUpdate(write, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "task-1", "status": "completed", "rawOutput": map[string]any{
		"content": []map[string]string{{"type": "text", "text": "2 of 3 agents finished"}},
		"details": map[string]any{"results": results},
	}})
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "Two agents mapped the IPC server; the reviewer found the help test out of date."}})
}

// fakeSubagentsClaude streams an Agent call the way Claude Code reports it:
// the subagent's own calls follow as tool calls that name the Agent call
func fakeSubagentsClaude(write func(any)) {
	pace := fakeAgentPace()
	plan := fakeAgentPlans[0]
	writeUpdate(write, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "agent-1", "title": plan.task, "kind": "think", "status": "pending",
		"rawInput": map[string]string{"description": plan.task, "prompt": "Map the IPC server and report how a command is handled.", "subagent_type": "Explore"},
		"_meta":    map[string]any{"claudeCode": map[string]string{"toolName": "Agent"}}})
	for index, step := range plan.steps {
		call := "agent-1-step-" + strconv.Itoa(index)
		meta := map[string]any{"claudeCode": map[string]string{"toolName": "Read", "parentToolUseId": "agent-1"}}
		writeUpdate(write, map[string]any{"sessionUpdate": "tool_call", "toolCallId": call, "title": step, "kind": "read", "status": "in_progress", "_meta": meta})
		time.Sleep(pace)
		writeUpdate(write, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": call, "status": "completed", "content": []map[string]any{{"type": "content", "content": map[string]string{"type": "text", "text": "read " + step}}}, "_meta": meta})
	}
	writeUpdate(write, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "agent-1", "status": "completed", "content": []map[string]any{{"type": "content", "content": map[string]string{"type": "text", "text": plan.output}}}, "_meta": map[string]any{"claudeCode": map[string]string{"toolName": "Agent"}}})
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "The Explore agent mapped the IPC server."}})
}
