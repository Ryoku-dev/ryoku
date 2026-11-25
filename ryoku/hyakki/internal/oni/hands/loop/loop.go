// Package loop is the Hands task's state machine: it drives one task in
// one scope (the browser tab the person authorized, one live window they
// picked, or the separate nested desktop Hyakki owns), a step at a time,
// until the model says the goal is done, asks the person something, or the
// task ends by budget, repetition, refusal, expiry or stop.
//
// Every step goes the same way: the tab is read, the model is asked for
// one action, and the action is sorted by the consent rules. A safe step
// is announced and run. Anything else that is not refused is a handoff:
// Hands points at the control with a card and waits for the person to
// press it themselves; the card ends only when they did, on a stop, or
// when it lapses. There is no answer that makes Hands press it, so no Act
// is ever issued for a step the rules hand off or refuse.
//
// Outside a browser tab every action is bracketed: the caller checks the
// window (or the nested desktop) before and after it, and the action runs
// only after a check that passed. Text goes in chunks of at most
// hands.app.typeChunkRunes runes, each one checked after, and is then read
// back from the screen; a read-back that could not tell (Unknown, the zero
// value) never passes, and a second one ends the task. Any check that fails,
// and any Away (another window, a title or geometry change, a key the
// person pressed), pauses the task: the action in flight is cut, the rest of
// its text is dropped and never typed anywhere, and only the Resume button
// goes on, with a fresh look. A target that closed ends the task. After a
// restart the caller sends Held: the goal was not kept, so nothing resumes,
// and only a Start from the button with the goal said again begins a task.
//
// It is a pure reducer: Handle takes one Event and returns the Effects the
// caller carries out, in order. It reads no clock but the one it is given,
// starts no process and touches no file, socket or page. An answer whose
// token is no longer awaited changes nothing. After a Stop it answers
// nothing at all. Handle is not safe for concurrent use.
package loop

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/hands/apprules"
	"ryoku-rashin/internal/oni/hands/consent"
	"ryoku-rashin/internal/oni/hands/proto"
	"ryoku-rashin/internal/oni/spec"
)

// Config holds the machine's numbers; FromSpec fills them from the spec.
type Config struct {
	// Steps is a task's step budget, hands.budget.steps; Extra more steps
	// are allowed only while no step was refused or rejected,
	// hands.budget.extra.
	Steps, Extra int
	// RepeatLimit is how many times in a row the same action on the same
	// control may be asked for: the request that reaches it ends the task
	// without running, hands.repeatLimit.
	RepeatLimit int
	// DoneRejections is how many "done" replies the page may contradict
	// before the task ends unconfirmed, hands.doneRejections.
	DoneRejections int
	// HistoryCarry is how many past steps go with each model request and
	// into a follow-up, hands.historyCarry.
	HistoryCarry int
	// Resume is how long a paused task may be resumed, hands.resumeS.
	Resume time.Duration
	// FollowUp is how long after a finished task a new goal continues it,
	// hands.followUpS.
	FollowUp time.Duration
	// CardTTL is how long a handoff card waits, hands.consentTtlS.
	CardTTL time.Duration
	// AskTimeout bounds one model request, hands.readTimeoutS.
	AskTimeout time.Duration
	// PageTimeout bounds one read of the tab, hands.actMs.
	PageTimeout time.Duration
	// ActTimeout bounds one action with its read-back, hands.actMs plus
	// hands.readbackMs.
	ActTimeout time.Duration
	// MaxNameRunes and MaxFillRunes bound a control name in a line and one
	// fill, hands.page.maxNameRunes and hands.page.maxFillRunes.
	MaxNameRunes, MaxFillRunes int
	// VerifyTimeout bounds one check of the window before or after an
	// action outside a tab, hands.app.verifyMs.
	VerifyTimeout time.Duration
	// ChunkTimeout bounds typing one chunk, hands.app.chunkMs.
	ChunkTimeout time.Duration
	// ReadbackTimeout bounds one read-back of typed text from the screen,
	// hands.app.readbackMs.
	ReadbackTimeout time.Duration
	// TypeChunkRunes and MaxTypeRunes bound one chunk and one type action,
	// hands.app.typeChunkRunes and hands.app.maxTypeRunes.
	TypeChunkRunes, MaxTypeRunes int
	// Base is the last token the caller issued before this machine; tokens
	// and card ids count on from it.
	Base uint64
}

var limits = sync.OnceValue(func() Config {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return FromSpec(s)
})

// FromSpec returns the machine's numbers from the spec's hands section,
// with Base 0.
func FromSpec(s *spec.OniSpec) Config {
	h := s.Hands
	return Config{
		Steps:          h.Budget.Steps,
		Extra:          h.Budget.Extra,
		RepeatLimit:    h.RepeatLimit,
		DoneRejections: h.DoneRejections,
		HistoryCarry:   h.HistoryCarry,
		Resume:         h.Resume(),
		FollowUp:       h.FollowUp(),
		CardTTL:        h.ConsentTTL(),
		AskTimeout:     h.ReadTimeout(),
		PageTimeout:    h.Act(),
		ActTimeout:     h.Act() + h.Readback(),
		MaxNameRunes:   h.Page.MaxNameRunes,
		MaxFillRunes:   h.Page.MaxFillRunes,

		VerifyTimeout:   h.App.Verify(),
		ChunkTimeout:    h.App.Chunk(),
		ReadbackTimeout: h.App.Readback(),
		TypeChunkRunes:  h.App.TypeChunkRunes,
		MaxTypeRunes:    h.App.MaxTypeRunes,
	}
}

