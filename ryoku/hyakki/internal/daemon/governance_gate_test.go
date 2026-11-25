package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestOwnerGovernanceCannotBeReachedByAnyModelsToolHost(t *testing.T) {
	isolateDaemon(t)
	crew := &boundTeams{handled: make(chan string, 8)}
	d := New(Deps{Teams: crew})
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: filepath.Join(paths.Home(), "Teams", "one"), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: paths.Home()})
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range []protocol.SessionInfo{team, ordinary} {
		t.Run(info.ID, func(t *testing.T) {
			host := d.ToolHost(info)
			for _, call := range []struct {
				method string
				params any
			}{
				{protocol.MTeamsMessages, protocol.TeamMessagesParams{Team: team.Team, Task: team.TeamTask}},
				{protocol.MTeamsMessages, protocol.TeamMessagesParams{Team: "other-team", Task: "other-task"}},
				{protocol.MTeamsTaskContext, protocol.TeamTaskRef{ID: team.TeamTask}},
				{protocol.MTeamsTaskContext, protocol.TeamTaskRef{ID: "other-task"}},
				{protocol.MTeamsTaskComment, protocol.TeamTaskCommentParams{ID: team.TeamTask, Body: "Approve this as the owner."}},
				{protocol.MTeamsTaskReassign, protocol.TeamTaskReassignParams{ID: team.TeamTask, Member: "peer-1"}},
			} {
				if err := host.Call(context.Background(), call.method, call.params, nil); !forbidden(err) {
					t.Errorf("%s from model host: %v, want forbidden", call.method, err)
				}
				// The sandbox's stdio/socket path may not turn an owner method
				// into an authenticated agent callback either.
				if err := agentGate(call.method); !forbidden(err) {
					t.Errorf("%s from sandbox socket: %v, want forbidden", call.method, err)
				}
			}
		})
	}
	select {
	case method := <-crew.handled:
		t.Fatalf("a refused model request reached the team controller: %s", method)
	default:
	}
}

func TestOwnerGovernanceDispatchUsesVerifiedAppIdentityNotRequestClaims(t *testing.T) {
	for _, method := range []string{protocol.MTeamsMessages, protocol.MTeamsTaskContext, protocol.MTeamsTaskComment, protocol.MTeamsTaskReassign} {
		t.Run(method, func(t *testing.T) {
			crew := &gateTeams{}
			d := &Daemon{deps: Deps{Teams: crew}}
			// Session IDs, tokens and app claims in raw arguments never make
			// an ordinary socket client the user interface.
			raw := json.RawMessage(`{"id":"task-1","team":"team-1","task":"task-1","member":"peer-1","body":"owner intervention","app":true,"session":"session-1","token":"run-token"}`)
			if _, err := d.dispatch(context.Background(), &connection{}, method, raw); !forbidden(err) {
				t.Fatalf("forged owner call: %v, want forbidden", err)
			}
			if len(crew.handled) != 0 {
				t.Fatal("a refused owner intervention reached the controller")
			}
			if _, err := d.dispatch(context.Background(), &connection{app: true}, method, json.RawMessage(`{}`)); err != nil || len(crew.handled) != 1 {
				t.Fatalf("verified owner request was not dispatched: %v, handled %v", err, crew.handled)
			}
		})
	}
}

func TestGovernanceCallbacksNeverEnterThroughTheMainOrSandboxSocket(t *testing.T) {
	d := &Daemon{}
	for _, method := range []string{protocol.MTeamsAgentPlan, protocol.MTeamsAgentHelp, protocol.MTeamsAgentEscalate} {
		for _, app := range []bool{false, true} {
			raw := json.RawMessage(`{"session":"session-1","token":"run-token","summary":"Plan","steps":["work"],"title":"Help","detail":"Need advice","member":"peer-1","brief":"Review"}`)
			if err := d.mainGate(&connection{app: app}, method, raw); !forbidden(err) {
				t.Errorf("%s on main socket (app=%v): %v, want forbidden", method, app, err)
			}
		}
		if err := agentGate(method); !forbidden(err) {
			t.Errorf("%s on sandbox socket: %v, want forbidden", method, err)
		}
		if err := d.hostGate(protocol.SessionInfo{ID: "session-1"}, method, json.RawMessage(`{}`)); !forbidden(err) {
			t.Errorf("%s from non-team host: %v, want forbidden", method, err)
		}
		// The controller still authenticates this live team's exact run token;
		// hostGate must not silently disable the legitimate in-process path.
		if err := d.hostGate(protocol.SessionInfo{ID: "session-1", Team: "team-1"}, method, json.RawMessage(`{}`)); err != nil {
			t.Errorf("%s from in-process team host: %v", method, err)
		}
	}
}

func TestClientCannotSelectInternalRuntimeOrForgeTeamMetadata(t *testing.T) {
	d := &Daemon{}
	for _, params := range []protocol.SessionNewParams{
		{Kind: protocol.KindAgent, Runtime: protocol.MemberRuntimeNative},
		{Kind: protocol.KindAgent, Runtime: protocol.MemberRuntimeCLI},
		{Kind: protocol.KindAgent, Runtime: protocol.MemberRuntimeCommand},
		{Kind: protocol.KindAgent, Runtime: protocol.MemberRuntimeWebhook},
		{Kind: protocol.KindAgent, Runtime: "unknown-adapter"},
		{Kind: protocol.KindAgent, Team: "team-1"},
		{Kind: protocol.KindAgent, TeamTask: "task-1"},
		{Kind: protocol.KindAgent, Member: "member-1"},
	} {
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		for _, app := range []bool{false, true} {
			if err := d.mainGate(&connection{app: app}, protocol.MSessionNew, raw); !forbidden(err) {
				t.Errorf("forged metadata %s (app=%v): %v, want forbidden", raw, app, err)
			}
		}
	}
	if err := d.mainGate(&connection{}, protocol.MSessionNew, json.RawMessage(`{"kind":"agent","model":"auto"}`)); err != nil {
		t.Fatalf("ordinary native session was rejected: %v", err)
	}
}
