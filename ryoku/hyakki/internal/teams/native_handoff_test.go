package teams

import (
	"context"
	"testing"

	"ryoku-rashin/internal/protocol"
)

type observedModeSessions struct {
	*fakeSessions
	changed func(protocol.SessionInfo)
}

func (s *observedModeSessions) SetMode(ctx context.Context, params protocol.SetModeParams) error {
	if err := s.fakeSessions.SetMode(ctx, params); err != nil {
		return err
	}
	s.changed(s.info(params.ID))
	return nil
}

func TestNativeModeEventDoesNotConsumeAnApprovedStrategyHandoff(t *testing.T) {
	h, team := governedTeam(t)
	team.PlanApproval = true
	team.Approvals = protocol.ApprovalsAuto
	team.Members[0].Trust = protocol.ApprovalsReadOnly
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	h.s.sessions = &observedModeSessions{fakeSessions: h.sessions, changed: h.s.SessionChanged}
	task := h.newTask(t, team, "Read the input and deliver a checked report")
	ref := h.agent(t, task.ID)
	h.answer(t, protocol.MTeamsAgentPlan, protocol.AgentPlanParams{AgentRef: ref, Summary: "Check the local input", Steps: []string{"Read the input", "Report the verified result"}})
	approval := h.pending(t)
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: approval.ID, Decision: protocol.DecideOnce})
	waitFor(t, "the approved strategy to reach the actual session continuation", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	current := h.task(t, task.ID)
	if current.Status != protocol.TaskWorking || current.Waiting != "" || current.ResumeVersion != 1 || current.Plan == nil || !current.Plan.Approved {
		t.Fatalf("the accepted strategy was not delivered exactly once: %+v", current)
	}
	if h.sessions.info(ref.Session).Approvals != protocol.ApprovalsReadOnly {
		t.Fatal("resuming the strategy widened the seat's trust")
	}
}

func TestNativeSessionEventCannotReleaseAClaimedGovernanceDecision(t *testing.T) {
	h := newHarness(t, 0)
	task, ref, approval := askingTask(t, h, "Should this task continue?")
	claimed, err := h.s.claimApproval(approval.ID, protocol.ApprovalDenied, protocol.DecideDeny, "Keep it local")
	if err != nil {
		t.Fatal(err)
	}
	h.s.SessionChanged(h.sessions.info(ref.Session))
	current := h.task(t, task.ID)
	if current.Status != protocol.TaskNeedsYou || current.Waiting != protocol.WaitingOnYou || current.ResumeVersion != 0 {
		t.Fatalf("an unrelated native session event consumed a decision before delivery: %+v", current)
	}
	if _, err := h.s.settleApproval(context.Background(), claimed, protocol.ApprovalDecideParams{ID: approval.ID, Decision: protocol.DecideDeny}, "approval.deny", "you", protocol.InitiatorYou); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the decision to reach its original execution", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	current = h.task(t, task.ID)
	if current.Waiting != "" || current.ResumeVersion != 1 {
		t.Fatalf("the claimed decision was not delivered once: %+v", current)
	}
}

func TestNativePermissionRoundTripDoesNotCreateAGovernanceContinuation(t *testing.T) {
	h, team := governedTeam(t)
	task := h.newTask(t, team, "Inspect before making a change")
	ref := h.agent(t, task.ID)
	info := h.sessions.info(ref.Session)
	info.State, info.Pending = protocol.StateWaiting, 1
	h.s.SessionChanged(info)
	current := h.task(t, task.ID)
	if current.Status != protocol.TaskNeedsYou || current.Waiting != protocol.WaitingOnPermission || !h.s.Hold(info) {
		t.Fatalf("the native permission was not held distinctly: %+v", current)
	}
	refused(t, h, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: ref, Member: team.Members[1].ID, Brief: "Do not bypass the pending permission"}, protocol.CodeConflict)
	info.State, info.Pending = protocol.StateWorking, 0
	h.s.SessionChanged(info)
	current = h.task(t, task.ID)
	if current.Status != protocol.TaskWorking || current.Waiting != "" || current.ResumeVersion != 0 || len(h.sessions.notes(ref.Session)) != 0 {
		t.Fatalf("a native permission answer tried to create a second prompt instead of resuming its own runner: %+v", current)
	}
}

func TestSummonerDecisionDoesNotWaitForMessagePublication(t *testing.T) {
	for _, kind := range []string{"help", "escalate"} {
		t.Run(kind, func(t *testing.T) {
			h, team := governedTeam(t)
			task := h.newTask(t, team, "Resolve the next decision")
			ref := h.agent(t, task.ID)
			ctx := WithAgentSession(context.Background(), ref.Session)
			caller, err := h.s.caller(ctx, ref)
			if err != nil {
				t.Fatal(err)
			}
			message, err := h.s.recordTeamMessage(ctx, protocol.TeamMessage{Team: team.ID, Task: task.ID, From: caller.Member, Kind: kind, Title: "Need a decision", Body: "The Summoner must decide the next step."})
			if err != nil {
				t.Fatal(err)
			}
			approval, err := h.s.ask(ctx, caller, protocol.Approval{Kind: protocol.ApprovalAction, Title: message.Title, Detail: message.Body}, nil)
			if err != nil {
				t.Fatal(err)
			}
			do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: approval.ID, Decision: protocol.DecideOnce})
			waitFor(t, "the Summoner's early answer to reach its original session", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
			current := h.task(t, task.ID)
			if current.Waiting != "" || current.ResumeVersion != 1 {
				t.Fatalf("an undelivered Summoner message stranded the answered task: %+v", current)
			}
			if err := h.s.state.deliverMessage(message.ID); err != nil {
				t.Fatal(err)
			}
			if len(h.sessions.notes(ref.Session)) != 1 {
				t.Fatal("late message publication delivered the decision twice")
			}
		})
	}
}
