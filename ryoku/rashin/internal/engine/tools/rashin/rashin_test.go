package rashin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// recordingHost answers every call with reply and keeps what it was sent.
type recordingHost struct {
	methods []string
	params  []json.RawMessage
	reply   string
}

func (h *recordingHost) Call(_ context.Context, method string, params, result any) error {
	raw, _ := json.Marshal(params)
	h.methods = append(h.methods, method)
	h.params = append(h.params, raw)
	if result != nil && h.reply != "" {
		return json.Unmarshal([]byte(h.reply), result)
	}
	return nil
}

func find(t *testing.T, name string) tool.Tool {
	t.Helper()
	for _, candidate := range Tools() {
		if candidate.Name() == name {
			return candidate
		}
	}
	t.Fatalf("no tool %s", name)
	return nil
}

func teamEnv(host tool.Host) tool.Env {
	return tool.Env{
		Session: protocol.SessionInfo{ID: "session-1"},
		Cwd:     "/teams/watch",
		Profile: tool.Profile{Mode: tool.ModeTeam, Class: tool.ClassFull},
		Host:    host,
		Context: core.ToolContext{Session: "session-1", Team: &core.TeamToolContext{Team: "watch", Token: "run-token", Lead: true}},
	}
}

func TestUnknownArgumentIsRefusedWithTheOnesTheToolTakes(t *testing.T) {
	host := &recordingHost{}
	result := find(t, tool.Report).Run(context.Background(), tool.Call{Args: json.RawMessage(`{"title":"Done","summary":"all of it"}`)}, teamEnv(host))
	if !result.IsError || !strings.Contains(result.Text, "summary") || !strings.Contains(result.Text, "title, body, files, quiet") {
		t.Fatalf("result = %+v", result)
	}
	if len(host.methods) != 0 {
		t.Fatalf("a refused call reached the daemon: %v", host.methods)
	}
}

func TestToolWithoutArgumentsRefusesAny(t *testing.T) {
	host := &recordingHost{}
	result := find(t, tool.TeamStatus).Run(context.Background(), tool.Call{Args: json.RawMessage(`{"all":true}`)}, teamEnv(host))
	if !result.IsError || !strings.Contains(result.Text, "takes none") || len(host.methods) != 0 {
		t.Fatalf("result = %+v, calls %v", result, host.methods)
	}
	if result := find(t, tool.TeamStatus).Run(context.Background(), tool.Call{Args: json.RawMessage(``)}, teamEnv(&recordingHost{reply: `{"tasks":[]}`})); result.IsError {
		t.Fatalf("empty arguments were refused: %+v", result)
	}
}

func TestMissingRequiredArgumentIsNamed(t *testing.T) {
	for _, args := range []string{`{"member":"Ada","title":"Tests"}`, `{"member":"Ada","title":"Tests","brief":""}`, `{"member":"Ada","title":"Tests","brief":null}`} {
		result := find(t, tool.Delegate).Run(context.Background(), tool.Call{Args: json.RawMessage(args)}, teamEnv(&recordingHost{}))
		if !result.IsError || result.Text != "Missing brief." {
			t.Fatalf("%s: result = %+v", args, result)
		}
	}
	result := find(t, tool.SendMail).Run(context.Background(), tool.Call{Args: json.RawMessage(`{"to":[],"subject":"Hi","body":"Hello"}`)}, teamEnv(&recordingHost{}))
	if !result.IsError || result.Text != "Missing to." {
		t.Fatalf("empty list: %+v", result)
	}
}

func TestInvalidJSONIsRefused(t *testing.T) {
	for _, args := range []string{`[1,2]`, `{"title":`, `"text"`} {
		result := find(t, tool.Report).Run(context.Background(), tool.Call{Args: json.RawMessage(args)}, teamEnv(&recordingHost{}))
		if !result.IsError || !strings.Contains(result.Text, "not valid JSON") {
			t.Fatalf("%s: result = %+v", args, result)
		}
	}
	result := find(t, tool.Report).Run(context.Background(), tool.Call{Args: json.RawMessage(`{"title":"A","body":"B","quiet":"yes"}`)}, teamEnv(&recordingHost{}))
	if !result.IsError || !strings.Contains(result.Text, "not valid JSON") {
		t.Fatalf("wrong type: %+v", result)
	}
}

