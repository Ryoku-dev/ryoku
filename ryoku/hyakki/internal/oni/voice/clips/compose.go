package clips

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Phrase is one part of an utterance: Word, Number, Ordinal, Count, Time,
// Day or Date.
type Phrase interface {
	ids() ([]string, error)
	// pause reports whether the phrase ends in a comma pause.
	pause() bool
}

// Word is one clip said as it is: a lead-in, an app name, a short line.
type Word string

// Number is a number from 0 to 100, said from the word clips: 45 is
// "forty" then "five", 100 is "one" then "hundred".
type Number int

// Ordinal is a day of the month, 1 to 31: 21 is "twenty" then "first".
type Ordinal int

// Count is a number with its noun: One is said for 1, Many otherwise. An
// empty noun says the number alone.
type Count struct {
	N         int
	One, Many string
}

// Time is a clock time, given on the 24-hour clock and said on the
// 12-hour one: 15:45 is "forty five past three", 9:00 "nine o'clock".
type Time struct{ Hour, Minute int }

// Day is a weekday, said with a comma pause after it.
type Day time.Weekday

// Date is a day of a month: "march twenty first".
type Date struct {
	Month time.Month
	Day   int
}

// Utterance is what Compose assembles: an optional lead-in said first with
// a pause, then the phrases in order.
type Utterance struct {
	LeadIn  string
	Phrases []Phrase
	Fast    bool // prefer fast variants inside the run
}

// Clip ids the composer needs. A pack that wants to say times, dates and
// counts records these.
var (
	ones = []string{
		"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
		"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen",
		"seventeen", "eighteen", "nineteen",
	}
	tens = []string{2: "twenty", 3: "thirty", 4: "forty", 5: "fifty", 6: "sixty", 7: "seventy", 8: "eighty", 9: "ninety"}
	nths = []string{
		1: "first", 2: "second", 3: "third", 4: "fourth", 5: "fifth", 6: "sixth", 7: "seventh",
		8: "eighth", 9: "ninth", 10: "tenth", 11: "eleventh", 12: "twelfth", 13: "thirteenth",
		14: "fourteenth", 15: "fifteenth", 16: "sixteenth", 17: "seventeenth", 18: "eighteenth",
		19: "nineteenth", 20: "twentieth", 30: "thirtieth",
	}
	days   = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	months = []string{1: "january", 2: "february", 3: "march", 4: "april", 5: "may", 6: "june", 7: "july", 8: "august", 9: "september", 10: "october", 11: "november", 12: "december"}
)

// Clip ids for the words around numbers.
const (
	Hundred = "hundred"
	Past    = "past"
	OClock  = "oclock"
)

func (w Word) ids() ([]string, error) { return []string{string(w)}, nil }
func (Word) pause() bool              { return false }

func (n Number) ids() ([]string, error) {
	switch {
	case n < 0 || n > 100:
		return nil, fmt.Errorf("clips: number %d is outside 0-100", int(n))
	case n == 100:
		return []string{ones[1], Hundred}, nil
	case n < 20:
		return []string{ones[n]}, nil
	case n%10 == 0:
		return []string{tens[n/10]}, nil
	}
	return []string{tens[n/10], ones[n%10]}, nil
}
func (Number) pause() bool { return false }

func (o Ordinal) ids() ([]string, error) {
	switch {
	case o < 1 || o > 31:
		return nil, fmt.Errorf("clips: ordinal %d is outside 1-31", int(o))
	case int(o) < len(nths) && nths[o] != "":
		return []string{nths[o]}, nil
	}
	return []string{tens[o/10], nths[o%10]}, nil
}
func (Ordinal) pause() bool { return false }

func (c Count) ids() ([]string, error) {
	out, err := Number(c.N).ids()
	if err != nil {
		return nil, err
	}
	noun := c.Many
	if c.N == 1 {
		noun = c.One
	}
	if noun != "" {
		out = append(out, noun)
	}
	return out, nil
}
func (Count) pause() bool { return false }

func (t Time) ids() ([]string, error) {
	if t.Hour < 0 || t.Hour > 23 || t.Minute < 0 || t.Minute > 59 {
		return nil, fmt.Errorf("clips: time %d:%02d is not a clock time", t.Hour, t.Minute)
	}
	h := t.Hour % 12
	if h == 0 {
		h = 12
	}
	hour, _ := Number(h).ids()
	if t.Minute == 0 {
		return append(hour, OClock), nil
	}
	mins, _ := Number(t.Minute).ids()
	return append(append(mins, Past), hour...), nil
}
func (Time) pause() bool { return false }

func (d Day) ids() ([]string, error) {
	if d < 0 || int(d) >= len(days) {
		return nil, fmt.Errorf("clips: weekday %d", int(d))
	}
	return []string{days[d]}, nil
}
func (Day) pause() bool { return true }

func (d Date) ids() ([]string, error) {
	if d.Month < 1 || d.Month > 12 {
		return nil, fmt.Errorf("clips: month %d", int(d.Month))
	}
	day, err := Ordinal(d.Day).ids()
	if err != nil {
		return nil, err
	}
	return append([]string{months[d.Month]}, day...), nil
}
func (Date) pause() bool { return false }

