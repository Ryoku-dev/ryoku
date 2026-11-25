package demo

import (
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestDemoMemberConfigurationSharesFormsHistoryAndStaleRestore(t *testing.T) {
	s := &server{data: makeFixtures()}
	team := call(t, s, protocol.MTeamsCreate, protocol.TeamCreateParams{Name: "Config demo", Members: []protocol.Member{{Name: "Root", Lead: true}, {Name: "Reader"}}}).(protocol.Team)
	ref := protocol.MemberConfigRef{Team: team.ID, Member: team.Members[1].ID}
	first := call(t, s, protocol.MTeamsMemberConfigGet, ref).(protocol.MemberConfigDocument)
	if first.Revision != 1 {
		t.Fatal("demo did not initialize canonical revision")
	}
	saved := call(t, s, protocol.MTeamsMemberConfigSave, protocol.MemberConfigSaveParams{Team: ref.Team, Member: ref.Member, YAML: strings.Replace(first.YAML, `title: ""`, "title: Reviewed", 1), ExpectedRevision: first.Revision}).(protocol.MemberConfigDocument)
	team = s.data.teams.teams[s.data.teams.teamIndex(team.ID)]
	if team.Members[1].Title != "Reviewed" || team.Members[1].Revision != saved.Revision {
		t.Fatal("YAML did not update canonical demo state")
	}
	team.Members = append([]protocol.Member(nil), team.Members...)
	team.Members[1].Title = "Form edit"
	patch, _ := json.Marshal(map[string]any{"members": team.Members})
	updated := call(t, s, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, Patch: patch, ExpectedUpdated: team.Updated}).(protocol.Team)
	stale, _ := json.Marshal(protocol.MemberConfigRestoreParams{Team: ref.Team, Member: ref.Member, Revision: 1, ExpectedRevision: saved.Revision})
	if _, err := s.call(protocol.MTeamsMemberConfigRestore, stale); err == nil {
		t.Fatal("stale demo restore succeeded")
	}
	history := call(t, s, protocol.MTeamsMemberConfigHistory, ref).(protocol.MemberConfigHistoryResult)
	if len(history.Revisions) != 3 || history.RetentionLimit != protocol.MemberConfigHistoryLimit {
		t.Fatal("demo form edits did not enter bounded history")
	}
	preview := call(t, s, protocol.MTeamsMemberConfigGet, protocol.MemberConfigRef{Team: ref.Team, Member: ref.Member, Revision: 1}).(protocol.MemberConfigDocument)
	if preview.Revision != 1 || preview.CurrentRevision != updated.Members[1].Revision {
		t.Fatal("demo preview confused current and historical revision")
	}
	restored := call(t, s, protocol.MTeamsMemberConfigRestore, protocol.MemberConfigRestoreParams{Team: ref.Team, Member: ref.Member, Revision: 1, ExpectedRevision: preview.CurrentRevision}).(protocol.MemberConfigDocument)
	if restored.Revision != preview.CurrentRevision+1 || !strings.Contains(restored.YAML, `title: ""`) {
		t.Fatal("demo restore did not create a new canonical revision")
	}
}

func TestDemoReviewedDreamRootDoesNotAcceptPrivateProgressEdits(t *testing.T) {
	s := &server{data: makeFixtures()}
	team := call(t, s, protocol.MTeamsCreate, protocol.TeamCreateParams{Name: "Reviewed dream", Members: []protocol.Member{{Name: "Manager", Lead: true}, {Name: "Replacement"}}}).(protocol.Team)
	original := protocol.Dream{Manager: team.Members[0].ID, Statement: "Preserve the owner's goal.", Task: "standing-task", Hired: 4, Phase: protocol.DreamTesting}
	team.Dream = &original
	s.data.teams.teams[s.data.teams.teamIndex(team.ID)] = team
	members := append([]protocol.Member(nil), team.Members...)
	members[0].Lead, members[0].ReportsTo = false, members[1].ID
	members[1].Lead, members[1].ReportsTo = true, ""
	patch, err := json.Marshal(map[string]any{"members": members, "reporting": protocol.ReportingTree, "dream": map[string]any{"statement": "Unapproved goal", "phase": protocol.DreamPresenting, "hired": 999, "task": "unrelated-task"}})
	if err != nil {
		t.Fatal(err)
	}
	saved := call(t, s, protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: team.ID, ExpectedUpdated: team.Updated, Patch: patch}).(protocol.Team)
	if saved.Dream == nil || saved.Dream.Manager != members[1].ID || saved.Dream.Statement != original.Statement || saved.Dream.Phase != original.Phase || saved.Dream.Hired != original.Hired || saved.Dream.Task != original.Task {
		t.Fatal("reviewed root did not move atomically or private Dream progress was editable")
	}
	if err := protocol.ValidateReporting(saved); err != nil {
		t.Fatal(err)
	}
}