func TestAliasNamesTheArgument(t *testing.T) {
	host := &recordingHost{reply: `{"screen":{"action":"Opened kitty","width":1280,"height":800}}`}
	env := teamEnv(host)
	env.Context.Screen = true
	result := find(t, tool.ScreenOpen).Run(context.Background(), tool.Call{Args: json.RawMessage(`{"cmd":"kitty","look":false}`)}, env)
	if result.IsError || len(host.params) == 0 || !strings.Contains(string(host.params[0]), `"command":"kitty"`) {
		t.Fatalf("result = %+v, sent %s", result, host.params)
	}
}

func TestTeamToolsNameTheirRun(t *testing.T) {
	host := &recordingHost{reply: `{"text":"Handed to Ada."}`}
	result := find(t, tool.Report).Run(context.Background(), tool.Call{Args: json.RawMessage(`{"title":"Done","body":"Read it.","files":["notes.md","/abs/out.txt"]}`)}, teamEnv(host))
	if result.IsError || result.Text != "Handed to Ada." {
		t.Fatalf("result = %+v", result)
	}
	var sent protocol.AgentReportParams
	if err := json.Unmarshal(host.params[0], &sent); err != nil {
		t.Fatal(err)
	}
	if host.methods[0] != protocol.MTeamsAgentReport || sent.Session != "session-1" || sent.Token != "run-token" {
		t.Fatalf("sent %s %+v", host.methods[0], sent)
	}
	if len(sent.Files) != 2 || sent.Files[0] != "/teams/watch/notes.md" || sent.Files[1] != "/abs/out.txt" {
		t.Fatalf("files = %v", sent.Files)
	}
}

func TestTeamToolsFollowTheRun(t *testing.T) {
	env := teamEnv(nil)
	member := *env.Context.Team
	member.Lead = false
	env.Context.Team = &member
	enabled := func(name string) bool {
		c, ok := find(t, name).(tool.Conditional)
		return !ok || c.Enabled(env)
	}
	if !enabled(tool.Report) || enabled(tool.Delegate) || enabled(tool.Hire) || enabled(tool.RequestSetup) {
		t.Fatal("a member's tools are wrong")
	}
	member.Lead, member.Dream = true, true
	if enabled(tool.Report) || !enabled(tool.Delegate) || !enabled(tool.Present) {
		t.Fatal("a dream manager's tools are wrong")
	}
	env.Context.Team = nil
	if enabled(tool.AskUser) || enabled(tool.ScreenLook) || enabled(tool.MemorySearch) || !enabled(tool.Skills) {
		t.Fatal("a session without a team, screen or memory got their tools")
	}
}

func TestScreenWaitSaysNothingWasDone(t *testing.T) {
	host := &recordingHost{reply: `{"screen":{"state":"starting"},"waiting":"consent"}`}
	env := teamEnv(host)
	env.Context.Screen = true
	result := find(t, tool.ScreenClick).Run(context.Background(), tool.Call{Args: json.RawMessage(`{"x":10,"y":20}`)}, env)
	if result.IsError || !strings.HasPrefix(result.Text, "Nothing was done yet") {
		t.Fatalf("result = %+v", result)
	}
}

func TestCompactSchemasStayFlat(t *testing.T) {
	for _, mode := range tool.Modes {
		profile := tool.Profile{Mode: mode, Class: tool.ClassCompact}
		for _, candidate := range Tools() {
			if !tool.Offers(mode, tool.ClassCompact, candidate.Name()) {
				continue
			}
			var schema struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
				Required []string `json:"required"`
			}
			if err := json.Unmarshal(candidate.Spec(profile).Parameters, &schema); err != nil {
				t.Fatal(err)
			}
			lists := 0
			for name, property := range schema.Properties {
				switch property.Type {
				case "string", "boolean", "integer":
				case "array":
					lists++
				default:
					t.Errorf("%s %s: %s is %q", mode, candidate.Name(), name, property.Type)
				}
			}
			if lists > 1 || len(schema.Required) > 3 {
				t.Errorf("%s %s: %d lists, %d required", mode, candidate.Name(), lists, len(schema.Required))
			}
		}
	}
}
