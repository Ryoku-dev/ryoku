package teamsui

import (
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

// a task opens on its page in Work
func OpenTask(e *env.Env, id string) {
	e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabWork, Task: id})
}

// a routine opens in the Routines editor
func OpenRoutine(e *env.Env, id string) {
	e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabRoutines, Routine: id})
}
