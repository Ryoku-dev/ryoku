package remind

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	i18n "ryoku-i18n"
)

// Parse reads a request to set a reminder or a timer, said at now in loc (a
// nil loc is now's). c may be the transcript as heard or already folded.
//
// claimed is false when c is not such a request, or names no time: "remind
// me" and "remind me to call Sam" go on to be asked. A claimed request
// returns the reminder and the line confirming it, or, when its time cannot
// be kept, a zero Reminder (Kind "") and the line saying why: too far
// ahead, a timer too long, or a time already gone today.
//
// The reminder's ID is fresh, Created is now, and Text is only the words
// of the reminder itself, as the user said them.
func Parse(c string, now time.Time, loc *time.Location) (r Reminder, line string, claimed bool) {
	if loc == nil {
		loc = now.Location()
	}
	q := read(c)
	if r, line, ok := q.timer(now); ok {
		return r, line, true
	}
	return q.reminder(now, loc)
}

var (
	timerVerbs = [][]string{{"set"}, {"start"}, {"make"}, {"create"}, {"put", "on"}, {"add"}, {"begin"}}
	// labelWords join a timer's duration to what it is for: "for the pasta".
	labelWords = map[string]bool{"for": true, "called": true, "named": true, "to": true, "labeled": true, "labelled": true}
)

// timer reads "set a timer for 5 minutes", "a 10 minute timer for the
// eggs", "timer for an hour and a half".
func (q request) timer(now time.Time) (Reminder, string, bool) {
	ws := q.ws[:q.hi]
	p := q.lo
	verb := false
	for _, v := range timerVerbs {
		if has(ws, p, v...) {
			p += len(v)
			verb = true
			break
		}
	}
	if verb && has(ws, p, "me") {
		p++
	}
	article := has(ws, p, "a") || has(ws, p, "an")
	if article {
		p++
	}
	var s span
	end := 0
	switch {
	case has(ws, p, "timer") && (verb || p == q.lo):
		at := p + 1
		if has(ws, at, "for") || has(ws, at, "of") {
			at++
		}
		d, n := duration(ws, at, q.hi)
		if n == 0 {
			return Reminder{}, "", false
		}
		s, end = d, at+n
	default:
		start := p
		if article {
			start--
		}
		for _, from := range []int{p, start} {
			if d, n := duration(ws, from, q.hi); n > 0 && has(ws, from+n, "timer") {
				s, end = d, from+n+1
				break
			}
		}
		if end == 0 || !(verb || article || p == q.lo) {
			return Reminder{}, "", false
		}
	}
	text := ""
	if end < q.hi {
		if !labelWords[ws[end]] || end+1 >= q.hi {
			return Reminder{}, "", false
		}
		text = clip(q.span(end+1, q.hi))
	}
	d := s.total()
	if d > MaxTimer {
		return Reminder{}, i18n.Tf("Timers stop at %s.", spanWords(MaxTimer)), true
	}
	if d < MinTimer {
		return Reminder{}, "", false
	}
	r := Reminder{ID: newID(), Kind: KindTimer, Due: now.Add(d), Created: now, Text: text}
	return r, i18n.Tf("Timer set, %s.", spanWords(d)), true
}

var (
	reminderLeads = [][]string{
		{"remind", "me"}, {"remind", "us"},
		{"set", "me", "a", "reminder"}, {"set", "a", "reminder"}, {"set", "reminder"},
		{"add", "a", "reminder"}, {"create", "a", "reminder"}, {"make", "a", "reminder"},
		{"new", "reminder"}, {"reminder"},
	}
	// textLeads join a reminder's own words to the request: "to", "that",
	// "about".
	textLeads = map[string]bool{"to": true, "that": true, "about": true, "of": true}
	// questions start a request to be told something now, not later:
	// "remind me how to set a timer".
	questions = map[string]bool{"how": true, "what": true, "whats": true, "why": true, "who": true, "whos": true, "where": true, "when": true, "which": true, "whether": true, "if": true}
)

