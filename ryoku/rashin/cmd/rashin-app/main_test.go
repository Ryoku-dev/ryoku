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
	if settings.tab != protocol.TabSettings || settings.settingsSection != 2 {
		t.Fatalf("settings route = %#v", settings)
	}
	agent := parseRouteTarget("agent/agent-working")
	if agent.tab != protocol.TabAgent || agent.session != "agent-working" {
		t.Fatalf("agent route = %#v", agent)
	}
}
