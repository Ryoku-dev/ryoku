package demo

import (
	"encoding/json"
	"slices"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func call(t *testing.T, s *server, method string, params any) any {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.call(method, raw)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return result
}

// the requests a team has open for credentials, by name
func requested(s *server, team string) []string {
	var names []string
	for _, approval := range s.data.teams.approvals {
		if approval.Team == team && approval.Kind == protocol.ApprovalRequest && approval.Status == protocol.ApprovalPending {
			names = append(names, s.data.teams.actions[approval.ID].request.Name)
		}
	}
	return names
}

// the templates come in the daemon's order, light to advanced, the blank one last
func TestDemoTemplatesFollowTheDaemonsOrder(t *testing.T) {
	var ids, levels []string
	for _, template := range demoTemplates() {
		ids, levels = append(ids, template.ID), append(levels, template.Level)
	}
	want := []string{"system", "mail", "repo", "research", "news", "blank"}
	if !slices.Equal(ids, want) {
		t.Fatalf("templates = %v, want %v", ids, want)
	}
	wantLevels := []string{protocol.LevelLight, protocol.LevelLight, protocol.LevelMedium, protocol.LevelMedium, protocol.LevelAdvanced, ""}
	if !slices.Equal(levels, wantLevels) {
		t.Fatalf("levels = %v, want %v", levels, wantLevels)
	}
}

// approving the proposed team makes it with its routines off and asks for the credential it named
func TestApprovingAProposalMakesTheTeamAndAsksForItsCredential(t *testing.T) {
	s := &server{data: makeFixtures()}
	approval := call(t, s, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: "approval-translations", Decision: protocol.DecideOnce}).(protocol.Approval)
	if approval.Status != protocol.ApprovalApproved || approval.Team == "" || approval.Result == "" {
		t.Fatalf("the proposal was not settled with its team: %+v", approval)
	}
	index := s.data.teams.teamIndex(approval.Team)
	if index < 0 || s.data.teams.teams[index].Name != "Translations" || len(s.data.teams.teams[index].Members) != 2 {
		t.Fatalf("the proposed team was not made: index %d", index)
	}
	for _, routine := range s.data.teams.routines {
		if routine.Team == approval.Team && routine.Enabled {
			t.Fatalf("a proposed routine arrived on: %+v", routine)
		}
	}
	if names := requested(s, approval.Team); !slices.Equal(names, []string{"DEEPL_API_KEY"}) {
		t.Fatalf("credential requests = %v, want DEEPL_API_KEY", names)
	}
}

// a denied proposal makes nothing
func TestDenyingAProposalMakesNothing(t *testing.T) {
	s := &server{data: makeFixtures()}
	teams := len(s.data.teams.teams)
	call(t, s, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: "approval-translations", Decision: protocol.DecideDeny})
	if len(s.data.teams.teams) != teams {
		t.Fatalf("a denied proposal made a team")
	}
}

// a team made from a template asks only for the credentials the vault does not hold yet
func TestATemplateTeamAsksForWhatTheVaultLacks(t *testing.T) {
	s := &server{data: makeFixtures()}
	news := demoTemplate("news")
	params := protocol.TeamCreateParams{Name: "Morning news", Template: news.ID, Members: news.Members, Routines: news.Routines}
	held := call(t, s, protocol.MTeamsCreate, params).(protocol.Team)
	if names := requested(s, held.ID); len(names) != 0 {
		t.Fatalf("asked for a credential the vault holds: %v", names)
	}
	s.data.teams.vault.Credentials = slices.DeleteFunc(s.data.teams.vault.Credentials, func(credential protocol.Credential) bool {
		return credential.Name == "NEWS_API_KEY"
	})
	params.Name = "Evening news"
	lacking := call(t, s, protocol.MTeamsCreate, params).(protocol.Team)
	if names := requested(s, lacking.ID); !slices.Equal(names, []string{"NEWS_API_KEY"}) {
		t.Fatalf("credential requests = %v, want NEWS_API_KEY", names)
	}
}
