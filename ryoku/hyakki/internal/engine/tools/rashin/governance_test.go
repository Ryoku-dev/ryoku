package rashin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

type governanceHost func(context.Context, string, any, any) error

func (h governanceHost) Call(ctx context.Context, method string, params, result any) error {
	return h(ctx, method, params, result)
}

var governanceCalls = []struct {
	name string
	args string
}{
	{tool.ProposePlan, `{"summary":"Verify the export","steps":["Inspect the parser","Check the result"]}`},
	{tool.RequestHelp, `{"member":"peer-1","title":"Review the parser","brief":"Find an independent counterexample."}`},
	{tool.Escalate, `{"title":"Need a decision","detail":"The requested policy conflicts with the task."}`},
}

func TestGovernanceYieldsOnlyAfterTheLiveRunAcceptsTheRequest(t *testing.T) {
	for _, call := range governanceCalls {
		t.Run(call.name, func(t *testing.T) {
			accepted := false
			calls := 0
			host := governanceHost(func(_ context.Context, _ string, params, result any) error {
				calls++
				var ref protocol.AgentRef
				switch p := params.(type) {
				case protocol.AgentPlanParams:
					ref = p.AgentRef
				case protocol.AgentHelpParams:
					ref = p.AgentRef
				case protocol.AgentEscalateParams:
					ref = p.AgentRef
				default:
					t.Fatalf("unexpected governance request %T", params)
				}
				if ref.Session != "session-1" || ref.Token != "run-token" {
					return protocol.Errorf(protocol.CodeForbidden, "This run is no longer current.")
				}
				accepted = true
				result.(*protocol.AgentAnswer).Text = "Waiting for the manager or Summoner."
				return nil
			})
			env := teamEnv(host)
			env.Context.Team.Lead, env.Context.Team.Delegate = false, false
			env.Context.Team.Token = "revoked-token"
			set := tool.NewRegistry(Tools()...).For(context.Background(), env)
			candidate, ok := set.Lookup(call.name)
			if !ok {
				t.Fatal("a live leaf was not offered governance")
			}
			result := candidate.Run(context.Background(), tool.Call{Args: json.RawMessage(call.args)}, env)
			if accepted || !result.IsError || result.End || !strings.Contains(result.Text, "no longer current") {
				t.Fatalf("a rejected request yielded: %+v, accepted %v", result, accepted)
			}
			// The in-process context, not another envelope's session ID, is the authority.
			env.Session.ID = "another-session"
			env.Context.Team.Token = "run-token"
			result = candidate.Run(context.Background(), tool.Call{Args: json.RawMessage(call.args)}, env)
			if !accepted || result.IsError || !result.End || result.Text != "Waiting for the manager or Summoner." || calls != 2 {
				t.Fatalf("an accepted request failed to yield: %+v, accepted %v, calls %d", result, accepted, calls)
			}
		})
	}
}

func TestGovernanceFailureDoesNotEndTheRun(t *testing.T) {
	for _, call := range governanceCalls {
		for _, failure := range []error{context.Canceled, errors.New("Strategy is already waiting for a decision.")} {
			t.Run(call.name+"/"+failure.Error(), func(t *testing.T) {
				env := teamEnv(governanceHost(func(context.Context, string, any, any) error { return failure }))
				result := find(t, call.name).Run(context.Background(), tool.Call{Args: json.RawMessage(call.args)}, env)
				if !result.IsError || result.End || result.Text != failure.Error() {
					t.Fatalf("failure ended the run or was hidden: %+v", result)
				}
			})
		}
	}
}

