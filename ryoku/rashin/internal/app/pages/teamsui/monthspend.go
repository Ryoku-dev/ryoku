package teamsui

import (
	"fmt"
	"strings"
	"time"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a team's month, or every team's, as teams.spend tells it, read again only when a task could have ended or the day turned
type MonthSpend struct {
	// empty for every team
	Team string

	key    string
	busy   bool
	loaded bool
	// the team the month read last belongs to, so a page never shows another team's month
	of    string
	spend protocol.TeamSpend
}

// a failed read is told once and waits for the next change, not asked every frame
func (m *MonthSpend) Want(e *env.Env, now time.Time) {
	snapshot := e.Snapshot()
	if snapshot.Connection != state.Ready || m.busy {
		return
	}
	key := monthSpendKey(snapshot, now, m.Team)
	if key == m.key {
		return
	}
	m.key, m.busy = key, true
	team := m.Team
	var result protocol.TeamSpend
	e.Call(protocol.MTeamsSpend, protocol.TeamSpendParams{Team: team}, &result, func(err error) {
		m.busy = false
		if err != nil {
			e.Notify(err.Error())
			return
		}
		m.spend, m.loaded, m.of = result, true, team
	})
}

// a chart never shows an empty month it has not read yet
func (m *MonthSpend) Loaded() bool { return m.loaded && m.of == m.Team }

func (m *MonthSpend) Spend() protocol.TeamSpend { return m.spend }

// one point per day from the first to today, zero on a day the daemon has none of
func spendDays(now time.Time, spend protocol.TeamSpend, pick func(protocol.DaySpend) int) ([]string, []float64) {
	days := look.MonthDays(now, spend.Month)
	byDay := make(map[string]float64, len(spend.Days))
	for _, day := range spend.Days {
		byDay[day.Day] = float64(pick(day))
	}
	return days, look.DayValues(days, byDay)
}

// a task ending moves some team's counts, and a new day starts a new point
func monthSpendKey(snapshot state.Snapshot, now time.Time, only string) string {
	var key strings.Builder
	fmt.Fprintf(&key, "%d/%s/%s", snapshot.Epoch, now.Local().Format("2006-01-02"), only)
	for _, team := range snapshot.Teams {
		if only != "" && team.ID != only {
			continue
		}
		stats := team.Stats
		fmt.Fprintf(&key, "/%s:%d:%d:%d:%d:%d:%d", team.ID, stats.Working, stats.Queued, stats.NeedsYou, stats.Done, stats.Tasks, stats.MonthTokens)
	}
	return key.String()
}
