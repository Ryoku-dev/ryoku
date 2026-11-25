package teamsui

import (
	"fmt"
	"slices"
	"testing"

	"ryoku-rashin/internal/protocol"
	backendteams "ryoku-rashin/internal/teams"
)

func TestCrewHierarchyFollowsMultilevelCourt(t *testing.T) {
	// Deliberately neither rank order nor reporting order: the King at index
	// zero reports to the Duke, and the configured root is last.
	team := protocol.Team{Reporting: protocol.ReportingTree, Members: []protocol.Member{
		{ID: "builder", Name: "Builder", Rank: "king", ReportsTo: "manager"},
		{ID: "reviewer", Name: "Reviewer", ReportsTo: "root"},
		{ID: "manager", Name: "Manager", Rank: "duke", ReportsTo: "root"},
		{ID: "root", Name: "Paimon", Lead: true},
	}}
	want := []crewBranch{{Member: 3, Parent: -1, Depth: 0}, {Member: 1, Parent: 3, Depth: 1}, {Member: 2, Parent: 3, Depth: 1}, {Member: 0, Parent: 2, Depth: 2}}
	if got := crewHierarchy(team); !slices.Equal(got, want) {
		t.Fatalf("court = %+v, want %+v", got, want)
	}
	team.Members[0].ReportsTo = "reviewer"
	want = []crewBranch{{Member: 3, Parent: -1, Depth: 0}, {Member: 1, Parent: 3, Depth: 1}, {Member: 0, Parent: 1, Depth: 2}, {Member: 2, Parent: 3, Depth: 1}}
	if got := crewHierarchy(team); !slices.Equal(got, want) {
		t.Fatalf("reconfigured court = %+v, want %+v", got, want)
	}
}

func TestCrewHierarchyPeersRemainEqualWithOrWithoutSynthesizer(t *testing.T) {
	team := protocol.Team{Reporting: protocol.ReportingPeers, Members: []protocol.Member{
		{ID: "earth", Name: "Earth", Rank: "king"},
		{ID: "air", Name: "Air"},
		{ID: "fire", Name: "Fire"},
	}}
	want := []crewBranch{{Member: 0, Parent: -1, Depth: 0}, {Member: 1, Parent: -1, Depth: 0}, {Member: 2, Parent: -1, Depth: 0}}
	if member, at := Lead(team); at != -1 || member.ID != "" {
		t.Fatalf("a first peer became leader: %+v at %d", member, at)
	}
	if got := crewHierarchy(team); !slices.Equal(got, want) {
		t.Fatalf("peers = %+v, want %+v", got, want)
	}
	team.Members[1].Lead = true
	if member, at := Lead(team); at != 1 || member.ID != "air" {
		t.Fatalf("configured synthesizer not selected: %+v at %d", member, at)
	}
	if got := crewHierarchy(team); !slices.Equal(got, want) {
		t.Fatalf("synthesizer gained subordinates or priority placement: %+v", got)
	}
}

func TestCrewHierarchyPreservesLegacyStarAndDreamManager(t *testing.T) {
	legacy := protocol.Team{Members: []protocol.Member{
		{ID: "worker"}, {ID: "lead", Lead: true}, {ID: "other"},
	}}
	want := []crewBranch{{Member: 1, Parent: -1, Depth: 0}, {Member: 0, Parent: 1, Depth: 1}, {Member: 2, Parent: 1, Depth: 1}}
	if got := crewHierarchy(legacy); !slices.Equal(got, want) {
		t.Fatalf("legacy star = %+v, want %+v", got, want)
	}
	dream := protocol.Team{Reporting: protocol.ReportingTree, Dream: &protocol.Dream{Manager: "lead"}, Members: []protocol.Member{
		{ID: "worker", ReportsTo: "lead"}, {ID: "lead", Lead: true}, {ID: "other", ReportsTo: "worker"},
	}}
	want = []crewBranch{{Member: 1, Parent: -1, Depth: 0}, {Member: 0, Parent: 1, Depth: 1}, {Member: 2, Parent: 0, Depth: 2}}
	if got := crewHierarchy(dream); !slices.Equal(got, want) {
		t.Fatalf("dream's CEO root and manager chain = %+v, want %+v", got, want)
	}
}

