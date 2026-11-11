package shell

import (
	"strings"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

// TeamsRoute reads a Teams route: inbox, teams, teams/new[/<template>],
// teams/<team>[/<view>], work[/<task>], routines, routines/new[/<team>],
// routines/<routine>, vault[/<credential>] or activity. It is false for any
// other route.
func TeamsRoute(route string) (env.TeamsPlace, bool) {
	parts := strings.Split(strings.Trim(route, "/"), "/")
	if !teamsTab(parts[0]) {
		return env.TeamsPlace{}, false
	}
	part := func(index int) string {
		if index < len(parts) {
			return parts[index]
		}
		return ""
	}
	place := env.TeamsPlace{Tab: parts[0]}
	switch place.Tab {
	case protocol.TabTeams:
		if part(1) == "new" {
			place.Draft, place.Template = env.DraftTeam, part(2)
		} else {
			place.Team, place.View = part(1), part(2)
		}
	case protocol.TabWork:
		place.Task = part(1)
	case protocol.TabRoutines:
		if part(1) == "new" {
			place.Draft, place.Team = env.DraftRoutine, part(2)
		} else {
			place.Routine = part(1)
		}
	case protocol.TabVault:
		place.Credential = part(1)
	}
	return place, true
}

// the Teams part of a place, as the Teams pages read it
func (place Place) teams() env.TeamsPlace {
	return env.TeamsPlace{
		Tab: place.Tab, Team: place.Team, View: place.TeamView, Task: place.TeamTask, Routine: place.Routine,
		Credential: place.Credential, Draft: place.Draft, Template: place.Template,
	}
}

func teamsPlace(teams env.TeamsPlace) Place {
	return Place{
		Tab: teams.Tab, Team: teams.Team, TeamView: teams.View, TeamTask: teams.Task, Routine: teams.Routine,
		Credential: teams.Credential, Draft: teams.Draft, Template: teams.Template,
	}
}

// a ui.focus that names a team, a task, a routine, a credential or a Teams destination
func focusedTeams(focus protocol.UIOpenParams) (env.TeamsPlace, bool) {
	switch {
	case focus.TeamTask != "":
		return env.TeamsPlace{Tab: protocol.TabWork, Task: focus.TeamTask}, true
	case focus.Routine != "":
		return env.TeamsPlace{Tab: protocol.TabRoutines, Routine: focus.Routine}, true
	case focus.Credential != "":
		return env.TeamsPlace{Tab: protocol.TabVault, Credential: focus.Credential}, true
	case focus.Team != "":
		return env.TeamsPlace{Tab: protocol.TabTeams, Team: focus.Team, View: focus.View}, true
	case teamsTab(focus.Tab):
		return env.TeamsPlace{Tab: focus.Tab}, true
	}
	return env.TeamsPlace{}, false
}

func (s *uiShell) navigateTeams(place env.TeamsPlace) {
	s.router.Push(teamsPlace(place))
}

func (s *uiShell) teamsPage(tab string) env.TeamsPage {
	switch tab {
	case protocol.TabTeams:
		return s.teams
	case protocol.TabWork:
		return s.work
	case protocol.TabRoutines:
		return s.routines
	case protocol.TabVault:
		return s.vault
	case protocol.TabActivity:
		return s.activity
	default:
		return s.inbox
	}
}
