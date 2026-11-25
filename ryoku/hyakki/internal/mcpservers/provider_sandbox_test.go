package mcpservers

import (
	"context"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

func TestSandboxedSessionsNeverOpenHostMCPServers(t *testing.T) {
	opened := 0
	p := NewProvider(func() (*Store, error) {
		opened++
		return nil, nil
	})
	for _, env := range []tool.Env{
		{Sandbox: &sandbox.Spec{}},
		{Context: core.ToolContext{Team: &core.TeamToolContext{Team: "team-one"}}},
		{Session: protocol.SessionInfo{Team: "team-one"}},
		{Profile: tool.Profile{Mode: tool.ModeDreamManager}},
	} {
		env.Settings = func() protocol.Settings {
			return protocol.Settings{Features: map[string]bool{protocol.FeatureMCP: true}}
		}
		if tools := p.Tools(context.Background(), env); len(tools) != 0 {
			t.Fatalf("sandboxed session got %d MCP tools", len(tools))
		}
		if got := (&mcpTool{}).Run(context.Background(), tool.Call{}, env); !got.IsError {
			t.Fatal("an already-discovered MCP tool ran for a sandboxed session")
		}
	}
	if opened != 0 {
		t.Fatalf("opened %d MCP stores from sandboxed sessions", opened)
	}
}
