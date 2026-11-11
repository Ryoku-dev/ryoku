package harness

import (
	"context"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

// lastAgents is the latest subagent list the runner reported for a call
func lastAgents(sink *testSink, call string) []protocol.AgentWork {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	var agents []protocol.AgentWork
	for _, event := range sink.events {
		if tool, ok := event.data.(protocol.ToolData); ok && tool.Call == call && tool.Agents != nil {
			agents = tool.Agents
		}
	}
	return agents
}

// omp reports its subagents inside its task tool's output: each keeps its
// order, the step it is on, and once done its answer or why it failed.
func TestOmpTaskProgressBecomesSubagents(t *testing.T) {
	r, sink := helperRunner(t, "subagents", protocol.ApprovalsAsk, nil)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Text: "map it"}, sink); err != nil {
		t.Fatal(err)
	}
	sink.mu.Lock()
	var midway []protocol.AgentWork
	for _, event := range sink.events {
		if tool, ok := event.data.(protocol.ToolData); ok && tool.Call == "task-1" && len(tool.Agents) == 3 && tool.Agents[0].StepCount == 2 {
			midway = tool.Agents
		}
	}
	sink.mu.Unlock()
	if len(midway) != 3 || midway[0].Status != protocol.ToolRunning || midway[0].Doing != "Search for registerHandler" || len(midway[0].Steps) != 2 || midway[0].Steps[1].Title != "Read src/ipc/ipc_service.cpp" {
		t.Fatalf("midway agents = %+v", midway)
	}
	final := lastAgents(sink, "task-1")
	if len(final) != 3 {
		t.Fatalf("final agents = %+v", final)
	}
	wants := []struct{ id, status string }{{"ipc", protocol.ToolDone}, {"paths", protocol.ToolDone}, {"tests", protocol.ToolFailed}}
	for index, want := range wants {
		if final[index].ID != want.id || final[index].Status != want.status || final[index].Output == "" || final[index].Doing != "" || final[index].StepCount == 0 || len(final[index].Steps) == 0 {
			t.Fatalf("agent %d = %+v, want %s %s with its steps", index, final[index], want.id, want.status)
		}
	}
	if final[2].Output != "ctest exited 8: the ipc_help test expects the old usage line." {
		t.Fatalf("the failed agent says %q", final[2].Output)
	}
}

// Claude Code reports the calls a subagent makes as tool calls of their own:
// they are marked as its parent's, and the Agent call counts them.
func TestClaudeSubagentCallsBelongToTheirAgentCall(t *testing.T) {
	r, sink := helperRunner(t, "subagents-claude", protocol.ApprovalsAsk, nil)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Text: "map it"}, sink); err != nil {
		t.Fatal(err)
	}
	sink.mu.Lock()
	children := map[string]bool{}
	for _, event := range sink.events {
		if tool, ok := event.data.(protocol.ToolData); ok && tool.Call != "agent-1" {
			if tool.Parent != "agent-1" {
				sink.mu.Unlock()
				t.Fatalf("a subagent call is not marked as its parent's: %+v", tool)
			}
			children[tool.Call] = true
		}
	}
	sink.mu.Unlock()
	agents := lastAgents(sink, "agent-1")
	if len(children) != 4 || len(agents) != 1 {
		t.Fatalf("children %v agents %+v", children, agents)
	}
	agent := agents[0]
	if agent.Name != "Explore" || agent.Task != "Map how the IPC server reads a command" || agent.Status != protocol.ToolDone || agent.StepCount != 4 || agent.Steps[3].Title != "Read src/app/application.cpp" || agent.Output == "" {
		t.Fatalf("agent = %+v", agent)
	}
}

// omp's task text opens with its template, so a subagent without a
// description is named by the ID the model gave it
func TestOmpSubagentNamesItsTask(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"description", `{"details":{"progress":[{"id":"ListFiles","description":"List the screen package","task":"Complete assignment thoroughly:\nList it"}]}}`, "List the screen package"},
		{"id", `{"details":{"progress":[{"id":"ListScreenFiles","task":"Complete assignment thoroughly:\nList it"}]}}`, "List screen files"},
		{"acronym", `{"details":{"results":[{"id":"MapIPCServer","task":"Complete assignment thoroughly:"}]}}`, "Map IPC server"},
		{"no id", `{"details":{"progress":[{"index":0,"task":"Count the lines in keys.go"}]}}`, "Count the lines in keys.go"},
	}
	for _, tc := range cases {
		agents, ok := ompAgents([]byte(tc.raw))
		if !ok || len(agents) != 1 || agents[0].Task != tc.want {
			t.Fatalf("%s: agents = %+v, want the task %q", tc.name, agents, tc.want)
		}
	}
}
