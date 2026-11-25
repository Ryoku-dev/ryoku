// Package guide is the guided walkthrough's state machine. It walks the
// person through a goal one step at a time: it asks for the window to be
// read, points at the step's control, waits for the person to act, watches
// whether the screen changed, and reads the next step, until the reply says
// the goal is done, the person stops, the step ceiling or the read deadline
// ends it. A walkthrough kept from before replays from the keep entry with
// no model read until a step's control is missing.
//
// It is a pure reducer: Handle takes one Event and returns the Effects the
// caller carries out, in order. It reads no clock but the one it is given,
// starts no process and touches no file, socket or picture. The caller
// captures, reads, points, watches the screen and answers with the matching
// event; a reply whose token or strand seq is no longer the one awaited
// changes nothing. Handle is not safe for concurrent use: the caller runs
// one event at a time, and answers a Point with its Pointed before it hands
// the machine anything else.
package guide

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/spec"
)

// Config holds the machine's numbers; FromSpec fills them from the spec.
type Config struct {
	// MaxSteps is the most steps one guide takes, guide.maxSteps.
	MaxSteps int
	// ReadTimeout is how long one read of the window may take before the
	// guide ends, guide.readTimeoutS.
	ReadTimeout time.Duration
	// PlanDone and PlanNext bound the checklist: the steps done and the
	// steps ahead it shows, session.planDoneMax and session.planNextMax.
	PlanDone, PlanNext int
	// Keep is the guide.keep switch: a finished guide whose every step was
	// trusted is offered for keeping only while it is on.
	Keep bool
	// SeqBase is the last guide frame seq sent before this machine; its
	// frames count on from it, so guide after guide keeps one rising seq.
	SeqBase uint64
}

var limits = sync.OnceValue(func() Config {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return FromSpec(s)
})

// FromSpec returns the machine's numbers from the spec's guide and session
// sections, with Keep off and SeqBase 0.
func FromSpec(s *spec.OniSpec) Config {
	return Config{
		MaxSteps:    s.Guide.MaxSteps,
		ReadTimeout: time.Duration(s.Guide.ReadTimeoutS * float64(time.Second)),
		PlanDone:    s.Session.PlanDoneMax,
		PlanNext:    s.Session.PlanNextMax,
	}
}

// normal holds c to what a guide frame may carry: a number past the
// spec's, or none, is the spec's.
func normal(c Config) Config {
	l := limits()
	if c.MaxSteps <= 0 || c.MaxSteps > l.MaxSteps {
		c.MaxSteps = l.MaxSteps
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = l.ReadTimeout
	}
	if c.PlanDone < 0 || c.PlanDone > l.PlanDone {
		c.PlanDone = l.PlanDone
	}
	if c.PlanNext < 0 || c.PlanNext > l.PlanNext {
		c.PlanNext = l.PlanNext
	}
	return c
}

// How a waiting step was advanced: Next on the card, in the app or from the
// CLI; a spoken next from a valid press; or a change the compositor
// reported on the window. There is no click guessing.
const (
	ByNext   = "next"
	ByVoice  = "voice"
	ByChange = "change"
)

// Why a guide ended.
const (
	EndDone    = "done"    // the goal is reached; Line is the closing line
	EndStopped = "stopped" // the person or the daemon stopped it; Why says which
	EndCeiling = "ceiling" // the step ceiling was reached
	EndTimeout = "timeout" // a read outlived ReadTimeout
	EndRefused = "refused" // a step pointed at a private field or asked for an action
	EndFailed  = "failed"  // the window could not be read, or there was no goal
	EndLost    = "lost"    // the strand could not be sent, or its target was cancelled
)

// Event is something the machine is told.
type Event interface{ event() }

// Start begins a guide toward Goal in App, the perched window's app. With
// a kept Entry the steps replay from it; a blank goal with no entry ends at
// once.
type Start struct {
	Goal  string
	App   string
	Entry *keep.Entry
}

// Read answers ReadScreen: the step reply as screenask.ParseStep read it, or
// Err, a *screenask.Refusal for a refused reply, a *Barred for a read the
// route may not take, and anything else for a read that failed.
type Read struct {
	Token    uint64
	Pointing screenask.Pointing
	Err      error
}

