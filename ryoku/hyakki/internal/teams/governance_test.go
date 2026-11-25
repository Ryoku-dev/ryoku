package teams

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func governedTeam(t *testing.T) (*harness, protocol.Team) {
	t.Helper()
	h := newHarness(t, 0)
	team := h.newTeam(t, "Governed delivery", func(p *protocol.TeamCreateParams) {
		p.Reporting = protocol.ReportingTree
		p.Purpose = "Ship a reviewed local artifact"
		p.Members = []protocol.Member{
			{ID: "root", Name: "Coordinator", Lead: true, Model: "test/small", Heartbeat: &protocol.MemberHeartbeat{Modes: []string{protocol.HeartbeatTask, protocol.HeartbeatMessage}}},
			{ID: "manager", Name: "Engineering", ReportsTo: "root", Model: "test/small"},
			{ID: "leaf", Name: "Builder", ReportsTo: "manager", Model: "test/small"},
			{ID: "reviewer", Name: "Review", ReportsTo: "root", Model: "test/small"},
		}
	})
	return h, team
}

func governedPiece(t *testing.T, h *harness, parent protocol.TeamTask, ref protocol.AgentRef, member string) protocol.TeamTask {
	t.Helper()
	h.answer(t, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: ref, Member: member, Title: "Deliver " + member, Brief: "Complete the assigned part and report the concrete result."})
	pieces := h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Parent == parent.ID && task.Member == member })
	if len(pieces) != 1 {
		t.Fatalf("delegation created %d matching assignments, want exactly one", len(pieces))
	}
	return pieces[0]
}

func TestGovernedTreeCompletesUpwardWithoutGrantingRankAuthority(t *testing.T) {
	h, team := governedTeam(t)
	root := h.newTask(t, team, "Build and review the local artifact")
	rootRef := h.agent(t, root.ID)
	refused(t, h, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: rootRef, Member: team.Members[2].ID, Brief: "Skip the real manager"}, protocol.CodeForbidden)
	manager := governedPiece(t, h, root, rootRef, team.Members[1].ID)
	managerRef := h.agent(t, manager.ID)
	refused(t, h, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: managerRef, Member: team.Members[0].ID, Brief: "Assign an ancestor"}, protocol.CodeForbidden)
	leaf := governedPiece(t, h, manager, managerRef, team.Members[2].ID)
	leafRef := h.agent(t, leaf.ID)
	refused(t, h, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: leafRef, Member: team.Members[3].ID, Brief: "Assign another branch"}, protocol.CodeForbidden)
	refused(t, h, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: rootRef, Title: "Premature", Body: "Children still work."}, protocol.CodeConflict)
	lineage := do[protocol.TeamTaskContext](t, h, protocol.MTeamsTaskContext, protocol.TeamTaskRef{ID: leaf.ID})
	if lineage.Goal != team.Purpose || len(lineage.Because) != 2 || lineage.Because[0].ID != manager.ID || lineage.Because[1].ID != root.ID {
		t.Fatalf("leaf lost its canonical goal or reason chain: %+v", lineage)
	}
	h.answer(t, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: leafRef, Title: "Built", Body: "The local artifact is built."})
	waitFor(t, "the intermediate manager to receive its result", func() bool { return h.task(t, manager.ID).Waiting == "" })
	h.answer(t, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: managerRef, Title: "Reviewed build", Body: "The builder result is checked."})
	waitFor(t, "the root to receive its manager result", func() bool { return h.task(t, root.ID).Waiting == "" })
	h.answer(t, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: rootRef, Title: "Delivered", Body: "The reviewed artifact is delivered."})
	for _, id := range []string{root.ID, manager.ID, leaf.ID} {
		task := h.task(t, id)
		if task.Status != protocol.TaskDone || task.Report == nil || task.Goal != team.Purpose {
			t.Fatalf("upward completion lost its report or goal: %+v", task)
		}
	}
}

