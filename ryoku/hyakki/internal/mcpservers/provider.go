package mcpservers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// NewProvider delays opening the credential vault and MCP store until a turn
// with the MCP feature on asks for tools.
func NewProvider(open func() (*Store, error)) tool.Provider { return provider{open: open} }

type provider struct{ open func() (*Store, error) }

func sandboxedMCP(env tool.Env) bool {
	return env.Sandbox != nil || env.Context.Team != nil || env.Session.Team != "" ||
		env.Profile.Mode == tool.ModeTeam || env.Profile.Mode == tool.ModeDreamManager
}

func (p provider) Tools(ctx context.Context, env tool.Env) []tool.Tool {
	if sandboxedMCP(env) || !env.Feature(protocol.FeatureMCP) || p.open == nil {
		return nil
	}
	store, err := p.open()
	if err != nil || store == nil {
		return nil
	}
	return store.Tools(ctx, env)
}

type remoteTool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Input       json.RawMessage `json:"inputSchema"`
	Annotations struct {
		ReadOnly bool `json:"readOnlyHint"`
	} `json:"annotations"`
}

type mcpTool struct {
	store  *Store
	server protocol.MCPServerInfo
	remote remoteTool
	name   string
}

// Tools discovers each ready server's tools during this turn. Its stdio
// subprocesses are short-lived, and the vault values stay inside the store.
func (s *Store) Tools(ctx context.Context, env tool.Env) []tool.Tool {
	if sandboxedMCP(env) || !env.Feature(protocol.FeatureMCP) {
		return nil
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	servers := make([]protocol.MCPServerInfo, 0, len(s.records))
	for _, r := range s.records {
		if r.Info.Enabled && r.Info.State == "ready" {
			servers = append(servers, r.Info)
		}
	}
	s.mu.Unlock()
	var offered []tool.Tool
	seen := make(map[string]bool)
	for _, server := range servers {
		s.mu.Lock()
		cached := s.catalogues[server.ID]
		s.mu.Unlock()
		definitions := cached.tools
		if cached.at.IsZero() || time.Since(cached.at) >= time.Minute {
			listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			headers, err := s.currentHeaders(listCtx, server)
			if err != nil {
				cancel()
				continue
			}
			definitions, err = probe(listCtx, server, headers)
			cancel()
			if err != nil {
				continue
			}
			s.mu.Lock()
			for _, r := range s.records {
				if r.Info.ID == server.ID && r.Info.Enabled && r.Info.State == "ready" && r.Info.URL == server.URL && r.Info.Command == server.Command {
					s.catalogues[server.ID] = catalogue{at: time.Now(), tools: definitions}
					break
				}
			}
			s.mu.Unlock()
		}
		for _, definition := range definitions {
			if definition.Name == "" || !validInputSchema(definition.Input) {
				continue
			}
			serverName, remoteName := toolPart(server.Name), toolPart(definition.Name)
			if serverName == "" || remoteName == "" {
				continue
			}
			if len(serverName) > 24 {
				serverName = serverName[:24]
			}
			if len(remoteName) > 32 {
				remoteName = remoteName[:32]
			}
			base := "mcp_" + serverName + "_" + remoteName
			name := base
			for suffix := 2; seen[name]; suffix++ {
				ending := "_" + strconv.Itoa(suffix)
				name = base[:min(len(base), 64-len(ending))] + ending
			}
			seen[name] = true
			offered = append(offered, &mcpTool{store: s, server: server, remote: definition, name: name})
		}
	}
	return offered
}

func validInputSchema(raw json.RawMessage) bool {
	var schema struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(raw, &schema) == nil && schema.Type == "object"
}

func toolPart(name string) string {
	var b strings.Builder
	underscore := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			underscore = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			underscore = false
		case b.Len() > 0 && !underscore:
			b.WriteByte('_')
			underscore = true
		}
	}
	return strings.TrimSuffix(b.String(), "_")
}