func TestGovernanceRejectsInjectedIdentityAndUnexpectedArguments(t *testing.T) {
	for _, call := range governanceCalls {
		for _, injected := range []string{`"session":"another-session"`, `"token":"run-token"`, `"team":"another-team"`, `"app":true`, `"unknown":"discard me"`} {
			t.Run(call.name+"/"+injected, func(t *testing.T) {
				host := &recordingHost{}
				args := strings.TrimSuffix(call.args, "}") + "," + injected + "}"
				result := find(t, call.name).Run(context.Background(), tool.Call{Args: json.RawMessage(args)}, teamEnv(host))
				if !result.IsError || result.End || !strings.Contains(result.Text, "not an argument") || len(host.methods) != 0 {
					t.Fatalf("injected argument reached governance: %+v, calls %v", result, host.methods)
				}
			})
		}
	}
}

func TestGovernanceRequiresLiveInProcessIdentity(t *testing.T) {
	registry := tool.NewRegistry(Tools()...)
	for _, missing := range []string{"team", "session", "token"} {
		t.Run(missing, func(t *testing.T) {
			host := &recordingHost{}
			env := teamEnv(host)
			switch missing {
			case "team":
				env.Context.Team = nil
			case "session":
				env.Context.Session = ""
			case "token":
				env.Context.Team.Token = ""
			}
			set := registry.For(context.Background(), env)
			for _, call := range governanceCalls {
				if _, ok := set.Lookup(call.name); ok {
					t.Errorf("%s offered without live %s", call.name, missing)
				}
				result := find(t, call.name).Run(context.Background(), tool.Call{Args: json.RawMessage(call.args)}, env)
				if !result.IsError || result.End || len(host.methods) != 0 {
					t.Fatalf("missing %s reached governance: %+v", missing, result)
				}
			}
		})
	}
}

func TestGovernanceIsAvailableInReadOnlyTeamAndDreamRuns(t *testing.T) {
	registry := tool.NewRegistry(Tools()...)
	for _, mode := range []tool.Mode{tool.ModeTeam, tool.ModeDreamManager, tool.ModeAgent, tool.ModeAskRead} {
		env := teamEnv(nil)
		env.Profile.Mode, env.Profile.ReadOnly = mode, true
		set := registry.For(context.Background(), env)
		for _, call := range governanceCalls {
			_, offered := set.Lookup(call.name)
			if want := mode == tool.ModeTeam || mode == tool.ModeDreamManager; offered != want {
				t.Errorf("%s in %s read-only = %v, want %v", call.name, mode, offered, want)
			}
		}
	}
}

func TestGovernanceRejectsMalformedOrOversizedRequestsBeforeCallingTheDaemon(t *testing.T) {
	encode := func(value any) string { raw, _ := json.Marshal(value); return string(raw) }
	tests := []struct{ name, args string }{
		{tool.ProposePlan, `{"summary":"Plan","steps":"not a list"}`},
		{tool.ProposePlan, `{"summary":"Plan","steps":[null]}`},
		{tool.ProposePlan, `{"summary":"Plan","steps":["   "]}`},
		{tool.ProposePlan, `{"summary":" ","steps":["work"]}`},
		{tool.ProposePlan, `{"summary":"Plan","steps":[]}`},
		{tool.ProposePlan, encode(map[string]any{"summary": strings.Repeat("界", 1366), "steps": []string{"work"}})},
		{tool.ProposePlan, encode(map[string]any{"summary": "Plan", "steps": make([]string, 33)})},
		{tool.ProposePlan, encode(map[string]any{"summary": "Plan", "steps": []string{strings.Repeat("s", 2049)}})},
		{tool.RequestHelp, `{"member":" ","title":"Help","brief":"Review"}`},
		{tool.RequestHelp, `{"member":"peer","title":true,"brief":"Review"}`},
		{tool.RequestHelp, `{"member":"peer","title":"Help","brief":null}`},
		{tool.Escalate, `{"title":" ","detail":"Need advice"}`},
		{tool.Escalate, `{"title":"Blocked","detail":" "}`},
	}
	for _, name := range []string{tool.RequestHelp, tool.Escalate} {
		body := "brief"
		if name == tool.Escalate {
			body = "detail"
		}
		for _, fields := range []map[string]string{
			{"member": "peer", "title": strings.Repeat("界", 201), body: "Need advice"},
			{"member": "peer", "title": "Blocked", body: strings.Repeat("b", 16*1024+1)},
		} {
			if name == tool.Escalate {
				delete(fields, "member")
			}
			tests = append(tests, struct{ name, args string }{name, encode(fields)})
		}
	}
	for _, test := range tests {
		host := &recordingHost{}
		result := find(t, test.name).Run(context.Background(), tool.Call{Args: json.RawMessage(test.args)}, teamEnv(host))
		if !result.IsError || result.End || len(host.methods) != 0 {
			t.Errorf("%s accepted malformed request: %+v, calls %v", test.name, result, host.methods)
		}
	}
}

