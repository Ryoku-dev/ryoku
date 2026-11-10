package shell

import (
	"testing"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

func TestTheWindowReopensOnTheWorkItHadOpen(t *testing.T) {
	session := lastPlace(Place{Tab: protocol.TabAgent, Session: "agent-1"})
	if resumed := resume(Options{Preferences: state.AppPreferences{Last: session}}); resumed.Route != protocol.TabAgent || resumed.Session != "agent-1" {
		t.Fatalf("reopened at %+v, want the agent session", resumed)
	}
	project := lastPlace(Place{Tab: protocol.TabProjects, Project: "hyperion", ProjectView: "changes", Worktree: "/work/tree"})
	resumed := resume(Options{Preferences: state.AppPreferences{Last: project}})
	if resumed.Project != "hyperion" || resumed.WorkTab != "changes" || resumed.Worktree != "/work/tree" {
		t.Fatalf("reopened at %+v, want the project's changes", resumed)
	}
}

func TestAQuestionIsNotComeBackTo(t *testing.T) {
	asked := lastPlace(Place{Tab: protocol.TabAsk, Session: "ask-1"})
	if resumed := resume(Options{Preferences: state.AppPreferences{Last: asked}}); resumed.Route != "" || resumed.Session != "" {
		t.Fatalf("the app reopened on an old question: %+v", resumed)
	}
}

func TestALaunchThatNamesAPlaceGoesThere(t *testing.T) {
	last := lastPlace(Place{Tab: protocol.TabAgent, Session: "agent-1"})
	asked := Options{Route: protocol.TabSettings, SettingsSection: "api", Preferences: state.AppPreferences{Last: last}}
	if resumed := resume(asked); resumed.Route != protocol.TabSettings || resumed.Session != "" {
		t.Fatalf("a launch for Settings went to %+v", resumed)
	}
}