// Barred is a model read the planned route may not take for whoever
// started the guide, decided before any picture is taken; the guide ends,
// refused, with Line.
type Barred struct{ Line string }

func (b *Barred) Error() string { return "guide read barred: " + b.Line }

// Looked answers Look: the words read in the window, in fractions of it,
// or Err when the window could not be captured or read.
type Looked struct {
	Token uint64
	Words []screenask.Word
	Err   error
}

// Pointed answers Point: the strand target's seq, or Err when no strand
// could be sent.
type Pointed struct {
	Token uint64
	Seq   uint64
	Err   error
}

// Arrived is the presenter's report that the strand target Seq landed, or
// with Skipped that it could not be drawn; either lets the step wait.
type Arrived struct {
	Seq     uint64
	Skipped string
}

// Cancelled is a strand target that ended without arriving: superseded,
// retracted, its presenter gone, Oni off or the arrival timeout passed.
type Cancelled struct {
	Seq uint64
	Why string
}

// Advance says the person acted on the step, one of By*.
type Advance struct{ By string }

// Circle asks for the ring around the current target to be thrown again.
type Circle struct{}

// Settled answers StartSettle: whether the window changed after the advance.
type Settled struct {
	Token   uint64
	Changed bool
}

// Stop ends the guide; Why is passed on in End.
type Stop struct{ Why string }

// Deadline says the read with Token reached its Until.
type Deadline struct{ Token uint64 }

// Asking says the picture for the read with Token waits on the user's yes
// before it is sent. The read's deadline is held meanwhile, answered with
// SetDeadline and a zero Until; the consent's own expiry bounds the wait.
// A read already past its deadline ends the guide instead.
type Asking struct{ Token uint64 }

// Allowed says the user allowed the picture for the read with Token: the
// read starts again from now with a whole ReadTimeout, answered with
// SetDeadline.
type Allowed struct{ Token uint64 }

// KeepAnswer answers OfferKeep: Keep after the entry was kept, otherwise a
// skip or a lapsed offer.
type KeepAnswer struct{ Keep bool }

func (Start) event()      {}
func (Read) event()       {}
func (Looked) event()     {}
func (Pointed) event()    {}
func (Arrived) event()    {}
func (Cancelled) event()  {}
func (Advance) event()    {}
func (Circle) event()     {}
func (Settled) event()    {}
func (Stop) event()       {}
func (Deadline) event()   {}
func (Asking) event()     {}
func (Allowed) event()    {}
func (KeepAnswer) event() {}

// Effect is something the caller does for the machine.
type Effect interface{ effect() }

// ReadScreen captures the window, asks the model for the next step toward
// Goal and answers with Read. Done lists the steps already done, to be
// fenced in the prompt; Again says the step is read once more because the
// screen did not change. Until is the deadline: answer Deadline then.
type ReadScreen struct {
	Token uint64
	Goal  string
	Done  []string
	Again bool
	Until time.Time
}

// Look captures the window and reads its text on this computer, sending
// nothing anywhere, and answers with Looked by Until or with Deadline.
type Look struct {
	Token uint64
	Until time.Time
}

// Point sends the strand to Box, in fractions of the window, with Mode (and
// Region for a region pen) and answers with Pointed.
type Point struct {
	Token  uint64
	Step   int
	Box    feature.Box
	Region *feature.Box
	Mode   string
	Label  string
}

// Show sends Frame, a valid guide frame.
type Show struct{ Frame feature.GuideFrame }

// Retract rewinds the strand that went out for Seq.
type Retract struct{ Seq uint64 }

// CancelRead abandons the read or look with Token; its late answer is
// dropped.
type CancelRead struct{ Token uint64 }

// SetDeadline moves the deadline of the read with Token to Until, or with
// a zero Until holds it: no Deadline is answered until the next
// SetDeadline.
type SetDeadline struct {
	Token uint64
	Until time.Time
}

// StartSettle watches the window round Box, in fractions of it, against the
// picture the step was read from, for at most the spec's settle frames, and
// answers with Settled.
type StartSettle struct {
	Token uint64
	Box   feature.Box
}

