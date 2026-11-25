package remind

import (
	"slices"
	"time"
)

// part is a part of the day, said on its own ("tomorrow morning") or after
// a clock time ("at 7 in the evening").
type part int

const (
	noPart part = iota
	morning
	afternoon
	evening
	night
)

// defaultHour is the hour a part of the day means when no clock time is
// said; a named day with neither means the morning.
var defaultHour = map[part]int{noPart: 9, morning: 9, afternoon: 15, evening: 18, night: 20}

// span is an amount of time: whole days, counted on the wall clock so "in 2
// days" keeps the hour across a daylight saving change, and elapsed time.
type span struct {
	days int
	d    time.Duration
}

func (s span) total() time.Duration { return time.Duration(s.days)*24*time.Hour + s.d }

// dayRef is a day: an offset from today, or a weekday.
type dayRef struct {
	off     int
	today   bool
	weekday bool
	wd      time.Weekday
	next    bool
}

// clock is a time of day. mer is the part of the day said with it, if any;
// h is 0..23 as said ("16:45", "09:00") or 1..12 as on a clock face, with
// exact telling which.
type clock struct {
	h, m     int
	mer      part
	exact    bool
	midnight bool
}

// when is everything a request said about time. A kind of thing said twice
// is a conflict, and the request is not claimed.
type when struct {
	dur  *span
	day  *dayRef
	clk  *clock
	part part
}

// merge adds o to w, failing when both said the same kind of thing.
func (w when) merge(o when) (when, bool) {
	if (w.dur != nil && o.dur != nil) || (w.day != nil && o.day != nil) || (w.clk != nil && o.clk != nil) || (w.part != noPart && o.part != noPart) {
		return w, false
	}
	if o.dur != nil {
		w.dur = o.dur
	}
	if o.day != nil {
		w.day = o.day
	}
	if o.clk != nil {
		w.clk = o.clk
	}
	if o.part != noPart {
		w.part = o.part
	}
	return w, true
}

var units = map[string]time.Duration{
	"second": time.Second, "seconds": time.Second, "sec": time.Second, "secs": time.Second,
	"minute": time.Minute, "minutes": time.Minute, "min": time.Minute, "mins": time.Minute,
	"hour": time.Hour, "hours": time.Hour, "hr": time.Hour, "hrs": time.Hour,
	"day": 24 * time.Hour, "days": 24 * time.Hour,
	"week": 7 * 24 * time.Hour, "weeks": 7 * 24 * time.Hour,
}

// add puts halves halves of unit u into s; days and weeks count whole days
// on the wall clock, and a half day is twelve hours.
func (s *span) add(halves int, u time.Duration) {
	if u >= 24*time.Hour {
		days := int(u / (24 * time.Hour))
		s.days += halves / 2 * days
		if halves%2 == 1 {
			s.d += time.Duration(days) * 12 * time.Hour
		}
		return
	}
	s.d += time.Duration(halves) * u / 2
}

// count reads how many of a unit: a number, "a" or "an" for one, or "a
// couple of" for two. It returns the count in halves, so "1.5" is 3, and
// the words taken.
func count(ws []string, p int) (int, int) {
	switch {
	case has(ws, p, "a", "couple", "of"):
		return 4, 3
	case has(ws, p, "couple", "of"):
		return 4, 2
	case has(ws, p, "a") || has(ws, p, "an"):
		return 2, 1
	}
	if w := at(ws, p); len(w) >= 3 && w[len(w)-2:] == ".5" {
		if n, k := number([]string{w[:len(w)-2]}, 0); k == 1 {
			return 2*n + 1, 1
		}
	}
	n, k := number(ws, p)
	if k == 0 || n <= 0 {
		return 0, 0
	}
	return 2 * n, k
}

