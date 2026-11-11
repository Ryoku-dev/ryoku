package harness

import (
	"sort"

	"ryoku-rashin/internal/core"
)

// mcpServers writes the session's MCP servers the way ACP names a stdio one
func mcpServers(servers []core.MCPServer) []any {
	out := make([]any, 0, len(servers))
	for _, server := range servers {
		names := make([]string, 0, len(server.Env))
		for name := range server.Env {
			names = append(names, name)
		}
		sort.Strings(names)
		env := make([]map[string]string, 0, len(names))
		for _, name := range names {
			env = append(env, map[string]string{"name": name, "value": server.Env[name]})
		}
		args := server.Args
		if args == nil {
			args = []string{}
		}
		out = append(out, map[string]any{"name": server.Name, "command": server.Command, "args": args, "env": env})
	}
	return out
}
