package route

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"ryoku-rashin/internal/oni/remind"
)

// The phrase lists below are Hyakki's own wording. Each is matched against
// the whole utterance after it is folded (wake.Normalize) and the sounds,
// courtesies and the character's name around it are taken off, so "um,
// never mind, thanks" dismisses but "never mind the weather, what is it
// like" does not.

// leading and trailing are taken off an utterance when other words remain.
var (
	leading  = []string{"um", "uh", "er", "erm", "hm", "hmm", "oh", "ah", "ok", "okay", "so", "well", "please", "and", "hey"}
	trailing = []string{"please", "now", "then", "thanks", "thank you", "for me"}
)

var (
	dismissals = []string{
		"never mind", "nevermind", "cancel", "cancel that", "cancel it", "stop", "stop it",
		"stop talking", "be quiet", "quiet", "hush", "shush", "shh", "forget it", "forget that",
		"thats all", "thats it", "nothing", "no thanks", "no thank you", "go away", "dismiss",
		"bye", "goodbye", "good bye", "bye bye", "see you", "see you later", "good night",
		"goodnight", "leave it", "im done", "all done", "not now",
	}
	allows  = []string{"yes", "yeah", "yep", "sure", "allow", "allow it", "approve", "approve it", "go ahead", "do it", "yes do it", "yes allow it"}
	denies  = []string{"no", "nope", "deny", "deny it", "decline", "reject", "dont", "do not", "dont do it", "no dont", "no way"}
	greets  = []string{"hi", "hello", "hey", "hey there", "hi there", "hello there", "hiya", "howdy", "good morning", "morning", "good afternoon", "good evening", "evening"}
	thanks  = []string{"thanks", "thank you", "thanks a lot", "thank you so much", "thanks so much", "cheers", "much appreciated", "appreciate it", "nice", "great", "perfect", "good job", "well done", "nice work"}
	howRU   = []string{"how are you", "how are you doing", "hows it going", "how is it going", "you okay", "are you okay", "how do you do", "whats up"}
	whoRU   = []string{"who are you", "whats your name", "what is your name", "what are you"}
	moreAsk = [][]string{{"in", "detail"}, {"tell", "me", "more"}, {"step", "by", "step"}, {"walk", "me", "through"}, {"explain"}, {"elaborate"}, {"the", "whole"}, {"long", "version"}, {"full", "answer"}, {"read", "it", "all"}, {"all", "the", "details"}}
	// guideNexts tell a running guide its step is done; guideCircles ask
	// for its strand again. Both mean something only while one runs.
	guideNexts = []string{
		"next", "next step", "next one", "go on", "carry on", "continue", "keep going", "done",
		"did it", "i did it", "done that", "ive done it", "i have done it", "got it", "whats next",
		"what is next", "and next", "on to the next", "next please",
	}
	guideCircles = []string{
		"circle it", "circle that", "circle it again", "circle that again", "circle", "show me again",
		"show me where", "point again", "point at it", "point at it again", "point to it", "where again",
		"where is it", "wheres that", "where is that", "which one", "highlight it",
	}
)

// screenAsk is a question about doing something on screen: with Oni
// perched and guides on it starts a guide rather than an Ask.
var screenAsk = regexp.MustCompile(`^(?:(?:how (?:do|can|could|would|should) (?:i|we)|how to|where (?:do|can|could|would|should) (?:i|we)|where (?:is|are)|wheres|show me (?:how|where)|teach me how|(?:walk|guide|take) me through|help me (?:find|get to)|which (?:button|menu|tab|option|key) do i) .+|what do i (?:click|press|tap|choose|select)(?: .+)?)$`)

// Default small-talk lines, said when the character brings none. Each keeps
// the voice discipline.
var (
	defaultGreet  = []string{"Hello.", "Hi, I'm here.", "Hey there."}
	defaultThanks = []string{"Anytime.", "Glad to help.", "Happy to help."}
	defaultFine   = []string{"All good here.", "Doing well, thanks.", "Fine, and listening."}
)

