package rashin

import (
	"context"
	"encoding/json"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// skillTools find and read the skills Rashin ships, from the definitions
// Ask and agents share. The Host holds a team task to its team's sets.
func skillTools() []tool.Tool {
	return []tool.Tool{
		skillTool(tool.Skills, protocol.ToolSearch, "query", func() string { return i18n.T("Searching skills") }),
		skillTool(tool.Skill, protocol.ToolRead, "id", func() string { return i18n.T("Reading a skill") }),
	}
}

func skillTool(name string, kind tool.Kind, show string, title func() string) tool.Tool {
	var definition brief.SkillTool
	for _, t := range brief.SkillTools() {
		if t.Name == name {
			definition = t
		}
	}
	return &def{
		name: name, kind: kind, title: title, show: show,
		describe: static(definition.Description),
		schema:   fixed(fromSchema(definition.Schema)),
		run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
			method, params, err := brief.SkillParams(name, args.raw())
			if err != nil {
				return failed(err)
			}
			var result json.RawMessage
			if err := env.Host.Call(ctx, method, params, &result); err != nil {
				return failed(err)
			}
			return answer(env, brief.SkillAnswer(method, result))
		},
	}
}
