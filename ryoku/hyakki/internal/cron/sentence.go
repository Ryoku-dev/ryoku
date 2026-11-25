package cron

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
)

// clockTimesMax is how many times of day a sentence names one by one before
// it says them as a repeat across hours.
const clockTimesMax = 8

// dayRule is which dates a schedule runs on, as a sentence says it: doms nil
// means any day of the month, dows nil any day of the week. Either marks the
// cron day rule's "or" when both day fields are restricted.
type dayRule struct {
	doms, dows []int
	either     bool
}

func (s *Schedule) dayRule() dayRule {
	doms, dows := values(s.dom), values(s.dow)
	anyDom, anyDow := len(doms) == 31, len(dows) == 7
	if !s.domStar && !s.dowStar {
		if anyDom || anyDow {
			return dayRule{}
		}
		return dayRule{doms: doms, dows: dows, either: true}
	}
	var rule dayRule
	if !anyDom {
		rule.doms = doms
	}
	if !anyDow {
		rule.dows = dows
	}
	return rule
}

// Sentence says when the schedule runs in plain words: "Every weekday at
// 09:00", "Every week on Mon and Thu at 08:00", "At 09:15 and 17:15 on
// weekdays in March".
func (s *Schedule) Sentence() string {
	minutes, hours, months := values(s.minute), values(s.hour), values(s.month)
	rule := s.dayRule()
	explicit := len(hours) < 24 && len(hours)*len(minutes) <= clockTimesMax
	if explicit && len(months) == 12 && !rule.either {
		times := clockList(hours, minutes)
		switch {
		case rule.doms == nil && rule.dows == nil:
			return i18n.Tf("Every day at %s", times)
		case rule.doms == nil && isWeekdays(rule.dows):
			return i18n.Tf("Every weekday at %s", times)
		case rule.doms == nil && len(rule.dows) == 1:
			return i18n.Tf("Every %s at %s", dayName(rule.dows[0]), times)
		case rule.doms == nil:
			return i18n.Tf("Every week on %s at %s", dayList(rule.dows), times)
		case rule.dows == nil && len(rule.doms) == 1:
			return i18n.Tf("On day %d of every month at %s", rule.doms[0], times)
		case rule.dows == nil:
			return i18n.Tf("On days %s of every month at %s", numberList(rule.doms), times)
		}
	}
	times := timesPhrase(hours, minutes, explicit)
	day := dayPhrase(rule, explicit)
	month := ""
	if len(months) < 12 {
		month = monthList(months)
	}
	switch {
	case day == "" && month == "":
		return times
	case month == "":
		return i18n.Tf("%s %s", times, day)
	case day == "":
		return i18n.Tf("%s in %s", times, month)
	}
	return i18n.Tf("%s %s in %s", times, day, month)
}

// timesPhrase says the times of day a schedule runs at. Explicit names
// each time; otherwise the runs repeat through the hours.
func timesPhrase(hours, minutes []int, explicit bool) string {
	every := len(minutes) == 60
	step := minuteStep(minutes)
	if len(hours) == 24 {
		switch {
		case every:
			return i18n.T("Every minute")
		case step > 0:
			return i18n.Tf("Every %d minutes", step)
		}
		return i18n.Tf("Every hour at %s", pastList(minutes))
	}
	if explicit {
		return i18n.Tf("At %s", clockList(hours, minutes))
	}
	spans := spanList(hours, minutes[0], minutes[len(minutes)-1])
	switch {
	case every:
		return i18n.Tf("Every minute from %s", spans)
	case step > 0:
		return i18n.Tf("Every %d minutes from %s", step, spans)
	}
	return i18n.Tf("At %s past each hour from %s", pastList(minutes), spans)
}

// dayPhrase says the dates a schedule runs on; empty when it runs every day
// and the times already say so.
func dayPhrase(rule dayRule, explicit bool) string {
	switch {
	case rule.either:
		return i18n.Tf("on day %s of the month or on %s", numberList(rule.doms), dayList(rule.dows))
	case rule.doms != nil && rule.dows != nil:
		return i18n.Tf("on day %s of the month when it falls on %s", numberList(rule.doms), dayList(rule.dows))
	case rule.doms != nil:
		return i18n.Tf("on day %s of the month", numberList(rule.doms))
	case isWeekdays(rule.dows):
		return i18n.T("on weekdays")
	case slices.Equal(rule.dows, []int{0, 6}):
		return i18n.T("on weekends")
	case rule.dows != nil:
		return i18n.Tf("on %s", dayList(rule.dows))
	case explicit:
		return i18n.T("every day")
	}
	return ""
}

