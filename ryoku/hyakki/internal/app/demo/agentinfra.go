package demo

import (
	"encoding/json"
	"strings"
	"time"

	"ryoku-rashin/internal/protocol"
)

type agentInfraDemo struct {
	loaded  bool
	servers []protocol.MCPServerInfo
	usage   protocol.DiskUsage
}

func (s *server) callAgentInfra(method string, raw json.RawMessage) (any, bool, error) {
	if !strings.HasPrefix(method, "mcp.") && !strings.HasPrefix(method, "disk.") {
		return nil, false, nil
	}
	d := &s.infra
	if !d.loaded {
		d.loaded = true
		d.servers = []protocol.MCPServerInfo{
			{ID: "demo-files", Name: "Project files", Type: "stdio", Command: "mcp-files", Args: []string{"serve"}, Enabled: true, State: "ready", Tools: 8},
			{ID: "demo-notes", Name: "Notes", Type: "http", URL: "https://notes.example/mcp", Enabled: true, State: "needs-signin", HeaderNames: []string{"X-Workspace"}, OAuth: &protocol.MCPOAuth{ClientID: "hyakki", AuthURL: "https://notes.example/authorize", TokenURL: "https://notes.example/token", Scopes: "notes.read"}},
			{ID: "demo-off", Name: "Reading list", Type: "sse", URL: "https://reading.example/sse", State: "off"},
		}
		const gib = 1024 * 1024 * 1024
		d.usage = protocol.DiskUsage{Free: 4 * gib, Total: 512 * gib, Measured: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC), Items: []protocol.DiskItem{
			{ID: "teams", Name: "Team folders", Bytes: 3 * gib}, {ID: "dreams", Name: "Dream folders", Bytes: 2 * gib},
			{ID: "models", Name: "Local models", Bytes: gib},
			{ID: "logs", Name: "Ended session logs", Bytes: gib / 4, Cleanable: true, Plan: "demo-logs", Files: []protocol.DiskFile{{Path: "/home/demo/.local/state/hyakki/sessions/agent-finished/events.jsonl", Bytes: gib / 4}}},
			{ID: "cache", Name: "Model cache", Bytes: gib / 8, Cleanable: true, Plan: "demo-cache", Files: []protocol.DiskFile{{Path: "/home/demo/.local/share/hyakki/models/partial.gguf", Bytes: gib / 8}}},
		}}
	}
	switch method {
	case protocol.MMCPList:
		return protocol.MCPListResult{Servers: d.servers}, true, nil
	case protocol.MMCPSave:
		var p protocol.MCPServer
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, true, err
		}
		if p.ID == "" {
			p.ID = "demo-" + strings.ReplaceAll(strings.ToLower(p.Name), " ", "-")
		}
		info := protocol.MCPServerInfo{ID: p.ID, Name: p.Name, Type: p.Type, Command: p.Command, Args: p.Args, URL: p.URL, OAuth: p.OAuth, Enabled: p.Enabled, State: "failing"}
		if !p.Enabled {
			info.State = "off"
		}
		if p.Enabled && p.OAuth != nil {
			info.State = "needs-signin"
		}
		for name := range p.Headers {
			info.HeaderNames = append(info.HeaderNames, name)
		}
		for index := range d.servers {
			if d.servers[index].ID == p.ID {
				d.servers[index] = info
				return info, true, nil
			}
		}
		d.servers = append(d.servers, info)
		return info, true, nil
	case protocol.MMCPDelete:
		var p protocol.MCPDeleteParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, true, err
		}
		for index := range d.servers {
			if d.servers[index].ID == p.ID {
				d.servers = append(d.servers[:index], d.servers[index+1:]...)
				break
			}
		}
		return protocol.Empty{}, true, nil
	case protocol.MMCPCheck:
		var p protocol.MCPCheckParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, true, err
		}
		for index := range d.servers {
			if d.servers[index].ID == p.ID {
				info := &d.servers[index]
				if info.Enabled {
					info.State, info.Tools = "ready", 8
				}
				return *info, true, nil
			}
		}
		return nil, true, protocol.Errorf(protocol.CodeNotFound, "No such demo server")
	case protocol.MMCPSignIn:
		return protocol.MCPSignInResult{URL: "https://notes.example/authorize"}, true, nil
	case protocol.MDiskUsage:
		return d.usage, true, nil
	case protocol.MDiskClean:
		var p protocol.DiskCleanParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, true, err
		}
		for index := range d.usage.Items {
			item := &d.usage.Items[index]
			if item.Plan == p.Plan && item.Cleanable {
				d.usage.Free += uint64(item.Bytes)
				item.Bytes, item.Files, item.Cleanable, item.Plan = 0, nil, false, ""
				return d.usage, true, nil
			}
		}
		return nil, true, protocol.Errorf(protocol.CodeBadRequest, "Review the cleanup list again")
	}
	return nil, false, nil
}
