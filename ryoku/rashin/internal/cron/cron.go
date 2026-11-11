// Package cron reads the five-field schedules routines run on (minute, hour,
// day of the month, month, day of the week), finds when they next fire in
// the computer's time zone and says them in plain words.
package cron

import (
	"errors"
	"math/bits"
	"strconv"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// Preview counts: how many next runs a preview lists unless asked, and at
// most.
const (
	PreviewCount = 5
	PreviewMax   = 20
)

// searchYears bounds the search for a next run. A schedule that parses runs
// at least once in any eight years (29 February is the rarest day).
const searchYears = 9

// Schedule is a parsed five-field schedule. Each field is a bit set of the
// values it matches.
type Schedule struct {
	minute, hour, dom, month, dow uint64
	// domStar and dowStar record a day field written with a leading "*". The
	// standard cron day rule matches either day field when both are
	// restricted, and both when one of them starts with "*".
	domStar, dowStar bool
}

var presets = map[string]string{
	"@hourly":  "0 * * * *",
	"@daily":   "0 0 * * *",
	"@weekly":  "0 0 * * 0",
	"@monthly": "0 0 1 * *",
}

// field describes one of the five positions of a schedule.
type field struct {
	min, max int
	names    map[string]int
	invalid  func() string
}

var (
	minuteField = field{min: 0, max: 59, invalid: func() string { return i18n.T("The minute must be between 0 and 59.") }}
	hourField   = field{min: 0, max: 23, invalid: func() string { return i18n.T("The hour must be between 0 and 23.") }}
	domField    = field{min: 1, max: 31, invalid: func() string { return i18n.T("The day of the month must be between 1 and 31.") }}
	monthField  = field{min: 1, max: 12, invalid: func() string { return i18n.T("The month must be between 1 and 12, or jan to dec.") }, names: map[string]int{
		"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
		"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
	}}
	// 7 is Sunday as well as 0; parse folds it onto 0.
	dowField = field{min: 0, max: 7, invalid: func() string { return i18n.T("The day of the week must be between 0 and 7, or sun to sat.") }, names: map[string]int{
		"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
	}}
)

// Parse reads a schedule: five fields separated by spaces, each a "*", a
// number or name, a range "N-M", a step "*/S" or "N-M/S", or a comma list of
// those; or one of @hourly, @daily, @weekly and @monthly. Errors are plain
// sentences the user can read.
func Parse(expr string) (*Schedule, error) {
	text := strings.TrimSpace(expr)
	if strings.HasPrefix(text, "@") {
		preset, ok := presets[strings.ToLower(text)]
		if !ok {
			return nil, errors.New(i18n.T("The shortcuts are @hourly, @daily, @weekly and @monthly."))
		}
		text = preset
	}
	parts := strings.Fields(text)
	if len(parts) != 5 {
		return nil, errors.New(i18n.T("A schedule has five parts: minute, hour, day of the month, month and day of the week."))
	}
	s := &Schedule{
		domStar: strings.HasPrefix(parts[2], "*"),
		dowStar: strings.HasPrefix(parts[4], "*"),
	}
	var err error
	if s.minute, err = minuteField.parse(parts[0]); err != nil {
		return nil, err
	}
	if s.hour, err = hourField.parse(parts[1]); err != nil {
		return nil, err
	}
	if s.dom, err = domField.parse(parts[2]); err != nil {
		return nil, err
	}
	if s.month, err = monthField.parse(parts[3]); err != nil {
		return nil, err
	}
	if s.dow, err = dowField.parse(parts[4]); err != nil {
		return nil, err
	}
	if s.dow&(1<<7) != 0 {
		s.dow = s.dow&^(1<<7) | 1
	}
	if _, ok := s.dayGap(); !ok {
		return nil, errors.New(i18n.T("That day never comes in the months chosen, so this schedule would never run."))
	}
	return s, nil
}

// parse reads one field into its bit set.
func (f field) parse(text string) (uint64, error) {
	var set uint64
	for _, item := range strings.Split(strings.ToLower(text), ",") {
		span, stepText, stepped := strings.Cut(item, "/")
		step := 1
		if stepped {
			n, err := strconv.Atoi(stepText)
			if err != nil || n < 1 {
				return 0, errors.New(i18n.T("A step after / must be a whole number of 1 or more."))
			}
			step = n
		}
		lo, hi := f.min, f.max
		if span != "*" {
			first, last, ranged := strings.Cut(span, "-")
			if !ranged && stepped {
				return 0, errors.New(i18n.T("A step needs * or a range before it, such as */15 or 0-30/5."))
			}
			var err error
			if lo, err = f.value(first); err != nil {
				return 0, err
			}
			hi = lo
			if ranged {
				if hi, err = f.value(last); err != nil {
					return 0, err
				}
				if hi < lo {
					return 0, errors.New(i18n.T("A range goes from the smaller value to the larger, such as 1-5."))
				}
			}
		}
		for v := lo; v <= hi; v += step {
			set |= 1 << v
		}
	}
	return set, nil
}

// value reads one number or name of a field.
func (f field) value(text string) (int, error) {
	if v, ok := f.names[text]; ok {
		return v, nil
	}
	v, err := strconv.Atoi(text)
	if err != nil || v < f.min || v > f.max {
		return 0, errors.New(f.invalid())
	}
	return v, nil
}

func has(set uint64, v int) bool { return set&(1<<v) != 0 }

// dayMatches applies the day rule to a calendar date.
func (s *Schedule) dayMatches(date time.Time) bool {
	dom := has(s.dom, date.Day())
	dow := has(s.dow, int(date.Weekday()))
	if s.domStar || s.dowStar {
		return dom && dow
	}
	return dom || dow
}

// dayGap finds the fewest days between two dates the schedule runs on, and
// whether it runs on any date at all. Twenty-nine years cover every pairing of
// weekdays with dates, leap days included.
func (s *Schedule) dayGap() (int, bool) {
	var prev time.Time
	gap := 0
	end := time.Date(2029, time.January, 1, 0, 0, 0, 0, time.UTC)
	for date := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC); date.Before(end); date = date.AddDate(0, 0, 1) {
		if !has(s.month, int(date.Month())) || !s.dayMatches(date) {
			continue
		}
		if !prev.IsZero() {
			days := int(date.Sub(prev).Hours()) / 24
			if gap == 0 || days < gap {
				gap = days
			}
			if gap == 1 {
				break
			}
		}
		prev = date
	}
	return gap, !prev.IsZero()
}

