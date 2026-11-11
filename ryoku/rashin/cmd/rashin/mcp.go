package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/mcp"
)

func init() {
	register(&Command{Name: "mcp", Group: "tools", Summary: i18n.T("Serve Rashin's tools to an agent over MCP."), Run: runMCP})
}

// mcpSession is the agent session the server was started for, from the
// environment its harness was given; token and lead are set for a session
// that works a team task
type mcpSession struct {
	id    string
	cwd   string
	token string
	lead  bool
}

// path resolves a path the agent gave relative to its session folder
func (session mcpSession) path(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(session.cwd, path)
}

func runMCP(args []string) int {
	if len(args) != 0 {
		return usageError("mcp")
	}
	session := mcpSession{id: os.Getenv("RASHIN_SESSION"), cwd: os.Getenv("RASHIN_CWD")}
	if session.cwd == "" {
		session.cwd, _ = os.Getwd()
	}
	instructions := "Rashin's tools for this session. search, find, def, outline, references, impact and peek read Rashin's index of the session folder, which covers every source language in it. goal reports and ends the goal the user set with /goal."
	tools := append(codeTools(session), goalTool(session))
	// the daemon says whether this desktop can host a screen for the agent
	if os.Getenv("RASHIN_SCREEN") == "1" {
		instructions += " The screen tools give you a hidden desktop of your own to use apps on, once the user agrees."
		tools = append(tools, screenTools(session)...)
	}
	// the daemon names the team task the session works and the run's token
	if os.Getenv("RASHIN_TEAM") != "" {
		session.token = os.Getenv("RASHIN_TEAM_TOKEN")
		session.lead = os.Getenv("RASHIN_TEAM_LEAD") == "1"
		instructions += " The team tools serve your team task: report ends it, ask_user puts a decision to the user, and credentials, http_request, send_mail and request_credential use your team's credentials and mail without you ever seeing a value."
		tools = append(tools, teamTools(session)...)
	}
	server := &mcp.Server{Name: "rashin", Version: version, Instructions: instructions, Tools: tools}
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		return report(err)
	}
	return 0
}

// toolCall runs one daemon method for a tool and renders its answer the way
// the CLI does in TOON, which costs a model the fewest tokens
func toolCall(ctx context.Context, method string, params, result any) mcp.Result {
	conn, cancel, err := dial(true)
	if err != nil {
		return mcp.Failure(err)
	}
	defer cancel()
	defer conn.Close()
	if err := conn.Call(ctx, method, params, result); err != nil {
		return mcp.Failure(err)
	}
	var out strings.Builder
	if err := encodeTOON(&out, result); err != nil {
		return mcp.Failure(err)
	}
	return mcp.Text(out.String())
}

// toolArguments decodes a call's arguments and checks the required ones
func toolArguments(raw json.RawMessage, target any, required ...string) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return errors.New(i18n.T("The arguments are not valid JSON for this tool."))
	}
	var present map[string]json.RawMessage
	_ = json.Unmarshal(raw, &present)
	for _, name := range required {
		value, ok := present[name]
		if !ok || string(value) == `""` || string(value) == "null" {
			return errors.New(i18n.Tf("Missing %s.", name))
		}
	}
	return nil
}

// schema builds a JSON schema for an object of the named properties
func schema(required []string, properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

func stringProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integerProperty(description string) map[string]any {
	return map[string]any{"type": "integer", "minimum": 1, "description": description}
}