// duration reads an amount of time at ws[p:hi]: "20 minutes", "an hour and
// a half", "half an hour", "one and a half hours", "2 hours and 15
// minutes", "a quarter of an hour".
func duration(ws []string, p, hi int) (span, int) {
	ws = ws[:hi]
	var s span
	end := p
	for i := 0; ; i++ {
		q := end
		if i > 0 && has(ws, q, "and") {
			q++
		}
		n := durationPart(ws, q, &s)
		if n == 0 {
			break
		}
		end = q + n
	}
	if end == p || s.total() <= 0 || s.d%time.Second != 0 {
		return span{}, 0
	}
	return s, end - p
}

func durationPart(ws []string, p int, s *span) int {
	for _, q := range [][]string{{"a", "quarter", "of", "an", "hour"}, {"quarter", "of", "an", "hour"}, {"a", "quarter", "hour"}, {"quarter", "hour"}} {
		if has(ws, p, q...) {
			s.d += 15 * time.Minute
			return len(q)
		}
	}
	if has(ws, p, "half") && (at(ws, p+1) == "a" || at(ws, p+1) == "an") {
		if u, ok := units[at(ws, p+2)]; ok {
			s.add(1, u)
			return 3
		}
	}
	halves, n := count(ws, p)
	if n == 0 {
		return 0
	}
	q := p + n
	if has(ws, q, "and", "a", "half") && halves%2 == 0 {
		halves++
		q += 3
	}
	u, ok := units[at(ws, q)]
	if !ok {
		return 0
	}
	q++
	if has(ws, q, "and", "a", "half") && halves%2 == 0 {
		halves++
		q += 3
	}
	s.add(halves, u)
	return q - p
}

// at is ws[p], or "" past either end.
func at(ws []string, p int) string {
	if p < 0 || p >= len(ws) {
		return ""
	}
	return ws[p]
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday,
	"thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
}

var dayParts = map[string]part{"morning": morning, "afternoon": afternoon, "evening": evening, "night": night}

// day reads a day at ws[p:]: "today", "tonight", "tomorrow morning", "the
// day after tomorrow", "this evening", "on Friday", "next Monday night".
func day(ws []string, p int) (dayRef, part, int) {
	q := p
	var d dayRef
	switch {
	case has(ws, q, "later", "today"):
		d, q = dayRef{today: true}, q+2
	case has(ws, q, "today"):
		d, q = dayRef{today: true}, q+1
	case has(ws, q, "tonight"):
		return dayRef{today: true}, night, 1
	case has(ws, q, "tomorrow") || has(ws, q, "tmrw"):
		d, q = dayRef{off: 1}, q+1
	case has(ws, q, "the", "day", "after", "tomorrow"):
		d, q = dayRef{off: 2}, q+4
	case has(ws, q, "day", "after", "tomorrow"):
		d, q = dayRef{off: 2}, q+3
	case has(ws, q, "this") && dayParts[at(ws, q+1)] != noPart && at(ws, q+1) != "night":
		return dayRef{today: true}, dayParts[ws[q+1]], 2
	default:
		if has(ws, q, "on") {
			q++
		}
		next := false
		switch {
		case has(ws, q, "next"):
			next = true
			q++
		case has(ws, q, "this", "coming"):
			q += 2
		case has(ws, q, "this") || has(ws, q, "coming"):
			q++
		}
		wd, ok := weekdays[at(ws, q)]
		if !ok {
			return dayRef{}, noPart, 0
		}
		d, q = dayRef{weekday: true, wd: wd, next: next}, q+1
	}
	pt := dayParts[at(ws, q)]
	if pt != noPart {
		q++
	}
	return d, pt, q - p
}

// meridiem reads what follows a clock time to say which half of the day:
// "am", "p.m.", "in the morning", "at night".
func meridiem(ws []string, p int) (part, int) {
	switch {
	case has(ws, p, "am"):
		return morning, 1
	case has(ws, p, "a", "m"):
		return morning, 2
	case has(ws, p, "pm"):
		return afternoon, 1
	case has(ws, p, "p", "m"):
		return afternoon, 2
	case has(ws, p, "in", "the", "morning"):
		return morning, 3
	case has(ws, p, "in", "the", "afternoon"):
		return afternoon, 3
	case has(ws, p, "in", "the", "evening"):
		return evening, 3
	case has(ws, p, "at", "night"):
		return night, 2
	}
	return noPart, 0
}

