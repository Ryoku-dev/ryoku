package routines

import (
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

func openBoard(e *env.Env) {
	e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabRoutines})
}

func deleteRoutine(e *env.Env, id string, done func(error)) {
	e.Call(protocol.MTeamsRoutineDelete, protocol.RoutineRef{ID: id}, new(protocol.Empty), done)
}