// normal holds c to the spec: a number past the spec's, or none, is the
// spec's. A caller may tighten a number, never loosen it.
func normal(c Config) Config {
	l := limits()
	clamp := func(v *int, max int) {
		if *v <= 0 || *v > max {
			*v = max
		}
	}
	clampD := func(v *time.Duration, max time.Duration) {
		if *v <= 0 || *v > max {
			*v = max
		}
	}
	clamp(&c.Steps, l.Steps)
	if c.Extra < 0 || c.Extra > l.Extra {
		c.Extra = l.Extra
	}
	clamp(&c.RepeatLimit, l.RepeatLimit)
	clamp(&c.DoneRejections, l.DoneRejections)
	clamp(&c.HistoryCarry, l.HistoryCarry)
	clampD(&c.Resume, l.Resume)
	clampD(&c.FollowUp, l.FollowUp)
	clampD(&c.CardTTL, l.CardTTL)
	clampD(&c.AskTimeout, l.AskTimeout)
	clampD(&c.PageTimeout, l.PageTimeout)
	clampD(&c.ActTimeout, l.ActTimeout)
	clamp(&c.MaxNameRunes, l.MaxNameRunes)
	clamp(&c.MaxFillRunes, l.MaxFillRunes)
	clampD(&c.VerifyTimeout, l.VerifyTimeout)
	clampD(&c.ChunkTimeout, l.ChunkTimeout)
	clampD(&c.ReadbackTimeout, l.ReadbackTimeout)
	clamp(&c.TypeChunkRunes, min(l.TypeChunkRunes, apprules.MaxChunk))
	clamp(&c.MaxTypeRunes, l.MaxTypeRunes)
	return c
}

// The machine's phases.
const (
	PhaseIdle      = "idle"      // no task yet
	PhaseReading   = "reading"   // the tab is being read
	PhaseThinking  = "thinking"  // the model is asked for one action
	PhaseVerifying = "verifying" // the window is checked before or after an action
	PhaseActing    = "acting"    // a safe step runs
	PhaseChecking  = "checking"  // typed text is read back from the screen
	PhaseWaiting   = "waiting"   // a handoff card waits for the person
	PhasePaused    = "paused"    // the person went away; only Resume goes on
	PhaseFollowing = "following" // finished; a new goal continues it
	PhaseEnded     = "ended"     // over; the machine answers nothing more
	PhaseStopped   = "stopped"   // stopped; the machine answers nothing more
	PhaseHeld      = "held"      // after a restart; only the Start button begins a task
)

// The phases a Hands frame shows.
const (
	FrameWorking = "working"
	FrameWaiting = "waiting"
	FramePaused  = "paused"
)

// Why a task ended.
const (
	EndDone        = "done"        // the model said done and the page agreed
	EndAsked       = "asked"       // the model asked the person something
	EndAnswered    = "answered"    // the model answered without acting
	EndStopped     = "stopped"     // a Stop, from any source
	EndBudget      = "budget"      // the step budget ran out
	EndRepeat      = "repeat"      // the same action was asked for too often
	EndUnconfirmed = "unconfirmed" // too many "done" replies the page contradicted
	EndRefused     = "refused"     // the model broke the reply contract twice in a row
	EndExpired     = "expired"     // a handoff card lapsed
	EndAway        = "away"        // the pause was not resumed in time
	EndTimeout     = "timeout"     // a read, request or action took too long
	EndFailed      = "failed"      // a read, request or action failed, or no task could start
	EndClosed      = "closed"      // the window Hands worked in closed
	EndUnread      = "unread"      // typed text could not be read back, twice
)

// Pause reasons the machine itself gives. PauseClosed is the check result
// and the Away reason for a target that is gone: it ends the task rather
// than pausing it.
const (
	PauseClosed     = "closed"
	PauseNavigation = "navigation" // a tab read showed another origin
	PauseWindow     = "window"     // a read showed another window, or a check gave no reason
	PauseFocus      = "focus"      // an Away that gave no reason
)

// The two checks around an action outside a tab.
const (
	VerifyBefore = "before"
	VerifyAfter  = "after"
)

// Seen is what a read-back of typed text found. The zero value is
// SeenUnknown, so an answer nobody filled in never passes.
type Seen uint8

const (
	// SeenUnknown: the screen could not be read well enough to tell.
	SeenUnknown Seen = iota
	// SeenYes: the screen shows the typed text.
	SeenYes
	// SeenNo: the screen was read and does not show it.
	SeenNo
)

// ByButton is the only source that resumes a paused task: the app's or the
// presenter's Resume button. A Press from the talk chord or toggle, wake,
// open mic and the CLI never resume Hands.
const ByButton = "button"

// Event is something the machine is told.
type Event interface{ handsEvent() }

// Start begins a task toward Goal in Scope ("" is the browser tab, one of
// proto.Scopes otherwise). Origin is the authorized tab's origin, or the
// picked window's app id; the nested desktop needs none. By is the source
// that started it; after Held only ByButton begins a task. While the
// machine is following, a new goal in the same scope and place continues
// the finished task with its carried history.
type Start struct{ Goal, Origin, Scope, By string }

// Control is one control of the page read: the id the model may name and
// what the page says it is.
type Control struct {
	ID      string
	Subject consent.Subject
}

// PageRead answers ReadPage: the tab's origin, its controls and Print, a
// fingerprint of what it shows; or Err when it could not be read. Outside
// a tab, Origin is the app id of the window read (the focused one on the
// nested desktop) and Title its title, matched against and never kept;
// Controls are the regions the screen reader found, the words read there
// in Subject.Name; Private is the caller's own finding that the focus sits
// at a private field.
type PageRead struct {
	Token    uint64
	Origin   string
	Title    string
	Controls []Control
	Print    string
	Private  bool
	Err      error
}

// Reply answers Ask: the model's raw reply, or Err when the request
// failed.
type Reply struct {
	Token uint64
	Text  string
	Err   error
}

// Result is what one action did. Err is a reason code when it did not
// run; Refused says the executor refused it on its own check; ReadBack,
// for a fill, says the field now shows what was typed; Found, for
// find_files, is how many files the search listed.
type Result struct {
	Err      string
	Refused  bool
	ReadBack bool
	Found    int
}

// Acted answers Act.
type Acted struct {
	Token  uint64
	Result Result
}

// UserDid says the person pressed the control of the handoff card ID.
type UserDid struct{ ID string }