// said is one clip in an utterance before its variant is chosen.
type said struct {
	id    string
	comma bool
}

// Compose turns an utterance into the clips to play, each named "id" or
// "id.variant": the last clip takes its final variant, a clip before a
// pause its comma variant, and with Fast the others their fast variant,
// each only when the pack has it. It fails with ErrMissingClip naming the
// first clip the pack lacks, or on a number, ordinal or time out of range.
func (p *Pack) Compose(u Utterance) ([]string, error) {
	var run []said
	if u.LeadIn != "" {
		run = append(run, said{id: u.LeadIn, comma: true})
	}
	for _, ph := range u.Phrases {
		ids, err := ph.ids()
		if err != nil {
			return nil, err
		}
		for i, id := range ids {
			run = append(run, said{id: id, comma: ph.pause() && i == len(ids)-1})
		}
	}
	return p.refs(run, u.Fast)
}

func (p *Pack) refs(run []said, fast bool) ([]string, error) {
	out := make([]string, 0, len(run))
	for i, s := range run {
		c, ok := p.clips[s.id]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrMissingClip, s.id)
		}
		variant := Plain
		switch {
		case i == len(run)-1:
			variant = Final
		case s.comma:
			variant = Comma
		case fast:
			variant = Fast
		}
		if _, ok := c.Variants[variant]; ok {
			out = append(out, s.id+"."+variant)
		} else {
			out = append(out, s.id)
		}
	}
	return out, nil
}

var (
	clockToken   = regexp.MustCompile(`^([01]?\d|2[0-3]):([0-5]\d)$`)
	ordinalToken = regexp.MustCompile(`^(\d{1,2})(st|nd|rd|th)$`)
	numberToken  = regexp.MustCompile(`^\d{1,3}$`)
)

// Speak turns a line of text into clips: runs of words matching a clip's
// text (longest first), clock times like 3:45, ordinals like 21st and
// numbers up to 100. A comma, semicolon or colon gives the clip before it
// its comma variant and the last clip takes its final variant. It fails
// with ErrMissingClip on the first word no clip says, so a line the pack
// cannot say is never half spoken.
func (p *Pack) Speak(line string) ([]string, error) {
	var run []said
	toks := tokens(line)
	for i := 0; i < len(toks); {
		t := toks[i]
		if t == "," {
			if len(run) > 0 {
				run[len(run)-1].comma = true
			}
			i++
			continue
		}
		if n := p.match(toks[i:]); n > 0 {
			run = append(run, said{id: p.texts[strings.Join(toks[i:i+n], " ")]})
			i += n
			continue
		}
		var ph Phrase
		if m := clockToken.FindStringSubmatch(t); m != nil {
			h, _ := strconv.Atoi(m[1])
			mi, _ := strconv.Atoi(m[2])
			ph = Time{Hour: h, Minute: mi}
		} else if m := ordinalToken.FindStringSubmatch(t); m != nil {
			n, _ := strconv.Atoi(m[1])
			ph = Ordinal(n)
		} else if numberToken.MatchString(t) {
			n, _ := strconv.Atoi(t)
			ph = Number(n)
		} else {
			return nil, fmt.Errorf("%w: no clip says %q", ErrMissingClip, t)
		}
		ids, err := ph.ids()
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			run = append(run, said{id: id})
		}
		i++
	}
	if len(run) == 0 {
		return nil, fmt.Errorf("%w: %q says nothing", ErrMissingClip, line)
	}
	return p.refs(run, false)
}

// match returns how many tokens the longest clip text starting at toks
// covers, or 0.
func (p *Pack) match(toks []string) int {
	for n := min(p.words, len(toks)); n > 0; n-- {
		if slices.Contains(toks[:n], ",") {
			continue
		}
		if _, ok := p.texts[strings.Join(toks[:n], " ")]; ok {
			return n
		}
	}
	return 0
}

// tokens splits a line into lower-case words (letters, digits, apostrophes
// and a colon between digits) and "," for every clause pause. Sentence
// punctuation and other symbols are dropped.
func tokens(line string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if w := strings.Trim(b.String(), "':"); w != "" {
			out = append(out, w)
		}
		b.Reset()
	}
	rs := []rune(line)
	for i, r := range rs {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '\'' || r == '’':
			b.WriteRune('\'')
		case r == ':' && i > 0 && i+1 < len(rs) && unicode.IsDigit(rs[i-1]) && unicode.IsDigit(rs[i+1]):
			b.WriteRune(':')
		case r == ',' || r == ';' || r == ':' || r == '—' || r == '–':
			flush()
			out = append(out, ",")
		default:
			flush()
		}
	}
	flush()
	return out
}

// words is a clip text as Speak compares it.
func words(text string) []string {
	var out []string
	for _, t := range tokens(text) {
		if t != "," {
			out = append(out, t)
		}
	}
	return out
}
