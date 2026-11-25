package teams

import (
	"strings"
	"testing"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/structures"
)

func TestReplacingAReviewedRosterKeepsItsExplicitPolicy(t *testing.T) {
	for _, nextID := range []string{"night-parade", "elemental-ring"} {
		t.Run(nextID, func(t *testing.T) {
			settings := protocol.Settings{}
			settings.Teams.Defaults.Approvals = protocol.ApprovalsAuto
			settings.Teams.Defaults.Notify = protocol.NotifyNone
			settings.Teams.Defaults.Budget = protocol.TeamBudget{MonthTokens: 900_000, WarnPercent: 80, TaskTurns: 20, TaskMinutes: 10}
			store := state.New(nil)
			store.ApplySnapshot(protocol.Snapshot{Settings: settings})
			e := &env.Env{Store: store}
			parade, _ := structures.Lookup("night-parade")
			members, err := structures.Instantiate(parade, 2)
			if err != nil {
				t.Fatal(err)
			}
			var draft draft
			draft.applyStructure(e, parade, members)
			draft.name.SetText("My reviewed team")
			draft.purpose.SetText("Keep this exact goal")
			draft.approvalsValue = protocol.ApprovalsAsk
			draft.planApproval.Value = false
			draft.screen.Value, draft.mail.Value = true, true
			draft.notifyValue = protocol.NotifyNeeds
			draft.limit.Value, draft.warn, draft.turns.Value, draft.minutes.Value = 300_000, 67, 9, 3
			draft.reviewed.Value = true
			before := draft.params()
			next, _ := structures.Lookup(nextID)
			count := 3
			if nextID == "elemental-ring" {
				count = 4
			}
			members, err = structures.Instantiate(next, count)
			if err != nil {
				t.Fatal(err)
			}
			draft.applyStructure(e, next, members)
			after := draft.params()
			if after.Approvals != before.Approvals || after.PlanApproval != before.PlanApproval || after.Budget != before.Budget || after.Notify != before.Notify || after.Tools != before.Tools {
				t.Fatalf("roster replacement changed policy: approvals %q -> %q; plan %t -> %t; budget %+v -> %+v; notify %q -> %q; tools %+v -> %+v", before.Approvals, after.Approvals, before.PlanApproval, after.PlanApproval, before.Budget, after.Budget, before.Notify, after.Notify, before.Tools, after.Tools)
			}
			if after.Name != before.Name || after.Purpose != before.Purpose || after.Structure != nextID || len(after.Members) != count {
				t.Fatal("replacement lost the goal or did not use the selected roster")
			}
			if draft.reviewed.Value {
				t.Fatal("replacement retained the review acknowledgment for the previous roster")
			}
		})
	}
}

func TestFirstSuccessfulRosterInitializesPolicyAfterALoadFailure(t *testing.T) {
	settings := protocol.Settings{}
	settings.Teams.Defaults.Approvals = protocol.ApprovalsAsk
	settings.Teams.Defaults.Budget = protocol.TeamBudget{MonthTokens: 300_000, WarnPercent: 70, TaskTurns: 9, TaskMinutes: 3}
	store := state.New(nil)
	store.ApplySnapshot(protocol.Snapshot{Settings: settings})
	e := &env.Env{Store: store}
	recipe, _ := structures.Lookup("elemental-ring")
	members, err := structures.Instantiate(recipe, 0)
	if err != nil {
		t.Fatal(err)
	}
	draft := draft{filled: true, problem: "Initial roster load failed"}
	draft.applyStructure(e, recipe, members)
	params := draft.params()
	if params.Budget != settings.Teams.Defaults.Budget || params.Approvals != protocol.ApprovalsAsk || !params.PlanApproval {
		t.Fatalf("first successful roster did not recover a finite reviewed policy: %+v", params)
	}
	if draft.reviewed.Value {
		t.Fatal("recovered roster was considered reviewed before the owner saw it")
	}
}

func TestWorkflowSelectionSupersedesPendingStructureLoad(t *testing.T) {
	settings := protocol.Settings{}
	settings.Teams.Defaults.Approvals = protocol.ApprovalsAsk
	settings.Teams.Defaults.Budget = protocol.TeamBudget{MonthTokens: 200_000, WarnPercent: 80, TaskTurns: 5, TaskMinutes: 2}
	store := state.New(nil)
	store.ApplySnapshot(protocol.Snapshot{Settings: settings})
	e := &env.Env{Store: store}
	draft := draft{rosterBusy: true, rosterRequest: 4}
	draft.apply(e, protocol.TeamTemplate{ID: "my-workflow", Name: "My workflow", Purpose: "Review my input", Members: []protocol.Member{{Name: "Reviewer"}}})
	if draft.rosterBusy || !draft.filled || draft.params().Name != "My workflow" || len(draft.params().Members) != 1 {
		t.Fatal("a superseded structure request still hides or blocks the selected workflow")
	}
	if draft.rosterRequest == 4 {
		t.Fatal("the superseded callback can still replace the selected workflow")
	}
}

func TestPeerRosterCannotPassReviewWithAManagerLink(t *testing.T) {
	store := state.New(nil)
	e := &env.Env{Store: store}
	recipe, _ := structures.Lookup("elemental-ring")
	members, err := structures.Instantiate(recipe, 0)
	if err != nil {
		t.Fatal(err)
	}
	var draft draft
	draft.applyStructure(e, recipe, members)
	if problem := draft.graphProblem(draft.crew()); problem != "" {
		t.Fatalf("valid peers were rejected: %s", problem)
	}
	draft.members[1].form.base.ReportsTo = draft.members[0].form.base.ID
	if problem := draft.graphProblem(draft.crew()); problem == "" {
		t.Fatal("a peer-to-peer authority link passed review")
	}
	draft.members[1].form.base.ReportsTo = ""
	if problem := draft.graphProblem(draft.crew()); problem != "" {
		t.Fatalf("removing the invalid manager did not restore a valid peer draft: %s", problem)
	}
}

func TestEditedDraftNamesCannotEnableAnUncreatableTeam(t *testing.T) {
	store := state.New(nil)
	e := &env.Env{Store: store}
	recipe, _ := structures.Lookup("elemental-ring")
	members, err := structures.Instantiate(recipe, 0)
	if err != nil {
		t.Fatal(err)
	}
	var draft draft
	draft.applyStructure(e, recipe, members)
	draft.name.SetText(strings.Repeat("界", protocol.TeamNameMaxRunes))
	if !draft.markNames(draft.crew()) {
		t.Fatal("an exact Unicode team-name boundary was rejected")
	}
	draft.name.SetText(strings.Repeat("界", protocol.TeamNameMaxRunes+1))
	if draft.markNames(draft.crew()) {
		t.Fatal("an oversized team name passed review")
	}
	draft.name.SetText("My team")
	draft.members[1].form.name.SetText(strings.Repeat("界", protocol.TeamNameMaxRunes+1))
	if draft.markNames(draft.crew()) {
		t.Fatal("an oversized member name passed review")
	}
	draft.members[1].form.name.SetText(strings.ToUpper(draft.members[0].form.name.Text()))
	if draft.markNames(draft.crew()) {
		t.Fatal("case-insensitively duplicate names passed review")
	}
	draft.members[1].form.name.SetText("My independent reviewer")
	if !draft.markNames(draft.crew()) {
		t.Fatal("correcting the edited names did not restore a creatable draft")
	}
}