func (t *mcpTool) Name() string    { return t.name }
func (t *mcpTool) Kind() tool.Kind { return protocol.ToolOther }
func (t *mcpTool) ReadOnly() bool  { return t.remote.Annotations.ReadOnly }
func (t *mcpTool) Spec(_ tool.Profile) llm.Tool {
	return llm.Tool{Name: t.name, Description: t.remote.Description, Parameters: t.remote.Input}
}
func (t *mcpTool) Present(args json.RawMessage) tool.Presentation {
	title := t.remote.Title
	if title == "" {
		title = t.remote.Name
	}
	input, _ := tool.ClipRunes(string(args), 400)
	return tool.Presentation{Title: title, Input: input}
}
func (t *mcpTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	if sandboxedMCP(env) {
		return tool.Failed(i18n.T("MCP tools are not available in team tasks."))
	}
	if !env.Feature(protocol.FeatureMCP) {
		return tool.Failed(i18n.T("Turn MCP on in Settings first."))
	}
	var args map[string]json.RawMessage
	if len(call.Args) == 0 || string(call.Args) == "null" {
		args = map[string]json.RawMessage{}
	} else if err := json.Unmarshal(call.Args, &args); err != nil || args == nil {
		return tool.Failed(i18n.T("The arguments are not valid JSON for this tool."))
	}
	if env.Profile.ReadOnly && !t.ReadOnly() {
		return tool.Failed(i18n.T("This MCP tool changes things and is not available in read-only mode."))
	}
	if !t.ReadOnly() {
		if env.Permissions == nil {
			return tool.Failed(i18n.T("Not run: nothing here can approve a change."))
		}
		if env.Emit != nil {
			env.Emit(protocol.ToolData{Status: protocol.ToolPending})
		}
		decision, err := env.Permissions.Allow(ctx, tool.Request{Call: call, Title: t.Present(call.Args).Title, Kind: protocol.ToolOther, Input: t.Present(call.Args).Input, Tier: protocol.TierWrite})
		if err != nil {
			return tool.Failed(i18n.T("Cancelled before it ran."))
		}
		if !decision.Allowed {
			reason := decision.Reason
			if reason == "" {
				reason = i18n.T("Not run: you refused it.")
			}
			return tool.Failed(reason)
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	headers, err := t.store.currentHeaders(callCtx, t.server)
	if err != nil {
		return tool.Failed(err.Error())
	}
	client, err := openClient(callCtx, t.server, headers)
	if err != nil {
		return tool.Failed(err.Error())
	}
	defer client.Close()
	raw, err := client.call("tools/call", map[string]any{"name": t.remote.Name, "arguments": args})
	if err != nil {
		return tool.Failed(err.Error())
	}
	return decodeToolResult(raw, env.Limits)
}

func (s *Store) currentHeaders(ctx context.Context, info protocol.MCPServerInfo) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return nil, errors.New(i18n.T("MCP is not running."))
	}
	for index := range s.records {
		r := &s.records[index]
		if r.Info.ID != info.ID {
			continue
		}
		if !r.Info.Enabled || r.Info.State != "ready" || r.Info.Type != info.Type || r.Info.URL != info.URL || r.Info.Command != info.Command || !slices.Equal(r.Info.Args, info.Args) {
			return nil, errors.New(i18n.T("This MCP server changed. Check it again."))
		}
		return s.headers(ctx, r)
	}
	return nil, errors.New(i18n.T("That server no longer exists."))
}

func decodeToolResult(raw json.RawMessage, limits tool.Limits) tool.Result {
	// The protocol's content blocks can carry text, images or links; preserve
	// text and images, and leave other blocks readable instead of dropping them.
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
			Data string `json:"data"`
			Mime string `json:"mimeType"`
		} `json:"content"`
		Structured json.RawMessage `json:"structuredContent"`
		IsError    bool            `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return tool.Failed(i18n.T("The MCP server returned an invalid result."))
	}
	var text []string
	out := tool.Result{IsError: result.IsError}
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			text = append(text, block.Text)
		case "image":
			if !strings.HasPrefix(block.Mime, "image/") || len(block.Data) > rpcLimit {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(block.Data)
			if err == nil {
				out.Images = append(out.Images, llm.Image{Mime: block.Mime, Data: data})
			}
		default:
			encoded, _ := json.Marshal(block)
			text = append(text, string(encoded))
		}
	}
	if len(result.Structured) > 0 && len(result.Content) == 0 {
		text = append(text, string(result.Structured))
	}
	out.Text = strings.Join(text, "\n")
	if limits.Told > 0 && len(out.Text) > limits.Told {
		out.Text = tool.HeadTail(out.Text, limits.Head, limits.Tail)
	}
	return out
}
