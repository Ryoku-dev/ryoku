package ask

import (
	"context"
	"encoding/json"
	"fmt"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// teamToolset are Ask's tools over Rashin's teams, offered in every mode:
// two read the teams and their templates, and propose_team only asks the
// user, who approves the team in the Rashin Inbox.
var teamToolset = func() []llm.Tool {
	tools := brief.TeamTools()
	out := make([]llm.Tool, 0, len(tools))
	for _, tool := range tools {
		parameters, _ := json.Marshal(tool.Schema)
		out = append(out, llm.Tool{Name: tool.Name, Description: tool.Description, Parameters: parameters})
	}
	return out
}()

// runTeamTool runs one of teamToolset through teams for session; handled is
// false for any other tool.
func runTeamTool(ctx context.Context, teams core.Teams, call llm.ToolCall, session string) (out string, handled bool, err error) {
	var method string
	for _, tool := range brief.TeamTools() {
		if tool.Name == call.Name {
			method = tool.Method
		}
	}
	if method == "" {
		return "", false, nil
	}
	if teams == nil {
		return "", true, fmt.Errorf("%s", i18n.T("Teams are unavailable."))
	}
	params := json.RawMessage(`{}`)
	if method == protocol.MTeamsPropose {
		proposal, err := brief.Proposal(json.RawMessage(call.Arguments), session)
		if err != nil {
			return "", true, err
		}
		if params, err = json.Marshal(proposal); err != nil {
			return "", true, err
		}
	} else if err := decodeArgs(call.Arguments, &struct{}{}); err != nil {
		return "", true, err
	}
	result, err := teams.Handle(ctx, method, params)
	if err != nil {
		return "", true, err
	}
	if method == protocol.MTeamsPropose {
		return brief.Proposed, true, nil
	}
	return marshalToolResult(result, nil), true, nil
}

// teamToolPresentation is how a team tool's call reads in the transcript:
// reading the teams and templates is a lookup, proposing one a change the
// user approves; ok is false for any other tool.
func teamToolPresentation(call llm.ToolCall) (title, kind string, ok bool) {
	switch call.Name {
	case brief.ToolTeamsTemplates:
		return i18n.T("Reading the team templates"), protocol.ToolSearch, true
	case brief.ToolTeamsList:
		return i18n.T("Reading your teams"), protocol.ToolSearch, true
	case brief.ToolProposeTeam:
		return i18n.T("Proposing a team"), protocol.ToolEdit, true
	}
	return "", "", false
}