// hour reads an hour on a clock face, 1..12, as a digit or a word.
func hour(ws []string, p int) (int, int) {
	n, k := number(ws, p)
	if k != 1 || n < 1 || n > 12 {
		return 0, 0
	}
	return n, 1
}

func digit(w string) bool { return w != "" && w[0] >= '0' && w[0] <= '9' }

// clockAt reads a time of day at ws[p:]. strong says the words alone mark
// it as a time ("5 pm", "noon", "half past four", "6 o'clock"), so it needs
// no "at" before it.
func clockAt(ws []string, p int) (clock, int, bool) {
	switch {
	case has(ws, p, "noon"), has(ws, p, "midday"):
		return clock{h: 12, mer: afternoon}, 1, true
	case has(ws, p, "midnight"):
		return clock{midnight: true}, 1, true
	}
	if c, n, ok := relative(ws, p); ok {
		return c, n, true
	}
	// "9", "16 45", "930", "four thirty", "nine oh five", "6 o'clock", each
	// with an optional "am"/"pm".
	w := at(ws, p)
	var c clock
	minutes := false
	q := p
	switch n, k := number(ws, p); {
	case k == 1 && digit(w) && len(w) >= 3 && len(w) <= 4:
		c = clock{h: n / 100, m: n % 100, exact: n/100 > 12 || w[0] == '0'}
		minutes = true
	case k == 1 && digit(w) && len(w) <= 2:
		c = clock{h: n, exact: n > 12 || n == 0 || (len(w) == 2 && w[0] == '0')}
	case k == 1 && n >= 1 && n <= 12:
		c = clock{h: n}
	default:
		return clock{}, 0, false
	}
	q++
	if !minutes {
		m := at(ws, q)
		switch n, k := number(ws, q); {
		case len(m) == 2 && digit(m) && k == 1 && n <= 59:
			c.m, minutes = n, true
			q++
		case m == "oh" || m == "o":
			if n, k := number(ws, q+1); k == 1 && n >= 1 && n <= 9 {
				c.m, minutes = n, true
				q += 2
			}
		case k > 0 && !digit(m) && n >= 10 && n <= 59:
			c.m, minutes = n, true
			q += k
		}
	}
	strong := false
	if !minutes && has(ws, q, "oclock") {
		strong = true
		q++
	}
	if mer, k := meridiem(ws, q); k > 0 {
		c.mer, strong = mer, true
		q += k
	}
	if c.h > 23 || c.m > 59 || (c.mer != noPart && (c.h < 1 || c.h > 12)) {
		return clock{}, 0, false
	}
	return c, q - p, strong
}

// relative reads a time told against the hour: "half past four", "a
// quarter to five", "ten past six", "twenty five to nine pm".
func relative(ws []string, p int) (clock, int, bool) {
	q, mins := p, 0
	switch {
	case has(ws, q, "half"):
		mins, q = 30, q+1
	case has(ws, q, "a", "quarter"):
		mins, q = 15, q+2
	case has(ws, q, "quarter"):
		mins, q = 15, q+1
	default:
		n, k := number(ws, q)
		if k == 0 || n < 1 || n > 30 {
			return clock{}, 0, false
		}
		mins, q = n, q+k
		if has(ws, q, "minutes") || has(ws, q, "minute") {
			q++
		}
	}
	w := at(ws, q)
	if w != "past" && w != "after" && w != "to" {
		return clock{}, 0, false
	}
	h, k := hour(ws, q+1)
	if k == 0 || (w == "to" && mins == 30) {
		return clock{}, 0, false
	}
	c := clock{h: h, m: mins}
	if w == "to" {
		c.h, c.m = h-1, 60-mins
		if c.h == 0 {
			c.h = 12
		}
	}
	q += 1 + k
	mer, mk := meridiem(ws, q)
	c.mer = mer
	return c, q + mk - p, true
}

