package routines

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"ryoku-i18n"
)

// the shapes of schedule the builder makes; anything else is written by hand
type preset int

const (
	everyHour preset = iota
	everyDay
	everyWeekday
	everyWeek
	everyMonth
	custom
)

func presetLabels() []string {
	return []string{
		i18n.T("Every hour"), i18n.T("Every day"), i18n.T("Every weekday"),
		i18n.T("Every week"), i18n.T("Every month"), i18n.T("Custom"),
	}
}

// the week as the chips show it, Monday first, with each day's number in a schedule
var weekdays = [7]int{1, 2, 3, 4, 5, 6, 0}

func weekdayLabels() [7]string {
	return [7]string{
		i18n.T("Mon"), i18n.T("Tue"), i18n.T("Wed"), i18n.T("Thu"),
		i18n.T("Fri"), i18n.T("Sat"), i18n.T("Sun"),
	}
}

// what the builder holds: a preset and the few numbers it needs
type plan struct {
	preset preset
	minute int
	hour   int
	// days are Monday to Sunday, as the chips stand
	days [7]bool
	// day is the day of the month, 1 to 28 so every month has it
	day    int
	custom string
}

func defaultPlan() plan {
	return plan{preset: everyWeekday, hour: 9, days: [7]bool{true}, day: 1}
}

// the five-field schedule the plan makes, or why it makes none
func (p plan) expression() (string, string) {
	switch p.preset {
	case everyHour:
		return fmt.Sprintf("%d * * * *", p.minute), ""
	case everyDay:
		return fmt.Sprintf("%d %d * * *", p.minute, p.hour), ""
	case everyWeekday:
		return fmt.Sprintf("%d %d * * 1-5", p.minute, p.hour), ""
	case everyWeek:
		var numbers []int
		for index, on := range p.days {
			if on {
				numbers = append(numbers, weekdays[index])
			}
		}
		if len(numbers) == 0 {
			return "", i18n.T("Pick at least one day.")
		}
		sort.Ints(numbers)
		words := make([]string, len(numbers))
		for index, value := range numbers {
			words[index] = strconv.Itoa(value)
		}
		return fmt.Sprintf("%d %d * * %s", p.minute, p.hour, strings.Join(words, ",")), ""
	case everyMonth:
		return fmt.Sprintf("%d %d %d * *", p.minute, p.hour, p.day), ""
	}
	expression := strings.Join(strings.Fields(p.custom), " ")
	if expression == "" {
		return "", i18n.T("Write a schedule, such as 30 8 * * 1-5.")
	}
	return expression, ""
}

// a schedule no preset makes exactly, such as days out of order, opens as custom
func readPlan(expression string) plan {
	read := readPreset(expression)
	if made, _ := read.expression(); made != read.custom {
		read.preset = custom
	}
	return read
}

func readPreset(expression string) plan {
	read := defaultPlan()
	read.custom = strings.Join(strings.Fields(expression), " ")
	fields := strings.Fields(expression)
	asCustom := read
	asCustom.preset = custom
	if len(fields) != 5 || fields[3] != "*" {
		return asCustom
	}
	minute, ok := number(fields[0], 0, 59)
	if !ok {
		return asCustom
	}
	read.minute = minute
	hourField, day, weekday := fields[1], fields[2], fields[4]
	if hourField == "*" {
		if day == "*" && weekday == "*" {
			read.preset = everyHour
			return read
		}
		return asCustom
	}
	hour, ok := number(hourField, 0, 23)
	if !ok {
		return asCustom
	}
	read.hour = hour
	switch {
	case day == "*" && weekday == "*":
		read.preset = everyDay
	case day == "*" && weekday == "1-5":
		read.preset = everyWeekday
	case day == "*":
		days, ok := weekdayList(weekday)
		if !ok {
			return asCustom
		}
		read.preset, read.days = everyWeek, days
	case weekday == "*":
		date, ok := number(day, 1, 28)
		if !ok {
			return asCustom
		}
		read.preset, read.day = everyMonth, date
	default:
		return asCustom
	}
	return read
}

// a plain number in range, written the way the builder writes it
func number(field string, low, high int) (int, bool) {
	value, err := strconv.Atoi(field)
	if err != nil || value < low || value > high || strconv.Itoa(value) != field {
		return 0, false
	}
	return value, true
}

// a list of day numbers, 0 and 7 both Sunday, as the week preset writes it
func weekdayList(field string) ([7]bool, bool) {
	var days [7]bool
	for _, part := range strings.Split(field, ",") {
		value, ok := number(part, 0, 7)
		if !ok {
			return days, false
		}
		if value == 7 {
			value = 0
		}
		for index, weekday := range weekdays {
			if weekday == value {
				days[index] = true
			}
		}
	}
	return days, true
}

// a time of day as the field shows it
func clockText(hour, minute int) string { return fmt.Sprintf("%02d:%02d", hour, minute) }

// reads H:MM or HH:MM
func readClock(text string) (hour, minute int, ok bool) {
	before, after, found := strings.Cut(strings.TrimSpace(text), ":")
	if !found || len(before) == 0 || len(before) > 2 || len(after) != 2 {
		return 0, 0, false
	}
	hour, errHour := strconv.Atoi(before)
	minute, errMinute := strconv.Atoi(after)
	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, false
	}
	return hour, minute, true
}
