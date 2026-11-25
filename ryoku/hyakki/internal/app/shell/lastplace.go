package shell

import (
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// an agent session, a project view or a Teams place is worth coming back to, a question is one exchange so the app opens on a new one
func lastPlace(place Place) state.LastPlace {
	switch {
	case place.Tab == protocol.TabAgent:
		return state.LastPlace{Tab: place.Tab, Session: place.Session}
	case place.Tab == protocol.TabProjects:
		return state.LastPlace{Tab: place.Tab, Project: place.Project, View: place.ProjectView, Worktree: place.Worktree}
	case teamsTab(place.Tab) && place.Draft != "":
		// a draft is not kept, the destination it was started in is
		return state.LastPlace{Tab: place.Tab}
	case teamsTab(place.Tab):
		return state.LastPlace{
			Tab: place.Tab, Team: place.Team, View: place.TeamView, Task: place.TeamTask, Routine: place.Routine, Credential: place.Credential,
		}
	}
	return state.LastPlace{}
}

// the place a remembered one stands for, its view belongs to a team page in Teams and to a project elsewhere
func restoredPlace(last state.LastPlace) Place {
	place := Place{
		Tab: last.Tab, Session: last.Session, Project: last.Project, Worktree: last.Worktree,
		Team: last.Team, TeamTask: last.Task, Routine: last.Routine, Credential: last.Credential,
	}
	if teamsTab(last.Tab) {
		place.TeamView = last.View
	} else {
		place.ProjectView = last.View
	}
	return place
}

// a launch that named nothing opens where the window was closed, in the mode it was in
func resume(options Options) Options {
	last := options.Preferences.Last
	if options.Preferences.InTeams {
		last = options.Preferences.Teams
	}
	named := options.Route != "" || options.Session != "" || options.Project != "" || options.SettingsSection != "" || options.Teams.Tab != ""
	if named || last.Tab == "" {
		return options
	}
	if teamsTab(last.Tab) {
		options.Teams = restoredPlace(last).teams()
		return options
	}
	options.Route, options.Session = last.Tab, last.Session
	options.Project, options.WorkTab, options.Worktree = last.Project, last.View, last.Worktree
	return options
}