// phrase reads one time phrase at ws[p:hi] into w, returning the words
// taken, or 0 when there is none or it repeats what w already says.
func phrase(ws []string, p, hi int, w *when) int {
	ws = ws[:hi]
	var o when
	n := 0
	switch first := at(ws, p); {
	case first == "in" || first == "after" || first == "for" || first == "within":
		if s, k := duration(ws, p+1, hi); k > 0 {
			o.dur, n = &s, 1+k
			if has(ws, p+n, "time") {
				n++
			} else if has(ws, p+n, "from", "now") {
				n += 2
			}
			break
		}
		if pt, k := meridiem(ws, p); k > 0 && first == "in" {
			o.part, n = pt, k
			break
		}
		if first == "for" {
			if c, k, strong := clockAt(ws, p+1); k > 0 && strong {
				o.clk, n = &c, 1+k
			} else if d, pt, k := day(ws, p+1); k > 0 {
				o.day, o.part, n = &d, pt, 1+k
			}
		}
	case first == "at" || first == "around":
		if c, k, _ := clockAt(ws, p+1); k > 0 {
			o.clk, n = &c, 1+k
		} else if has(ws, p, "at", "night") {
			o.part, n = night, 2
		}
	default:
		if s, k := duration(ws, p, hi); k > 0 && has(ws, p+k, "from", "now") {
			o.dur, n = &s, k+2
			break
		}
		if d, pt, k := day(ws, p); k > 0 {
			o.day, o.part, n = &d, pt, k
			break
		}
		if c, k, strong := clockAt(ws, p); k > 0 && strong {
			o.clk, n = &c, k
		}
	}
	if n == 0 {
		return 0
	}
	merged, ok := w.merge(o)
	if !ok {
		return 0
	}
	*w = merged
	return n
}

// sequence reads ws[p:hi] wholly as time phrases.
func sequence(ws []string, p, hi int) (when, bool) {
	var w when
	for p < hi {
		n := phrase(ws, p, hi, &w)
		if n == 0 {
			return when{}, false
		}
		p += n
	}
	return w, true
}

// moment is an hour and minute on a date, or the midnight that ends it.
type moment struct {
	h, m     int
	nextDay  bool
	midnight bool
}

// hours are the times of day w can mean. anchored says a day other than
// today was named, so an hour said with no "am" or "pm" takes its usual
// half: 7 to 11 in the morning, 12 to 6 in the afternoon. Otherwise both
// halves are candidates and the sooner wins: "at 4" is the next 4 o'clock.
func (w when) hours(anchored bool) []moment {
	if w.clk == nil {
		return []moment{{h: defaultHour[w.part]}}
	}
	c := *w.clk
	if c.midnight {
		return []moment{{midnight: true}}
	}
	mer := c.mer
	if mer == noPart {
		mer = w.part
	}
	switch {
	case c.exact && (mer == noPart || (mer == morning) == (c.h < 12)):
		return []moment{{h: c.h, m: c.m}}
	case mer == morning:
		return []moment{{h: c.h % 12, m: c.m}}
	case mer == night && c.h == 12 && c.m == 0:
		return []moment{{midnight: true}}
	case mer == night && c.h%12 < 5:
		return []moment{{h: c.h % 12, m: c.m, nextDay: true}}
	case mer != noPart:
		return []moment{{h: c.h%12 + 12, m: c.m}}
	case anchored:
		if c.h >= 7 && c.h <= 11 {
			return []moment{{h: c.h, m: c.m}}
		}
		return []moment{{h: c.h%12 + 12, m: c.m}}
	}
	return []moment{{h: c.h % 12, m: c.m}, {h: c.h%12 + 12, m: c.m}}
}