// Away says the person or the tab left the authorized scope: another
// window took focus, another tab, a navigation off the origin, a popup, a
// dialog, a download, a cover; outside a tab also a title or geometry
// change, and any key the person pressed (the reader says only that a key
// went down). Why is the pause reason the frame shows; PauseClosed ends the
// task.
type Away struct{ Why string }

// Verified answers Verify: Pause is "" when the check passed, otherwise
// why it did not (PauseClosed when the target is gone).
type Verified struct {
	Token uint64
	Pause string
}

// ReadBackResult answers ReadBack.
type ReadBackResult struct {
	Token uint64
	Seen  Seen
}

// Held says the daemon restarted and found a task that was running: its
// goal was not kept, so it cannot resume. Only an idle machine is held.
type Held struct{}

// Resume goes on with a paused task. By must be ByButton; any other source
// is ignored.
type Resume struct{ By string }

// Stop ends the task from any source; Why is passed on in End.
type Stop struct{ Why string }

// Deadline says the wait with Token reached its Until.
type Deadline struct{ Token uint64 }

// Asking says the picture for the request with Token waits on the
// person's disclosure yes; the request's deadline is held meanwhile.
type Asking struct{ Token uint64 }

// Allowed says the picture for the request with Token was allowed: it gets
// a whole AskTimeout from now.
type Allowed struct{ Token uint64 }

func (Start) handsEvent()    {}
func (PageRead) handsEvent() {}
func (Reply) handsEvent()    {}
func (Acted) handsEvent()    {}
func (UserDid) handsEvent()  {}
func (Away) handsEvent()     {}
func (Resume) handsEvent()   {}
func (Stop) handsEvent()     {}
func (Deadline) handsEvent() {}
func (Asking) handsEvent()   {}
func (Allowed) handsEvent()  {}

func (Verified) handsEvent()       {}
func (ReadBackResult) handsEvent() {}
func (Held) handsEvent()           {}

// Effect is something the caller does for the machine.
type Effect interface{ handsEffect() }

// ReadPage reads the authorized tab and answers with PageRead by Until.
type ReadPage struct {
	Token uint64
	Until time.Time
}

// Ask asks the model for one action toward Goal with the page last read,
// History the last steps in the model's terms, and Correct the one retry
// line after a reply that broke the contract. It answers with Reply by
// Until.
type Ask struct {
	Token   uint64
	Goal    string
	History []string
	Correct string
	Until   time.Time
}

// Act runs one safe step in the authorized scope, announcing Line, and
// answers with Acted by Until. It is issued only for a step the consent
// rules sorted safe, and outside a tab only after a Verify that passed; a
// type carries one chunk in Args.Text.
type Act struct {
	Token uint64
	Tool  string
	Args  proto.Args
	Line  string
	Until time.Time
}

// Refused keeps the audit code even when no executor is called
type Refused struct{ Tool, Reason string }

// ShowCard points at Control with the handoff card ID and its Line. The
// card has no yes: it ends on UserDid, a stop or Until.
type ShowCard struct {
	ID      string
	Control string
	Reason  consent.Reason
	Line    string
	Until   time.Time
}

// HideCard takes the card ID down.
type HideCard struct{ ID string }

// Verify checks the window (or the nested desktop) When, before or after
// one action, and answers with Verified by Until.
type Verify struct {
	Token uint64
	When  string
	Until time.Time
}

// ReadBack looks for Text, what one type action typed, on the screen and
// answers with ReadBackResult by Until. Text is held in memory for the look
// and never kept.
type ReadBack struct {
	Token uint64
	Text  string
	Until time.Time
}

// Frame is the Hands frame to show while the task is active: Scope where
// Hands works, Phase one of Frame*, Step of Max, Pause the reason while
// paused, and Line what to say.
type Frame struct {
	Scope     string
	Phase     string
	Step, Max int
	Pause     string
	Line      string
}

// Arm asks for Deadline with Token at Until.
type Arm struct {
	Token uint64
	Until time.Time
}

// Cancel abandons the read, request or action with Token: a model stream
// is cut, and its late answer is dropped.
type Cancel struct{ Token uint64 }

// End is the task's end: Reason one of End*, Why the Stop's, and Line what
// to show or say. Every indication of the task goes with it.
type End struct{ Reason, Why, Line string }

func (ReadPage) handsEffect() {}
func (Ask) handsEffect()      {}
func (Act) handsEffect()      {}
func (Refused) handsEffect()  {}
func (ShowCard) handsEffect() {}
func (HideCard) handsEffect() {}
func (Frame) handsEffect()    {}
func (Arm) handsEffect()      {}
func (Cancel) handsEffect()   {}
func (End) handsEffect()      {}
func (Verify) handsEffect()   {}
func (ReadBack) handsEffect() {}

// Lines Hands says or shows. Each opens with a short sentence and none
// holds a typed word.
var (
	lineNoGoal     = i18n.T("Tell me what to do on this page.")
	lineNoTab      = i18n.T("Pick a tab for me first.")
	lineDone       = i18n.T("Done.")
	lineStopped    = i18n.T("Stopped.")
	lineBudget     = i18n.T("That is as far as I got.")
	lineRepeat     = i18n.T("I was going in circles. I stopped here.")
	lineUnsure     = i18n.T("Not sure that worked. The page did not show it.")
	lineRefused    = i18n.T("I stopped there. The reply broke my rules.")
	lineExpired    = i18n.T("I waited too long. I stopped here.")
	lineAway       = i18n.T("Paused too long. I stopped here.")
	lineTimeout    = i18n.T("That took too long. I stopped.")
	lineFailed     = i18n.T("Something went wrong. I stopped.")
	linePaused     = i18n.T("Paused. Press Resume to go on.")
	lineHandoff    = i18n.T("Your turn. Press %s when it looks right.")
	lineFill       = i18n.T("Typing in %s.")
	lineClick      = i18n.T("Clicking %s.")
	lineSelect     = i18n.T("Choosing in %s.")
	lineScroll     = i18n.T("Scrolling.")
	lineWait       = i18n.T("Waiting a moment.")
	nameUnnamed    = i18n.T("the marked control")
	nameFieldBlank = i18n.T("this field")

	lineNoWindow    = i18n.T("Pick a window for me first.")
	lineClosed      = i18n.T("The window closed. I stopped.")
	lineUnread      = i18n.T("I could not read back what I typed. I stopped.")
	lineHandoffText = i18n.T("Your turn. That text needs a new line, so type it yourself.")
	lineType        = i18n.T("Typing.")
	linePress       = i18n.T("Pressing %s.")
	lineOpen        = i18n.T("Opening %s.")
	lineFind        = i18n.T("Looking for files.")
	lineShow        = i18n.T("Showing the folder it is in.")
	nameApp         = i18n.T("the app")
)

