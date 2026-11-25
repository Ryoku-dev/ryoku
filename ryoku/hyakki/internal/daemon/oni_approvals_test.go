package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/teams"
)

// approvalsRig is a daemon on the real socket with real Teams, its members
// played by turn, Hands' own loop recorded instead of driving anything.
type approvalsRig struct {
	d    *Daemon
	crew *teams.Service
	app  *atomic.Bool

	mu     sync.Mutex
	starts []string
	turns  func(turn int, ref protocol.AgentRef, sink core.Sink, runner *fakeRunner) (core.TurnResult, error)
}

// serveApprovals starts the daemon with Oni on and Hands set to work in
// apps; each turn of a team task's member runs turns.
func serveApprovals(t *testing.T, turns func(turn int, ref protocol.AgentRef, sink core.Sink, runner *fakeRunner) (core.TurnResult, error)) *approvalsRig {
	t.Helper()
	isolateDaemon(t)
	shortRuntime(t)
	t.Setenv("HYAKKI_SOCKET", "")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "absent"))
	if err := os.MkdirAll(os.Getenv("XDG_RUNTIME_DIR"), 0o700); err != nil {
		t.Fatal(err)
	}
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.Enabled, settings.Oni.Master, settings.Oni.Inspect, settings.Oni.Hands = true, true, true, protocol.OniHandsApp
	settings.Teams.RoutinesWake = false
	if err := config.Save(settings); err != nil {
		t.Fatal(err)
	}
	r := &approvalsRig{app: &atomic.Bool{}, turns: turns}
	events := NewBroadcaster()
	var daemon atomic.Pointer[Daemon]
	crew, err := teams.Open(events, func() protocol.Settings {
		if d := daemon.Load(); d != nil {
			return d.Settings()
		}
		return settings
	})
	if err != nil {
		t.Fatal(err)
	}
	crew.SetModels(func(context.Context, protocol.ModelsParams) (protocol.ModelsResult, error) {
		return protocol.ModelsResult{Models: []protocol.ModelInfo{{ID: "test/local", Provider: "test", Available: true, Tools: true, Tier: protocol.TierFree}}}, nil
	})
	r.crew = crew
	var turn atomic.Int32
	agents := supervisedTeamAgents(func(_ context.Context, opts core.AgentOptions) (core.Runner, error) {
		runner := &fakeRunner{alive: true}
		runner.prompt = func(_ context.Context, _ core.PromptInput, sink core.Sink) (core.TurnResult, error) {
			ref := protocol.AgentRef{Session: opts.Session.ID, Token: opts.Tools.Team.Token}
			return r.turns(int(turn.Add(1)), ref, sink, runner)
		}
		return runner, nil
	})
	d := New(refuseRealDesktop(t, Deps{Version: "oni-approvals-test", Events: events, Teams: crew, Agents: agents}))
	daemon.Store(d)
	d.appPeer = func(int32) bool { return r.app.Load() }
	crew.SetHands(func(_ context.Context, goal, scope, window string, live func() bool) (string, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.starts = append(r.starts, scope+":"+window+":"+goal)
		if !live() {
			t.Error("Hands started on a grant that was not live")
		}
		return "j1", nil
	})
	r.d = d
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	t.Cleanup(func() {
		for _, info := range d.manager.List(protocol.KindAgent) {
			_ = d.manager.Stop(context.Background(), info.ID)
		}
		stop()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not stop")
		}
		_ = crew.Close()
	})
	deadline := time.Now().Add(5 * time.Second)
	for !client.Reachable(context.Background(), paths.Socket()) {
		if time.Now().After(deadline) {
			t.Fatal("the daemon never listened")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return r
}

func (r *approvalsRig) started() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.starts...)
}

func (r *approvalsRig) dialApp(t *testing.T) *socketPeer {
	t.Helper()
	r.app.Store(true)
	defer r.app.Store(false)
	return dialSocket(t, protocol.ClientApp)
}

