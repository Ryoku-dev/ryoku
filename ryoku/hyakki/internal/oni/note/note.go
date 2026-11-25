// Package note reads what a spoken sentence asks of memory beyond keeping
// a note: forget something, correct something, or say what is remembered.
// Keeping a note stays the reflex rule's ("remember that ..."); "don't
// forget that ..." is a note too and never a forget here.
//
// The package is pure: it reads words and returns a command. Whether the
// command runs, and the card that asks before memory changes, are the
// daemon's.
package note

import (
	"regexp"
	"strings"
	"unicode"
)

// Kind is what a command asks of memory.
type Kind string

const (
	// Forget forgets what Arg names; an empty Arg is the note just kept.
	Forget Kind = "forget"
	// Correct changes From to To in a remembered fact; an empty From
	// splices To into the note just kept.
	Correct Kind = "correct"
	// Know says what is remembered about Arg; an empty Arg is the user.
	Know Kind = "know"
)

// Command is one parsed memory command. Its words keep the case they were
// said in, trimmed of the punctuation around them.
type Command struct {
	Kind Kind
	// Arg is what Forget and Know are about.
	Arg string
	// From and To are Correct's.
	From, To string
}

// Writes reports whether c changes memory: only a press may start one.
func (c Command) Writes() bool { return c.Kind == Forget || c.Kind == Correct }

// MaxArg is the longest argument a command takes, in runes.
const MaxArg = 200

var (
	lead = `^(?:(?:ok(?:ay)?|hey|please|so|and|now)[\s,]+)*`
	tail = `(?:[\s,]+please)?[\s.!?]*$`

	// "don't forget" and "do not forget" are notes, never forgets
	notForget = regexp.MustCompile(`(?i)\b(?:don'?t|do not|never)\s+forget\b`)

	forgetThat = regexp.MustCompile(`(?i)` + lead + `forget (?:that|it|what i just said|the last (?:note|thing))` + tail)
	forgetArg  = regexp.MustCompile(`(?i)` + lead + `forget (?:what (?:you know|i said|i told you) about|(?:everything|anything) about|that|the (?:fact|note) that|about) (.+?)` + tail)
	forgetBare = regexp.MustCompile(`(?i)` + lead + `forget (.+?)` + tail)

	meant   = regexp.MustCompile(`(?i)` + lead + `(?:no|nope|sorry|actually|wait)[\s,]+(?:i )?meant:? (.+?)` + tail)
	correct = regexp.MustCompile(`(?i)` + lead + `correct (?:the note |my note |the fact )?["“']?(.+?)["”']? to ["“']?(.+?)["”']?` + tail)

	knowMe  = regexp.MustCompile(`(?i)` + lead + `what do you (?:know|remember) about me` + tail)
	knowArg = regexp.MustCompile(`(?i)` + lead + `what do you (?:know|remember) about (.+?)` + tail)
)

// Parse reads text as a memory command.
func Parse(text string) (Command, bool) {
	text = strings.TrimSpace(text)
	if text == "" || notForget.MatchString(text) {
		return Command{}, false
	}
	switch {
	case forgetThat.MatchString(text):
		return Command{Kind: Forget}, true
	case knowMe.MatchString(text):
		return Command{Kind: Know}, true
	}
	if m := correct.FindStringSubmatch(text); m != nil {
		from, to := arg(m[1]), arg(m[2])
		if from == "" || to == "" || strings.EqualFold(from, to) {
			return Command{}, false
		}
		return Command{Kind: Correct, From: from, To: to}, true
	}
	if m := meant.FindStringSubmatch(text); m != nil {
		if to := arg(m[1]); to != "" {
			return Command{Kind: Correct, To: to}, true
		}
		return Command{}, false
	}
	if m := knowArg.FindStringSubmatch(text); m != nil {
		if a := arg(m[1]); a != "" {
			return Command{Kind: Know, Arg: a}, true
		}
		return Command{}, false
	}
	for _, re := range []*regexp.Regexp{forgetArg, forgetBare} {
		if m := re.FindStringSubmatch(text); m != nil {
			if a := arg(m[1]); a != "" && !filler(a) {
				return Command{Kind: Forget, Arg: a}, true
			}
			return Command{}, false
		}
	}
	return Command{}, false
}

// filler is an argument that names nothing to forget.
func filler(a string) bool {
	switch strings.ToLower(a) {
	case "it", "that", "this", "everything", "anything", "all", "all of it", "about it", "me":
		return true
	}
	return false
}

// arg trims an argument: quotes and punctuation around it, at most MaxArg
// runes, "" when nothing is left.
func arg(s string) string {
	s = strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(`"“”'‘’.,!?;:`, r)
	})
	if r := []rune(s); len(r) > MaxArg {
		return ""
	}
	return s
}

// Splice is the note kept with the correction to said into it: a
// correction that shares a word with the note is the whole new note; a
// shorter one that shares none takes the place of as many words at the
// note's end ("I live in Lisbon" and "Porto" make "I live in Porto"). It
// reports false when to cannot be placed.
func Splice(note, to string) (string, bool) {
	nw, tw := strings.Fields(note), strings.Fields(to)
	if len(tw) == 0 || len(nw) == 0 {
		return "", false
	}
	if shares(nw, tw) || len(tw) >= len(nw) {
		return to, !strings.EqualFold(strings.Join(nw, " "), strings.Join(tw, " "))
	}
	out := append(append([]string{}, nw[:len(nw)-len(tw)]...), tw...)
	// keep the full stop the note ended with
	if last := nw[len(nw)-1]; strings.HasSuffix(last, ".") && !strings.HasSuffix(to, ".") {
		out[len(out)-1] += "."
	}
	return strings.Join(out, " "), true
}

// Replace is fact with every case-insensitive from changed to to, and
// whether from was in it.
func Replace(fact, from, to string) (string, bool) {
	lower, find := strings.ToLower(fact), strings.ToLower(from)
	if find == "" || !strings.Contains(lower, find) {
		return fact, false
	}
	var b strings.Builder
	for {
		i := strings.Index(lower, find)
		if i < 0 {
			b.WriteString(fact)
			return b.String(), true
		}
		b.WriteString(fact[:i])
		b.WriteString(to)
		fact, lower = fact[i+len(find):], lower[i+len(find):]
	}
}

func shares(a, b []string) bool {
	set := map[string]bool{}
	for _, w := range a {
		if w = word(w); len(w) > 2 {
			set[w] = true
		}
	}
	for _, w := range b {
		if set[word(w)] {
			return true
		}
	}
	return false
}

func word(w string) string {
	return strings.ToLower(strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }))
}
