package brief

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"ryoku-rashin/internal/protocol"
)

// The team tools' names, the same in an agent's rashin MCP server and in Ask.
const (
	ToolTeamsTemplates = "teams_templates"
	ToolTeamsList      = "teams_list"
	ToolProposeTeam    = "propose_team"
)

// TeamTool is one of the tools that let a model read Rashin's teams and
// propose one. Agents get them from the rashin MCP server and Ask from its
// own tool list, from this one definition.
type TeamTool struct {
	Name        string
	Description string
	// Schema is the JSON schema of the tool's arguments.
	Schema map[string]any
	// Method is the daemon method the tool calls.
	Method string
	// ReadOnly says the tool changes nothing.
	ReadOnly bool
}

// TeamTools are the team tools in the order they are offered.
func TeamTools() []TeamTool {
	text := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	object := func(required []string, properties map[string]any) map[string]any {
		if required == nil {
			required = []string{}
		}
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	list := func(description string, items any) map[string]any {
		return map[string]any{"type": "array", "items": items, "description": description}
	}
	member := object([]string{"name"}, map[string]any{
		"name":         text("The member's name, unique in the team."),
		"title":        text("Its role in a few words."),
		"lead":         map[string]any{"type": "boolean", "description": "The one member that takes the team's tasks and hands pieces to the others."},
		"harness":      text("A harness id such as claude, codex or omp; leave it out to follow the user's teams defaults."),
		"model":        text("That harness's model id; leave it out for the harness's default."),
		"instructions": text("Its standing role: what it does and how."),
	})
	routine := object([]string{"title", "brief", "schedule"}, map[string]any{
		"title":    text("One line naming the routine."),
		"brief":    text("What the team does on each run."),
		"member":   text("The member's name it goes to; leave it out for the lead."),
		"schedule": text("A cron schedule in local time, at most every 15 minutes, such as 0 8 * * 1."),
	})
	credential := object([]string{"name", "hosts", "why"}, map[string]any{
		"name":  text("The credential's name, such as GITHUB_TOKEN."),
		"hosts": list("The exact hosts it is sent to, such as api.github.com.", map[string]any{"type": "string"}),
		"why":   text("One sentence the user reads: what the team uses it for."),
	})
	team := object([]string{"name", "purpose"}, map[string]any{
		"template":  text("The id of the template it starts from; what you leave out comes from it."),
		"name":      text("The team's name."),
		"purpose":   text("What the team is for, in the user's words."),
		"members":   list("Its members, one of them the lead.", member),
		"tools":     object(nil, map[string]any{"screen": map[string]any{"type": "boolean"}, "mail": map[string]any{"type": "boolean"}}),
		"approvals": map[string]any{"type": "string", "enum": []string{protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}, "description": "What its agents may do without asking: read-only, ask before changes, or auto."},
		"routines":  list("Work it takes on by itself on a schedule; they arrive off and the user turns them on.", routine),
	})
	return []TeamTool{
		{
			Name:        ToolTeamsTemplates,
			Description: "The ready-made teams Rashin offers, by level (light, medium, advanced): their members, tools, permissions, routines and the credentials they ask for. Read it before propose_team.",
			Schema:      object(nil, map[string]any{}),
			Method:      protocol.MTeamsTemplates,
			ReadOnly:    true,
		},
		{
			Name:        ToolTeamsList,
			Description: "The teams that exist on this computer, with their purpose, members and routines' state. Teams do not talk to each other.",
			Schema:      object(nil, map[string]any{}),
			Method:      protocol.MTeamsList,
			ReadOnly:    true,
		},
		{
			Name: ToolProposeTeam,
			Description: "Propose a new Rashin team to the user. Nothing is made now: the user creates it or says no in the Rashin Inbox, and only then Rashin makes the team and asks the user for each credential you named, without you ever seeing a value. Its routines start off until the user turns them on. " +
				"Start from a template from teams_templates; the members, tools, permissions, routines and credentials you leave out come from it. Use it when the user asks for a team, or for work that should run again and again on its own. " +
				"Arguments, for example: " + proposeExample,
			Schema: object([]string{"team", "reason"}, map[string]any{
				"team":        team,
				"credentials": list("Credentials the team will need beyond its template's.", credential),
				"reason":      text("Why you propose it, in one or two sentences the user reads."),
			}),
			Method: protocol.MTeamsPropose,
		},
	}
}

// Proposal reads propose_team's arguments into what teams.propose takes, as
// proposed by session.
func Proposal(arguments json.RawMessage, session string) (protocol.TeamProposeParams, error) {
	var p protocol.TeamProposeParams
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return protocol.TeamProposeParams{}, fmt.Errorf("the arguments do not describe a team proposal: %w", err)
	}
	if p.Reason == "" {
		return protocol.TeamProposeParams{}, errors.New("missing reason")
	}
	p.Session = session
	return p, nil
}

// proposeExample is a minimal valid propose_team call, shown in the tool's
// description so a model gets the shape right on its first try.
const proposeExample = `{"team":{"template":"repo","name":"Repo watch","purpose":"Tell me which pull requests need me.","members":[{"name":"Reviewer","lead":true}]},"reason":"You asked for a team that watches your pull requests.","credentials":[{"name":"GITHUB_TOKEN","hosts":["api.github.com"],"why":"To read your pull requests."}]}`

// Proposed is what a model is told once its proposal waits for the user: what
// happens next, so it never tells them the team or its routines start by
// themselves.
const Proposed = "Proposed. Nothing is made and nothing runs yet. Tell the user plainly what happens next: they create the team from the Rashin Inbox (or say no there); its routines start off and run only once they turn them on; and Rashin asks them for each credential in the Inbox after the team is made. Do not say it starts automatically."