// task makes a team with one member and gives it a task.
func (r *approvalsRig) task(t *testing.T) protocol.TeamTask {
	t.Helper()
	team, err := supervisedTeamCall[protocol.Team](r.crew, context.Background(), protocol.MTeamsCreate, protocol.TeamCreateParams{
		Name: "Desk", Folder: filepath.Join(paths.Home(), "Teams", "desk"), Approvals: protocol.ApprovalsAsk,
		Budget:  protocol.TeamBudget{TaskTurns: 6},
		Members: []protocol.Member{{Name: "Coordinator", Lead: true, Model: "test/local"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := supervisedTeamCall[protocol.TeamTask](r.crew, context.Background(), protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: team.ID, Brief: "File the receipt."})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func (r *approvalsRig) pending(t *testing.T) protocol.Approval {
	t.Helper()
	var found protocol.Approval
	eventually(t, "an approval to wait", func() bool {
		res, err := supervisedTeamCall[protocol.ApprovalsResult](r.crew, context.Background(), protocol.MTeamsApprovals, protocol.ApprovalsParams{Pending: true})
		if err != nil || len(res.Approvals) != 1 {
			return false
		}
		found = res.Approvals[0]
		return true
	})
	return found
}

// handsTurns asks for Hands in the nested desktop on the first turn and
// reports on the next.
func handsTurns(crew func() *teams.Service, scope, window string) func(int, protocol.AgentRef, core.Sink, *fakeRunner) (core.TurnResult, error) {
	return func(turn int, ref protocol.AgentRef, sink core.Sink, _ *fakeRunner) (core.TurnResult, error) {
		ctx := teams.WithAgentSession(context.Background(), ref.Session)
		stepTurn(turn, sink)
		if turn == 1 {
			_, err := supervisedTeamCall[protocol.AgentAnswer](crew(), ctx, protocol.MTeamsAgentHands, protocol.AgentHandsParams{AgentRef: ref, Goal: "file the receipt", Scope: scope, Window: window})
			return core.TurnResult{Stop: protocol.StopEnd}, err
		}
		_, err := supervisedTeamCall[protocol.AgentAnswer](crew(), ctx, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: ref, Title: "Filed", Body: "Hands filed it."})
		return core.TurnResult{Stop: protocol.StopEnd}, err
	}
}

func approvalFrame(open bool) func(feature.Payload) bool {
	return func(p feature.Payload) bool {
		f, ok := p.(*feature.ApprovalFrame)
		return ok && f.Open == open
	}
}

// A member's Hands request shows on the pill with once, no and the app;
// the CLI answers neither the card nor the approval; the pill's once starts
// Hands' own loop exactly once in the scope asked for, and the card goes.
func TestApprovalHandsRequestAnsweredOnceFromThePill(t *testing.T) {
	var r *approvalsRig
	r = serveApprovals(t, handsTurns(func() *teams.Service { return r.crew }, protocol.OniHandsScopeWindow, "0x7"))
	surface := dialSocket(t, protocol.ClientSurface)
	if !surface.open(protocol.OniRendererNative).Presenter {
		t.Fatal("the surface is no presenter")
	}
	r.task(t)
	asked := r.pending(t)
	card := surface.await("the Hands card", approvalFrame(true)).(*feature.ApprovalFrame)
	if !strings.HasPrefix(card.ID, feature.ApprovalHandsPrefix) || card.Scope != protocol.OniHandsScopeWindow ||
		strings.Join(card.Options, ",") != "once,no,app" || !strings.Contains(card.Title, "file the receipt") {
		t.Fatalf("the card is %+v", card)
	}

	cli := dialSocket(t, protocol.ClientCLI)
	if code, _ := reasonOf(cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: feature.ApprovalOnce}, nil)); code != protocol.CodeForbidden {
		t.Fatalf("the CLI answered the card: %s", code)
	}
	if code, _ := reasonOf(cli.call(protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideOnce}, nil)); code != protocol.CodeForbidden {
		t.Fatalf("the CLI answered the Hands approval: %s", code)
	}
	second := dialSocket(t, protocol.ClientSurface)
	if second.open(protocol.OniRendererNative).Presenter {
		t.Fatal("a second surface became presenter")
	}
	if code, _ := reasonOf(second.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: feature.ApprovalOnce}, nil)); code != protocol.CodeForbidden {
		t.Fatalf("a surface not drawing Oni answered the card: %s", code)
	}
	if code, _ := reasonOf(surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: feature.ApprovalAlways}, nil)); code != protocol.CodeForbidden {
		t.Fatalf("the pill allowed a Hands request for good: %s", code)
	}
	if len(r.started()) != 0 {
		t.Fatal("a refused answer started Hands")
	}

	var res protocol.OniAskResult
	if err := surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: feature.ApprovalOnce}, &res); err != nil || res.Status != protocol.OniAskAnswered {
		t.Fatalf("the pill's once: %+v %v", res, err)
	}
	if got := r.started(); len(got) != 1 || got[0] != "window:0x7:file the receipt" {
		t.Fatalf("Hands started %q", got)
	}
	closed := surface.await("the card to go", approvalFrame(false)).(*feature.ApprovalFrame)
	if closed.ID != card.ID {
		t.Fatalf("the card that went is %s, want %s", closed.ID, card.ID)
	}
	if code, _ := reasonOf(surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: feature.ApprovalOnce}, nil)); code != protocol.CodeNotFound {
		t.Fatalf("a second once: %s", code)
	}
	if len(r.started()) != 1 {
		t.Fatal("a second answer started Hands again")
	}
}

