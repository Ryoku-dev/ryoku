package shell

import (
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// an agent session or a project view is worth coming back to, a question is one exchange so the app opens on a new one
func lastPlace(place Place) state.LastPlace {
	switch place.Tab {
	case protocol.TabAgent:
		return state.LastPlace{Tab: place.Tab, Session: place.Session}
	case protocol.TabProjects:
		return state.LastPlace{Tab: place.Tab, Project: place.Project, View: place.ProjectView, Worktree: place.Worktree}
	}
	return state.LastPlace{}
}

// a launch that named nothing opens where the window was closed
func resume(options Options) Options {
	last := options.Preferences.Last
	if options.Route != "" || options.Session != "" || options.Project != "" || options.SettingsSection != "" || last.Tab == "" {
		return options
	}
	options.Route, options.Session = last.Tab, last.Session
	options.Project, options.WorkTab, options.Worktree = last.Project, last.View, last.Worktree
	return options
}