// OfferKeep offers to keep Entry, every step of it trusted; KeepAnswer
// answers it.
type OfferKeep struct{ Entry keep.Entry }

// Hit marks the kept entry Key used: it replayed to the end.
type Hit struct{ Key string }

// Miss records that a step of the kept entry Key was not found.
type Miss struct{ Key string }

// Forget drops the kept entry Key: a step of it now points at a private
// field.
type Forget struct{ Key string }

// End is the guide's end: Reason one of End*, Why the Stop's, and Line what
// to show or say. The caller drops anything the guide still holds, a keep
// offer included.
type End struct {
	Reason string
	Why    string
	Line   string
}

func (ReadScreen) effect()  {}
func (Look) effect()        {}
func (Point) effect()       {}
func (Show) effect()        {}
func (Retract) effect()     {}
func (CancelRead) effect()  {}
func (SetDeadline) effect() {}
func (StartSettle) effect() {}
func (OfferKeep) effect()   {}
func (Hit) effect()         {}
func (Miss) effect()        {}
func (Forget) effect()      {}
func (End) effect()         {}

// what the machine waits on
type waitKind int

const (
	waitNone   waitKind = iota
	waitRead            // a ReadScreen's Read
	waitLook            // a Look's Looked
	waitPoint           // a Point's Pointed
	waitArrive          // the strand's Arrived
	waitSettle          // a StartSettle's Settled
)

// step is one pointed step.
type step struct {
	text, label string
	box         feature.Box
	region      *feature.Box
	mode        string
	// sure: verified against the window and confident, or found again on
	// replay
	sure bool
	// observed: the window changed after it was advanced
	observed bool
	// forced: Next moved past it although the window did not change
	forced bool
	// last: the reply said this step finishes the goal
	last bool
}

func (s step) trusted() bool { return s.sure && s.observed && !s.forced && s.label != "" }

// Machine is one guide.
type Machine struct {
	cfg   Config
	now   func() time.Time
	phase string

	seq   uint64 // frames sent
	token uint64 // tokens issued

	wait    waitKind
	waitTok uint64
	until   time.Time
	// asking: the read's picture waits on the user's yes, its deadline
	// held
	asking bool
	// orphan is the token of a Point left unanswered when the guide ended:
	// its strand is retracted when Pointed comes
	orphan uint64

	out  uint64 // the strand on screen
	held bool   // an advance that came before the strand landed

	goal, app string
	entry     *keep.Entry
	replay    bool // the current step comes from the entry
	replayed  bool // the guide started from the entry
	missed    bool // a replayed step was not found
	refused   bool // a read was refused or answered with no point

	steps     []step
	done      []string
	n         int // the current step, from 1
	again     bool
	unchanged int
	plan      []string
	explained bool // the explaining step has a point
	line      string
}

// New returns an idle machine. now is its only clock.
func New(c Config, now func() time.Time) *Machine {
	if now == nil {
		panic("guide: New needs a clock")
	}
	return &Machine{cfg: normal(c), now: now, phase: feature.GuideIdle}
}

// Phase is the machine's phase, one of feature.GuidePhases but nudging.
func (m *Machine) Phase() string { return m.phase }

// Seq is the seq of the last frame shown, SeqBase before any.
func (m *Machine) Seq() uint64 { return m.cfg.SeqBase + m.seq }

// Handle takes one event and returns what the caller does, in order. An
// event the phase has no use for, or one answering a token or strand that
// is no longer awaited, returns nothing and changes nothing.
func (m *Machine) Handle(ev Event) []Effect {
	switch ev := ev.(type) {
	case Start:
		return m.start(ev)
	case Read:
		return m.onRead(ev)
	case Looked:
		return m.onLooked(ev)
	case Pointed:
		return m.onPointed(ev)
	case Arrived:
		return m.onArrived(ev)
	case Cancelled:
		return m.onCancelled(ev)
	case Advance:
		return m.onAdvance(ev)
	case Circle:
		return m.onCircle()
	case Settled:
		return m.onSettled(ev)
	case Stop:
		return m.onStop(ev)
	case Deadline:
		return m.onDeadline(ev)
	case Asking:
		return m.onAsking(ev)
	case Allowed:
		return m.onAllowed(ev)
	case KeepAnswer:
		return m.onKeepAnswer(ev)
	}
	return nil
}

