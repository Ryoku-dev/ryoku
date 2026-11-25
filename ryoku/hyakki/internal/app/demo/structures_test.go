package demo

import (
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/structures"
)

func TestDemoReviewedPeersAndLargeParadeUseProductionDefinitionRules(t *testing.T) {
	s := &server{data: makeFixtures()}
	for _, id := range []string{"seven-princes", "night-parade"} {
		recipe, _ := structures.Lookup(id)
		count := recipe.MinSeats
		if recipe.Variable {
			count = protocol.TeamsMembersMax
		}
		members, err := structures.Instantiate(recipe, count)
		if err != nil {
			t.Fatal(err)
		}
		members[0].Name = "Owner edited name"
		team, err := s.createTeam(protocol.TeamCreateParams{Name: recipe.Name, Purpose: "A reviewed task", Folder: filepath.Join(t.TempDir(), id), Structure: id, Reporting: recipe.Reporting, Members: members, PlanApproval: true}, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(team.Members) != count || team.Members[0].Name != "Owner edited name" || team.Reporting != recipe.Reporting || team.Structure != id || !team.PlanApproval {
			t.Fatalf("demo changed the reviewed definition: %+v", team)
		}
		if id == "seven-princes" {
			for _, member := range team.Members {
				if member.Lead {
					t.Fatal("demo fabricated a boss for peers")
				}
			}
		}
		if err := protocol.ValidateReporting(team); err != nil {
			t.Fatal(err)
		}
	}
	_, err := s.createTeam(protocol.TeamCreateParams{Name: "Broken tree", Purpose: "A reviewed task", Folder: t.TempDir(), Structure: "night-parade", Reporting: protocol.ReportingTree, Members: []protocol.Member{{ID: "a", Name: "A", Lead: true}, {ID: "b", Name: "B", ReportsTo: "absent"}}}, false)
	if err == nil || protocol.AsError(err).Code != protocol.CodeBadRequest {
		t.Fatalf("demo accepted invalid reviewed reporting: %v", err)
	}
}
