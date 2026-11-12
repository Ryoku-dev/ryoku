package main

import (
	"context"
	"encoding/json"
	"errors"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/mcp"
	"ryoku-rashin/internal/protocol"
)

// teamsTools let any agent session read Rashin's teams and their templates
// and propose a team, which waits for the user's approval in the Inbox
func teamsTools(session mcpSession) []mcp.Tool {
	defined := brief.TeamTools()
	tools := make([]mcp.Tool, 0, len(defined))
	for _, tool := range defined {
		method := tool.Method
		tools = append(tools, mcp.Tool{
			Name: tool.Name, Description: tool.Description, Input: tool.Schema, ReadOnly: tool.ReadOnly,
			Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
				switch method {
				case protocol.MTeamsTemplates:
					return toolCall(ctx, method, protocol.Empty{}, &protocol.TeamTemplatesResult{})
				case protocol.MTeamsList:
					return toolCall(ctx, method, protocol.Empty{}, &protocol.TeamsResult{})
				}
				if session.id == "" {
					return mcp.Failure(errors.New(i18n.T("This server was not started for a Rashin session.")))
				}
				proposal, err := brief.Proposal(raw, session.id)
				if err != nil {
					return mcp.Failure(err)
				}
				var approval protocol.Approval
				if err := daemonCall(ctx, method, proposal, &approval); err != nil {
					return mcp.Failure(err)
				}
				return mcp.Text(brief.Proposed)
			},
		})
	}
	return tools
}