// Lines the guide shows. They are Oni's own.
var (
	lineNoGoal     = i18n.T("Tell me the task and I will show you each step.")
	lineDone       = i18n.T("That is it. You are done.")
	lineReplayDone = i18n.T("All done, just as you kept it.")
	lineKept       = i18n.T("Kept. Next time I can show you this without asking anyone.")
	lineStopped    = i18n.T("Stopped. Ask again whenever you like.")
	lineCeiling    = i18n.T("This guide has reached its step limit. Ask again to pick up from here.")
	lineTimeout    = i18n.T("The window was too slow to read, so I stopped the guide.")
	lineFailed     = i18n.T("Something went wrong reading the window, so I stopped the guide.")
	lineLost       = i18n.T("I lost the spot I was pointing at, so I stopped the guide.")
	lineUnchanged  = i18n.T("Nothing seemed to change. Try it once more, or press Next to go on.")
	lineNotHere    = i18n.T("I could not find this step on the window. Press Next to look again.")
	lineAnswer     = i18n.T("This step is not something I can point at on this window. Press Next to look again.")
	lineStep       = i18n.T("Use the spot I am pointing at.")
	kickerGuess    = i18n.T("Only a guess from this picture.")
	kickerReplay   = i18n.T("Kept from before")
)

func (m *Machine) next() uint64 {
	m.token++
	return m.token
}

func (m *Machine) cur() *step {
	if m.n < 1 || m.n > len(m.steps) {
		return nil
	}
	return &m.steps[m.n-1]
}

func (m *Machine) start(ev Start) []Effect {
	if m.phase != feature.GuideIdle {
		return nil
	}
	m.goal, m.app = strings.TrimSpace(ev.Goal), strings.TrimSpace(ev.App)
	if e := ev.Entry; e != nil && len(e.Steps) > 0 && len(e.Steps) <= m.cfg.MaxSteps {
		c := *e
		c.Steps = slices.Clone(e.Steps)
		m.entry, m.replay, m.replayed = &c, true, true
		if m.goal == "" {
			m.goal = c.Goal
		}
	}
	if m.goal == "" && m.entry == nil {
		return m.end(EndFailed, "", lineNoGoal)
	}
	m.n = 1
	return m.read(false)
}

// read reads the window for step n: a local look while replaying, a model
// read otherwise.
func (m *Machine) read(again bool) []Effect {
	m.phase = feature.GuideReading
	m.waitTok, m.until, m.asking = m.next(), m.now().Add(m.cfg.ReadTimeout), false
	show := m.show(feature.GuideReading)
	if m.replay {
		m.wait = waitLook
		return []Effect{show, Look{Token: m.waitTok, Until: m.until}}
	}
	m.wait = waitRead
	return []Effect{show, ReadScreen{Token: m.waitTok, Goal: m.goal, Done: slices.Clone(m.done), Again: again, Until: m.until}}
}

func (m *Machine) late() bool { return m.now().After(m.until) }

