package rashin

import (
	"context"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// teamsTools read Rashin's teams and their templates and propose a team,
// which waits for the user's approval in the Inbox.
func teamsTools() []tool.Tool {
	definitions := map[string]brief.TeamTool{}
	for _, t := range brief.TeamTools() {
		definitions[t.Name] = t
	}
	listing := func(name string, title func() string, read func(ctx context.Context, env tool.Env) (any, error)) tool.Tool {
		definition := definitions[name]
		return &def{
			name: name, kind: protocol.ToolRead, title: title,
			describe: static(definition.Description),
			schema:   fixed(fromSchema(definition.Schema)),
			run: func(ctx context.Context, _ arguments, env tool.Env) tool.Result {
				result, err := read(ctx, env)
				if err != nil {
					return failed(err)
				}
				return answer(env, toon(result))
			},
		}
	}
	propose := definitions[tool.ProposeTeam]
	return []tool.Tool{
		listing(tool.TeamsTemplates, func() string { return i18n.T("Reading team templates") }, func(ctx context.Context, env tool.Env) (any, error) {
			var result protocol.TeamTemplatesResult
			return &result, env.Host.Call(ctx, protocol.MTeamsTemplates, protocol.Empty{}, &result)
		}),
		listing(tool.TeamsList, func() string { return i18n.T("Reading teams") }, func(ctx context.Context, env tool.Env) (any, error) {
			var result protocol.TeamsResult
			return &result, env.Host.Call(ctx, protocol.MTeamsList, protocol.Empty{}, &result)
		}),
		&def{
			name: tool.ProposeTeam, kind: protocol.ToolOther, title: func() string { return i18n.T("Proposing a team") },
			describe: static(propose.Description),
			schema:   fixed(fromSchema(propose.Schema)),
			run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
				proposal, err := brief.Proposal(args.raw(), env.Session.ID)
				if err != nil {
					return failed(err)
				}
				var approval protocol.Approval
				if err := env.Host.Call(ctx, protocol.MTeamsPropose, proposal, &approval); err != nil {
					return failed(err)
				}
				return tool.Result{Text: brief.Proposed}
			},
		},
	}
}
