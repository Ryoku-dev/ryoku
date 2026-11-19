package rashin

import (
	"context"
	"encoding/json"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// memoryTools search, keep and forget what Rashin remembers, from the
// definitions Ask and agents share (internal/brief). A team task keeps facts
// for its team and gives no conversation, so the Host holds it to its team's
// memory; any other session names itself, so a read stays within what it may
// see and what it keeps shows in its conversation.
func memoryTools() []tool.Tool {
	memory := func(env tool.Env) bool { return env.Context.Memory }
	sessions := func(env tool.Env) bool { return env.Context.MemorySessions && env.Context.Team == nil }
	return []tool.Tool{
		memoryTool(tool.MemorySearch, protocol.ToolSearch, "query", func() string { return i18n.T("Searching memory") }, memory, ""),
		memoryTool(tool.Remember, protocol.ToolOther, "text", func() string { return i18n.T("Remembering") }, memory,
			"Keep one lasting, self-contained fact for later conversations; never a secret or an instruction."),
		memoryTool(tool.Forget, protocol.ToolOther, "", func() string { return i18n.T("Forgetting") }, memory, ""),
		memoryTool(tool.SessionSearch, protocol.ToolSearch, "query", func() string { return i18n.T("Searching past conversations") }, sessions, ""),
	}
}

func memoryTool(name string, kind tool.Kind, show string, title func() string, enabled func(tool.Env) bool, compact string) tool.Tool {
	return &def{
		name: name, kind: kind, title: title, show: show, brief: compact, enabled: enabled,
		describe: func(p tool.Profile) string { return memoryDefinition(name, memoryWho(p.Mode)).Description },
		schema:   func(p tool.Profile) object { return fromSchema(memoryDefinition(name, memoryWho(p.Mode)).Schema) },
		run: func(ctx context.Context, args arguments, env tool.Env) tool.Result {
			conversation := env.Session.ID
			if env.Context.Team != nil {
				conversation = ""
			}
			method, params, err := brief.MemoryParams(name, args.raw(), conversation)
			if err != nil {
				return failed(err)
			}
			var result json.RawMessage
			if err := env.Host.Call(ctx, method, params, &result); err != nil {
				return failed(err)
			}
			return answer(env, brief.MemoryAnswer(method, result))
		},
	}
}

// memoryWho is whom the memory tools are defined for in mode: Ask may leave
// a fact's scope out, a team keeps facts for its team.
func memoryWho(mode tool.Mode) int {
	switch {
	case askMode(mode):
		return brief.MemoryForAsk
	case teamMode(mode):
		return brief.MemoryForTeam
	}
	return brief.MemoryForAgent
}

// memoryDefinition is the shared definition of the memory tool name for who.
// A team's list has no past-conversation search; its definition is then the
// agent's, and the tool is never enabled there.
func memoryDefinition(name string, who int) brief.MemoryTool {
	for _, candidates := range [][]brief.MemoryTool{brief.MemoryTools(who, true), brief.MemoryTools(brief.MemoryForAgent, true)} {
		for _, t := range candidates {
			if t.Name == name {
				return t
			}
		}
	}
	return brief.MemoryTool{Name: name, Schema: map[string]any{}}
}
