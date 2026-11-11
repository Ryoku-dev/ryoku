package activity

import (
	"time"

	"ryoku-i18n"
)

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
