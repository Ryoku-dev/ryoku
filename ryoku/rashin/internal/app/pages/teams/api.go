package teams

import (
	"encoding/json"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/protocol"
)

// a raised limit rounds up to a whole number of these, so it reads at a glance
const tokenStep = 100_000

// a merge patch of plain maps and protocol values, which always marshal
func rawJSON(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

// patch holds the team fields that change, the rest stay as they are
func updateTeam(e *env.Env, id string, patch map[string]any, done func(error)) {
	e.Call(protocol.MTeamsUpdate, protocol.TeamUpdateParams{ID: id, Patch: rawJSON(patch)}, new(protocol.Team), func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
		done(err)
	})
}

func togglePause(e *env.Env, team protocol.Team) {
	params := protocol.TeamPauseParams{ID: team.ID, Paused: !team.Paused}
	if team.Paused && team.PauseReason == protocol.PausedByBudget {
		params.Raise = raisedLimit(team)
	}
	e.Call(protocol.MTeamsPause, params, new(protocol.Team), func(err error) {
		switch {
		case err != nil:
			e.Notify(err.Error())
		case params.Raise > 0:
			e.Notify(i18n.Tf("%s works again with a limit of %s tokens a month.", team.Name, teamsui.Tokens(params.Raise)))
		case params.Paused:
			e.Notify(i18n.Tf("%s is paused. Its running tasks stopped.", team.Name))
		}
	})
}

// a pause at the limit lifts only above this month's use, so the limit grows by half the way the Inbox card raises it
func raisedLimit(team protocol.Team) int64 {
	base := max(team.Budget.MonthTokens, team.Stats.MonthTokens)
	raised := roundTokens(base + (base+1)/2)
	return max(raised, roundTokens(team.Stats.MonthTokens+1))
}

func roundTokens(tokens int64) int64 {
	return (tokens + tokenStep - 1) / tokenStep * tokenStep
}
