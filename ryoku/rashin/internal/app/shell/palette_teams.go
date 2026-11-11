package shell

import (
	"sort"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// how many finished tasks search offers beside the open ones
const paletteFinishedTasks = 20

// the first four lead an empty search in Teams mode
func (s *uiShell) teamsCommands(e *env.Env) []paletteAction {
	command := i18n.T("Command")
	switchMode := paletteAction{icon: look.Icon.Teams, label: i18n.T("Open Teams"), kind: "Ctrl T", run: s.router.SwitchMode}
	if s.router.TeamsMode() {
		switchMode.label = i18n.T("Back to Rashin")
	}
	return []paletteAction{
		{icon: look.Icon.Add, label: i18n.T("Give a team a task"), kind: command, run: func() { e.NewTeamTask("") }},
		{icon: look.Icon.Teams, label: i18n.T("New team"), kind: command, run: func() {
			s.navigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Draft: env.DraftTeam})
		}},
		{icon: look.Icon.Routine, label: i18n.T("New routine"), kind: command, run: func() {
			s.navigateTeams(env.TeamsPlace{Tab: protocol.TabRoutines, Draft: env.DraftRoutine})
		}},
		switchMode,
		{icon: look.Icon.Vault, label: i18n.T("Add credential"), kind: command, run: func() {
			s.openDestination(protocol.TabVault)
			s.vault.New(e)
		}},
	}
}

// every team, the open tasks and the newest finished ones, and every routine
func (s *uiShell) teamsPlaces(snapshot state.Snapshot) []paletteAction {
	tasks := make([]protocol.TeamTask, 0, len(snapshot.TeamTasks))
	for _, task := range snapshot.TeamTasks {
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Updated.After(tasks[j].Updated) })
	actions := make([]paletteAction, 0, len(snapshot.Teams)+len(tasks)+len(snapshot.Routines))
	for _, team := range snapshot.Teams {
		place := env.TeamsPlace{Tab: protocol.TabTeams, Team: team.ID}
		actions = append(actions, paletteAction{
			icon: look.Icon.Teams, label: team.Name, kind: i18n.T("Team"), run: func() { s.navigateTeams(place) },
		})
	}
	finished := 0
	for _, task := range tasks {
		if !taskOpen(task) {
			if finished == paletteFinishedTasks {
				continue
			}
			finished++
		}
		place := env.TeamsPlace{Tab: protocol.TabWork, Task: task.ID}
		actions = append(actions, paletteAction{
			icon: look.Icon.Tasks, label: task.Title, kind: teamsui.TeamName(snapshot.Teams, task.Team), run: func() { s.navigateTeams(place) },
		})
	}
	for _, routine := range snapshot.Routines {
		place := env.TeamsPlace{Tab: protocol.TabRoutines, Routine: routine.ID}
		actions = append(actions, paletteAction{
			icon: look.Icon.Routine, label: routine.Title, kind: i18n.T("Routine"), run: func() { s.navigateTeams(place) },
		})
	}
	return actions
}

func taskOpen(task protocol.TeamTask) bool {
	switch task.Status {
	case protocol.TaskDone, protocol.TaskBlocked, protocol.TaskStopped:
		return false
	}
	return true
}

// a Teams place from the history, named as what it has open
func (s *uiShell) teamsPlaceAction(snapshot state.Snapshot, place Place) (paletteAction, bool) {
	run := func() { s.router.Push(place) }
	switch {
	case place.Draft != "":
		// a draft left behind is not a place to go back to
		return paletteAction{}, false
	case place.TeamTask != "":
		task, ok := snapshot.TeamTasks[place.TeamTask]
		return paletteAction{icon: look.Icon.Tasks, label: task.Title, kind: teamsui.TeamName(snapshot.Teams, task.Team), run: run}, ok
	case place.Routine != "":
		for _, routine := range snapshot.Routines {
			if routine.ID == place.Routine {
				return paletteAction{icon: look.Icon.Routine, label: routine.Title, kind: i18n.T("Routine"), run: run}, true
			}
		}
		return paletteAction{}, false
	case place.Credential != "":
		for _, credential := range snapshot.Vault.Credentials {
			if credential.ID == place.Credential {
				return paletteAction{icon: look.Icon.Key, label: credential.Name, kind: i18n.T("Credential"), run: run}, true
			}
		}
		return paletteAction{}, false
	case place.Team != "":
		team, ok := teamsui.TeamByID(snapshot.Teams, place.Team)
		return paletteAction{icon: look.Icon.Teams, label: team.Name, kind: i18n.T("Team"), run: run}, ok
	}
	items, count := destinations(true)
	for _, item := range items[:count] {
		if item.tab == place.Tab {
			return paletteAction{icon: item.icon, label: item.label, run: func() { s.openDestination(place.Tab) }}, true
		}
	}
	return paletteAction{}, false
}
