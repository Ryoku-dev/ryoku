package teams

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/structures"
)

func TestCatalogueRostersUseTheExistingCreationBoundaryWithoutRehydration(t *testing.T) {
	h := newHarness(t, 0)
	for _, recipe := range structures.Builtin() {
		t.Run(recipe.ID, func(t *testing.T) {
			members, err := structures.Instantiate(recipe, 0)
			if err != nil {
				t.Fatal(err)
			}
			// The review removes an optional seat, and changes identity/duties.
			// A recipe ID must not silently restore the original members.
			if recipe.Reporting == protocol.ReportingPeers && len(members) > 1 {
				members = members[:len(members)-1]
			}
			members[0].Name, members[0].Title, members[0].Rank, members[0].Spirit = "Reviewed name", "Reviewed title", "Reviewed rank", ""
			members[0].Instructions = "Follow the edited, explicitly reviewed assignment."
			team := do[protocol.Team](t, h, protocol.MTeamsCreate, protocol.TeamCreateParams{
				Name: recipe.Name, Purpose: "The reviewed root goal", Folder: filepath.Join(h.dir, "folders", recipe.ID),
				Structure: recipe.ID, Culture: recipe.Culture, Reporting: recipe.Reporting, Members: members,
				Approvals: protocol.ApprovalsAsk, PlanApproval: true, Budget: protocol.TeamBudget{MonthTokens: 1200, TaskTurns: 6},
			})
			if len(team.Members) != len(members) || team.Purpose != "The reviewed root goal" || team.Structure != recipe.ID || team.Reporting != recipe.Reporting || !team.PlanApproval {
				t.Fatalf("submitted definition lost: %+v", team)
			}
			if team.Approvals != protocol.ApprovalsAsk || team.Budget.MonthTokens != 1200 || !reflect.DeepEqual(team.Tools, protocol.TeamTools{}) {
				t.Fatal("recipe granted permissions or widened the reviewed budget")
			}
			byOldID := make(map[string]string)
			for i, member := range team.Members {
				byOldID[members[i].ID] = member.ID
			}
			for i, member := range team.Members {
				want := members[i]
				if member.Name != want.Name || member.Title != want.Title || member.Rank != want.Rank || member.Spirit != want.Spirit || member.Instructions != want.Instructions || member.Lead != want.Lead || member.Model != want.Model || member.ReportsTo != byOldID[want.ReportsTo] {
					t.Fatalf("seat %d was rehydrated or graph not rebased: %+v", i, member)
				}
			}
			persisted, ok := h.s.state.team(team.ID)
			if !ok || !reflect.DeepEqual(persisted.Members, team.Members) {
				t.Fatal("reopening changed the reviewed roster")
			}
		})
	}
	parade, _ := structures.Lookup("night-parade")
	members, err := structures.Instantiate(parade, protocol.TeamsMembersMax)
	if err != nil {
		t.Fatal(err)
	}
	team := do[protocol.Team](t, h, protocol.MTeamsCreate, protocol.TeamCreateParams{Name: "Full parade", Purpose: "Review every assigned batch.", Folder: filepath.Join(h.dir, "folders", "full"), Structure: parade.ID, Reporting: parade.Reporting, Members: members})
	if len(team.Members) != protocol.TeamsMembersMax {
		t.Fatal("controller rejected the supported full parade")
	}
}

func TestAuthenticatedAgentCallbacksCannotMutateTheRecipeCatalogue(t *testing.T) {
	h := newHarness(t, 0)
	team := h.newTeam(t, "Callback owner", nil)
	task := h.newTask(t, team, "Inspect the team without changing recipes.")
	ref := h.agent(t, task.ID)
	ctx := WithAgentSession(context.Background(), ref.Session)
	custom := do[protocol.StructureTemplate](t, h, protocol.MTeamsStructureClone, protocol.StructureCloneParams{ID: "night-parade", NewID: "custom:owner-parade", Name: "Owner parade"})
	before := do[protocol.TeamStructuresResult](t, h, protocol.MTeamsStructures, protocol.Empty{})
	for _, call := range []struct {
		method string
		params any
	}{
		{protocol.MTeamsStructureSave, protocol.StructureSaveParams{Template: custom, ExpectedRevision: custom.Revision}},
		{protocol.MTeamsStructureClone, protocol.StructureCloneParams{ID: custom.ID, NewID: "custom:agent-copy", Name: "Agent copy"}},
		{protocol.MTeamsStructureDelete, protocol.StructureRef{ID: custom.ID, ExpectedRevision: custom.Revision}},
		{protocol.MTeamsStructureRestore, protocol.StructureRef{ID: custom.ID, ExpectedRevision: custom.Revision}},
	} {
		_, err := h.s.Handle(ctx, call.method, mustJSON(t, call.params))
		var failure *protocol.Error
		if !errors.As(err, &failure) || failure.Code != protocol.CodeForbidden {
			t.Fatalf("agent mutation %s = %v", call.method, err)
		}
	}
	after := do[protocol.TeamStructuresResult](t, h, protocol.MTeamsStructures, protocol.Empty{})
	if !reflect.DeepEqual(before, after) {
		t.Fatal("authenticated callback mutated catalogue")
	}
	if _, err := h.s.Handle(ctx, protocol.MTeamsStructureDraft, mustJSON(t, protocol.StructureDraftParams{ID: custom.ID})); err != nil {
		t.Fatalf("safe local preview rejected: %v", err)
	}
}
