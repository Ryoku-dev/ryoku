package demo

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestOptionalFeaturesStartOffInTheDemo(t *testing.T) {
	fixtures := makeFixtures()
	settings := fixtures.snapshot.Settings
	for _, id := range protocol.FeatureIDs {
		if settings.Features[id] {
			t.Errorf("%s starts on", id)
		}
	}
	for _, id := range []string{"ask-compare", "ask-artifacts", "ask-searchsteps-live", "ask-searchsteps-done", "ask-scheduled-morning"} {
		if _, exists := fixtures.events[id]; exists {
			t.Errorf("optional session %s starts in the default demo", id)
		}
	}
	for _, info := range fixtures.snapshot.Sessions {
		if info.Starred || info.Archived {
			t.Errorf("default session %s has optional history flags", info.ID)
		}
	}
}

func TestTheDemoEnablesOnlyRequestedFeatures(t *testing.T) {
	for _, feature := range protocol.FeatureIDs {
		t.Run(feature, func(t *testing.T) {
			fixtures := makeFixtures(feature)
			for _, id := range protocol.FeatureIDs {
				if got, want := fixtures.snapshot.Settings.Features[id], id == feature; got != want {
					t.Errorf("%s = %v, want %v", id, got, want)
				}
			}
			for id, enabled := range map[string]bool{
				"ask-compare":           feature == "compare",
				"ask-artifacts":         feature == "artifacts" || feature == "mindmap",
				"ask-searchsteps-live":  feature == "searchsteps",
				"ask-searchsteps-done":  feature == "searchsteps",
				"ask-scheduled-morning": feature == "schedules",
			} {
				if _, exists := fixtures.events[id]; exists != enabled {
					t.Errorf("fixture %s present = %v, want %v", id, exists, enabled)
				}
			}
			if feature == "research" && !hasDemoTeam(fixtures, "dream-research") {
				t.Fatal("research has no dream fixture")
			}
			if feature == "schedules" {
				found := false
				for _, routine := range fixtures.teams.routines {
					found = found || routine.ID == "routine-personal-ask"
				}
				if !found {
					t.Fatal("schedules has no personal Ask fixture")
				}
			}
		})
	}
}

func hasDemoTeam(fixtures fixtures, id string) bool {
	for _, team := range fixtures.teams.teams {
		if team.ID == id {
			return true
		}
	}
	return false
}