// The history lines the model reads. They name control ids and rule codes,
// never page text or a typed word.
const (
	histDone      = "%s %s: done"
	histFailed    = "%s %s: did not run (%s)"
	histRefused   = "%s %s: refused (%s)"
	histReadBack  = "fill %s: done, but the field does not show what was typed"
	histTooLong   = "fill %s: refused (text too long)"
	histHandoff   = "%s %s: handed to the person (%s)"
	histUserDid   = "the person pressed %s themselves"
	histUnchanged = "done not accepted: nothing on the page changed since the last step"
	histMismatch  = "done not accepted: a filled field does not show what was typed"
	histPaused    = "paused while away; the page may have changed"

	histTypeTooLong = "type: refused (text too long)"
	histTypeCut     = "type: stopped after %d of %d parts; the rest was not typed"
	histTypeUnseen  = "type: done, but the screen does not show what was typed"
	histTypeUnknown = "type: done, but the screen could not be read to check it; look before going on"
	histFound       = "find_files: %d found"
)

// Machine is one Hands task, with its follow-ups.
type Machine struct {
	cfg   Config
	now   func() time.Time
	phase string
	token uint64

	// the wait in flight: its token and deadline
	waitTok uint64
	until   time.Time
	asking  bool

	// the handoff card on screen
	card     uint64
	cardID   string
	cardCtrl string

	// the pause window, and the phase that resumes
	pauseTok uint64
	pause    string
	follow   uint64

	goal, origin string
	controls     map[string]consent.Subject
	ids          []string
	print        string
	base         string // the print before the last action or handoff
	fresh        bool   // the next page read is the task's first
	actTool      string // the action in flight
	actTarget    string
	actLine      string // what the action in flight announces

	steps      int
	clean      bool
	retried    bool
	rejections int
	repeatKey  string
	repeats    int
	readBack   map[string]bool
	history    []string

	// outside a tab
	scope    string
	title    string // the window's title at the last read; matched, never kept
	focused  string // on the nested desktop, the focused window's app id
	private  bool   // the caller found a private field at the focus
	files    int    // files the last search listed
	unknowns int    // read-backs that could not tell
	when     string // the check in flight: VerifyBefore or VerifyAfter
	actArgs  proto.Args
	chunks   []string // chunks still to type; dropped on any pause
	parts    int      // chunks the type action was cut into
	typed    string   // the type action's text, until its read-back
	failed   bool     // the action in flight did not run cleanly
}

// New returns an idle machine. now is its only clock.
func New(c Config, now func() time.Time) *Machine {
	if now == nil {
		panic("loop: New needs a clock")
	}
	c = normal(c)
	return &Machine{cfg: c, now: now, phase: PhaseIdle, token: c.Base}
}

// Phase is the machine's phase, one of Phase*.
func (m *Machine) Phase() string { return m.phase }

// Token is the last token issued, Base before any.
func (m *Machine) Token() uint64 { return m.token }

// Max is the step budget in force: Steps, plus Extra while every step went
// through.
func (m *Machine) Max() int {
	if m.clean {
		return m.cfg.Steps + m.cfg.Extra
	}
	return m.cfg.Steps
}

// Handle takes one event and returns what the caller does, in order. An
// event the phase has no use for, or one answering a token that is no
// longer awaited, returns nothing and changes nothing.
func (m *Machine) Handle(ev Event) []Effect {
	if m.phase == PhaseStopped {
		return nil
	}
	switch ev := ev.(type) {
	case Start:
		return m.onStart(ev)
	case PageRead:
		return m.onPage(ev)
	case Reply:
		return m.onReply(ev)
	case Acted:
		return m.onActed(ev)
	case UserDid:
		return m.onUserDid(ev)
	case Away:
		return m.onAway(ev)
	case Resume:
		return m.onResume(ev)
	case Stop:
		return m.onStop(ev)
	case Deadline:
		return m.onDeadline(ev)
	case Asking:
		return m.onAsking(ev)
	case Allowed:
		return m.onAllowed(ev)
	case Verified:
		return m.onVerified(ev)
	case ReadBackResult:
		return m.onReadBack(ev)
	case Held:
		if m.phase == PhaseIdle {
			m.phase = PhaseHeld
		}
		return nil
	}
	return nil
}

func (m *Machine) next() uint64 {
	m.token++
	return m.token
}

func (m *Machine) active() bool {
	switch m.phase {
	case PhaseReading, PhaseThinking, PhaseVerifying, PhaseActing, PhaseChecking, PhaseWaiting, PhasePaused:
		return true
	}
	return false
}

// browser says whether the task works in a browser tab.
func (m *Machine) browser() bool { return m.scope == proto.ScopeBrowser }