// core is the folded utterance with the sounds, courtesies and name around
// it taken off, as long as words remain.
func core(norm, name string) string {
	words := strings.Fields(norm)
	nameWords := strings.Fields(name)
	for changed := true; changed && len(words) > 1; {
		changed = false
		if n := prefixOf(words, nameWords); n > 0 && n < len(words) {
			words, changed = words[n:], true
		}
		if len(words) > 1 && slices.Contains(leading, words[0]) {
			words, changed = words[1:], true
		}
		if n := suffixOf(words, nameWords); n > 0 && n < len(words) {
			words, changed = words[:len(words)-n], true
		}
		for _, t := range trailing {
			if tw := strings.Fields(t); suffixOf(words, tw) > 0 && len(tw) < len(words) {
				words, changed = words[:len(words)-len(tw)], true
				break
			}
		}
	}
	return strings.Join(words, " ")
}

func prefixOf(words, p []string) int {
	if len(p) == 0 || len(p) > len(words) || !slices.Equal(words[:len(p)], p) {
		return 0
	}
	return len(p)
}

func suffixOf(words, p []string) int {
	if len(p) == 0 || len(p) > len(words) || !slices.Equal(words[len(words)-len(p):], p) {
		return 0
	}
	return len(p)
}

// askedForMore reports whether the user asked for a full answer.
func askedForMore(norm string) bool {
	words := strings.Fields(norm)
	for _, p := range moreAsk {
		for i := 0; i+len(p) <= len(words); i++ {
			if slices.Equal(words[i:i+len(p)], p) {
				return true
			}
		}
	}
	return false
}

// Card answers: the time and the date, said with no model.
var (
	timeAsk = regexp.MustCompile(`^(?:what time is it(?: now| right now)?|(?:whats|what is) the time(?: now| right now)?|(?:tell me|give me|do you have|have you got|got) the time|(?:the |current )?time|what time do you have)$`)
	dateAsk = regexp.MustCompile(`^(?:(?:whats|what is) (?:the date|todays date|the date today|the day|today|the day today)|what day is (?:it|today)(?: today)?|(?:todays |the )?date|what is the day and date|whats the day and date)$`)
)

// reminder answers a reminder or timer to set, cancel or list, read by the
// reminder parser with no model; the daemon keeps them. A request said to
// the character by name is read with the name taken off, else as said, so
// a reminder keeps the user's own spelling.
func (r *Router) reminder(text, norm, c string, at time.Time) (Decision, bool) {
	said := text
	if c != norm {
		said = c
	}
	if rem, line, ok := remind.Parse(said, at, r.c.Location); ok {
		card := CardReminder
		if rem.Kind == remind.KindTimer || rem.Kind == "" && strings.Contains(c, "timer") {
			card = CardTimer
		}
		d := Decision{Kind: Answered, Card: card, Line: line}
		if rem.Kind != "" {
			d.Reminder = &rem
		}
		return d, true
	}
	if k, ok := remind.ParseCancel(c); ok {
		return Decision{Kind: Answered, Card: CardRemindCancel, Cancel: k}, true
	}
	if remind.ParseList(c) {
		return Decision{Kind: Answered, Card: CardRemindList}, true
	}
	return Decision{}, false
}

// clockLine says a time on the 12-hour clock, as the clip composer reads it:
// "It's 3:45."
func clockLine(t time.Time) string {
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	return fmt.Sprintf("It's %d:%02d.", h, t.Minute())
}

// dateLine says the weekday and date: "It's Monday, March 21st."
func dateLine(t time.Time) string {
	d := t.Day()
	suffix := "th"
	if d < 11 || d > 13 {
		switch d % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("It's %s, %s %d%s.", t.Weekday(), t.Month(), d, suffix)
}