func TestGovernedHelpRoutesThroughCommonManagerAndReturnsToRequester(t *testing.T) {
	h, team := governedTeam(t)
	root := h.newTask(t, team, "Build the artifact")
	rootRef := h.agent(t, root.ID)
	manager := governedPiece(t, h, root, rootRef, team.Members[1].ID)
	leaf := governedPiece(t, h, manager, h.agent(t, manager.ID), team.Members[2].ID)
	leafRef := h.agent(t, leaf.ID)
	h.answer(t, protocol.MTeamsAgentHelp, protocol.AgentHelpParams{AgentRef: leafRef, Member: team.Members[3].ID, Title: "Need review", Brief: "Ask the other branch to review this artifact."})
	messages := h.s.state.messages(func(message protocol.TeamMessage) bool { return message.Task == leaf.ID && message.Kind == "help" })
	if len(messages) != 1 || messages[0].To != team.Members[0].ID || messages[0].Member != team.Members[3].ID || !messages[0].Delivered {
		t.Fatalf("help did not reach the actual common manager: %+v", messages)
	}
	if h.task(t, leaf.ID).Member != team.Members[2].ID || h.task(t, leaf.ID).Waiting != protocol.WaitingOnMembers {
		t.Fatal("help reassigned the caller or failed to hold its execution")
	}
	if tasks := h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Member == team.Members[3].ID }); len(tasks) != 0 {
		t.Fatal("requesting a peer bypassed the manager and assigned it directly")
	}
	decisions := h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Trigger == messages[0].ID })
	if len(decisions) != 1 || decisions[0].Member != team.Members[0].ID || decisions[0].Parent != leaf.ID || decisions[0].Goal != root.Goal {
		t.Fatalf("manager attention lost request causality: %+v", decisions)
	}
	decisionRef := h.agent(t, decisions[0].ID)
	h.answer(t, protocol.MTeamsAgentReport, protocol.AgentReportParams{AgentRef: decisionRef, Title: "Manager decision", Body: "Continue with this reviewed approach; no extra peer assignment is needed."})
	waitFor(t, "the requesting leaf to resume after the manager decision", func() bool { return h.task(t, leaf.ID).Waiting == "" })
	if h.task(t, leaf.ID).Status != protocol.TaskWorking || h.task(t, root.ID).Waiting != protocol.WaitingOnMembers {
		t.Fatal("a lateral decision completed the wrong task or unblocked an unfinished ancestor")
	}
}

func TestGovernedEscalationWithMessageWakeDisabledUsesRealSummonerDecision(t *testing.T) {
	h, team := governedTeam(t)
	team.Members[1].Heartbeat = &protocol.MemberHeartbeat{Modes: []string{protocol.HeartbeatTask}}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	task := do[protocol.TeamTask](t, h, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: team.ID, Member: team.Members[2].ID, Brief: "Inspect the artifact"})
	ref := h.agent(t, task.ID)
	h.answer(t, protocol.MTeamsAgentEscalate, protocol.AgentEscalateParams{AgentRef: ref, Title: "Decision required", Detail: "The manager does not wake on messages."})
	approval := h.pending(t)
	if approval.Task != task.ID || approval.Member != team.Members[2].ID || h.task(t, task.ID).Waiting != protocol.WaitingOnYou {
		t.Fatal("disabled wake lost the request instead of obtaining a real Summoner decision")
	}
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: approval.ID, Decision: protocol.DecideOnce, Note: "Proceed with the narrow scope"})
	waitFor(t, "the Summoner answer to release the original leaf", func() bool { return h.task(t, task.ID).Waiting == "" })
	if h.task(t, task.ID).Member != team.Members[2].ID {
		t.Fatal("answering an escalation changed the task's assignee")
	}
}