// Leaving a card to the app takes it off the pill and decides nothing: the
// approval stays open and the app answers it; a presenter that opens later
// sees no card for it. A no from the app runs nothing.
func TestApprovalLeftToTheAppStaysOpenThere(t *testing.T) {
	var r *approvalsRig
	r = serveApprovals(t, handsTurns(func() *teams.Service { return r.crew }, protocol.OniHandsScopeNested, ""))
	surface := dialSocket(t, protocol.ClientSurface)
	surface.open(protocol.OniRendererNative)
	r.task(t)
	asked := r.pending(t)
	card := surface.await("the Hands card", approvalFrame(true)).(*feature.ApprovalFrame)
	var res protocol.OniAskResult
	if err := surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: card.ID, Answer: feature.ApprovalInApp}, &res); err != nil {
		t.Fatal(err)
	}
	surface.await("the card to leave the pill", approvalFrame(false))
	if still := r.pending(t); still.ID != asked.ID || still.Status != protocol.ApprovalPending {
		t.Fatalf("leaving it to the app answered it: %+v", still)
	}
	app := r.dialApp(t)
	var decided protocol.Approval
	if err := app.call(protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: asked.ID, Decision: protocol.DecideDeny}, &decided); err != nil {
		t.Fatalf("the app's no: %v", err)
	}
	if decided.Status != protocol.ApprovalDenied || len(r.started()) != 0 {
		t.Fatalf("the app's no is %q and Hands started %d times", decided.Status, len(r.started()))
	}
}

// A hosted harness's permission shows on the pill: a write-tier one may be
// allowed once, for the conversation or refused there; a danger-tier one
// offers only the app and nothing else answers it from the pill.
func TestApprovalHarnessPermissionTiersOnThePill(t *testing.T) {
	var answered sync.Map
	r := serveApprovals(t, func(turn int, ref protocol.AgentRef, sink core.Sink, runner *fakeRunner) (core.TurnResult, error) {
		if turn > 1 {
			return core.TurnResult{Stop: protocol.StopEnd}, nil
		}
		runner.mu.Lock()
		runner.permission = func(request, option string) error {
			answered.Store(request, option)
			sink.Emit(protocol.EvPermissionResolved, protocol.PermissionResolvedData{Request: request, Option: option, Outcome: "selected", By: "user"})
			return nil
		}
		runner.mu.Unlock()
		options := []protocol.PermissionOption{{ID: "allow", Label: "Allow once", Kind: protocol.OptAllowOnce}, {ID: "refuse", Label: "Refuse", Kind: protocol.OptRejectOnce}}
		sink.Emit(protocol.EvPermission, protocol.PermissionData{Request: "r1", Title: "Run an ACP terminal program inside the team sandbox", Tier: protocol.TierDanger, Options: options})
		sink.Emit(protocol.EvPermission, protocol.PermissionData{Request: "r2", Title: "Write a file through the team sandbox", Tier: protocol.TierWrite,
			Options: append(options[:1:1], protocol.PermissionOption{ID: "always", Label: "Allow in this conversation", Kind: protocol.OptAllowAlways}, options[1])})
		deadline := time.After(10 * time.Second)
		for {
			if _, ok := answered.Load("r2"); ok {
				return core.TurnResult{Stop: protocol.StopEnd}, nil
			}
			select {
			case <-deadline:
				return core.TurnResult{Stop: protocol.StopEnd}, nil
			case <-time.After(5 * time.Millisecond):
			}
		}
	})
	surface := dialSocket(t, protocol.ClientSurface)
	surface.open(protocol.OniRendererNative)
	r.task(t)
	danger := surface.await("the danger card", approvalFrame(true)).(*feature.ApprovalFrame)
	if !strings.HasPrefix(danger.ID, feature.ApprovalPermissionPrefix) || strings.Join(danger.Options, ",") != "app" || danger.Scope != "" {
		t.Fatalf("the danger card is %+v", danger)
	}
	for _, answer := range []string{feature.ApprovalOnce, feature.ApprovalAlways, feature.ApprovalNo} {
		if code, _ := reasonOf(surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: danger.ID, Answer: answer}, nil)); code != protocol.CodeForbidden {
			t.Fatalf("the pill answered a danger-tier permission %s: %s", answer, code)
		}
	}
	if _, ok := answered.Load("r1"); ok {
		t.Fatal("a danger-tier permission was answered from the pill")
	}
	if err := surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: danger.ID, Answer: feature.ApprovalInApp}, nil); err != nil {
		t.Fatal(err)
	}
	write := surface.await("the write card", func(p feature.Payload) bool {
		f, ok := p.(*feature.ApprovalFrame)
		return ok && f.Open && f.ID != danger.ID
	}).(*feature.ApprovalFrame)
	if strings.Join(write.Options, ",") != "once,always,no,app" {
		t.Fatalf("the write card is %+v", write)
	}
	cli := dialSocket(t, protocol.ClientCLI)
	if code, _ := reasonOf(cli.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: write.ID, Answer: feature.ApprovalAlways}, nil)); code != protocol.CodeForbidden {
		t.Fatalf("the CLI answered the permission: %s", code)
	}
	if err := surface.call(protocol.MOniConsentAnswer, protocol.OniConsentAnswerParams{ID: write.ID, Answer: feature.ApprovalAlways}, nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the permission to be answered", func() bool {
		option, ok := answered.Load("r2")
		return ok && option == "always"
	})
	if _, ok := answered.Load("r1"); ok {
		t.Fatal("the danger-tier permission was answered")
	}
	gone := surface.await("the write card to go", approvalFrame(false)).(*feature.ApprovalFrame)
	if gone.ID != write.ID {
		t.Fatalf("the card that went is %s", gone.ID)
	}
}
