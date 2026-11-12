package projects

import (
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

const (
	commitDays = 30
	// a page of the graph, read until a page holds nothing inside the month
	commitPage = 500
)

// commits per day across every branch, read once for the project and again after Git changes
type commitTrend struct {
	trend look.Trend
	// the project the trend counts, kept on screen while a refresh after a commit is read
	of      string
	loaded  string
	loading bool
}

func (c *commitTrend) load(e *env.Env, project string, now time.Time) {
	if c.loaded == project || c.loading {
		return
	}
	c.loading = true
	days := look.LastDays(now.Local(), commitDays)
	byDay := make(map[string]float64, commitDays)
	var read func(cursor string)
	read = func(cursor string) {
		var result protocol.GitLogResult
		e.Call(protocol.MGitLog, protocol.GitLogParams{Project: project, Limit: commitPage, Cursor: cursor}, &result, func(err error) {
			if err != nil {
				// marked read so a history that cannot be read is said once, not asked for every frame
				c.loading, c.loaded, c.of = false, project, project
				c.trend = look.Trend{Title: i18n.T("Commits"), Empty: i18n.T("The commit history could not be read.")}
				return
			}
			inside := false
			for _, commit := range result.Commits {
				if day := commit.Time.Local().Format("2006-01-02"); day >= days[0] {
					byDay[day]++
					inside = true
				}
			}
			if inside && result.Next != "" {
				read(result.Next)
				return
			}
			c.loading, c.loaded, c.of, c.trend = false, project, project, commitsTrend(days, byDay)
		})
	}
	read("")
}

// the trend of the project shown, or false before its first reading, with its title so the place it holds is close to its size
func (c *commitTrend) shown(project string) (look.Trend, bool) {
	if c.of != project {
		return look.Trend{Title: i18n.T("Commits")}, false
	}
	return c.trend, true
}

func commitsTrend(days []string, byDay map[string]float64) look.Trend {
	trend := look.Trend{
		Title: i18n.T("Commits"), Ticks: look.DayTicks(days),
		Empty: i18n.Tf("No commits in the last %d days.", commitDays),
	}
	values := look.DayValues(days, byDay)
	total := 0
	for _, value := range values {
		total += int(value)
	}
	if total == 0 {
		return trend
	}
	trend.Values = values
	day, most := look.Peak(days, values)
	switch {
	case total == 1:
		trend.Proof = i18n.Tf("One commit in %d days, on %s", commitDays, look.DayName(day))
	case most == 1:
		trend.Proof = i18n.Tf("%d commits in %d days across every branch, never two on one day", total, commitDays)
	default:
		trend.Proof = i18n.Tf("%d commits in %d days across every branch; the busiest day was %s with %d", total, commitDays, look.DayName(day), int(most))
	}
	return trend
}
