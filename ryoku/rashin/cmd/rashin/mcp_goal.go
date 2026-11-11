package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/mcp"
	"ryoku-rashin/internal/protocol"
)

// goalTool lets the agent read the session's goal and end it: done once every
// deliverable is checked, blocked when it needs the user
func goalTool(session mcpSession) mcp.Tool {
	return mcp.Tool{
		Name: "goal",
		Description: "The goal Rashin keeps sending you on toward, when the user set one with /goal. op get reports it. " +
			"op complete with a summary ends it, only after you checked every deliverable against the current state of the work. " +
			"op block with the exact blocker ends it when you need the user. Without a goal set there is nothing to complete.",
		Input: schema([]string{"op"}, map[string]any{
			"op":      map[string]any{"type": "string", "enum": []string{protocol.GoalGet, protocol.GoalComplete, protocol.GoalBlock}, "description": "get, complete or block."},
			"summary": stringProperty("For complete, what was done and how it was checked. For block, what stops you and what you need from the user."),
		}),
		Call: func(ctx context.Context, raw json.RawMessage) mcp.Result {
			var args struct {
				Op      string `json:"op"`
				Summary string `json:"summary"`
			}
			if err := toolArguments(raw, &args, "op"); err != nil {
				return mcp.Failure(err)
			}
			if session.id == "" {
				return mcp.Failure(errors.New(i18n.T("This server was not started for a Rashin session.")))
			}
			if (args.Op == protocol.GoalComplete || args.Op == protocol.GoalBlock) && args.Summary == "" {
				return mcp.Failure(errors.New(i18n.T("Missing summary.")))
			}
			conn, cancel, err := dial(true)
			if err != nil {
				return mcp.Failure(err)
			}
			defer cancel()
			defer conn.Close()
			var info protocol.SessionInfo
			if err := conn.Call(ctx, protocol.MSessionGoal, protocol.GoalParams{ID: session.id, Op: args.Op, Summary: args.Summary}, &info); err != nil {
				return mcp.Failure(err)
			}
			goal := info.Goal
			if goal == nil {
				return mcp.Text("No goal is set for this session.")
			}
			switch args.Op {
			case protocol.GoalComplete:
				return mcp.Text("The goal is marked done. End this turn with a short report for the user.")
			case protocol.GoalBlock:
				return mcp.Text("The goal is marked blocked. End this turn by telling the user what you need.")
			}
			return mcp.Text(fmt.Sprintf("Objective: %s\nStatus: %s\nTurns: %d of %d", goal.Objective, goal.Status, goal.Turns, goal.MaxTurns))
		},
	}
}
