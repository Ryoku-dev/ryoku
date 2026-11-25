package structures_test

import (
	"fmt"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/structures"
	"ryoku-rashin/internal/teams"
)

func peerRecipe(titles ...string) protocol.StructureTemplate {
	recipe := protocol.StructureTemplate{
		ID: "custom:creation-boundary", Name: "Creation boundary", Shape: "flat",
		Reporting: protocol.ReportingPeers, MinSeats: 1, MaxSeats: len(titles), DefaultSeats: len(titles),
	}
	for i, title := range titles {
		recipe.Seats = append(recipe.Seats, protocol.StructureSeat{
			ID: fmt.Sprintf("seat-%d", i), Title: title, Rank: "worker", Duty: "Review an assigned batch",
		})
	}
	return recipe
}

func paradeRecipe(worker string, maximum int) protocol.StructureTemplate {
	return protocol.StructureTemplate{
		ID: "custom:parade-boundary", Name: "Parade boundary", Shape: "horde",
		Reporting: protocol.ReportingTree, Variable: true, MinSeats: 2, MaxSeats: maximum, DefaultSeats: 2,
		Seats: []protocol.StructureSeat{
			{ID: "marshal", Title: "Marshal", Rank: "marshal", Duty: "Coordinate the work", Lead: true},
			{ID: "worker", Title: worker, Rank: "worker", Duty: "Review an assigned batch", ReportsToSeat: "marshal"},
		},
	}
}

func TestRecipeRejectsPeerManagers(t *testing.T) {
	recipe := peerRecipe("First", "Second")
	recipe.MinSeats = 2
	recipe.Seats[1].ReportsToSeat = recipe.Seats[0].ID
	if err := structures.Validate(recipe); err == nil {
		t.Fatal("accepted a peer recipe with a manager instead of direct Summoner reporting")
	}
	if _, err := structures.Instantiate(recipe, 2); err == nil {
		t.Fatal("produced a peer draft the team reporting boundary rejects")
	}
}

func TestRecipeRejectsUncreatableMemberText(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*protocol.StructureTemplate)
	}{
		{"ascii title", func(r *protocol.StructureTemplate) { r.Seats[0].Title = strings.Repeat("x", 81) }},
		{"unicode title", func(r *protocol.StructureTemplate) { r.Seats[0].Title = strings.Repeat("鬼", 81) }},
		{"ascii custom spirit", func(r *protocol.StructureTemplate) { r.Seats[0].DefaultSpirit = strings.Repeat("x", 81) }},
		{"unicode custom spirit", func(r *protocol.StructureTemplate) { r.Seats[0].DefaultSpirit = strings.Repeat("鬼", 81) }},
		{"blank custom spirit name", func(r *protocol.StructureTemplate) { r.Seats[0].DefaultSpirit = "   " }},
		{"ascii capabilities", func(r *protocol.StructureTemplate) { r.Seats[0].CapabilitiesHint = strings.Repeat("x", 4097) }},
		{"unicode capabilities", func(r *protocol.StructureTemplate) { r.Seats[0].CapabilitiesHint = strings.Repeat("鬼", 4097) }},
		{"rank zero byte", func(r *protocol.StructureTemplate) { r.Seats[0].Rank = "worker\x00" }},
		{"spirit zero byte", func(r *protocol.StructureTemplate) { r.Seats[0].DefaultSpirit = "custom\x00" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recipe := peerRecipe("Worker")
			tc.mutate(&recipe)
			if err := structures.Validate(recipe); err == nil {
				t.Fatal("accepted member text that cannot survive team normalization")
			}
			if _, err := structures.Instantiate(recipe, 0); err == nil {
				t.Fatal("produced an uncreatable member draft")
			}
		})
	}
}

func TestRecipeRejectsOverflowAtAnySelectableCount(t *testing.T) {
	fixed := peerRecipe(strings.Repeat("x", 79), strings.Repeat("X", 79))
	// At the default count the first name fits; the second needs a suffix.
	fixed.DefaultSeats = 1
	variable := paradeRecipe(strings.Repeat("x", 78), 11)
	// At ten total seats the worker index is one digit; at eleven it is two.
	collision := paradeRecipe(strings.Repeat("x", 77), 2)
	collision.Seats[0].Title = strings.Repeat("X", 77) + "-1"
	for _, recipe := range []protocol.StructureTemplate{fixed, variable, collision} {
		t.Run(recipe.ID+"/"+recipe.Seats[0].Title, func(t *testing.T) {
			if err := structures.Validate(recipe); err == nil {
				t.Fatal("accepted a selectable expansion whose numeric suffix overflows the name limit")
			}
			if _, err := structures.Instantiate(recipe, recipe.DefaultSeats); err == nil {
				t.Fatal("default draft hid an invalid optional expansion")
			}
		})
	}
}

