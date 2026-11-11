package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// boundTeams answers every Teams method and records which ones arrived.
type boundTeams struct {
	core.Teams
	handled chan string
}

func (t *boundTeams) Bind(core.TeamSessions) {}

func (t *boundTeams) Handle(_ context.Context, method string, _ json.RawMessage) (any, error) {
	t.handled <- method
	return protocol.Empty{}, nil
}

func (t *boundTeams) SessionChanged(protocol.SessionInfo) {}

// shortRuntime keeps socket paths under the length a Unix socket allows.
func shortRuntime(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "rashin-run")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
}

func forbidden(err error) bool {
	var refused *protocol.Error
	return errors.As(err, &refused) && refused.Code == protocol.CodeForbidden
}

func TestAnAgentSocketServesOnlyItsOwnSessionsTools(t *testing.T) {
	isolateDaemon(t)
	shortRuntime(t)
	crew := &boundTeams{handled: make(chan string, 8)}
	d := New(Deps{Teams: crew})
	team, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: filepath.Join(paths.Home(), "Teams", "one"), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := d.manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: paths.Home()})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := d.agentSandbox(team)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.closeAgentSockets)
	if info, statErr := os.Stat(team.Cwd); statErr != nil || !info.IsDir() {
		t.Fatalf("the team folder was not made: %v", statErr)
	}
	runtime := paths.Resolved(paths.RuntimeDir())
	if len(spec.Binds) != 1 || spec.Binds[0].Target != runtime || spec.Env["RASHIN_SOCKET"] != filepath.Join(runtime, "rashin.sock") {
		t.Fatalf("the agent socket is not where the daemon's is inside: %+v", spec)
	}
	for _, hidden := range []string{paths.StateDir(), paths.Keyrings(), paths.VaultDir(), paths.TeamsHome()} {
		if !slices.Contains(spec.Hidden, paths.Resolved(hidden)) {
			t.Fatalf("the sandbox does not hide %s: %v", hidden, spec.Hidden)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := client.Dial(ctx, client.Options{Client: protocol.ClientCLI, Socket: paths.AgentSocket(team.ID)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var info protocol.SessionInfo
	if err := conn.Call(ctx, protocol.MSessionGoal, protocol.GoalParams{ID: team.ID, Op: protocol.GoalGet}, &info); err != nil || info.ID != team.ID {
		t.Fatalf("its own goal: %v, %+v", err, info)
	}
	if err := conn.Call(ctx, protocol.MTeamsAgentStatus, protocol.AgentRef{Session: team.ID, Token: "token"}, nil); err != nil {
		t.Fatalf("a team tool: %v", err)
	}
	if method := <-crew.handled; method != protocol.MTeamsAgentStatus {
		t.Fatalf("Teams heard %s", method)
	}
	for _, refused := range []struct {
		method string
		params any
	}{
		{protocol.MVaultAdd, protocol.VaultAddParams{Name: "TOKEN", Value: "secret"}},
		{protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: "approval-1", Decision: protocol.DecideAlways}},
		{protocol.MSessionGoal, protocol.GoalParams{ID: other.ID, Op: protocol.GoalGet}},
		{protocol.MSessionList, protocol.SessionListParams{}},
		{protocol.MSessionPermission, protocol.PermissionParams{ID: team.ID, Request: "r", Option: "allow"}},
		{protocol.MScreenAct, protocol.ScreenActParams{ID: team.ID, By: protocol.HolderUser}},
		{protocol.MIndexAdd, protocol.IndexAddParams{Path: paths.Home()}},
	} {
		if err := conn.Call(ctx, refused.method, refused.params, nil); !forbidden(err) {
			t.Fatalf("%s on an agent socket: %v, want forbidden", refused.method, err)
		}
	}
	select {
	case method := <-crew.handled:
		t.Fatalf("a refused call reached Teams: %s", method)
	default:
	}

	d.closeAgentSocket(team.ID)
	select {
	case <-conn.Done():
	case <-ctx.Done():
		t.Fatal("closing the agent socket left its connection open")
	}
	if _, err := os.Stat(filepath.Dir(paths.AgentSocket(team.ID))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the agent socket's dir is still there: %v", err)
	}
}

func TestTheMainSocketLeavesTeamSessionsToTheApp(t *testing.T) {
	isolateDaemon(t)
	settings := func() protocol.Settings { return protocol.Settings{} }
	manager := newSessionManager(nil, nil, nil, settings, func(string, any) {}, func() {}, func(string, ...any) {})
	team, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: paths.Home(), Team: "team-1", TeamTask: "task-1", Member: "member-1"})
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := manager.create(protocol.SessionNewParams{Kind: protocol.KindAgent, Cwd: paths.Home()})
	if err != nil {
		t.Fatal(err)
	}
	d := &Daemon{manager: manager}
	permission := func(id string) json.RawMessage {
		raw, _ := json.Marshal(protocol.PermissionParams{ID: id, Request: "r", Option: "allow"})
		return raw
	}
	if err := d.mainGate(&connection{}, protocol.MSessionPermission, permission(team.ID)); !forbidden(err) {
		t.Fatalf("another client answering a team session: %v, want forbidden", err)
	}
	if err := d.mainGate(&connection{}, protocol.MSessionPermission, permission(ordinary.ID)); err != nil {
		t.Fatalf("another client answering an ordinary session: %v", err)
	}
	if err := d.mainGate(&connection{app: true}, protocol.MSessionPermission, permission(team.ID)); err != nil {
		t.Fatalf("the app answering a team session: %v", err)
	}
	made, _ := json.Marshal(protocol.SessionNewParams{Kind: protocol.KindAgent, Team: "team-1"})
	if err := d.mainGate(&connection{app: true}, protocol.MSessionNew, made); !forbidden(err) {
		t.Fatalf("a client making a team session: %v, want forbidden", err)
	}
	report, _ := json.Marshal(protocol.AgentReportParams{AgentRef: protocol.AgentRef{Session: team.ID}})
	if err := d.mainGate(&connection{}, protocol.MTeamsAgentReport, report); !forbidden(err) {
		t.Fatalf("a team tool on the main socket: %v, want forbidden", err)
	}
}
