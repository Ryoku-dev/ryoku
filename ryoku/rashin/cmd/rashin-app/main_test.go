package main

import (
	"context"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestSendOpenForwardsLaunchTarget(t *testing.T) {
	want := protocol.UIOpenParams{Session: "agent-20261110-101500-b82d", Tab: protocol.TabProjects, Project: "hyperion", View: "changes", Worktree: "/tmp/hyperion/settings-panel"}
	called := false
	err := sendOpen(context.Background(), want, func(_ context.Context, method string, params, result any) error {
		called = true
		if method != protocol.MUIOpen {
			t.Fatalf("method = %q, want %q", method, protocol.MUIOpen)
		}
		got, ok := params.(protocol.UIOpenParams)
		if !ok || got != want {
			t.Fatalf("params = %#v, want %#v", params, want)
		}
		if _, ok := result.(*protocol.UIOpenResult); !ok {
			t.Fatalf("result = %T, want *protocol.UIOpenResult", result)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("ui.open was not sent")
	}
}

func TestAStoryboardRunsOnlyAgainstTheDemo(t *testing.T) {
	if demoOnly("search.story", false) == nil {
		t.Fatal("--script without --demo was accepted")
	}
	if err := demoOnly("search.story", true); err != nil {
		t.Fatalf("--script with --demo: %v", err)
	}
	if err := demoOnly("", false); err != nil {
		t.Fatalf("no script: %v", err)
	}
}

func TestApplyLaunchTargetOpensProject(t *testing.T) {
	target := applyLaunchTarget(
		parseRouteTarget("agent/agent-working"),
		protocol.TabSettings,
		"agent-other",
		"hyperion",
		"changes",
		"/tmp/hyperion/settings-panel",
	)
	if target.tab != protocol.TabProjects || target.session != "" || target.project != "hyperion" || target.workTab != "changes" || target.worktree != "/tmp/hyperion/settings-panel" {
		t.Fatalf("project launch target = %#v", target)
	}

	target = applyLaunchTarget(parseRouteTarget(protocol.TabAsk), "", "", "hyperion", "", "")
	if target.tab != protocol.TabProjects || target.workTab != "overview" {
		t.Fatalf("default project launch target = %#v", target)
	}
}

func TestParseRouteTargetSupportsWorkAndSettings(t *testing.T) {
	work := parseRouteTarget("projects/hyperion/changes/settings-panel")
	if work.tab != protocol.TabProjects || work.project != "hyperion" || work.workTab != "changes" || work.worktree != "settings-panel" {
		t.Fatalf("work route = %#v", work)
	}
	settings := parseRouteTarget("settings/agent")
	if settings.tab != protocol.TabSettings || settings.settingsSection != "agent" {
		t.Fatalf("settings route = %#v", settings)
	}
	agent := parseRouteTarget("agent/agent-working")
	if agent.tab != protocol.TabAgent || agent.session != "agent-working" {
		t.Fatalf("agent route = %#v", agent)
	}
}

func TestParseRouteTargetSupportsNewAskAndAgentBoard(t *testing.T) {
	newAsk := parseRouteTarget("ask/new")
	if newAsk.tab != protocol.TabAsk || newAsk.session != "" {
		t.Fatalf("new ask route = %#v", newAsk)
	}
	board := parseRouteTarget("agent")
	if board.tab != protocol.TabAgent || board.session != "" {
		t.Fatalf("agent board route = %#v", board)
	}
}

func TestASessionNamedAloneOpensInItsOwnTab(t *testing.T) {
	for _, check := range []struct {
		route, tab, session, want string
	}{
		{session: "agent-20261110-171755-5b30", want: protocol.TabAgent},
		{session: "ask-20261110-094200-a17c", want: protocol.TabAsk},
		{session: "agent-20261110-171755-5b30", tab: protocol.TabAsk, want: protocol.TabAsk},
		{route: "agent", want: protocol.TabAgent},
		{want: ""},
	} {
		if got := launchTarget(check.route, check.tab, check.session, "", "", "").tab; got != check.want {
			t.Errorf("route %q tab %q session %q opens tab %q, want %q", check.route, check.tab, check.session, got, check.want)
		}
	}
}

func TestATeamsRouteOpensItsPlaceAndAsksARunningAppForIt(t *testing.T) {
	target := launchTarget("teams/team-research/spend", "", "", "", "", "")
	if target.tab != protocol.TabTeams || target.teams.Team != "team-research" || target.teams.View != "spend" {
		t.Fatalf("team route = %#v", target)
	}
	params := openParams(launchTarget("work/task-6f1c2a9e", "", "", "", "", ""))
	if params.Tab != protocol.TabWork || params.TeamTask != "task-6f1c2a9e" {
		t.Fatalf("a running app was asked for %#v, want the task", params)
	}
	if inbox := launchTarget("inbox", "", "", "", "", ""); inbox.tab != protocol.TabInbox {
		t.Fatalf("inbox route = %#v", inbox)
	}
	if project := launchTarget("work/task-6f1c2a9e", "", "", "hyperion", "", ""); project.tab != protocol.TabProjects || project.teams.Task != "" {
		t.Fatalf("a named project kept the Teams place: %#v", project)
	}
}
