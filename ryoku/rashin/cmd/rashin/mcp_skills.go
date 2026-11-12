package main

import (
	"context"
	"encoding/json"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/mcp"
)

// skillsInstructions is what the server says of the skill tools
const skillsInstructions = " skills searches the skills Rashin ships built in and skill reads one: search before a task and follow the skill that fits."

// skillTools find and read the built-in skills. A team task's calls reach the
// daemon through its own socket, which holds them to its team's sets
func skillTools() []mcp.Tool {
	defined := brief.SkillTools()
	tools := make([]mcp.Tool, 0, len(defined))
	for _, tool := range defined {
		name := tool.Name
		tools = append(tools, mcp.Tool{
			Name: tool.Name, Description: tool.Description, Input: tool.Schema, ReadOnly: true,
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				method, params, err := brief.SkillParams(name, raw)
				if err != nil {
					return mcp.Failure(err)
				}
				var result json.RawMessage
				if err := daemonCall(ctx, method, params, &result); err != nil {
					return mcp.Failure(err)
				}
				return mcp.Text(brief.SkillAnswer(method, result))
			},
		})
	}
	return tools
}
