package shell

import (
	"image"
	"slices"
	"strings"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

func TestDemoStartupEnablesOnlyTheFeatureBeingShown(t *testing.T) {
	for _, check := range []struct {
		name     string
		options  Options
		features []string
	}{
		{"default", Options{}, nil},
		{"new Ask", Options{Route: protocol.TabAsk}, nil},
		{"ordinary Ask", Options{Route: protocol.TabAsk, Session: "ask-20261110-094200-a17c"}, nil},
		{"compare", Options{Route: protocol.TabAsk, Session: "ask-compare"}, []string{"compare"}},
		{"artifacts", Options{Route: protocol.TabAsk, Session: "ask-artifacts"}, []string{"artifacts", "mindmap"}},
		{"search progress", Options{Route: protocol.TabAsk, Session: "ask-searchsteps-live"}, []string{"searchsteps"}},
		{"agent settings", Options{Route: protocol.TabSettings, SettingsSection: "agents"}, []string{"mcp"}},
		{"team settings", Options{Route: protocol.TabSettings, SettingsSection: "teams"}, []string{"diskcare"}},
		{"memory settings", Options{Route: protocol.TabSettings, SettingsSection: "memory"}, []string{"profile"}},
		{"collections", Options{Route: protocol.TabIndex, Session: "collections/saved"}, []string{"collections"}},
		{"research", Options{Route: protocol.TabDreams, Teams: env.TeamsPlace{Team: "dream-research"}}, []string{"research"}},
		{"scheduled Ask", Options{Route: protocol.TabRoutines, Teams: env.TeamsPlace{Routine: "routine-personal-ask"}}, []string{"schedules"}},
	} {
		t.Run(check.name, func(t *testing.T) {
			stage := newStage(t, image.Pt(1440, 900), check.options)
			for _, id := range protocol.FeatureIDs {
				if got, want := stage.runtime.store.Snapshot().Settings.Features[id], slices.Contains(check.features, id); got != want {
					t.Errorf("%s = %v, want %v", id, got, want)
				}
			}
		})
	}
}

func TestPromptClipboardDoesNotKeepTranscriptOrComposerPresses(t *testing.T) {
	options := Options{Route: protocol.TabSettings, SettingsSection: "ask"}
	for _, size := range []image.Point{{1440, 900}, {1000, 720}, {560, 800}} {
		stage := newStage(t, size, options)
		for _, route := range []string{"ask/new", styledAsk} {
			stage.open(route)
			if dead := stage.unreachable(); len(dead) > 0 {
				t.Errorf("%dx%d %s with prompts on:\n  %s", size.X, size.Y, route, strings.Join(dead, "\n  "))
			}
		}
	}
	stage := newStage(t, image.Pt(1440, 2400), options)
	stage.open(styledAsk)
	stage.across(styledParagraph)
	stage.press("C", key.ModShortcut)
	copied := stage.clipboard()
	if len(copied) < 8 || !strings.HasPrefix(styledParagraph, copied) {
		t.Fatalf("marking text with prompts on copied %q", copied)
	}
}