func (m *Machine) onStart(ev Start) []Effect {
	switch {
	case m.phase == PhaseHeld && ev.By != ByButton:
		return nil
	case m.phase != PhaseIdle && m.phase != PhaseFollowing && m.phase != PhaseHeld:
		return nil
	}
	scope := ev.Scope
	if scope == "" {
		scope = proto.ScopeBrowser
	}
	goal, origin := line(ev.Goal, 0, ""), strings.TrimSpace(ev.Origin)
	if proto.Nested(scope) {
		origin = scope
	}
	switch {
	case !proto.KnownScope(scope):
		return m.end(EndFailed, "", lineFailed)
	case goal == "":
		return m.end(EndFailed, "", lineNoGoal)
	case origin == "" && scope == proto.ScopeWindow:
		return m.end(EndFailed, "", lineNoWindow)
	case origin == "":
		return m.end(EndFailed, "", lineNoTab)
	}
	if m.phase != PhaseFollowing || origin != m.origin || scope != m.scope {
		m.history = nil
	}
	m.follow = 0
	m.goal, m.origin, m.scope = goal, origin, scope
	m.title, m.private, m.focused, m.files, m.unknowns = "", false, "", 0, 0
	m.clearAct()
	m.steps, m.clean, m.retried, m.rejections = 0, true, false, 0
	m.repeatKey, m.repeats = "", 0
	m.readBack = map[string]bool{}
	m.print, m.base = "", ""
	m.controls, m.ids = nil, nil
	m.fresh = true
	return m.read()
}

// read reads the tab for the next step.
func (m *Machine) read(before ...Effect) []Effect {
	m.phase = PhaseReading
	m.waitTok, m.until, m.asking = m.next(), m.now().Add(m.cfg.PageTimeout), false
	effects := make([]Effect, 0, len(before)+2)
	effects = append(effects, before...)
	return append(effects, m.frame(FrameWorking, ""), ReadPage{Token: m.waitTok, Until: m.until})
}

func (m *Machine) late() bool { return m.now().After(m.until) }

func (m *Machine) onPage(ev PageRead) []Effect {
	if m.phase != PhaseReading || ev.Token != m.waitTok {
		return nil
	}
	m.waitTok = 0
	if ev.Err != nil {
		return m.end(EndFailed, "", lineFailed)
	}
	if !proto.Nested(m.scope) && strings.TrimSpace(ev.Origin) != m.origin {
		if m.browser() {
			return m.away(PauseNavigation)
		}
		return m.away(PauseWindow)
	}
	m.title, m.private = ev.Title, ev.Private
	if proto.Nested(m.scope) {
		m.focused = strings.TrimSpace(ev.Origin)
	}
	m.print = ev.Print
	if m.fresh {
		m.base, m.fresh = ev.Print, false
	}
	m.controls = make(map[string]consent.Subject, len(ev.Controls))
	m.ids = m.ids[:0]
	for _, c := range ev.Controls {
		if _, dup := m.controls[c.ID]; c.ID == "" || dup {
			continue
		}
		m.controls[c.ID] = c.Subject
		m.ids = append(m.ids, c.ID)
	}
	return m.ask("")
}

// ask asks the model for the next step, or with correct for the same step
// once more.
func (m *Machine) ask(correct string) []Effect {
	if correct == "" {
		if m.steps >= m.Max() {
			return m.end(EndBudget, "", lineBudget)
		}
		m.steps++
	}
	m.phase = PhaseThinking
	m.waitTok, m.until, m.asking = m.next(), m.now().Add(m.cfg.AskTimeout), false
	return []Effect{m.frame(FrameWorking, ""), Ask{
		Token: m.waitTok, Goal: m.goal, History: m.carry(), Correct: correct, Until: m.until,
	}}
}

// carry is the last HistoryCarry history lines.
func (m *Machine) carry() []string {
	h := m.history
	if len(h) > m.cfg.HistoryCarry {
		h = h[len(h)-m.cfg.HistoryCarry:]
	}
	return slices.Clone(h)
}

func (m *Machine) note(s string) {
	m.history = append(m.history, s)
	if over := len(m.history) - m.cfg.HistoryCarry; over > 0 {
		m.history = slices.Delete(m.history, 0, over)
	}
}

func (m *Machine) onReply(ev Reply) []Effect {
	if m.phase != PhaseThinking || ev.Token != m.waitTok {
		return nil
	}
	m.waitTok = 0
	if ev.Err != nil {
		return m.end(EndFailed, "", lineFailed)
	}
	resp, err := proto.Parse(ev.Text, proto.Offer{Scope: m.scope, Controls: slices.Clone(m.ids), Files: m.files})
	if err != nil {
		m.clean = false
		if m.retried {
			return m.end(EndRefused, "", lineRefused)
		}
		m.retried = true
		return m.ask(proto.Corrective(err))
	}
	m.retried = false
	if resp.Type != proto.TypeToolCall {
		return m.finish(EndAnswered, resp.Text)
	}
	switch resp.Tool {
	case proto.ToolDone:
		return m.done(resp.Args.Text)
	case proto.ToolAskUser:
		return m.finish(EndAsked, resp.Args.Text)
	}
	return m.step(resp)
}

// done takes the model's "done" only when the page agrees: something
// changed since the last step, and every filled field shows what was
// typed.
func (m *Machine) done(say string) []Effect {
	why := ""
	switch {
	case m.print == m.base:
		why = histUnchanged
	case m.mismatched():
		why = histMismatch
	}
	if why == "" {
		return m.finish(EndDone, say)
	}
	m.clean = false
	m.rejections++
	if m.rejections > m.cfg.DoneRejections {
		return m.end(EndUnconfirmed, "", lineUnsure)
	}
	m.note(why)
	return m.read()
}

// mismatched says whether a field filled in this task does not show what
// was typed.
func (m *Machine) mismatched() bool {
	for _, ok := range m.readBack {
		if !ok {
			return true
		}
	}
	return false
}

// finish ends the task with what the model said and opens the follow-up
// window.
func (m *Machine) finish(reason, say string) []Effect {
	eff := m.end(reason, "", line(say, 0, lineDone))
	m.phase = PhaseFollowing
	m.follow = m.next()
	return append(eff, Arm{Token: m.follow, Until: m.now().Add(m.cfg.FollowUp)})
}