func TestGovernedStrategyDenialAndApprovalNeverWidenSeatTrust(t *testing.T) {
	h, team := governedTeam(t)
	team.PlanApproval = true
	team.Approvals = protocol.ApprovalsAuto
	team.Members[0].Trust = protocol.ApprovalsReadOnly
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	task := h.newTask(t, team, "Plan the artifact delivery")
	ref := h.agent(t, task.ID)
	refused(t, h, protocol.MTeamsAgentDelegate, protocol.AgentDelegateParams{AgentRef: ref, Member: team.Members[1].ID, Brief: "Execute before approval"}, protocol.CodeForbidden)
	for _, decision := range []string{protocol.DecideDeny, protocol.DecideOnce} {
		h.answer(t, protocol.MTeamsAgentPlan, protocol.AgentPlanParams{AgentRef: ref, Summary: "Review before delivery", Steps: []string{"Inspect the existing artifact", "Delegate the reviewed plan"}})
		approval := h.pending(t)
		if approval.Kind != protocol.ApprovalStrategy || !h.s.Hold(h.sessions.info(ref.Session)) {
			t.Fatal("pending strategy did not hold real task execution")
		}
		do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: approval.ID, Decision: decision})
		waitFor(t, "the strategy decision to release its handoff", func() bool { return h.task(t, task.ID).Waiting == "" })
		current := h.task(t, task.ID)
		if decision == protocol.DecideDeny {
			if current.Plan != nil && current.Plan.Approved {
				t.Fatal("a declined strategy became executable")
			}
		} else if current.Plan == nil || !current.Plan.Approved {
			t.Fatal("an accepted strategy was not recorded for this execution")
		}
		if got := h.sessions.info(ref.Session).Approvals; got != protocol.ApprovalsReadOnly {
			t.Fatalf("strategy decision widened a read-only seat to %s", got)
		}
		if _, err := h.s.agentHTTP(context.Background(), protocol.AgentHTTPParams{AgentRef: ref, Credential: "not-exportable", URL: "https://example.invalid"}); err == nil {
			t.Fatal("strategy approval granted credential export to a read-only seat")
		}
	}
	piece := governedPiece(t, h, task, ref, team.Members[1].ID)
	if piece.Plan == nil || !piece.Plan.Approved || piece.Goal != task.Goal {
		t.Fatal("delegated work lost the accepted strategy or its original goal")
	}
}

func TestGovernedExternalPlanStaysParkedAcrossRecoveryUntilOwnerSuppliesIt(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "External plan", func(p *protocol.TeamCreateParams) {
		p.PlanApproval = true
		p.Members = []protocol.Member{{ID: "external", Name: "External", Lead: true, Runtime: &protocol.MemberRuntime{Kind: protocol.MemberRuntimeCommand, Command: []string{"/bin/true"}}}}
	})
	task := h.newTask(t, team, "Perform the explicitly configured command")
	waitFor(t, "the external task to park for a strategy", func() bool { return h.task(t, task.ID).Status == protocol.TaskNeedsYou })
	h.s.recoverRuns()
	parked := h.task(t, task.ID)
	if parked.Session != "" || parked.Status != protocol.TaskNeedsYou || parked.Waiting != protocol.WaitingOnYou {
		t.Fatalf("recovery started or lost the pending external plan: %+v", parked)
	}
	if _, made := h.sessions.madeFor(task.ID); made {
		t.Fatal("an unapproved external strategy silently started a worker")
	}
	do[protocol.TeamTask](t, h, protocol.MTeamsTaskPlan, protocol.TeamTaskPlanParams{ID: task.ID, Summary: "Run the configured local command", Steps: []string{"Execute only the configured argv", "Report its observed exit"}})
	ref := h.agent(t, task.ID)
	if info := h.sessions.info(ref.Session); info.Runtime != protocol.MemberRuntimeCommand {
		t.Fatalf("approved external work was silently replaced by native inference: %+v", info)
	}
	if current := h.task(t, task.ID); current.Plan == nil || !current.Plan.Approved || current.ID != task.ID {
		t.Fatal("owner plan approval replaced the task instead of authorizing its pending strategy")
	}
}

func TestGovernedGraphRefusalsLeaveNoRunnableTeam(t *testing.T) {
	for _, bad := range []string{"cycle", "missing-manager", "orphan", "peer-link"} {
		t.Run(bad, func(t *testing.T) {
			h := newHarness(t, 0)
			params := protocol.TeamCreateParams{Name: "Invalid graph", Folder: filepath.Join(h.dir, bad), Reporting: protocol.ReportingTree, Members: []protocol.Member{{ID: "root", Name: "Root", Lead: true}, {ID: "first", Name: "First", ReportsTo: "root"}, {ID: "second", Name: "Second", ReportsTo: "first"}}}
			switch bad {
			case "cycle":
				params.Members[1].ReportsTo = "second"
			case "missing-manager":
				params.Members[2].ReportsTo = "deleted"
			case "orphan":
				params.Members[2].ReportsTo = ""
			case "peer-link":
				params.Reporting = protocol.ReportingPeers
			}
			if _, err := h.call(protocol.MTeamsCreate, params); err == nil {
				t.Fatal("invalid reporting authority became a team")
			}
			for _, team := range h.s.state.teams() {
				if strings.EqualFold(team.Name, params.Name) {
					t.Fatal("refused graph left persisted runnable configuration")
				}
			}
		})
	}
}
