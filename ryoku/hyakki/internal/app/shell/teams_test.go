package shell

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

func TestEveryTeamsRouteParses(t *testing.T) {
	for route, want := range map[string]env.TeamsPlace{
		"inbox":                  {Tab: protocol.TabInbox},
		"teams":                  {Tab: protocol.TabTeams},
		"teams/new":              {Tab: protocol.TabTeams, Draft: env.DraftTeam},
		"teams/new/research":     {Tab: protocol.TabTeams, Draft: env.DraftTeam, Template: "research"},
		"teams/team-a1":          {Tab: protocol.TabTeams, Team: "team-a1"},
		"teams/team-a1/spend":    {Tab: protocol.TabTeams, Team: "team-a1", View: env.TeamSpend},
		"dreams":                 {Tab: protocol.TabDreams},
		"dreams/new":             {Tab: protocol.TabDreams, Draft: env.DraftDream},
		"dreams/dream-esp32":     {Tab: protocol.TabDreams, Team: "dream-esp32"},
		"work":                   {Tab: protocol.TabWork},
		"work/task-6f1c2a9e":     {Tab: protocol.TabWork, Task: "task-6f1c2a9e"},
		"routines":               {Tab: protocol.TabRoutines},
		"routines/new":           {Tab: protocol.TabRoutines, Draft: env.DraftRoutine},
		"routines/routine-b2":    {Tab: protocol.TabRoutines, Routine: "routine-b2"},
		"vault":                  {Tab: protocol.TabVault},
		"vault/credential-c3":    {Tab: protocol.TabVault, Credential: "credential-c3"},
		"activity":               {Tab: protocol.TabActivity},
		"/teams/team-a1/work/":   {Tab: protocol.TabTeams, Team: "team-a1", View: env.TeamWork},
		"routines/new/team-mail": {Tab: protocol.TabRoutines, Draft: env.DraftRoutine, Team: "team-mail"},
	} {
		got, ok := TeamsRoute(route)
		if !ok || got != want {
			t.Errorf("route %q = %+v, %v; want %+v", route, got, ok, want)
		}
	}
	for _, route := range []string{"", "ask", "agent/agent-1", "projects/hyperion", "settings/teams"} {
		if place, ok := TeamsRoute(route); ok {
			t.Errorf("route %q read as the Teams place %+v", route, place)
		}
	}
}

func TestTheModeSwitchGoesBothWays(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.click("The Yokai")
	if stage.shell.router.Tab != protocol.TabInbox || !stage.shell.router.TeamsMode() {
		t.Fatalf("the Teams segment went to %q, want the Inbox", stage.shell.router.Tab)
	}
	for _, name := range []string{"Inbox", "Teams", "Dreams", "Work", "Routines", "Vault", "Activity"} {
		if _, ok := stage.named(name); !ok {
			t.Errorf("Teams mode shows no %s", name)
		}
	}
	if _, ok := stage.named("Projects"); ok {
		t.Error("Teams mode still shows Hyakki's Projects")
	}
	stage.click("Hyakki")
	if stage.shell.router.Tab != protocol.TabAsk || stage.shell.router.TeamsMode() {
		t.Fatalf("the Hyakki segment went to %q, want Ask", stage.shell.router.Tab)
	}
	for _, name := range []string{"Ask", "Agents", "Projects", "Index"} {
		if _, ok := stage.named(name); !ok {
			t.Errorf("Hyakki mode shows no %s", name)
		}
	}
	if _, ok := stage.named("Routines"); ok {
		t.Error("Hyakki mode still shows Teams' Routines")
	}
}

func TestTheModeSwitchFitsSmallWindows(t *testing.T) {
	for _, size := range []struct {
		name string
		size image.Point
	}{
		{"compact", image.Pt(640, 700)},
		{"medium", image.Pt(900, 700)},
	} {
		t.Run(size.name, func(t *testing.T) {
			stage := newStage(t, size.size)
			stage.click("The Yokai")
			if stage.shell.router.Tab != protocol.TabInbox || !stage.shell.router.TeamsMode() {
				t.Fatalf("the team mode switch did not reach Inbox at %v", size.size)
			}
			stage.click("Hyakki")
			if stage.shell.router.Tab != protocol.TabAsk || stage.shell.router.TeamsMode() {
				t.Fatalf("the harness mode switch did not reach Ask at %v", size.size)
			}
		})
	}
}

func TestTeamSettingsUseModelsWithoutSeparateSignIn(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 1200))
	stage.open("teams/team-research/settings")
	want := map[string]bool{
		"Identity": true, "Crew and models": true, "Set every member's model": true,
		"Tools and access": true, "Permissions": true, "Budget and limits": true,
		"Notifications": true, "Memory": true, "Skills": true,
	}
	at := f32.Pt(1100, 600)
	for range 24 {
		for name := range want {
			if _, ok := stage.named(name); ok {
				delete(want, name)
			}
		}
		if _, ok := stage.named("Sign-in for Teams"); ok {
			t.Fatal("the native team's settings still ask for a harness sign-in")
		}
		if len(want) == 0 {
			return
		}
		stage.queue(
			pointer.Event{Kind: pointer.Move, Position: at},
			pointer.Event{Kind: pointer.Scroll, Position: at, Scroll: f32.Pt(0, 320)},
		)
		stage.settle()
	}
	for name := range want {
		t.Errorf("the team settings lack %q", name)
	}
}