// step sorts one action and runs it, hands it to the person, or refuses
// it.
func (m *Machine) step(resp proto.Response) []Effect {
	a := resp.Args
	key := strings.Join([]string{resp.Tool, a.ID, a.Option, a.Dir, a.Keys, a.Name, a.Query, strconv.Itoa(a.Index)}, "\x00")
	if key == m.repeatKey {
		m.repeats++
	} else {
		m.repeatKey, m.repeats = key, 1
	}
	if m.repeats >= m.cfg.RepeatLimit {
		return m.end(EndRepeat, "", lineRepeat)
	}
	if !m.browser() {
		return m.appStep(resp)
	}
	subj := m.controls[a.ID]
	if resp.Tool == proto.ToolSelect {
		subj.Option = a.Option
	}
	class, reason := consent.Classify(resp.Tool, subj, resp.Permission)
	target := a.ID
	if target == "" {
		target = "page"
	}
	if class == consent.Safe && resp.Tool == proto.ToolFill && utf8.RuneCountInString(a.Text) > m.cfg.MaxFillRunes {
		m.clean = false
		m.note(fmt.Sprintf(histTooLong, target))
		return m.read(Refused{Tool: resp.Tool, Reason: "text-too-long"})
	}
	switch class {
	case consent.Safe:
		if resp.Tool == proto.ToolWait && a.Ms > int(m.cfg.PageTimeout/time.Millisecond) {
			a.Ms = int(m.cfg.PageTimeout / time.Millisecond)
		}
		m.base, m.actTool, m.actTarget = m.print, resp.Tool, target
		m.phase = PhaseActing
		m.waitTok, m.until, m.asking = m.next(), m.now().Add(m.cfg.ActTimeout), false
		return []Effect{m.frame(FrameWorking, ""), Act{
			Token: m.waitTok, Tool: resp.Tool, Args: a, Line: m.announce(resp.Tool, subj), Until: m.until,
		}}
	case consent.Refused:
		m.clean = false
		m.note(fmt.Sprintf(histRefused, resp.Tool, target, string(reason)))
		return m.read(Refused{Tool: resp.Tool, Reason: string(reason)})
	}
	// Handoff, and anything a later rule adds: the person's press.
	m.note(fmt.Sprintf(histHandoff, resp.Tool, target, string(reason)))
	m.base = m.print
	m.phase = PhaseWaiting
	m.waitTok = 0
	m.card = m.next()
	m.cardID, m.cardCtrl = "h-"+strconv.FormatUint(m.card, 10), a.ID
	name := m.name(subj.Name, nameUnnamed)
	l := fmt.Sprintf(lineHandoff, name)
	return []Effect{
		ShowCard{ID: m.cardID, Control: a.ID, Reason: reason, Line: l, Until: m.now().Add(m.cfg.CardTTL)},
		m.frame(FrameWaiting, l),
		Arm{Token: m.card, Until: m.now().Add(m.cfg.CardTTL)},
	}
}

// announce is the line a safe step says. It names the control, never what
// is typed.
func (m *Machine) announce(tool string, s consent.Subject) string {
	switch tool {
	case proto.ToolFill:
		return fmt.Sprintf(lineFill, m.name(s.Name, nameFieldBlank))
	case proto.ToolClick:
		return fmt.Sprintf(lineClick, m.name(s.Name, nameUnnamed))
	case proto.ToolSelect:
		return fmt.Sprintf(lineSelect, m.name(s.Name, nameFieldBlank))
	case proto.ToolScroll:
		return lineScroll
	}
	return lineWait
}

func (m *Machine) name(s, fallback string) string {
	return line(s, m.cfg.MaxNameRunes, fallback)
}

func (m *Machine) onActed(ev Acted) []Effect {
	if m.phase != PhaseActing || ev.Token != m.waitTok {
		return nil
	}
	m.waitTok = 0
	if !m.browser() {
		return m.appActed(ev.Result)
	}
	r := ev.Result
	tool, target := m.actTool, m.actTarget
	switch {
	case r.Refused:
		m.clean = false
		m.note(fmt.Sprintf(histRefused, tool, target, code(r.Err)))
	case r.Err != "":
		m.note(fmt.Sprintf(histFailed, tool, target, code(r.Err)))
	case tool == proto.ToolFill && !r.ReadBack:
		m.readBack[target] = false
		m.note(fmt.Sprintf(histReadBack, target))
	default:
		if tool == proto.ToolFill {
			m.readBack[target] = true
		}
		m.note(fmt.Sprintf(histDone, tool, target))
	}
	return m.read()
}

// appStep sorts one action outside a tab and begins it, hands it to the
// person, or refuses it.
func (m *Machine) appStep(resp proto.Response) []Effect {
	a := resp.Args
	near := m.controls[a.ID].Name
	subj := consent.AppSubject{
		App: m.origin, Title: m.title, Near: near, Text: a.Text, Keys: a.Keys, Name: a.Name, Private: m.private,
	}
	if proto.Nested(m.scope) {
		subj.App = m.focused
	}
	class, reason := consent.ClassifyApp(resp.Tool, subj, resp.Permission)
	target := m.target(a.ID)
	if class == consent.Safe && resp.Tool == proto.ToolType && utf8.RuneCountInString(a.Text) > m.cfg.MaxTypeRunes {
		m.clean = false
		m.note(histTypeTooLong)
		return m.read(Refused{Tool: resp.Tool, Reason: "text-too-long"})
	}
	switch class {
	case consent.Safe:
		return m.begin(resp.Tool, a, target, m.announceApp(resp.Tool, a, near))
	case consent.Refused:
		m.clean = false
		m.note(fmt.Sprintf(histRefused, resp.Tool, target, string(reason)))
		return m.read(Refused{Tool: resp.Tool, Reason: string(reason)})
	}
	m.note(fmt.Sprintf(histHandoff, resp.Tool, target, string(reason)))
	m.base = m.print
	m.phase = PhaseWaiting
	m.waitTok = 0
	m.card = m.next()
	m.cardID, m.cardCtrl = "h-"+strconv.FormatUint(m.card, 10), a.ID
	var l string
	switch resp.Tool {
	case proto.ToolType:
		l = lineHandoffText
	case proto.ToolPress:
		l = fmt.Sprintf(lineHandoff, a.Keys)
	case proto.ToolOpenApp:
		l = fmt.Sprintf(lineHandoff, m.name(a.Name, nameApp))
	default:
		l = fmt.Sprintf(lineHandoff, m.name(near, nameUnnamed))
	}
	return []Effect{
		ShowCard{ID: m.cardID, Control: a.ID, Reason: reason, Line: l, Until: m.now().Add(m.cfg.CardTTL)},
		m.frame(FrameWaiting, l),
		Arm{Token: m.card, Until: m.now().Add(m.cfg.CardTTL)},
	}
}