func (m *Machine) onRead(ev Read) []Effect {
	if m.wait != waitRead || ev.Token != m.waitTok {
		return nil
	}
	// the reply is in, so there is nothing to cancel, late or not
	m.wait, m.asking = waitNone, false
	if m.late() {
		return m.end(EndTimeout, "", lineTimeout)
	}
	if ev.Err != nil {
		var barred *Barred
		if errors.As(ev.Err, &barred) {
			return m.end(EndRefused, "", barred.Line)
		}
		var r *screenask.Refusal
		if !errors.As(ev.Err, &r) {
			return m.end(EndFailed, "", lineFailed)
		}
		if r.Code == screenask.CodeSensitive || r.Code == screenask.CodeAction {
			return m.end(EndRefused, "", r.Line)
		}
		m.refused = true
		return m.explain(r.Line, false)
	}
	p := ev.Pointing
	switch {
	case (p.Kind == screenask.KindElement || p.Kind == screenask.KindBox) && p.Box != nil:
		s := step{
			text:  text(p.Say, lineStep),
			label: p.Label,
			box:   *p.Box,
			mode:  feature.StrandModeLasso,
			sure:  p.Verified && p.Sure,
			last:  p.Done,
		}
		if m.n > len(m.steps) {
			m.steps = append(m.steps, s)
		} else {
			m.steps[m.n-1] = s
		}
		m.plan = m.plan[:0]
		for _, item := range p.Plan {
			if t := text(item.Text, ""); t != "" && len(m.plan) < m.cfg.PlanNext {
				m.plan = append(m.plan, t)
			}
		}
		return m.point(s.mode)
	case p.Kind == screenask.KindNone && p.Done:
		return m.finish(text(p.Say, ""))
	case p.Kind == screenask.KindNone:
		m.refused = true
		return m.explain(text(p.Say, lineNotHere), false)
	case p.Kind == screenask.KindAnswer:
		m.refused = true
		return m.explain(lineAnswer, false)
	}
	m.refused = true
	return m.explain(lineNotHere, false)
}

func (m *Machine) onLooked(ev Looked) []Effect {
	if m.wait != waitLook || ev.Token != m.waitTok {
		return nil
	}
	m.wait = waitNone
	if m.late() {
		return m.end(EndTimeout, "", lineTimeout)
	}
	if ev.Err != nil {
		// the window could not be read here: no fault of the entry, so no
		// miss, but the rest is a model read
		m.missed, m.replay = true, false
		return m.read(false)
	}
	kept := m.entry.Steps[m.n-1]
	found, ok := keep.Locate(kept, ev.Words)
	if found.Refused != "" {
		return append([]Effect{Forget{Key: m.entry.Key}}, m.end(EndRefused, "", screenask.RefusalLine)...)
	}
	if !ok {
		m.missed, m.replay = true, false
		return append([]Effect{Miss{Key: m.entry.Key}}, m.read(false)...)
	}
	// the label still inside the control as it was kept: the control has
	// not moved, so the strand marks all of it as the model read it, not
	// only its words; a control that moved is marked where its words are
	box := found.Box
	if within(found.Box, kept.Box, replaySlack) {
		box = kept.Box
	}
	s := step{text: text(kept.Text, lineStep), label: kept.Label, box: box, mode: kept.Mode, sure: true}
	if s.mode == feature.StrandModeRegion {
		s.region = shift(kept.Region, box.X-kept.Box.X, box.Y-kept.Box.Y)
		if s.region == nil {
			s.mode = feature.StrandModeLasso
		}
	}
	if !slices.Contains([]string{feature.StrandModePoint, feature.StrandModeLasso, feature.StrandModeRegion}, s.mode) {
		s.mode = feature.StrandModeLasso
	}
	if m.n > len(m.steps) {
		m.steps = append(m.steps, s)
	} else {
		m.steps[m.n-1] = s
	}
	m.plan = m.plan[:0]
	for _, later := range m.entry.Steps[m.n:] {
		if t := text(later.Text, ""); t != "" && len(m.plan) < m.cfg.PlanNext {
			m.plan = append(m.plan, t)
		}
	}
	return m.point(s.mode)
}

// shift moves a kept region with its control, nil when it no longer fits
// the window.
func shift(r *feature.Box, dx, dy float64) *feature.Box {
	if r == nil {
		return nil
	}
	b := feature.Box{X: r.X + dx, Y: r.Y + dy, W: r.W, H: r.H}
	if b.X < 0 || b.Y < 0 || b.W <= 0 || b.H <= 0 || b.X+b.W > 1 || b.Y+b.H > 1 {
		return nil
	}
	return &b
}

// replaySlack is how far, in fractions of the window, a replayed label's
// words may reach past the kept control and still read as inside it: the
// text reader's boxes run a pixel or two past the glyphs.
const replaySlack = 0.005