func TestRecipeBoundaryDraftsSurviveActualTeamNormalization(t *testing.T) {
	ascii := peerRecipe(strings.Repeat("a", 80), strings.Repeat("b", 80))
	ascii.Seats[0].CapabilitiesHint = strings.Repeat("c", 4096)
	unicode := peerRecipe(strings.Repeat("鬼", 80), strings.Repeat("龍", 80))
	unicode.Seats[0].CapabilitiesHint = strings.Repeat("能", 4096)
	custom := peerRecipe("Role title")
	custom.Seats[0].DefaultSpirit = strings.Repeat("鬼", 80)
	canonical := peerRecipe(strings.Repeat("鬼", 160), "BAEL")
	canonical.Seats[0].DefaultSpirit = "bael"
	fixedSuffix := peerRecipe(strings.Repeat("x", 78), strings.Repeat("X", 78))
	unicodeSuffix := peerRecipe(strings.Repeat("鬼", 78), strings.Repeat("鬼", 78))
	variable := paradeRecipe(strings.Repeat("鬼", 77), 100)
	// Alternate two repeating prototypes; the final index must still fit.
	variable.Seats = append(variable.Seats, protocol.StructureSeat{
		ID: "second-worker", Title: strings.Repeat("龍", 77), Rank: "worker", Duty: "Review another batch", ReportsToSeat: "marshal",
	})
	collision := paradeRecipe(strings.Repeat("x", 76), 2)
	collision.Seats[0].Title = strings.Repeat("X", 76) + "-1"
	titles := make([]string, 100)
	for i := range titles {
		titles[i] = strings.Repeat("x", 76)
	}
	numeric := peerRecipe(titles...)
	for _, tc := range []struct {
		name   string
		recipe protocol.StructureTemplate
		first  string
		last   string
	}{
		{"ascii unique", ascii, strings.Repeat("a", 80), strings.Repeat("b", 80)},
		{"unicode unique", unicode, strings.Repeat("鬼", 80), strings.Repeat("龍", 80)},
		{"custom spirit", custom, strings.Repeat("鬼", 80), strings.Repeat("鬼", 80)},
		{"canonical spirit and case collision", canonical, "Bael", "BAEL-2"},
		{"case collision suffix", fixedSuffix, strings.Repeat("x", 78), strings.Repeat("X", 78) + "-2"},
		{"unicode suffix", unicodeSuffix, strings.Repeat("鬼", 78), strings.Repeat("鬼", 78) + "-2"},
		{"variable prototypes at hundred seats", variable, "Marshal", strings.Repeat("鬼", 77) + "-99"},
		{"variable collision suffix", collision, strings.Repeat("X", 76) + "-1", strings.Repeat("x", 76) + "-1-2"},
		{"three digit collision suffix", numeric, strings.Repeat("x", 76), strings.Repeat("x", 76) + "-100"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := structures.Validate(tc.recipe); err != nil {
				t.Fatalf("rejected valid boundary recipe: %v", err)
			}
			folder := t.TempDir()
			for count := tc.recipe.MinSeats; count <= tc.recipe.MaxSeats; count++ {
				members, err := structures.Instantiate(tc.recipe, count)
				if err != nil {
					t.Fatalf("count %d: %v", count, err)
				}
				if members[0].Name != tc.first {
					t.Fatalf("first name = %q, want %q", members[0].Name, tc.first)
				}
				if count == tc.recipe.MaxSeats && members[count-1].Name != tc.last {
					t.Fatalf("last name = %q, want %q", members[count-1].Name, tc.last)
				}
				team := protocol.Team{
					Name: "Boundary team", Purpose: "Review the assigned work", Folder: folder,
					Structure: tc.recipe.ID, Reporting: tc.recipe.Reporting, Members: members,
				}
				if _, err := teams.NormalizeDefinition(&team, true, protocol.Settings{}); err != nil {
					t.Fatalf("accepted count %d failed real team normalization: %v", count, err)
				}
				for i, member := range team.Members {
					if member.Name != members[i].Name || member.Capabilities != members[i].Capabilities || member.Model != "" {
						t.Fatalf("normalization changed owner text or inherited model at count %d seat %d", count, i)
					}
				}
			}
		})
	}
}