// target is what the history names for an action: the region it points
// at, or the scope.
func (m *Machine) target(id string) string {
	switch {
	case id != "":
		return id
	case m.scope == proto.ScopeWindow:
		return "window"
	}
	return "desktop"
}

// announceApp is the line a safe step outside a tab says. It never holds
// what is typed.
func (m *Machine) announceApp(tool string, a proto.Args, near string) string {
	switch tool {
	case proto.ToolType:
		return lineType
	case proto.ToolPress:
		return fmt.Sprintf(linePress, a.Keys)
	case proto.ToolClick:
		return fmt.Sprintf(lineClick, m.name(near, nameUnnamed))
	case proto.ToolOpenApp:
		return fmt.Sprintf(lineOpen, m.name(a.Name, nameApp))
	case proto.ToolFindFiles:
		return lineFind
	case proto.ToolShowFile:
		return lineShow
	case proto.ToolScroll:
		return lineScroll
	}
	return lineWait
}

// begin opens one safe action outside a tab: a type is cut into chunks,
// and nothing runs before the window passes its first check.
func (m *Machine) begin(tool string, a proto.Args, target, say string) []Effect {
	if tool == proto.ToolWait && a.Ms > int(m.cfg.PageTimeout/time.Millisecond) {
		a.Ms = int(m.cfg.PageTimeout / time.Millisecond)
	}
	m.base, m.actTool, m.actTarget, m.actArgs, m.actLine = m.print, tool, target, a, say
	m.failed = false
	if tool == proto.ToolType {
		m.typed = a.Text
		m.chunks = apprules.Chunks(a.Text, m.cfg.TypeChunkRunes)
		m.parts = len(m.chunks)
	} else {
		m.typed, m.chunks, m.parts = "", nil, 0
	}
	return m.verify(VerifyBefore)
}

// verify asks for the check when.
func (m *Machine) verify(when string) []Effect {
	m.phase, m.when = PhaseVerifying, when
	m.waitTok, m.until, m.asking = m.next(), m.now().Add(m.cfg.VerifyTimeout), false
	return []Effect{m.frame(FrameWorking, ""), Verify{Token: m.waitTok, When: when, Until: m.until}}
}

// act runs the action in flight, or its next chunk.
func (m *Machine) act() []Effect {
	a, timeout := m.actArgs, m.cfg.ActTimeout
	if m.actTool == proto.ToolType {
		a.Text, timeout = m.chunks[0], m.cfg.ChunkTimeout
	}
	m.phase = PhaseActing
	m.waitTok, m.until, m.asking = m.next(), m.now().Add(timeout), false
	return []Effect{m.frame(FrameWorking, ""), Act{Token: m.waitTok, Tool: m.actTool, Args: a, Line: m.actLine, Until: m.until}}
}

func (m *Machine) onVerified(ev Verified) []Effect {
	if m.phase != PhaseVerifying || ev.Token != m.waitTok {
		return nil
	}
	m.waitTok = 0
	if ev.Pause != "" {
		why := line(ev.Pause, feature.MaxGuideKicker, PauseWindow)
		if why == PauseClosed {
			return m.end(EndClosed, "", lineClosed)
		}
		return m.away(why)
	}
	switch {
	case m.when == VerifyBefore:
		return m.act()
	case len(m.chunks) > 0 && !m.failed:
		// the check after one chunk is the check before the next: nothing
		// ran between them
		return m.act()
	case m.actTool == proto.ToolType && !m.failed:
		m.phase = PhaseChecking
		m.waitTok, m.until, m.asking = m.next(), m.now().Add(m.cfg.ReadbackTimeout), false
		return []Effect{m.frame(FrameWorking, ""), ReadBack{Token: m.waitTok, Text: m.typed, Until: m.until}}
	}
	if !m.failed {
		if m.actTool == proto.ToolFindFiles {
			m.note(fmt.Sprintf(histFound, m.files))
		} else {
			m.note(fmt.Sprintf(histDone, m.actTool, m.actTarget))
		}
	}
	m.clearAct()
	return m.read()
}

// appActed takes one action's or chunk's result outside a tab: a failure
// drops what is left of it, and the window is checked after either way.
func (m *Machine) appActed(r Result) []Effect {
	tool, target := m.actTool, m.actTarget
	switch {
	case r.Refused || r.Err != "":
		if r.Refused {
			m.clean = false
			m.note(fmt.Sprintf(histRefused, tool, target, code(r.Err)))
		} else {
			m.note(fmt.Sprintf(histFailed, tool, target, code(r.Err)))
		}
		m.failed = true
		m.cut()
	case tool == proto.ToolType:
		m.chunks = m.chunks[1:]
	case tool == proto.ToolFindFiles:
		m.files = max(r.Found, 0)
	}
	return m.verify(VerifyAfter)
}

func (m *Machine) onReadBack(ev ReadBackResult) []Effect {
	if m.phase != PhaseChecking || ev.Token != m.waitTok {
		return nil
	}
	m.waitTok = 0
	target := m.actTarget
	switch ev.Seen {
	case SeenYes:
		m.readBack[target] = true
		m.note(fmt.Sprintf(histDone, proto.ToolType, target))
	case SeenNo:
		m.readBack[target] = false
		m.note(histTypeUnseen)
	default:
		m.unknowns++
		if m.unknowns >= 2 {
			return m.end(EndUnread, "", lineUnread)
		}
		m.readBack[target] = false
		m.note(histTypeUnknown)
	}
	m.clearAct()
	return m.read()
}

