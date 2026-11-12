package dreams

import (
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

// accepting ends the dream, notes send its manager back to work with them
func answer(e *env.Env, dream protocol.Team, verdict, note string, done func(error)) {
	params := protocol.DreamAnswerParams{Team: dream.ID, Verdict: verdict, Note: note}
	e.Call(protocol.MTeamsDreamAnswer, params, new(protocol.Team), func(err error) {
		switch {
		case err != nil:
			e.Notify(err.Error())
		case verdict == protocol.DreamAccepted:
			e.Notify(i18n.Tf("%s is done.", dream.Name))
		default:
			e.Notify(i18n.Tf("The manager of %s has your notes and goes on.", dream.Name))
		}
		done(err)
	})
}

// resuming a dream also starts its manager again when it was not running
func setPaused(e *env.Env, dream protocol.Team, paused bool) {
	e.Call(protocol.MTeamsPause, protocol.TeamPauseParams{ID: dream.ID, Paused: paused}, new(protocol.Team), func(err error) {
		switch {
		case err != nil:
			e.Notify(err.Error())
		case paused:
			e.Notify(i18n.Tf("%s is paused. Its team stopped.", dream.Name))
		default:
			e.Notify(i18n.Tf("%s goes on.", dream.Name))
		}
	})
}
