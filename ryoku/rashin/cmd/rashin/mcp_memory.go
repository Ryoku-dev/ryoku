package main

import (
	"context"
	"encoding/json"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/mcp"
)

// memoryTools let the session search, keep and forget what Rashin remembers.
// A team task's calls reach the daemon through its own socket, which holds
// them to its team's memory; any other session names itself so what it keeps
// shows in its conversation
func memoryTools(session mcpSession, team, sessions bool) []mcp.Tool {
	who := brief.MemoryForAgent
	if team {
		who = brief.MemoryForTeam
	}
	defined := brief.MemoryTools(who, sessions)
	tools := make([]mcp.Tool, 0, len(defined))
	for _, tool := range defined {
		name := tool.Name
		tools = append(tools, mcp.Tool{
			Name: tool.Name, Description: tool.Description, Input: tool.Schema, ReadOnly: tool.ReadOnly,
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				conversation := session.id
				if team {
					conversation = ""
				}
				method, params, err := brief.MemoryParams(name, raw, conversation)
				if err != nil {
					return mcp.Failure(err)
				}
				var result json.RawMessage
				if err := daemonCall(ctx, method, params, &result); err != nil {
					return mcp.Failure(err)
				}
				return mcp.Text(brief.MemoryAnswer(method, result))
			},
		})
	}
	return tools
}