// within says whether a lies inside b grown by slack on every side.
func within(a, b feature.Box, slack float64) bool {
	return b.W > 0 && b.H > 0 && a.X >= b.X-slack && a.Y >= b.Y-slack &&
		a.X+a.W <= b.X+b.W+slack && a.Y+a.H <= b.Y+b.H+slack
}

// point sends the strand to the current step.
func (m *Machine) point(mode string) []Effect {
	s := m.cur()
	m.phase, m.wait, m.waitTok, m.held = feature.GuidePointing, waitPoint, m.next(), false
	p := Point{Token: m.waitTok, Step: m.n, Box: s.box, Mode: mode, Label: s.label}
	if mode == feature.StrandModeRegion && s.region != nil {
		r := *s.region
		p.Region = &r
	}
	return []Effect{p}
}

func (m *Machine) onPointed(ev Pointed) []Effect {
	if m.orphan != 0 && ev.Token == m.orphan {
		m.orphan = 0
		if ev.Err == nil && ev.Seq != 0 {
			return []Effect{Retract{Seq: ev.Seq}}
		}
		return nil
	}
	if m.wait != waitPoint || ev.Token != m.waitTok {
		return nil
	}
	m.wait = waitNone
	if ev.Err != nil || ev.Seq == 0 {
		return m.end(EndLost, "", lineLost)
	}
	m.out, m.wait, m.waitTok = ev.Seq, waitArrive, ev.Seq
	return []Effect{m.show(feature.GuidePointing)}
}

func (m *Machine) onArrived(ev Arrived) []Effect {
	if m.wait != waitArrive || ev.Seq != m.waitTok {
		return nil
	}
	m.wait, m.phase = waitNone, feature.GuideWaiting
	if m.held {
		m.held = false
		return m.settle()
	}
	return []Effect{m.show(feature.GuideWaiting)}
}

func (m *Machine) onCancelled(ev Cancelled) []Effect {
	if m.wait != waitArrive || ev.Seq != m.waitTok {
		return nil
	}
	// the strand is gone or belongs to someone else now: nothing to retract
	m.out = 0
	return m.end(EndLost, "", lineLost)
}

func (m *Machine) onAdvance(ev Advance) []Effect {
	if ev.By != ByNext && ev.By != ByVoice && ev.By != ByChange {
		return nil
	}
	switch m.phase {
	case feature.GuidePointing:
		m.held = true
		return nil
	case feature.GuideWaiting:
		return m.settle()
	case feature.GuideExplaining:
		switch {
		case m.explained && ev.By == ByChange:
			return m.settle()
		case m.explained:
			m.cur().forced = true
			return m.stepDone()
		case ev.By == ByChange:
			return nil
		}
		return append(m.retract(), m.read(true)...)
	}
	return nil
}

// settle watches whether the window changed after the step was advanced.
func (m *Machine) settle() []Effect {
	m.phase, m.wait, m.waitTok = feature.GuideReading, waitSettle, m.next()
	return []Effect{m.show(feature.GuideReading), StartSettle{Token: m.waitTok, Box: m.cur().box}}
}

func (m *Machine) onSettled(ev Settled) []Effect {
	if m.wait != waitSettle || ev.Token != m.waitTok {
		return nil
	}
	m.wait = waitNone
	s := m.cur()
	if ev.Changed {
		s.observed = true
		return m.stepDone()
	}
	m.unchanged++
	switch {
	case m.unchanged >= 2:
		return m.explain(lineUnchanged, true)
	case s.sure:
		m.again = true
		return m.point(s.mode)
	}
	return m.read(true)
}

func (m *Machine) onCircle() []Effect {
	switch {
	case m.phase == feature.GuideWaiting,
		m.phase == feature.GuideExplaining && m.explained && m.out != 0:
		return m.point(feature.StrandModeLasso)
	}
	return nil
}

// stepDone moves past the current step: the guide finishes, ends at the
// ceiling, or reads the next step.
func (m *Machine) stepDone() []Effect {
	s := m.cur()
	m.done = append(m.done, s.text)
	if s.last || (m.replay && m.n == len(m.entry.Steps)) {
		return m.finish("")
	}
	if m.n >= m.cfg.MaxSteps {
		return m.end(EndCeiling, "", lineCeiling)
	}
	eff := m.retract()
	m.n++
	m.unchanged, m.again, m.explained = 0, false, false
	m.plan = m.plan[:0]
	return append(eff, m.read(false)...)
}