// date is the calendar day off days after t's, at noon in loc, so it never
// lands in a daylight saving gap.
func date(t time.Time, off int, loc *time.Location) time.Time {
	y, mo, d := t.Date()
	return time.Date(y, mo, d+off, 12, 0, 0, 0, loc)
}

// wallTimes are the instants the wall clock in loc reads h:m on day's date:
// one, or two when the clock is set back and the hour repeats, earliest
// first. A time the clock skips when set forward lands as far past the
// change as it was said past the hour, so 2:30 becomes 3:30.
func wallTimes(day time.Time, h, m int, loc *time.Location) []time.Time {
	y, mo, d := day.Date()
	naive := time.Date(y, mo, d, h, m, 0, 0, time.UTC)
	_, before := naive.Add(-24 * time.Hour).In(loc).Zone()
	_, after := naive.Add(24 * time.Hour).In(loc).Zone()
	var out []time.Time
	for _, off := range []int{before, after} {
		t := naive.Add(-time.Duration(off) * time.Second).In(loc)
		ty, tmo, td := t.Date()
		if ty == y && tmo == mo && td == d && t.Hour() == h && t.Minute() == m && !slices.ContainsFunc(out, t.Equal) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []time.Time{naive.Add(-time.Duration(before) * time.Second).In(loc)}
	}
	slices.SortFunc(out, func(a, b time.Time) int { return a.Compare(b) })
	return out
}

// instants are the times mo means on the date off days after local's.
func (mo moment) instants(local time.Time, off int, loc *time.Location) []time.Time {
	if mo.midnight {
		return wallTimes(date(local, off+1, loc), 0, 0, loc)
	}
	if mo.nextDay {
		off++
	}
	return wallTimes(date(local, off, loc), mo.h, mo.m, loc)
}

// refusal is why a time that was read cannot be kept.
type refusal int

const (
	fine refusal = iota
	passed
	tooFar
)

// resolve turns w into a due time after now. claimed is false when w does
// not say enough, or says things that do not fit together.
func (w when) resolve(now time.Time, loc *time.Location) (due time.Time, why refusal, claimed bool) {
	local := now.In(loc)
	if w.dur != nil {
		switch {
		case w.day == nil && w.clk == nil && w.part == noPart:
			return capped(local.AddDate(0, 0, w.dur.days).Add(w.dur.d), now, loc)
		case w.day == nil && w.dur.d == 0 && w.dur.days > 0:
			w.day = &dayRef{off: w.dur.days}
		default:
			return time.Time{}, fine, false
		}
	}
	if w.clk == nil && w.part == noPart && (w.day == nil || w.day.today) {
		return time.Time{}, fine, false
	}
	var offs []int
	anchored := false
	switch d := w.day; {
	case d == nil:
		offs = []int{0, 1}
	case d.today:
		offs = []int{0}
	case d.weekday:
		diff := (int(d.wd) - int(local.Weekday()) + 7) % 7
		switch {
		case diff == 0 && d.next:
			offs = []int{7}
		case diff == 0:
			offs = []int{0, 7}
		default:
			offs = []int{diff}
		}
		anchored = true
	default:
		offs = []int{d.off}
		anchored = true
	}
	moments := w.hours(anchored)
	for _, off := range offs {
		for _, mo := range moments {
			for _, t := range mo.instants(local, off, loc) {
				if t.After(now) && (due.IsZero() || t.Before(due)) {
					due = t
				}
			}
		}
		if !due.IsZero() {
			break
		}
	}
	if due.IsZero() {
		return time.Time{}, passed, true
	}
	return capped(due, now, loc)
}

// capped refuses a due time past the reminder limit: MaxDays on the wall
// clock or MaxReminder elapsed, whichever is later.
func capped(due, now time.Time, loc *time.Location) (time.Time, refusal, bool) {
	limit := now.In(loc).AddDate(0, 0, MaxDays)
	if l := now.Add(MaxReminder); l.After(limit) {
		limit = l
	}
	if due.After(limit) {
		return time.Time{}, tooFar, true
	}
	return due, fine, true
}
