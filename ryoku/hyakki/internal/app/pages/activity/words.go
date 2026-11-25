package activity

import (
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the page's claim counts today's entries in view, and its proof what of them wants a look
func ledgerLead(now time.Time, entries []protocol.Activity) (claim, proof string) {
	today, unwatched, failed := 0, 0, 0
	for _, entry := range entries {
		if local, at := now.Local(), entry.Time.Local(); local.Year() != at.Year() || local.YearDay() != at.YearDay() {
			continue
		}
		today++
		if entry.Unwatched {
			unwatched++
		}
		if !entry.OK {
			failed++
		}
	}
	switch today {
	case 0:
		claim = i18n.T("Nothing new today.")
	case 1:
		claim = i18n.T("One entry today.")
	default:
		claim = i18n.Tf("%s entries today.", look.CountStart(today))
	}
	var sentences []string
	switch {
	case unwatched == 1:
		sentences = append(sentences, i18n.T("One happened with nobody watching."))
	case unwatched > 1:
		sentences = append(sentences, i18n.Tf("%d happened with nobody watching.", unwatched))
	}
	switch {
	case failed == 1:
		sentences = append(sentences, i18n.T("One failed."))
	case failed > 1:
		sentences = append(sentences, i18n.Tf("%d failed.", failed))
	}
	if len(sentences) == 0 {
		return claim, i18n.T("Every task, report, approval and use of a credential is recorded here, newest first.")
	}
	return claim, strings.Join(sentences, " ")
}

// the day an entry belongs to, as its group is titled
func dayTitle(now, at time.Time) string {
	local, today := at.Local(), now.Local()
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local) }
	switch int(day(today).Sub(day(local)).Hours() / 24) {
	case 0:
		return i18n.T("Today")
	case 1:
		return i18n.T("Yesterday")
	}
	if local.Year() != today.Year() {
		return i18n.Tf("%s %s", i18n.T(local.Weekday().String()), local.Format("2 Jan 2006"))
	}
	return i18n.Tf("%s %s", i18n.T(local.Weekday().String()), local.Format("2 Jan"))
}
