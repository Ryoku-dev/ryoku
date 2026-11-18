package harness

import (
	"sort"

	"ryoku-rashin/internal/core"
)

// mcpServers writes each transport in the ACP connection form
func mcpServers(servers []core.MCPServer) []any {
	out := make([]any, 0, len(servers))
	for _, server := range servers {
		if server.Type == "http" || server.Type == "sse" {
			names := make([]string, 0, len(server.Headers))
			for name := range server.Headers {
				names = append(names, name)
			}
			sort.Strings(names)
			headers := make([]map[string]string, 0, len(names))
			for _, name := range names {
				headers = append(headers, map[string]string{"name": name, "value": server.Headers[name]})
			}
			out = append(out, map[string]any{"type": server.Type, "name": server.Name, "url": server.URL, "headers": headers})
			continue
		}
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
