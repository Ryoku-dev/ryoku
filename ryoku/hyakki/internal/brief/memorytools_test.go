package brief

import (
	"slices"
	"strings"
	"testing"
)

// remembered is the remember tool offered to who.
func remembered(t *testing.T, who int) MemoryTool {
	t.Helper()
	for _, tool := range MemoryTools(who, true) {
		if tool.Name == ToolRemember {
			return tool
		}
	}
	t.Fatalf("no remember tool for %d", who)
	return MemoryTool{}
}

func TestAnAgentAlwaysSaysWhatAFactIsAbout(t *testing.T) {
	agent := remembered(t, MemoryForAgent)
	required, _ := agent.Schema["required"].([]string)
	if !slices.Contains(required, "scope") {
		t.Fatalf("an agent's remember may leave the scope out: %v", required)
	}
	scope := agent.Schema["properties"].(map[string]any)["scope"].(map[string]any)
	if enum := scope["enum"].([]string); !slices.Equal(enum, []string{"you", "project"}) {
		t.Fatalf("an agent's scopes are %v", enum)
	}
	help := scope["description"].(string)
	if !strings.Contains(help, "project: this codebase") || strings.Contains(help, "Defaults to you") {
		t.Fatalf("the scope is described as %q", help)
	}
	if !strings.Contains(agent.Description, "about the project") {
		t.Fatalf("remember's rules leave out project facts: %q", agent.Description)
	}
	for _, who := range []int{MemoryForAsk, MemoryForTeam} {
		required, _ := remembered(t, who).Schema["required"].([]string)
		if slices.Contains(required, "scope") {
			t.Fatalf("remember for %d needs a scope it may leave out", who)
		}
	}
}
