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
		Context: core.ToolContext{Session: "session-1", Team: &core.TeamToolContext{Team: "watch", Token: "run-token", Lead: true, Delegate: true}},
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

func TestTeamToolsFollowTheRun(t *testing.T) {
	registry := tool.NewRegistry(Tools()...)
	for _, role := range []struct {
		name                  string
		lead, delegate, dream bool
	}{
		{name: "leaf"},
		{name: "peer"},
		{name: "non-lead manager", delegate: true},
		{name: "lead without reports", lead: true},
		{name: "lead with reports", lead: true, delegate: true},
		{name: "dream manager", lead: true, delegate: true, dream: true},
	} {
		t.Run(role.name, func(t *testing.T) {
			env := teamEnv(nil)
			env.Context.Team.Lead, env.Context.Team.Delegate, env.Context.Team.Dream = role.lead, role.delegate, role.dream
			if role.dream {
				env.Profile.Mode = tool.ModeDreamManager
			}
			set := registry.For(context.Background(), env)
			for name, want := range map[string]bool{
				tool.Report: !role.dream, tool.Delegate: role.delegate,
				tool.TeamStatus: true, tool.ProposePlan: true, tool.RequestHelp: true, tool.Escalate: true,
				tool.Hire: role.dream, tool.Release: role.dream, tool.Present: role.dream,
				tool.RequestSetup: false, tool.RequestHands: false,
			} {
				if _, got := set.Lookup(name); got != want {
					t.Errorf("%s offered = %v, want %v", name, got, want)
				}
			}
		})
	}
	env := teamEnv(nil)
	env.Profile.Mode = tool.ModeDreamManager // A mode name is not Dream authority.
	env.Context.Team.Lead, env.Context.Team.Delegate = false, false
	set := registry.For(context.Background(), env)
	for _, name := range []string{tool.Hire, tool.Release, tool.Present, tool.Delegate} {
		if _, ok := set.Lookup(name); ok {
			t.Errorf("a leaf selecting dream-manager mode received %s", name)
		}
	}
	env.Context.Team = nil
	set = registry.For(context.Background(), env)
	for _, name := range []string{tool.AskUser, tool.ProposePlan, tool.RequestHelp, tool.Escalate, tool.TeamStatus, tool.ScreenLook, tool.MemorySearch} {
		if _, ok := set.Lookup(name); ok {
			t.Errorf("a session without a live team, screen or memory received %s", name)
		}
	}
	if _, ok := set.Lookup(tool.Skills); !ok {
		t.Fatal("skills disappeared outside a team")
	}
}

// request_hands is offered only where Teams says the member may ask, and
// carries the goal and scope to teams.agent_hands with the run's token.
func TestRequestHandsAsksTeamsForHands(t *testing.T) {
	registry := tool.NewRegistry(Tools()...)
	host := &recordingHost{reply: `{"text":"Waiting for the user."}`}
	env := teamEnv(host)
	env.Context.Team.Hands = true
	if _, ok := registry.For(context.Background(), env).Lookup(tool.RequestHands); !ok {
		t.Fatal("request_hands is not offered where Teams allows it")
	}
	result := find(t, tool.RequestHands).Run(context.Background(), tool.Call{Args: json.RawMessage(`{"goal":"file the receipt","scope":"window","window":"0x7"}`)}, env)
	if result.IsError || len(host.methods) != 1 || host.methods[0] != protocol.MTeamsAgentHands {
		t.Fatalf("result = %+v, called %v", result, host.methods)
	}
	var sent protocol.AgentHandsParams
	if err := json.Unmarshal(host.params[0], &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Goal != "file the receipt" || sent.Scope != protocol.OniHandsScopeWindow || sent.Window != "0x7" || sent.Token != "run-token" {
		t.Fatalf("sent %+v", sent)
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
