// Package route decides what one finished voice utterance does, in a fixed
// order: drop Oni's own speech heard back, answer the time or the date
// from local lines, read a reminder or timer to set, cancel or list,
// dismiss (which also stops a running guide), answer a
// pending consent card, move a running guide on, small talk from the
// character's lines, start a guide for a question about the perched
// window, a memory command (forget, correct, what Oni knows) or a recall
// question, then the Reflex and Hands slots, and last the slow path, an
// Ask turn the daemon runs read-only.
//
// What may answer consent, move a guide on or reach Hands is decided by
// where the utterance came from, never by its words: only a Press can, and
// only an utterance from the talk chord or the talk toggle makes a Press.
// Wake phrase and open-mic speech cannot. Any source may dismiss.
//
// The package is pure: it starts nothing, keeps no transcript, logs nothing
// and calls the daemon only through the handlers it is given.
package route

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/note"
	"ryoku-rashin/internal/oni/recall"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/voice/turn"
	"ryoku-rashin/internal/oni/voice/wake"
	"ryoku-rashin/internal/protocol"
)

// Source is what opened the mic for an utterance.
type Source uint8

const (
	// Chord is the talk chord held down.
	Chord Source = iota + 1
	// Toggle is the talk toggle tapped in the app, presenter or CLI.
	Toggle
	// Wake is a session the wake phrase opened.
	Wake
	// Open is open mic.
	Open
)

func (s Source) String() string {
	switch s {
	case Chord:
		return "chord"
	case Toggle:
		return "toggle"
	case Wake:
		return "wake"
	case Open:
		return "open"
	}
	return "unknown"
}

// pressed reports whether a person pressed something for this utterance.
func (s Source) pressed() bool { return s == Chord || s == Toggle }

// Press proves that a person pressed the talk chord or tapped the talk
// toggle for the utterance a handler is given. Only Route makes a valid
// Press, and only for Chord and Toggle; the zero Press is never valid, so a
// handler that checks Valid cannot be reached from wake or open mic.
type Press struct {
	source Source
	turn   uint64
}

// Valid reports whether p came from Route for a pressed source.
func (p Press) Valid() bool { return p.source.pressed() && p.turn != 0 }

// Source is the pressed source.
func (p Press) Source() Source { return p.source }

// Turn is the turn the press began.
func (p Press) Turn() uint64 { return p.turn }

// press returns the Press for u, or false for wake and open mic.
func press(u Utterance) (Press, bool) {
	if !u.Source.pressed() || u.Turn == 0 {
		return Press{}, false
	}
	return Press{source: u.Source, turn: u.Turn}, true
}

// Utterance is one finished, tidied transcript.
type Utterance struct {
	Text   string
	Source Source
	// Turn is the turn the utterance belongs to (turn.Turn.ID at capture).
	Turn uint64
	// At is when the transcript was final; local answers read the time
	// from it.
	At time.Time
}

// Reflex is the slot for instant local commands: it plans, and never acts.
// Route hands it the utterance with a call of the character's name taken
// off, and p, which is valid only for the talk chord or the talk toggle.
// From open mic it is reached only when the utterance calls the character
// by name. A plan holding an intent that needs a Press, without one, is
// turned into NeedsPress before the daemon sees it.
type Reflex func(u Utterance, p Press) (Decision, bool)

// Hands is the slot for Hands in the authorized browser tab. Route calls
// it only with a valid Press, after Reflex and before the slow path; it
// reports whether it took the utterance (a start with an anchor phrase, or
// a follow-up within the follow-up window).
type Hands func(p Press, text string) bool

// Consent answers the firm consent card on screen.
type Consent interface {
	// Pending reports whether a consent card waits for an answer.
	Pending() bool
	// Answer answers it. Route calls it only with a valid Press.
	Answer(p Press, allow bool) error
}

// Guide is the running guide, or the offer of one, that speech can move.
type Guide interface {
	// Active reports whether a guide runs or an offer to start one waits
	// on Next.
	Active() bool
	// Next tells it the step is done, or takes the offer. Route calls it
	// only with a valid Press.
	Next(p Press) error
	// Circle throws the strand round the step's target again.
	Circle() error
	// Stop ends it. Route calls it on every dismissal, from any source.
	Stop()
}