// Next returns the first time after after that the schedule fires on this
// computer's clock (time.Local), whatever location after carries: a time read
// back from a file holds a fixed offset that daylight saving would leave
// wrong. A wall time a daylight saving change skips fires at the first minute
// that exists after it; a wall time that happens twice fires only the first
// time.
func (s *Schedule) Next(after time.Time) time.Time {
	loc := time.Local
	after = after.In(loc)
	c := time.Date(after.Year(), after.Month(), after.Day(), after.Hour(), after.Minute(), 0, 0, time.UTC).Add(time.Minute)
	limit := c.AddDate(searchYears, 0, 0)
	for c.Before(limit) {
		switch {
		case !has(s.month, int(c.Month())):
			c = time.Date(c.Year(), c.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		case !s.dayMatches(c):
			c = time.Date(c.Year(), c.Month(), c.Day()+1, 0, 0, 0, 0, time.UTC)
		case !has(s.hour, c.Hour()):
			c = c.Truncate(time.Hour).Add(time.Hour)
		case !has(s.minute, c.Minute()):
			c = c.Add(time.Minute)
		default:
			if t := instant(c, loc); t.After(after) {
				return t
			}
			c = c.Add(time.Minute)
		}
	}
	return time.Time{}
}

// instant turns a wall time, held as UTC fields, into the moment loc's clock
// first shows it, or the moment a clock change skipping it ends.
func instant(wall time.Time, loc *time.Location) time.Time {
	t := time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), 0, 0, loc)
	shown := civil(t)
	if !shown.Equal(wall) {
		start, end := t.ZoneBounds()
		if shown.Before(wall) {
			return end
		}
		return start
	}
	start, _ := t.ZoneBounds()
	if start.IsZero() {
		return t
	}
	// the hour a clock goes back repeats: the zone before the change may show
	// this wall time earlier
	_, offset := start.Add(-time.Second).Zone()
	earlier := time.Unix(wall.Unix()-int64(offset), 0).In(loc)
	if earlier.Before(start) && civil(earlier).Equal(wall) {
		return earlier
	}
	return t
}

// civil is a time's wall clock to the minute, held as UTC fields.
func civil(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC)
}

// MinInterval is the shortest time between two runs on the wall clock.
func (s *Schedule) MinInterval() time.Duration {
	var times []int
	for h := range 24 {
		if !has(s.hour, h) {
			continue
		}
		for m := range 60 {
			if has(s.minute, m) {
				times = append(times, h*60+m)
			}
		}
	}
	days, _ := s.dayGap()
	shortest := days*24*60 - (times[len(times)-1] - times[0])
	for i := 1; i < len(times); i++ {
		shortest = min(shortest, times[i]-times[i-1])
	}
	return time.Duration(shortest) * time.Minute
}

// Preview says a schedule in words and lists its next count runs after
// from (PreviewCount when count is 0, at most PreviewMax), or says why it is
// not a schedule.
func Preview(expr string, from time.Time, count int) protocol.RoutinePreview {
	s, err := Parse(expr)
	if err != nil {
		return protocol.RoutinePreview{Error: err.Error()}
	}
	if count <= 0 {
		count = PreviewCount
	}
	count = min(count, PreviewMax)
	preview := protocol.RoutinePreview{Sentence: s.Sentence()}
	at := from
	for range count {
		if at = s.Next(at); at.IsZero() {
			break
		}
		preview.Next = append(preview.Next, at)
	}
	return preview
}

// values lists the members of a field's set in order.
func values(set uint64) []int {
	out := make([]int, 0, bits.OnesCount64(set))
	for set != 0 {
		v := bits.TrailingZeros64(set)
		out = append(out, v)
		set &^= 1 << v
	}
	return out
}
