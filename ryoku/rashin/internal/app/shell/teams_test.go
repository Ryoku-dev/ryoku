package shell

import (
	"image"
	"testing"

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
	stage.click("Teams")
	if stage.shell.router.Tab != protocol.TabInbox || !stage.shell.router.TeamsMode() {
		t.Fatalf("the Teams segment went to %q, want the Inbox", stage.shell.router.Tab)
	}
	for _, name := range []string{"Inbox", "Teams", "Work", "Routines", "Vault", "Activity"} {
		if _, ok := stage.named(name); !ok {
			t.Errorf("Teams mode shows no %s", name)
		}
	}
	if _, ok := stage.named("Projects"); ok {
		t.Error("Teams mode still shows Rashin's Projects")
	}
	stage.click("Rashin")
	if stage.shell.router.Tab != protocol.TabAsk || stage.shell.router.TeamsMode() {
		t.Fatalf("the Rashin segment went to %q, want Ask", stage.shell.router.Tab)
	}
	for _, name := range []string{"Ask", "Agents", "Projects", "Index"} {
		if _, ok := stage.named(name); !ok {
			t.Errorf("Rashin mode shows no %s", name)
		}
	}
	if _, ok := stage.named("Routines"); ok {
		t.Error("Rashin mode still shows Teams' Routines")
	}
}