// Lines are what Oni says without a model. Ack is the character's
// acknowledgements; the others fall back to Hyakki's own when empty. A line
// that breaks the voice discipline (one sentence of at most MaxLeadWords
// words) is left out.
type Lines struct {
	Ack, Greet, Thanks, Fine []string
}

// Config is a Router's fixed parts.
type Config struct {
	// Turn is the voice loop's turn state; required.
	Turn *turn.Turn
	// Name is the character's display name, taken off an utterance that
	// addresses it and said by "who are you".
	Name  string
	Lines Lines
	// Location is the zone local answers say the time in; nil is local.
	Location *time.Location
	// Learn lets voice asks teach memory. Off by default: transcripts
	// are noisy.
	Learn bool

	Reflex  Reflex
	Hands   Hands
	Consent Consent
	Guide   Guide
	// HandsStop ends a running Hands task. Route calls it on every
	// dismissal, from any source, and for every pressed utterance the
	// Hands slot did not take.
	HandsStop func()
	// HandsCard reports whether Hands waits on something only a button
	// answers: a handoff card, or a step's picture question. While it
	// does, a yes or no from any source, the talk chord included, answers
	// nothing and gets FirmNeedsButtonLine.
	HandsCard func() bool
	// Perched reports whether Oni is perched on a window with guides on:
	// a question about doing something on screen then starts a guide
	// instead of an Ask. Nil is never.
	Perched func() bool
	// Memory reports whether spoken memory commands are on: forget,
	// correct and what Oni knows. Off, or nil, they are ordinary
	// questions. A forget or a correction needs a Press.
	Memory func() bool
	// Recall reports whether recall questions are on: what the user
	// asked before or keeps in notes. Off, or nil, they are ordinary
	// questions.
	Recall func() bool
}

// Router routes utterances. It is safe for concurrent use.
type Router struct {
	c     Config
	name  string
	lines Lines

	mu   sync.Mutex
	next map[*[]string]int
}

// New returns a Router.
func New(c Config) (*Router, error) {
	if c.Turn == nil {
		return nil, errors.New("route: a turn is required")
	}
	if c.Location == nil {
		c.Location = time.Local
	}
	r := &Router{c: c, name: wake.Normalize(c.Name), next: map[*[]string]int{}}
	r.lines = Lines{
		Ack:    fitting(c.Lines.Ack, nil),
		Greet:  fitting(c.Lines.Greet, defaultGreet),
		Thanks: fitting(c.Lines.Thanks, defaultThanks),
		Fine:   fitting(c.Lines.Fine, defaultFine),
	}
	return r, nil
}