// explain shows line for the current step and waits for Next. pointed says
// the step's strand is still out and the line is about it.
func (m *Machine) explain(line string, pointed bool) []Effect {
	var eff []Effect
	if !pointed {
		eff = m.retract()
	}
	m.phase, m.line, m.explained = feature.GuideExplaining, line, pointed
	return append(eff, m.show(feature.GuideExplaining))
}

func (m *Machine) retract() []Effect {
	if m.out == 0 {
		return nil
	}
	seq := m.out
	m.out = 0
	return []Effect{Retract{Seq: seq}}
}

// finish ends a guide that reached its goal, offering to keep it when every
// step was trusted.
func (m *Machine) finish(say string) []Effect {
	eff := m.retract()
	if m.replayed && !m.missed {
		m.line = lineReplayDone
		m.phase = feature.GuideDone
		eff = append(eff, Hit{Key: m.entry.Key}, m.show(feature.GuideDone))
		return append(eff, End{Reason: EndDone, Line: m.line})
	}
	m.line = text(say, lineDone)
	if e, ok := m.keepable(); ok {
		m.phase = feature.GuideKeeping
		return append(eff, m.show(feature.GuideKeeping), OfferKeep{Entry: e})
	}
	m.phase = feature.GuideDone
	return append(eff, m.show(feature.GuideDone), End{Reason: EndDone, Line: m.line})
}

// keepable is the entry to offer: Keep on, a guide that ran from the model
// alone with no refusal, and every step trusted.
func (m *Machine) keepable() (keep.Entry, bool) {
	if !m.cfg.Keep || m.replayed || m.refused || m.app == "" || len(m.steps) == 0 {
		return keep.Entry{}, false
	}
	at := m.now()
	e := keep.Entry{Key: keep.KeyFor(m.app, m.goal), App: m.app, Goal: m.goal, Kept: at, Used: at}
	for _, s := range m.steps {
		if !s.trusted() {
			return keep.Entry{}, false
		}
		k := keep.Step{Text: s.text, Label: s.label, Box: s.box, Mode: s.mode, Trusted: true}
		if s.region != nil {
			r := *s.region
			k.Region = &r
		}
		e.Steps = append(e.Steps, k)
	}
	return e, true
}

func (m *Machine) onKeepAnswer(ev KeepAnswer) []Effect {
	if m.phase != feature.GuideKeeping {
		return nil
	}
	if ev.Keep {
		m.line = lineKept
	}
	m.phase = feature.GuideDone
	return []Effect{m.show(feature.GuideDone), End{Reason: EndDone, Line: m.line}}
}

func (m *Machine) onStop(ev Stop) []Effect {
	switch m.phase {
	case feature.GuideIdle, feature.GuideDone, feature.GuideWinding:
		return nil
	case feature.GuideKeeping:
		return m.onKeepAnswer(KeepAnswer{})
	}
	return m.end(EndStopped, ev.Why, lineStopped)
}

func (m *Machine) onDeadline(ev Deadline) []Effect {
	if (m.wait != waitRead && m.wait != waitLook) || ev.Token != m.waitTok || m.asking {
		return nil
	}
	return m.end(EndTimeout, "", lineTimeout)
}

// onAsking holds the read's deadline while its picture waits on a yes, so
// a slow yes does not end the guide; a read already late ends it.
func (m *Machine) onAsking(ev Asking) []Effect {
	if m.wait != waitRead || ev.Token != m.waitTok || m.asking {
		return nil
	}
	if m.late() {
		return m.end(EndTimeout, "", lineTimeout)
	}
	m.asking = true
	return []Effect{SetDeadline{Token: ev.Token}}
}

// onAllowed gives the read whose picture was just allowed a whole
// ReadTimeout from now.
func (m *Machine) onAllowed(ev Allowed) []Effect {
	if m.wait != waitRead || ev.Token != m.waitTok || !m.asking {
		return nil
	}
	m.asking, m.until = false, m.now().Add(m.cfg.ReadTimeout)
	return []Effect{SetDeadline{Token: ev.Token, Until: m.until}}
}