func TestGovernanceAcceptsItsDocumentedUnicodeAndSizeBoundaries(t *testing.T) {
	for _, request := range []struct {
		name   string
		fields map[string]any
	}{
		{tool.ProposePlan, map[string]any{"summary": strings.Repeat("s", 4096), "steps": []string{strings.Repeat("s", 2048)}}},
		{tool.RequestHelp, map[string]any{"member": "peer-1", "title": strings.Repeat("界", 200), "brief": strings.Repeat("b", 16*1024)}},
		{tool.Escalate, map[string]any{"title": strings.Repeat("界", 200), "detail": strings.Repeat("b", 16*1024)}},
	} {
		host := &recordingHost{reply: `{"text":"Waiting."}`}
		raw, err := json.Marshal(request.fields)
		if err != nil {
			t.Fatal(err)
		}
		result := find(t, request.name).Run(context.Background(), tool.Call{Args: raw}, teamEnv(host))
		if result.IsError || !result.End || len(host.methods) != 1 {
			t.Errorf("%s refused its documented boundary: %+v", request.name, result)
		}
	}
}

func TestEverySeatReadsGoalLineageAndSafeDiscoveryWithoutRuntimeSecrets(t *testing.T) {
	host := &recordingHost{reply: `{
		"context":{"task":{"id":"task-1","title":"Inspect parser"},"goal":"Reliable export","because":[{"id":"parent-1","title":"Ship export","member":"manager-1"}]},
		"members":[{"id":"peer-1","name":"Reviewer","capabilities":"Independent verification","reportsTo":"manager-1","runtime":{"env":{"PRIVATE":"private-env-canary"},"secretEnv":{"KEY":"vault-ref-canary"}}}],
		"tasks":[{"id":"piece-1","title":"Review counterexample","status":"done","report":{"body":"Counterexample verified"}}],
		"runtime":{"command":["private-command-canary"]}}
	`}
	env := teamEnv(host)
	env.Context.Team.Lead, env.Context.Team.Delegate = false, false
	set := tool.NewRegistry(Tools()...).For(context.Background(), env)
	status, ok := set.Lookup(tool.TeamStatus)
	if !ok {
		t.Fatal("a leaf cannot read the team's safe status")
	}
	result := status.Run(context.Background(), tool.Call{Args: json.RawMessage(`{}`)}, env)
	if result.IsError || result.End {
		t.Fatalf("reading status ended or refused the run: %+v", result)
	}
	for _, text := range []string{"Reliable export", "Ship export", "Reviewer", "Independent verification", "Review counterexample"} {
		if !strings.Contains(result.Text, text) {
			t.Errorf("safe status omitted %q: %s", text, result.Text)
		}
	}
	for _, secret := range []string{"private-env-canary", "vault-ref-canary", "private-command-canary"} {
		if strings.Contains(result.Text, secret) {
			t.Errorf("status exposed private runtime data %q", secret)
		}
	}
}