func fitting(lines, fallback []string) []string {
	var out []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); Fits(l) {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

// pick returns the next line of a list in turn.
func (r *Router) pick(list *[]string) string {
	if len(*list) == 0 {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.next[list]
	r.next[list] = i + 1
	return (*list)[i%len(*list)]
}

// Kind is what Route decided.
type Kind uint8

const (
	// Dropped: nothing happens; Why says why.
	Dropped Kind = iota
	// Answered: Line is the answer, from no model. Card is set for the
	// time, the date, and a reminder or timer set, cancelled or listed.
	Answered
	// Dismissed: stop speaking and close the voice session, silently.
	Dismissed
	// ConsentAnswered: the pending card was answered; Allow says how.
	ConsentAnswered
	// NeedsPress: a consent answer or a guide's next came from wake or
	// open mic; Line asks for the key. The card and the guide are
	// untouched.
	NeedsPress
	// Reflexed: the Reflex slot took it.
	Reflexed
	// HandsTook: the Hands slot took it.
	HandsTook
	// Asked: run Ask as an Ask turn; Ack, when set, is said first.
	Asked
	// Guided: start a guide toward Goal on the perched window; Ack, when
	// set, is said first. Its pictures are asked about as any are, and
	// only a Press answers that.
	Guided
	// GuideNexted: the running guide was told its step is done, or the
	// offer of one taken.
	GuideNexted
	// GuideCircled: the running guide's strand was thrown again.
	GuideCircled
	// Noted: a memory command; Note says which, and the daemon shows the
	// card before anything is forgotten or changed.
	Noted
	// Recalled: a recall question; Recall says where it looks and what
	// about. Ack, when set, is said first.
	Recalled
)

func (k Kind) String() string {
	return [...]string{"dropped", "answered", "dismissed", "consent-answered", "needs-press", "reflexed", "hands", "asked", "guided", "guide-nexted", "guide-circled", "noted", "recalled"}[k]
}

// Why is why an utterance was dropped.
type Why uint8

const (
	WhyNone Why = iota
	// WhyEmpty: nothing was said.
	WhyEmpty
	// WhyStale: the utterance belongs to an older turn.
	WhyStale
	// WhyHeardBack: the words are Oni's own line heard back.
	WhyHeardBack
	// WhySource: the source is not one Route knows.
	WhySource
	// WhyNoCard: a yes or no with no consent card waiting.
	WhyNoCard
	// WhyNoGuide: the guide a next or circle was for is gone.
	WhyNoGuide
)

// Card is which local answer was given.
type Card uint8

const (
	CardNone Card = iota
	CardTime
	CardDate
	// CardTimer and CardReminder: the daemon keeps Decision.Reminder and
	// says Line, which confirms it. With Reminder nil, Line says why it
	// cannot be kept: too far ahead, too long, or a time gone by.
	CardTimer
	CardReminder
	// CardRemindCancel: the daemon takes away what Decision.Cancel names
	// and says what went; Line is empty.
	CardRemindCancel
	// CardRemindList: the daemon reads out the reminders and timers kept;
	// Line is empty.
	CardRemindList
)

// Decision is what Route decided. It holds no transcript except Ask's
// question, Guided's goal and the arguments of a Reflexed plan.
type Decision struct {
	Kind Kind
	Why  Why
	Card Card
	// Line is what Oni says now.
	Line string
	// Ack is the acknowledgement said before a slow answer: at most one
	// per press, voice.turn.ackGapMs apart.
	Ack   string
	Allow bool
	// Reminder is the reminder or timer a CardReminder or CardTimer
	// answer sets: its own words and times, not the sentence.
	Reminder *remind.Reminder
	// Cancel is what a CardRemindCancel answer takes away.
	Cancel remind.CancelKind
	Ask    *Ask
	// Goal is what a Guided guide walks toward: the question as said, at
	// most protocol.OniGuideGoalMax runes.
	Goal string
	// Plan is what a Reflexed decision asks for, when the slot planned
	// actions; the daemon carries it out.
	Plan *reflex.Plan
	// Press is the Press a Reflexed decision came with; valid only for
	// the talk chord or the talk toggle.
	Press Press
	// Note is a Noted decision's command; Press, valid only for a pressed
	// source, comes with it.
	Note *note.Command
	// Recall is a Recalled decision's question.
	Recall *recall.Query
}

// Origin is the session origin of a voice ask.
const Origin = protocol.OriginVoice

// NeedsPressLine is said when wake or open-mic speech tries to answer a
// consent card.
const NeedsPressLine = "Press the key to answer."

// NextNeedsPressLine is said when wake or open-mic speech tries to move a
// guide on.
const NextNeedsPressLine = "Press Next or the key."

// ReflexNeedsPressLine is said when wake or open-mic speech asks for a
// quick command only a press may start: a note, a routine or a setting.
const ReflexNeedsPressLine = "Press the key for that."

// FirmNeedsButtonLine is said for a yes or no, from any source, while Hands
// waits on a card: no spoken answer stands in for the button or for the
// person's own press on the page.
const FirmNeedsButtonLine = "Use the button for that."

// Ask is the slow path: an Ask turn the daemon runs through the same route
// and disclosure as a typed Ask, in a new read-only session of origin
// Origin.
type Ask struct {
	Question string
	Source   Source
	Turn     uint64
	// Long is set when the user asked for more: Prompt and Spoken allow
	// more than one sentence.
	Long bool
	// Learn lets the session teach memory; Config.Learn, off by default.
	Learn bool
}

// Session is the new session the ask runs in: an Ask, read-only, origin
// Origin, using memory but never learned from unless Learn.
func (a Ask) Session() protocol.SessionNewParams {
	memory := protocol.MemoryUseOnly
	if a.Learn {
		memory = protocol.MemoryRemember
	}
	return protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: Origin, Mode: protocol.ApprovalsReadOnly, Memory: memory}
}

// Params is the ask call into the session Session created.
func (a Ask) Params(session string) protocol.AskParams {
	return protocol.AskParams{Question: a.Question, Session: session, Origin: Origin}
}

// Route decides what u does. A pressed utterance that Hands does not take
// stops a running Hands task, as every dismissal does.
func (r *Router) Route(u Utterance) Decision {
	d := r.route(u)
	if _, pressed := press(u); pressed && r.c.HandsStop != nil {
		switch d.Kind {
		case Dropped, HandsTook, NeedsPress, Dismissed:
		default:
			r.c.HandsStop()
		}
	}
	return d
}

func (r *Router) route(u Utterance) Decision {
	text := strings.TrimSpace(u.Text)
	switch {
	case u.Source < Chord || u.Source > Open:
		return Decision{Why: WhySource}
	case !r.c.Turn.Accept(u.Turn):
		return Decision{Why: WhyStale}
	case text == "":
		return Decision{Why: WhyEmpty}
	}
	at := u.At
	if at.IsZero() {
		at = time.Now()
	}
	if r.c.Turn.Heard(text, at) {
		return Decision{Why: WhyHeardBack}
	}
	norm := wake.Normalize(text)
	if norm == "" {
		return Decision{Why: WhyEmpty}
	}
	c := core(norm, r.name)
	is := func(list []string) bool { return slices.Contains(list, c) || slices.Contains(list, norm) }

	switch {
	case timeAsk.MatchString(c):
		return Decision{Kind: Answered, Card: CardTime, Line: clockLine(at.In(r.c.Location))}
	case dateAsk.MatchString(c):
		return Decision{Kind: Answered, Card: CardDate, Line: dateLine(at.In(r.c.Location))}
	}
	if d, ok := r.reminder(text, norm, c, at); ok {
		return d
	}
	if is(dismissals) {
		if r.c.Guide != nil {
			r.c.Guide.Stop()
		}
		if r.c.HandsStop != nil {
			r.c.HandsStop()
		}
		return Decision{Kind: Dismissed}
	}
	if allow, deny := is(allows), is(denies); allow || deny {
		if r.c.HandsCard != nil && r.c.HandsCard() {
			return Decision{Kind: NeedsPress, Line: FirmNeedsButtonLine}
		}
		if r.c.Consent == nil || !r.c.Consent.Pending() {
			return Decision{Why: WhyNoCard}
		}
		p, ok := press(u)
		if !ok {
			return Decision{Kind: NeedsPress, Line: NeedsPressLine}
		}
		if err := r.c.Consent.Answer(p, allow); err != nil {
			return Decision{Why: WhyNoCard}
		}
		return Decision{Kind: ConsentAnswered, Allow: allow}
	}
	if next, circle := is(guideNexts), is(guideCircles); (next || circle) && r.c.Guide != nil && r.c.Guide.Active() {
		if circle {
			if err := r.c.Guide.Circle(); err != nil {
				return Decision{Why: WhyNoGuide}
			}
			return Decision{Kind: GuideCircled}
		}
		p, ok := press(u)
		if !ok {
			return Decision{Kind: NeedsPress, Line: NextNeedsPressLine}
		}
		if err := r.c.Guide.Next(p); err != nil {
			return Decision{Why: WhyNoGuide}
		}
		return Decision{Kind: GuideNexted}
	}
	switch {
	case is(greets):
		return Decision{Kind: Answered, Line: r.pick(&r.lines.Greet)}
	case is(thanks):
		return Decision{Kind: Answered, Line: r.pick(&r.lines.Thanks)}
	case is(howRU):
		return Decision{Kind: Answered, Line: r.pick(&r.lines.Fine)}
	case is(whoRU) && r.c.Name != "" && Fits("I'm "+r.c.Name+"."):
		return Decision{Kind: Answered, Line: "I'm " + r.c.Name + "."}
	}
	if r.c.Perched != nil && screenAsk.MatchString(c) && r.c.Perched() {
		return Decision{Kind: Guided, Ack: r.ack(at), Goal: goalOf(text)}
	}
	if d, ok := r.memory(u, text, at); ok {
		return d
	}
	if r.c.Reflex != nil {
		if ru, ok := r.reflexUtterance(u, text); ok {
			p, pressed := press(u)
			if d, ok := r.c.Reflex(ru, p); ok {
				if !pressed && d.Plan != nil && needsPress(*d.Plan) {
					return Decision{Kind: NeedsPress, Line: ReflexNeedsPressLine}
				}
				d.Kind, d.Press = Reflexed, p
				return d
			}
		}
	}
	if p, ok := press(u); ok && r.c.Hands != nil && r.c.Hands(p, text) {
		return Decision{Kind: HandsTook, Ack: r.ack(at)}
	}
	return Decision{Kind: Asked, Ack: r.ack(at), Ask: &Ask{Question: text, Source: u.Source, Turn: u.Turn, Long: askedForMore(norm), Learn: r.c.Learn}}
}

// ack is the acknowledgement for a slow answer, when the turn allows one.
func (r *Router) ack(at time.Time) string {
	if len(r.lines.Ack) == 0 || !r.c.Turn.Ack(at) {
		return ""
	}
	return r.pick(&r.lines.Ack)
}

// goalOf is a guide's goal from what was said: at most
// protocol.OniGuideGoalMax runes.
func goalOf(text string) string {
	if rs := []rune(text); len(rs) > protocol.OniGuideGoalMax {
		text = strings.TrimSpace(string(rs[:protocol.OniGuideGoalMax]))
	}
	return text
}

// reflexUtterance is u as the Reflex slot hears it: a call of the
// character's name taken off. Open-mic speech reaches the slot only when it
// called the character by name; nothing left after the name reaches it
// never.
func (r *Router) reflexUtterance(u Utterance, text string) (Utterance, bool) {
	rest, addressed := text, false
	if r.c.Name != "" {
		rest, addressed = reflex.Unaddress(text, []string{r.c.Name})
	}
	if u.Source == Open && !addressed || strings.TrimSpace(rest) == "" {
		return Utterance{}, false
	}
	u.Text = rest
	return u, true
}

// memory routes a memory command or a recall question, when it is one and
// its switch is on. Like the Reflex slot it hears the utterance with a call
// of the character's name taken off, and open-mic speech only when it
// called the character by name. A forget or a correction from wake or open
// mic gets the key line and changes nothing.
func (r *Router) memory(u Utterance, text string, at time.Time) (Decision, bool) {
	mu, ok := r.reflexUtterance(u, text)
	if !ok {
		return Decision{}, false
	}
	if r.c.Memory != nil {
		if cmd, ok := note.Parse(mu.Text); ok && r.c.Memory() {
			p, pressed := press(u)
			if cmd.Writes() && !pressed {
				return Decision{Kind: NeedsPress, Line: ReflexNeedsPressLine}, true
			}
			return Decision{Kind: Noted, Note: &cmd, Press: p}, true
		}
	}
	if r.c.Recall != nil {
		if q, ok := recall.Detect(mu.Text); ok && r.c.Recall() {
			return Decision{Kind: Recalled, Ack: r.ack(at), Recall: &q}, true
		}
	}
	return Decision{}, false
}

// needsPress reports whether p holds an intent only a Press may start, or
// one the registry does not know.
func needsPress(p reflex.Plan) bool {
	for _, a := range p.Actions {
		if info, ok := reflex.Lookup(a.Intent); !ok || info.PressOnly {
			return true
		}
	}
	return false
}