// end unwinds whatever is out and ends the guide: a stop winds back, any
// other end shows its line.
func (m *Machine) end(reason, why, line string) []Effect {
	var eff []Effect
	switch m.wait {
	case waitRead, waitLook:
		eff = append(eff, CancelRead{Token: m.waitTok})
	case waitPoint:
		m.orphan = m.waitTok
	}
	m.wait, m.held, m.asking = waitNone, false, false
	eff = append(eff, m.retract()...)
	m.line = line
	if reason == EndStopped {
		m.phase = feature.GuideWinding
	} else {
		m.phase = feature.GuideDone
	}
	return append(eff, m.show(m.phase), End{Reason: reason, Why: why, Line: line})
}

// show builds the frame for phase from the machine's state.
func (m *Machine) show(phase string) Effect {
	m.seq++
	f := feature.GuideFrame{Seq: m.cfg.SeqBase + m.seq, Phase: phase}
	stop := []string{feature.GuideActionStop}
	switch phase {
	case feature.GuideReading:
		f.Step, f.Kicker, f.Replay, f.Actions = m.n, m.stepKicker(), m.replay, stop
		f.Plan = m.checklist("")
	case feature.GuidePointing, feature.GuideWaiting:
		s := m.cur()
		f.Step, f.Kicker, f.Text, f.Replay, f.Target = m.n, m.pointKicker(s), s.text, m.replay, m.out
		f.Plan = m.checklist(s.text)
		f.Actions = []string{feature.GuideActionNext, feature.GuideActionStop, feature.GuideActionCircle}
	case feature.GuideExplaining:
		f.Step, f.Kicker, f.Text, f.Replay = m.n, m.stepKicker(), text(m.line, lineNotHere), m.replay
		f.Actions = []string{feature.GuideActionNext, feature.GuideActionStop}
		if m.explained && m.out != 0 {
			f.Target = m.out
			f.Actions = append(f.Actions, feature.GuideActionCircle)
			f.Plan = m.checklist(m.cur().text)
		} else {
			f.Plan = m.checklist("")
		}
	case feature.GuideKeeping:
		f.Step = max(1, len(m.steps))
		f.Actions = []string{feature.GuideActionKeep, feature.GuideActionSkip}
	case feature.GuideDone:
		f.Step, f.Text = min(len(m.done), m.cfg.MaxSteps), text(m.line, lineDone)
		f.Replay = m.replayed && !m.missed
	}
	return Show{Frame: f}
}

func (m *Machine) stepKicker() string {
	if m.replay {
		return kickerReplay
	}
	return i18n.Tf("Step %d", m.n)
}

func (m *Machine) pointKicker(s *step) string {
	switch {
	case m.again:
		return i18n.Tf("Step %d · again", m.n)
	case m.replay:
		return kickerReplay
	case !s.sure:
		return kickerGuess
	}
	return i18n.Tf("Step %d", m.n)
}

// checklist is the steps done, the current one and the ones ahead, nil
// when there is nothing to list.
func (m *Machine) checklist(current string) *feature.GuidePlan {
	var p feature.GuidePlan
	if m.cfg.PlanDone > 0 {
		for _, d := range m.done[max(0, len(m.done)-m.cfg.PlanDone):] {
			p.Done = append(p.Done, text(d, lineStep))
		}
	}
	p.Current = current
	if current != "" && len(m.plan) > 0 {
		p.Next = slices.Clone(m.plan)
	}
	if p.Done == nil && p.Current == "" && p.Next == nil {
		return nil
	}
	return &p
}

// text is s on one line with no control or format runes, at most
// feature.MaxGuideText runes, or fallback when nothing is left.
func text(s, fallback string) string {
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	}), " ")
	if utf8.RuneCountInString(s) > feature.MaxGuideText {
		s = strings.TrimSpace(string([]rune(s)[:feature.MaxGuideText]))
	}
	if s == "" {
		return fallback
	}
	return s
}
