package remind

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"
)

// spanWords says an amount of time in its largest whole unit: "5 minutes",
// "90 minutes", "1 hour", "45 seconds".
func spanWords(d time.Duration) string {
	n, unit := int64(d/time.Second), "second"
	switch {
	case d >= time.Hour && d%time.Hour == 0:
		n, unit = int64(d/time.Hour), "hour"
	case d >= time.Minute && d%time.Minute == 0:
		n, unit = int64(d/time.Minute), "minute"
	}
	if n != 1 {
		unit += "s"
	}
	return fmt.Sprintf("%d %s", n, i18n.T(unit))
}

// clockWords says a time of day on the 12-hour clock: "9 AM", "4:30 PM",
// "noon", "midnight".
func clockWords(t time.Time) string {
	h, m := t.Hour(), t.Minute()
	switch {
	case h == 12 && m == 0:
		return i18n.T("noon")
	case h == 0 && m == 0:
		return i18n.T("midnight")
	}
	half := "AM"
	if h >= 12 {
		half = "PM"
	}
	if h %= 12; h == 0 {
		h = 12
	}
	if m == 0 {
		return fmt.Sprintf("%d %s", h, half)
	}
	return fmt.Sprintf("%d:%02d %s", h, m, half)
}

// civil is t's calendar date as a count of days.
func civil(t time.Time) int {
	y, m, d := t.Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

// whenWords says when due falls, seen from now in loc: "4:30 PM" today,
// "tomorrow 9 AM", "Friday noon", "next Monday 8 AM" a week on. Midnight
// belongs to the day it ends: "tomorrow midnight" is the midnight after
// tomorrow.
func whenWords(due, now time.Time, loc *time.Location) string {
	d, n := due.In(loc), now.In(loc)
	day := d
	if d.Hour() == 0 && d.Minute() == 0 {
		day = date(d, -1, loc)
	}
	clock := clockWords(d)
	switch off := civil(day) - civil(n); {
	case off <= 0:
		return clock
	case off == 1:
		return i18n.T("tomorrow") + " " + clock
	case off < 7:
		return i18n.T(day.Weekday().String()) + " " + clock
	default:
		return i18n.T("next") + " " + i18n.T(day.Weekday().String()) + " " + clock
	}
}

// sentence makes a reminder's own words a sentence of their own.
func sentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	s = string(unicode.ToUpper(r)) + s[n:]
	if !strings.ContainsAny(s[len(s)-1:], ".!?") {
		s += "."
	}
	return s
}

// FireLine is what Oni says when r fires at at: a short lead, marked when
// the fire is late, and then the reminder's own words.
func FireLine(r Reminder, at time.Time) string {
	late := r.Late(at)
	var lead string
	switch {
	case r.Kind == KindTimer && late:
		lead = i18n.T("Your timer ended earlier.")
	case r.Kind == KindTimer:
		lead = i18n.T("Time's up.")
	case late:
		lead = i18n.T("Reminder, a bit late.")
	default:
		lead = i18n.T("Reminder.")
	}
	if r.Text == "" {
		return lead
	}
	return lead + " " + sentence(r.Text)
}

// maxListed is how many reminders ListLine reads out.
const maxListed = 5

// ListLine says the pending reminders and timers rs, soonest first, as of
// now in loc: a count, then each one's words and when it falls.
func ListLine(rs []Reminder, now time.Time, loc *time.Location) string {
	if loc == nil {
		loc = now.Location()
	}
	if len(rs) == 0 {
		return i18n.T("No reminders or timers.")
	}
	reminders, timers := 0, 0
	for _, r := range rs {
		if r.Kind == KindTimer {
			timers++
		} else {
			reminders++
		}
	}
	var counts []string
	if reminders > 0 {
		counts = append(counts, plural(reminders, "reminder"))
	}
	if timers > 0 {
		counts = append(counts, plural(timers, "timer"))
	}
	out := []string{sentence(strings.Join(counts, i18n.T(" and ")))}
	for i, r := range rs {
		if i == maxListed {
			out = append(out, i18n.Tf("And %d more.", len(rs)-maxListed))
			break
		}
		out = append(out, item(r, now, loc))
	}
	return strings.Join(out, " ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return i18n.Tf("1 %s", i18n.T(noun))
	}
	return i18n.Tf("%d %ss", n, i18n.T(noun))
}

func item(r Reminder, now time.Time, loc *time.Location) string {
	label := strings.TrimSpace(r.Text)
	if label == "" {
		label = i18n.T("Reminder")
		if r.Kind == KindTimer {
			label = i18n.T("Timer")
		}
	}
	left := r.Due.Sub(now)
	switch {
	case left <= 0:
		return sentence(i18n.Tf("%s, overdue", label))
	case r.Kind == KindTimer:
		return sentence(i18n.Tf("%s, %s left", label, spanWords(roundUp(left))))
	}
	return sentence(i18n.Tf("%s, %s", label, whenWords(r.Due, now, loc)))
}

// roundUp rounds time left up to whole minutes, or whole seconds under a
// minute.
func roundUp(d time.Duration) time.Duration {
	unit := time.Minute
	if d < time.Minute {
		unit = time.Second
	}
	if r := d % unit; r != 0 {
		d += unit - r
	}
	return d
}