// cut drops what is left of a type in flight: the rest is never typed, and
// the text typed so far does not count as read back.
func (m *Machine) cut() {
	if m.actTool == proto.ToolType && m.typed != "" {
		if done := m.parts - len(m.chunks); done < m.parts || m.phase != PhaseChecking {
			m.note(fmt.Sprintf(histTypeCut, done, m.parts))
		}
		m.readBack[m.actTarget] = false
	}
	m.chunks, m.typed = nil, ""
}

// clearAct forgets the action in flight, its text with it.
func (m *Machine) clearAct() {
	m.actTool, m.actTarget, m.actArgs, m.actLine, m.when = "", "", proto.Args{}, "", ""
	m.chunks, m.parts, m.typed, m.failed = nil, 0, "", false
}

func (m *Machine) onUserDid(ev UserDid) []Effect {
	if m.card == 0 || ev.ID != m.cardID {
		return nil
	}
	eff := []Effect{HideCard{ID: m.cardID}}
	target := m.cardCtrl
	if target == "" {
		target = "page"
	}
	m.note(fmt.Sprintf(histUserDid, target))
	m.card, m.cardID, m.cardCtrl = 0, "", ""
	if m.phase == PhasePaused {
		return append(eff, m.frame(FramePaused, linePaused))
	}
	return append(eff, m.read()...)
}

func (m *Machine) onAway(ev Away) []Effect {
	switch m.phase {
	case PhaseReading, PhaseThinking, PhaseVerifying, PhaseActing, PhaseChecking, PhaseWaiting, PhasePaused:
	default:
		return nil
	}
	why := line(ev.Why, feature.MaxGuideKicker, PauseFocus)
	if why == PauseClosed {
		return m.end(EndClosed, "", lineClosed)
	}
	if m.phase == PhasePaused {
		return nil
	}
	var eff []Effect
	if m.waitTok != 0 {
		eff = append(eff, Cancel{Token: m.waitTok})
	}
	return append(eff, m.away(why)...)
}

// away pauses the task for why: nothing runs until Resume, and the pause
// window ends it.
func (m *Machine) away(why string) []Effect {
	m.cut()
	m.phase, m.pause = PhasePaused, why
	m.waitTok, m.asking = 0, false
	m.pauseTok = m.next()
	m.note(histPaused)
	return []Effect{
		m.frame(FramePaused, linePaused),
		Arm{Token: m.pauseTok, Until: m.now().Add(m.cfg.Resume)},
	}
}

func (m *Machine) onResume(ev Resume) []Effect {
	if m.phase != PhasePaused || ev.By != ByButton {
		return nil
	}
	m.pauseTok, m.pause = 0, ""
	if m.card != 0 {
		m.phase = PhaseWaiting
		return []Effect{m.frame(FrameWaiting, "")}
	}
	return m.read()
}

func (m *Machine) onStop(ev Stop) []Effect {
	if !m.active() {
		m.phase = PhaseStopped
		return nil
	}
	eff := m.end(EndStopped, ev.Why, lineStopped)
	m.phase = PhaseStopped
	return eff
}

func (m *Machine) onDeadline(ev Deadline) []Effect {
	switch {
	case ev.Token == 0:
		return nil
	case m.phase == PhaseFollowing && ev.Token == m.follow:
		m.phase, m.follow, m.history = PhaseEnded, 0, nil
		return nil
	case !m.active():
		return nil
	case ev.Token == m.card:
		return m.end(EndExpired, "", lineExpired)
	case m.phase == PhasePaused && ev.Token == m.pauseTok:
		return m.end(EndAway, "", lineAway)
	case ev.Token == m.waitTok && !m.asking:
		return m.end(EndTimeout, "", lineTimeout)
	}
	return nil
}

// onAsking holds the request's deadline while its picture waits on the
// disclosure yes; a request already late ends the task.
func (m *Machine) onAsking(ev Asking) []Effect {
	if m.phase != PhaseThinking || ev.Token != m.waitTok || m.asking {
		return nil
	}
	if m.late() {
		return m.end(EndTimeout, "", lineTimeout)
	}
	m.asking = true
	return []Effect{Arm{Token: ev.Token}}
}

// onAllowed gives the request whose picture was allowed a whole
// AskTimeout from now.
func (m *Machine) onAllowed(ev Allowed) []Effect {
	if m.phase != PhaseThinking || ev.Token != m.waitTok || !m.asking {
		return nil
	}
	m.asking, m.until = false, m.now().Add(m.cfg.AskTimeout)
	return []Effect{Arm{Token: ev.Token, Until: m.until}}
}

// end unwinds whatever is out and ends the task.
func (m *Machine) end(reason, why, say string) []Effect {
	var eff []Effect
	if m.waitTok != 0 {
		eff = append(eff, Cancel{Token: m.waitTok})
	}
	if m.card != 0 {
		eff = append(eff, HideCard{ID: m.cardID})
	}
	m.waitTok, m.asking, m.card, m.cardID, m.cardCtrl = 0, false, 0, "", ""
	m.pauseTok, m.pause = 0, ""
	m.clearAct()
	m.title, m.private = "", false
	m.phase = PhaseEnded
	return append(eff, End{Reason: reason, Why: why, Line: say})
}

// frame is the Hands frame for phase with say.
func (m *Machine) frame(phase, say string) Frame {
	f := Frame{Scope: m.scope, Phase: phase, Step: m.steps, Max: m.Max(), Line: say}
	if phase == FramePaused {
		f.Pause = m.pause
	}
	return f
}

// line is s on one line with no control or format runes, at most max
// runes (feature.MaxGuideText when max is 0), or fallback when nothing is
// left.
func line(s string, max int, fallback string) string {
	if max <= 0 {
		max = feature.MaxGuideText
	}
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	}), " ")
	if utf8.RuneCountInString(s) > max {
		s = strings.TrimSpace(string([]rune(s)[:max]))
	}
	if s == "" {
		return fallback
	}
	return s
}

// code is a reason code as the history may carry it: one short token of
// letters, digits and dashes, or "error".
func code(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 40 || strings.IndexFunc(s, func(r rune) bool {
		return !(r == '-' || r == '_' || r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r)))
	}) >= 0 {
		return "error"
	}
	return s
}