func TestCompactTeamAndDreamSeatsCanCommunicateAndManagersKeepTheirAuthority(t *testing.T) {
	registry := tool.NewRegistry(Tools()...)
	for _, role := range []struct {
		name                  string
		mode                  tool.Mode
		lead, delegate, dream bool
	}{
		{name: "leaf", mode: tool.ModeTeam},
		{name: "non-lead manager", mode: tool.ModeTeam, delegate: true},
		{name: "dream manager", mode: tool.ModeDreamManager, lead: true, delegate: true, dream: true},
		{name: "leaf claiming Dream mode", mode: tool.ModeDreamManager},
	} {
		t.Run(role.name, func(t *testing.T) {
			host := &recordingHost{reply: `{"text":"Waiting."}`}
			env := teamEnv(host)
			env.Profile.Mode, env.Profile.Class, env.Profile.ReadOnly = role.mode, tool.ClassCompact, true
			env.Context.Team.Lead, env.Context.Team.Delegate, env.Context.Team.Dream = role.lead, role.delegate, role.dream
			set := registry.For(context.Background(), env)
			if set.Len() > tool.MaxCompactTools {
				t.Fatalf("compact seat received %d tools", set.Len())
			}
			for _, call := range governanceCalls {
				if role.mode == tool.ModeDreamManager && call.name == tool.RequestHelp {
					continue // The root manager delegates downline; compact keeps its phase mutation.
				}
				candidate, ok := set.Lookup(call.name)
				if !ok {
					t.Fatalf("compact seat cannot communicate through %s", call.name)
				}
				result := candidate.Run(context.Background(), tool.Call{Args: json.RawMessage(call.args)}, env)
				if result.IsError || !result.End {
					t.Errorf("%s did not yield after accepted communication: %+v", call.name, result)
				}
			}
			for name, want := range map[string]bool{
				tool.TeamStatus: true, tool.Delegate: role.delegate,
				tool.Hire: role.dream, tool.Release: role.dream, tool.Present: role.dream,
				tool.RequestHelp: role.mode == tool.ModeTeam, tool.DreamStatus: role.dream,
			} {
				if _, got := set.Lookup(name); got != want {
					t.Errorf("%s offered = %v, want %v", name, got, want)
				}
			}
			if role.dream {
				update, ok := set.Lookup(tool.DreamStatus)
				if !ok {
					t.Fatal("compact Dream manager cannot update its phase")
				}
				result := update.Run(context.Background(), tool.Call{Args: json.RawMessage(`{"phase":"building","summary":"Implementing the accepted brief"}`)}, env)
				if result.IsError || result.End {
					t.Errorf("updating the Dream phase failed or yielded: %+v", result)
				}
			}
		})
	}
}

func TestCompactDreamStatusIncludesCurrentPhaseAndCEOPresentationAnswer(t *testing.T) {
	host := &recordingHost{reply: `{
		"tasks":[],
		"dream":{"statement":"Build a reliable exporter","brief":"CSV with independent verification","phase":"building","summary":"Addressing CEO feedback","manager":"member-1","rounds":1,
			"presentation":{"title":"Exporter demonstration","body":"Ready for review","tests":"Parser cases checked","verdict":"notes","note":"Also support quoted newlines"}}}
	`}
	env := teamEnv(host)
	env.Profile.Mode, env.Profile.Class = tool.ModeDreamManager, tool.ClassCompact
	env.Context.Team.Dream = true
	set := tool.NewRegistry(Tools()...).For(context.Background(), env)
	status, ok := set.Lookup(tool.TeamStatus)
	if !ok {
		t.Fatal("compact Dream manager has no status view")
	}
	result := status.Run(context.Background(), tool.Call{Args: json.RawMessage(`{}`)}, env)
	if result.IsError || result.End {
		t.Fatalf("reading Dream status yielded or failed: %+v", result)
	}
	for _, text := range []string{"building", "CSV with independent verification", "Addressing CEO feedback", "Exporter demonstration", "notes", "Also support quoted newlines"} {
		if !strings.Contains(result.Text, text) {
			t.Errorf("Dream status omitted %q: %s", text, result.Text)
		}
	}
	if len(host.methods) != 1 || host.methods[0] != protocol.MTeamsAgentStatus {
		t.Errorf("status used a second RPC instead of the task's status view: %v", host.methods)
	}
}
