package shell

import (
	"image"
	"slices"
	"strings"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/app/demo"
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
		{"learning", Options{Route: protocol.TabLearning}, []string{protocol.FeatureAutolearn, protocol.FeatureDecisions, protocol.FeatureRules}},
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

// Settings sub-pages share the explicitly requested Oni demo scenario.
// This is demo setup only: ordinary startup and onboarding stay off.
func TestOniSubpageDemoStartupPreservesItsRequestedScenario(t *testing.T) {
	for _, check := range []struct {
		section, session                        string
		voice, guides, commands, writing, tools bool
	}{
		{section: "oni", session: demo.FeatureOni},
		{section: "oni-voice", session: demo.FeatureOniVoice, voice: true},
		{section: "oni-screen", session: demo.FeatureOniGuide, guides: true},
		{section: "oni-commands", session: demo.FeatureOniReflex, voice: true, commands: true},
		{section: "oni-writing", session: demo.FeatureOniPhase16, voice: true, commands: true, writing: true},
		{section: "oni-privacy", session: demo.FeatureOniPhase17, voice: true, commands: true, tools: true},
		{section: "parts", session: demo.FeatureOni},
	} {
		t.Run(check.section, func(t *testing.T) {
			s := newStage(t, image.Pt(1440, 900), Options{Route: protocol.TabSettings, SettingsSection: check.section, Session: check.session})
			snapshot := s.runtime.store.Snapshot()
			oni := snapshot.Settings.Oni
			if !oni.Master || !oni.Inspect {
				t.Fatalf("the requested Oni scenario was lost on %s: %+v", check.section, oni)
			}
			if oni.Voice != check.voice || oni.Guide.Enabled != check.guides || oni.Reflex != check.commands || oni.Rewrite.Enabled != check.writing || oni.MCP.Enabled != check.tools {
				t.Fatalf("%s did not receive its requested scenario: %+v", check.section, oni)
			}
			for _, id := range protocol.FeatureIDs {
				if snapshot.Settings.Features[id] {
					t.Fatalf("visiting the Oni scenario also enabled unrelated feature %s", id)
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
