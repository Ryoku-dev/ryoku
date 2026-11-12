package teamsui

import (
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// tokens a day, its proof the month's total and its busiest day
func TokensTrend(now time.Time, spend protocol.TeamSpend, title string) look.Trend {
	days, values := spendDays(now, spend, func(day protocol.DaySpend) int { return int(day.Input + day.Output) })
	trend := look.Trend{
		Title: title, Values: values, Ticks: look.DayTicks(days),
		Empty: i18n.T("A line of daily tokens shows from the second day of the month."),
	}
	peak, top := look.Peak(days, values)
	if peak == "" {
		trend.Proof = i18n.T("No tokens spent this month yet.")
		return trend
	}
	trend.Proof = i18n.Tf("%s tokens this month; the busiest day was %s at %s.", Tokens(spend.Tokens), look.DayName(peak), Tokens(int64(top)))
	return trend
}

// tasks that ended each day, its proof how many, how many were routine runs and the busiest day
func FinishedTrend(now time.Time, spend protocol.TeamSpend, title string) look.Trend {
	days, values := spendDays(now, spend, func(day protocol.DaySpend) int { return day.Tasks })
	tasks, routines := 0, 0
	for _, day := range spend.Days {
		tasks += day.Tasks
		routines += day.Routines
	}
	trend := look.Trend{
		Title: title, Values: values, Ticks: look.DayTicks(days),
		Empty: i18n.T("A line of finished tasks shows from the second day of the month."),
	}
	peak, top := look.Peak(days, values)
	switch {
	case peak == "":
		trend.Proof = i18n.T("No task has finished this month yet.")
	case tasks == 1 && routines == 1:
		trend.Proof = i18n.Tf("One routine run finished this month, on %s.", look.DayName(peak))
	case tasks == 1:
		trend.Proof = i18n.Tf("One task finished this month, on %s.", look.DayName(peak))
	case routines == 0:
		trend.Proof = i18n.Tf("%d tasks finished this month; the busiest day was %s with %d.", tasks, look.DayName(peak), int(top))
	case routines == 1:
		trend.Proof = i18n.Tf("%d tasks finished this month, one of them a routine run; the busiest day was %s with %d.", tasks, look.DayName(peak), int(top))
	default:
		trend.Proof = i18n.Tf("%d tasks finished this month, %d of them routine runs; the busiest day was %s with %d.", tasks, routines, look.DayName(peak), int(top))
	}
	return trend
}

// routine runs that ended each day, its proof how many and the busiest day
func RunsTrend(now time.Time, spend protocol.TeamSpend, title string) look.Trend {
	days, values := spendDays(now, spend, func(day protocol.DaySpend) int { return day.Routines })
	runs := 0
	for _, day := range spend.Days {
		runs += day.Routines
	}
	trend := look.Trend{
		Title: title, Values: values, Ticks: look.DayTicks(days),
		Empty: i18n.T("A line of runs shows from the second day of the month."),
	}
	peak, top := look.Peak(days, values)
	switch {
	case peak == "":
		trend.Proof = i18n.T("No routine has run this month yet.")
	case runs == 1:
		trend.Proof = i18n.Tf("One routine run this month, on %s.", look.DayName(peak))
	default:
		trend.Proof = i18n.Tf("%d routine runs this month; the busiest day was %s with %d.", runs, look.DayName(peak), int(top))
	}
	return trend
}
