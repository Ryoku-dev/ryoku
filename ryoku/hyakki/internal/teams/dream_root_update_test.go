package teams

import (
	"encoding/json"
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestReviewedDreamRootUpdateKeepsProgressPrivateAndSurvivesRestart(t *testing.T) {
	h := newHarness(t, 0)
	created, _ := h.newDream(t)
	paused := do[protocol.Team](t, h, protocol.MTeamsPause, protocol.TeamPauseParams{ID: created.ID, Paused: true})
	original := cloneDream(*paused.Dream)
	members := append([]protocol.Member(nil), paused.Members...)
	old := members[0].ID
	for i := range members {
		members[i].Lead = false
		members[i].ReportsTo = "reviewed-manager"
	}
	members = append(members, protocol.Member{ID: "reviewed-manager", Name: "Reviewed manager", Lead: true, Instructions: "Coordinate only the assigned dream.", Trust: protocol.ApprovalsAsk})
	patch, err := json.Marshal(map[string]any{"members": members, "reporting": protocol.ReportingTree, "dream": map[string]any{"manager": old, "statement": "unreviewed replacement goal", "hired": 999, "phase": protocol.DreamPresenting, "task": "unrelated-task"}})
	if err != nil {
		t.Fatal(err)
	}
	saved := do[protocol.Team](t, h, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: paused.ID, ExpectedUpdated: paused.Updated, Patch: patch})
	expected := original
	expected.Manager = "reviewed-manager"
	if saved.Dream == nil || !reflect.DeepEqual(*saved.Dream, expected) {
		t.Fatalf("reviewed root either failed to change manager or changed private Dream progress: %+v", saved.Dream)
	}
	if !saved.Paused || saved.Budget != paused.Budget || saved.Approvals != paused.Approvals || saved.Tools != paused.Tools {
		t.Fatal("root replacement widened policy or started a paused dream")
	}
	if err := protocol.ValidateReporting(saved); err != nil {
		t.Fatal(err)
	}
	oldRoot, exists := protocol.MemberByID(saved.Members, old)
	if !exists || oldRoot.Lead || oldRoot.ReportsTo != "reviewed-manager" {
		t.Fatal("reviewed reporting lines were not retained")
	}
	if err := h.s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	persisted, exists := reopened.team(saved.ID)
	if !exists || persisted.Dream == nil || !reflect.DeepEqual(*persisted.Dream, expected) || !reflect.DeepEqual(persisted.Members, saved.Members) {
		t.Fatal("restart lost the reviewed manager or restored an unapproved Dream-state edit")
	}
}
