package look

import (
	"time"

	"ryoku-i18n"
)

// how long ago something happened, in the words a list or a header shows
func Ago(now, when time.Time) string {
	elapsed := now.Sub(when)
	switch {
	case when.IsZero():
		return ""
	case elapsed < time.Minute:
		return i18n.T("just now")
	case elapsed < time.Hour:
		return i18n.Tf("%d min ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return i18n.Tf("%d h ago", int(elapsed.Hours()))
	default:
		return i18n.Tf("%d d ago", int(elapsed.Hours()/24))
	}
}

// how soon something will happen: minutes and hours while it is near, then the
// day and the time
func Until(now, when time.Time) string {
	ahead := when.Sub(now)
	switch {
	case when.IsZero():
		return ""
	case ahead < time.Minute:
		return i18n.T("now")
	case ahead < time.Hour:
		return i18n.Tf("in %d min", int(ahead.Minutes()))
	case ahead < 12*time.Hour:
		return i18n.Tf("in %d h", int(ahead.Hours()))
	}
	return When(now, when)
}

// a moment as a day and a time: today, tomorrow, a weekday this week, else a date
func When(now, at time.Time) string {
	local, today := at.Local(), now.Local()
	clock := local.Format("15:04")
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local) }
	days := int(day(local).Sub(day(today)).Hours() / 24)
	switch {
	case days == 0:
		return i18n.Tf("today %s", clock)
	case days == 1:
		return i18n.Tf("tomorrow %s", clock)
	case days == -1:
		return i18n.Tf("yesterday %s", clock)
	case days > 1 && days < 7:
		return i18n.Tf("%s %s", i18n.T(local.Weekday().String()), clock)
	}
	return i18n.Tf("%s %s", local.Format("2 Jan"), clock)
}