func TestCrewHierarchyRetainsEverySupportedSeat(t *testing.T) {
	for _, reporting := range []string{protocol.ReportingTree, protocol.ReportingPeers} {
		t.Run(reporting, func(t *testing.T) {
			team := protocol.Team{Reporting: reporting, Members: make([]protocol.Member, protocol.TeamsMembersMax)}
			for index := range team.Members {
				member := &team.Members[index]
				member.ID = fmt.Sprintf("seat-%d", index)
				if reporting == protocol.ReportingTree {
					member.Lead = index == 0
					if index > 0 {
						member.ReportsTo = team.Members[index-1].ID
					}
				}
			}
			got := crewHierarchy(team)
			if len(got) != len(team.Members) {
				t.Fatalf("lost seats: %d of %d", len(got), len(team.Members))
			}
			for index, branch := range got {
				parent, depth := -1, 0
				if reporting == protocol.ReportingTree {
					parent, depth = index-1, index
				}
				if branch != (crewBranch{Member: index, Parent: parent, Depth: depth}) {
					t.Fatalf("seat %d changed hierarchy: %+v", index, branch)
				}
			}
		})
	}
}

func TestProposedRoutineUsesOnlyConfiguredDefault(t *testing.T) {
	team := protocol.TeamCreateParams{Reporting: protocol.ReportingPeers, Members: []protocol.Member{
		{ID: "earth", Name: "Earth"}, {ID: "air", Name: "Air"},
	}}
	if got := proposalRunner(team, ""); got == team.Members[0].Name || got == team.Members[1].Name {
		t.Fatalf("unassigned peer routine was presented as running on %s", got)
	}
	if got := proposalRunner(team, "air"); got != "Air" {
		t.Fatalf("explicit peer assignment = %q, want Air", got)
	}
	team.Members[1].Lead = true
	if got := proposalRunner(team, ""); got != "Air" {
		t.Fatalf("synthesizer default = %q, want Air", got)
	}
	if got := proposalRunner(team, "removed"); got == "Air" || got == "Earth" {
		t.Fatalf("removed explicit assignment silently became %q", got)
	}
	team.Reporting = ""
	if got := proposalRunner(team, ""); got != "Air" {
		t.Fatalf("legacy root default = %q, want Air", got)
	}
}

func TestProposedLegacyRoutineUsesTheAcceptedDefault(t *testing.T) {
	proposal := protocol.TeamCreateParams{Members: []protocol.Member{
		{ID: "first", Name: "First"}, {ID: "second", Name: "Second"},
	}}
	if got := proposalRunner(proposal, ""); got != "First" {
		t.Fatalf("legacy default was previewed as %q rather than the accepted first member", got)
	}
	proposal.Members[1].Lead = true
	if got := proposalRunner(proposal, ""); got != "Second" {
		t.Fatalf("explicit legacy default was previewed as %q", got)
	}
	proposal.Reporting = protocol.ReportingPeers
	proposal.Members[1].Lead = false
	if got := proposalRunner(proposal, ""); got == "First" || got == "Second" {
		t.Fatalf("legacy preview normalization invented a default for explicit peers: %q", got)
	}
}

func TestLegacyProposalPreviewMatchesAcceptedReporting(t *testing.T) {
	for _, members := range [][]protocol.Member{
		{{Name: "First"}, {Name: "Second"}},
		{{Name: "Builder"}, {Name: "Coordinator", Lead: true}, {Name: "Reviewer"}},
		{{ID: "root", Name: "Root", Lead: true}, {ID: "manager", Name: "Manager", ReportsTo: "root"}, {ID: "worker", Name: "Worker", ReportsTo: "manager"}},
		{{ID: "first", Name: "First", Lead: true}, {ID: "second", Name: "Second", Lead: true}},
		{{Name: "First"}, {ID: "proposal-seat-0", Name: "Second"}},
		{{ID: " root ", Name: " Root ", Lead: true}, {ID: "child", Name: "Child", ReportsTo: " root "}},
	} {
		t.Run(members[0].Name+members[1].Name, func(t *testing.T) {
			original := slices.Clone(members)
			params := normalizedProposal(protocol.TeamCreateParams{Members: members})
			preview := protocol.Team{Members: params.Members, Reporting: params.Reporting}
			accepted := protocol.Team{Name: "Proposed legacy", Purpose: "Compare the preview and accepted reporting", Folder: t.TempDir(), Members: members}
			if _, err := backendteams.NormalizeDefinition(&accepted, true, protocol.Settings{}); err != nil {
				t.Fatal(err)
			}
			if got, want := crewHierarchy(preview), crewHierarchy(accepted); !slices.Equal(got, want) {
				t.Fatalf("proposal preview disagreed with accepted reporting: got %+v, want %+v", got, want)
			}
			for index, member := range preview.Members {
				if member.Name != accepted.Members[index].Name || member.Lead != accepted.Members[index].Lead {
					t.Fatal("proposal preview changed the accepted identity or default")
				}
				if members[index].ID != original[index].ID || members[index].Name != original[index].Name || members[index].Lead != original[index].Lead || members[index].ReportsTo != original[index].ReportsTo {
					t.Fatal("rendering the proposal changed the saved proposal")
				}
			}
		})
	}
}