// reminder reads "remind me to stretch in 20 minutes", "remind me tomorrow
// at 9:30 to call the bank", "in 10 minutes remind me to stretch", "set a
// reminder for 5 pm to call Sam".
func (q request) reminder(now time.Time, loc *time.Location) (Reminder, string, bool) {
	ws := q.ws[:q.hi]
	lead, start := -1, 0
	for t := q.lo; t < q.hi && lead < 0; t++ {
		for _, l := range reminderLeads {
			if !has(ws, t, l...) || (len(l) == 1 && t != q.lo) {
				continue
			}
			if t == q.lo {
				lead, start = t, t+len(l)
				break
			}
			if _, ok := sequence(ws, q.lo, t); ok {
				lead, start = t, t+len(l)
				break
			}
		}
	}
	if lead < 0 {
		return Reminder{}, "", false
	}
	w, _ := sequence(ws, q.lo, lead)
	// Time phrases straight after the lead.
	p := start
	for p < q.hi {
		n := phrase(ws, p, q.hi, &w)
		if n == 0 {
			break
		}
		p += n
	}
	// Time phrases ending the request, the longest run that reads wholly.
	end := q.hi
	for k := p; k < q.hi; k++ {
		if tail, ok := sequence(ws, k, q.hi); ok {
			if merged, ok := w.merge(tail); ok {
				w, end = merged, k
				break
			}
		}
	}
	if textLeads[at(ws, p)] && p < end {
		p++
	}
	if questions[at(ws, p)] && p < end {
		return Reminder{}, "", false
	}
	due, why, ok := w.resolve(now, loc)
	if !ok {
		return Reminder{}, "", false
	}
	switch why {
	case tooFar:
		return Reminder{}, i18n.Tf("%d days is my limit.", MaxDays), true
	case passed:
		return Reminder{}, i18n.T("That time has passed."), true
	}
	r := Reminder{ID: newID(), Kind: KindReminder, Due: due, Created: now, Text: clip(q.span(p, end))}
	if w.dur != nil && w.dur.days == 0 && w.day == nil {
		return r, i18n.Tf("Reminder in %s.", spanWords(w.dur.d)), true
	}
	return r, i18n.Tf("Reminder, %s.", whenWords(due, now, loc)), true
}

// clip bounds a reminder's own words to MaxTextRunes, ending on a whole
// word, and drops a closing stop.
func clip(s string) string {
	s = strings.TrimRight(strings.TrimSpace(s), ".,;:!?…")
	if utf8.RuneCountInString(s) <= MaxTextRunes {
		return s
	}
	rs := []rune(s)[:MaxTextRunes]
	cut := string(rs)
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " .,;:!?…")
}

// CancelKind is what a cancel request takes away. The values match the
// reflex cancel_timer action's argument.
type CancelKind string

const (
	// CancelTimers takes away every timer.
	CancelTimers CancelKind = "timers"
	// CancelReminder takes away the next reminder due, and only it.
	CancelReminder CancelKind = "reminder"
	// CancelReminders takes away every reminder.
	CancelReminders CancelKind = "reminders"
	// CancelAll takes away every reminder and timer.
	CancelAll CancelKind = "all"
)

var (
	cancelAsk = regexp.MustCompile(`^(?:cancel|stop|clear|end|remove|delete|drop|forget|kill|turn off|switch off|get rid of|never ?mind)(?: (all|both))?(?: of)?(?: (?:the|my|that|this|those|these))? (timers?|reminders?)(?: (?:and|&) (?:(?:the|my) )?(timers?|reminders?))?$`)
	listObj   = `(?:reminders?|timers?|reminders and timers|timers and reminders)`
	listAsk   = regexp.MustCompile(`^(?:` +
		`(?:what|which) (?:are|is) (?:my|the) ` + listObj + `(?: (?:i have|left|set|pending))?` +
		`|(?:what|which) ` + listObj + ` (?:do i have|have i got|are (?:there|set|pending|left)|did i set)` +
		`|(?:list|show|tell|read|give|check)(?: me| us)?(?: all)?(?: of)? (?:my|the) ` + listObj +
		`|(?:do|did) i (?:have|set) any ` + listObj +
		`|(?:are there|is there|any) (?:any )?` + listObj + `(?: set| left| pending)?` +
		`|my ` + listObj +
		`|(?:how much time|how long) is left(?: on (?:the|my) timer)?` +
		`|(?:whats|what is) left on (?:the|my) timer` +
		`)$`)
)

// ParseCancel reads a request to take reminders or timers away: "cancel the
// timer", "stop my timers", "cancel my reminder", "clear all my reminders",
// "cancel all my reminders and timers". A singular reminder means only the
// next one due, so one slip never takes several.
func ParseCancel(c string) (CancelKind, bool) {
	m := cancelAsk.FindStringSubmatch(read(c).core())
	if m == nil {
		return "", false
	}
	if m[3] != "" {
		if strings.HasPrefix(m[2], "timer") == strings.HasPrefix(m[3], "timer") {
			return "", false
		}
		return CancelAll, true
	}
	switch {
	case strings.HasPrefix(m[2], "timer"):
		return CancelTimers, true
	case m[1] != "" || m[2] == "reminders":
		return CancelReminders, true
	}
	return CancelReminder, true
}

// ParseList reads a request to hear the reminders and timers: "what are my
// reminders", "do I have any timers", "how much time is left on the
// timer".
func ParseList(c string) bool { return listAsk.MatchString(read(c).core()) }