// minuteStep is S when the minutes are exactly 0, S, 2S and so on through
// the hour, else 0.
func minuteStep(minutes []int) int {
	if len(minutes) < 2 || minutes[0] != 0 {
		return 0
	}
	step := minutes[1]
	if step == 1 || 60%step != 0 || len(minutes) != 60/step {
		return 0
	}
	for i, m := range minutes {
		if m != i*step {
			return 0
		}
	}
	return step
}

func isWeekdays(dows []int) bool { return slices.Equal(dows, []int{1, 2, 3, 4, 5}) }

func clock(hour, minute int) string { return fmt.Sprintf("%02d:%02d", hour, minute) }

func clockList(hours, minutes []int) string {
	var items []string
	for _, h := range hours {
		for _, m := range minutes {
			items = append(items, clock(h, m))
		}
	}
	return join(items)
}

func pastList(minutes []int) string {
	items := make([]string, len(minutes))
	for i, m := range minutes {
		items[i] = fmt.Sprintf(":%02d", m)
	}
	return join(items)
}

// spanList says runs of consecutive hours as "09:00 to 17:30", from the
// first run in the run's first hour to the last in its last hour.
func spanList(hours []int, first, last int) string {
	var items []string
	for i := 0; i < len(hours); {
		j := i
		for j+1 < len(hours) && hours[j+1] == hours[j]+1 {
			j++
		}
		items = append(items, i18n.Tf("%s to %s", clock(hours[i], first), clock(hours[j], last)))
		i = j + 1
	}
	return join(items)
}

func numberList(numbers []int) string {
	items := make([]string, len(numbers))
	for i, n := range numbers {
		items[i] = strconv.Itoa(n)
	}
	return join(items)
}

// dayList names days of the week in short, Monday first.
func dayList(dows []int) string {
	ordered := slices.Clone(dows)
	slices.SortFunc(ordered, func(a, b int) int { return (a+6)%7 - (b+6)%7 })
	items := make([]string, len(ordered))
	for i, d := range ordered {
		items[i] = dayShort(d)
	}
	return join(items)
}

func monthList(months []int) string {
	items := make([]string, len(months))
	for i, m := range months {
		items[i] = monthName(m)
	}
	return join(items)
}

// join lists items as "a, b and c".
func join(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return i18n.Tf("%s and %s", strings.Join(items[:len(items)-1], ", "), items[len(items)-1])
}

func dayName(d int) string {
	switch d {
	case 1:
		return i18n.T("Monday")
	case 2:
		return i18n.T("Tuesday")
	case 3:
		return i18n.T("Wednesday")
	case 4:
		return i18n.T("Thursday")
	case 5:
		return i18n.T("Friday")
	case 6:
		return i18n.T("Saturday")
	}
	return i18n.T("Sunday")
}

func dayShort(d int) string {
	switch d {
	case 1:
		return i18n.T("Mon")
	case 2:
		return i18n.T("Tue")
	case 3:
		return i18n.T("Wed")
	case 4:
		return i18n.T("Thu")
	case 5:
		return i18n.T("Fri")
	case 6:
		return i18n.T("Sat")
	}
	return i18n.T("Sun")
}

func monthName(m int) string {
	switch m {
	case 1:
		return i18n.T("January")
	case 2:
		return i18n.T("February")
	case 3:
		return i18n.T("March")
	case 4:
		return i18n.T("April")
	case 5:
		return i18n.T("May")
	case 6:
		return i18n.T("June")
	case 7:
		return i18n.T("July")
	case 8:
		return i18n.T("August")
	case 9:
		return i18n.T("September")
	case 10:
		return i18n.T("October")
	case 11:
		return i18n.T("November")
	}
	return i18n.T("December")
}
