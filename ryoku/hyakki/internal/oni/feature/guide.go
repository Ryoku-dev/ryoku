package feature

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// Guide phases, spec.GuidePhaseIDs: no guide; reading the window for a
// step; pointing at the step's target until the strand lands; waiting for
// the user to act on it; explaining a step the screen did not answer;
// offering to keep a finished guide; done; winding the strand back after a
// stop; and nudging, a hint offering a kept guide.
const (
	GuideIdle       = "idle"
	GuideReading    = "reading"
	GuidePointing   = "pointing"
	GuideWaiting    = "waiting"
	GuideExplaining = "explaining"
	GuideKeeping    = "keeping"
	GuideDone       = "done"
	GuideWinding    = "winding"
	GuideNudging    = "nudging"
)

// GuidePhases are every guide phase.
var GuidePhases = []string{GuideIdle, GuideReading, GuidePointing, GuideWaiting, GuideExplaining, GuideKeeping, GuideDone, GuideWinding, GuideNudging}

// What a guide frame offers the user: the next step, stopping, the ring
// thrown around the target again, and keeping or skipping a finished guide.
// Next on a hint starts the kept guide; skip lets the hint or the keep offer
// go.
const (
	GuideActionNext   = protocol.OniGuideNext
	GuideActionStop   = protocol.OniGuideStop
	GuideActionCircle = protocol.OniGuideCircle
	GuideActionKeep   = "keep"
	GuideActionSkip   = "skip"
)

// the actions each phase may offer; idle, done and winding offer none
var guidePhaseActions = map[string][]string{
	GuideReading:    {GuideActionStop},
	GuidePointing:   {GuideActionNext, GuideActionStop, GuideActionCircle},
	GuideWaiting:    {GuideActionNext, GuideActionStop, GuideActionCircle},
	GuideExplaining: {GuideActionNext, GuideActionStop, GuideActionCircle},
	GuideKeeping:    {GuideActionKeep, GuideActionSkip},
	GuideNudging:    {GuideActionNext, GuideActionSkip},
}

// Limits on a guide frame's lines, in runes.
const (
	MaxGuideText   = 200
	MaxGuideKicker = 48
)

// GuideFrame is the guide's state for the card beside Oni. Step counts from
// 1; Kicker is the line above the step ("Step 2", "Kept from before"); Text is
// the step, opening with the action, or the closing or hint line; Plan the
// checklist; Actions what the card offers; Replay a step from a kept guide;
// Target the strand target the step points with. Idle carries nothing but
// its seq; reading, keeping and winding carry no text; pointing and waiting
// carry both Text and Target. A guide frame never carries a picture, text
// read off the screen, a window title or the user's own words: it has no
// field for any of them.
type GuideFrame struct {
	Seq     uint64     `json:"seq"`
	Phase   string     `json:"phase"`
	Step    int        `json:"step,omitempty"`
	Kicker  string     `json:"kicker,omitempty"`
	Text    string     `json:"text,omitempty"`
	Plan    *GuidePlan `json:"plan,omitempty"`
	Actions []string   `json:"actions,omitempty"`
	Replay  bool       `json:"replay,omitempty"`
	Target  uint64     `json:"target,omitempty"`
}

// GuidePlan is the checklist: the steps done, at most spec
// session.planDoneMax, the current one, and at most session.planNextMax
// ahead. It is shown, never read back to the model.
type GuidePlan struct {
	Done    []string `json:"done,omitempty"`
	Current string   `json:"current,omitempty"`
	Next    []string `json:"next,omitempty"`
}

// PickFrame arms or disarms select-and-ask's pick overlay. The presenter
// maps one input surface per output only while the latest pick frame is
// Armed, reports the one click with oni.pick point and disarms on its own.
// The daemon sends armed on oni.pick arm and disarmed on the click, a
// cancel, an expiry or a stop. It is never in a snapshot: a presenter that
// returns starts disarmed.
type PickFrame struct {
	Seq   uint64 `json:"seq"`
	Armed bool   `json:"armed"`
}

func (f PickFrame) validate() error {
	if f.Seq == 0 {
		return errors.New("seq starts at 1")
	}
	return nil
}

type guideLimits struct {
	steps, done, next int
	keep, hint        time.Duration
}

var guideSpec = sync.OnceValue(func() guideLimits {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return guideLimits{
		steps: s.Guide.MaxSteps,
		done:  s.Session.PlanDoneMax,
		next:  s.Session.PlanNextMax,
		keep:  time.Duration(math.Round(s.Guide.Keep.ChipS * float64(time.Second))),
		hint:  time.Duration(math.Round(s.Guide.Hints.RetractS * float64(time.Second))),
	}
})

// MaxGuideSteps is the spec's guide.maxSteps: the most steps one guide takes.
func MaxGuideSteps() int { return guideSpec().steps }

// KeepOfferTTL is how long a keep offer waits, the spec's guide.keep.chipS;
// it then lapses to skip.
func KeepOfferTTL() time.Duration { return guideSpec().keep }

// HintOfferTTL is how long a hint waits, the spec's guide.hints.retractS;
// an untouched hint then retracts as if skipped.
func HintOfferTTL() time.Duration { return guideSpec().hint }

// KeepOffer is the consent frame that offers to keep a finished guide:
// per-action, answered by oni.keep or oni.skip within KeepOfferTTL. Keeping
// writes only on this computer, so it names no route.
func KeepOffer(id, question string) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: GuideKeep, Class: ConsentPerAction, Destination: EgressLocal, Question: question, ExpiresMs: KeepOfferTTL().Milliseconds()}
}

// HintOffer is the consent frame that carries a hint, beside its nudging
// guide frame: announced, so it asks nothing; oni.guide next takes it up
// and oni.skip with its id lets it go within HintOfferTTL. A hint comes
// from what was kept, so it names no route.
func HintOffer(id, question string) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: GuideHints, Class: ConsentAnnounced, Destination: EgressLocal, Question: question, ExpiresMs: HintOfferTTL().Milliseconds()}
}

// a keep, hint or routine offer stays on this computer and lapses with its
// spec time
func validLocalOffer(f ConsentFrame, ttl time.Duration) error {
	if f.Destination != EgressLocal || f.Route != "" {
		return fmt.Errorf("a %s offer stays on this computer and names no route", f.Feature)
	}
	if f.ExpiresMs > ttl.Milliseconds() {
		return fmt.Errorf("a %s offer lapses within %d ms", f.Feature, ttl.Milliseconds())
	}
	return nil
}

func (f GuideFrame) validate() error {
	if f.Seq == 0 {
		return errors.New("seq starts at 1")
	}
	if !slices.Contains(GuidePhases, f.Phase) {
		return fmt.Errorf("unknown guide phase %q", f.Phase)
	}
	if f.Phase == GuideIdle {
		if f.Step != 0 || f.Kicker != "" || f.Text != "" || f.Plan != nil || f.Actions != nil || f.Replay || f.Target != 0 {
			return errors.New("the idle phase carries nothing but its seq")
		}
		return nil
	}
	lim := guideSpec()
	switch {
	case f.Step < 0 || f.Step > lim.steps:
		return fmt.Errorf("a guide takes at most %d steps", lim.steps)
	case f.Phase == GuideNudging && f.Step != 0:
		return errors.New("a hint is no step")
	case f.Step == 0 && f.Phase != GuideNudging && f.Phase != GuideDone && f.Phase != GuideWinding:
		return fmt.Errorf("a %s guide names its step from 1", f.Phase)
	}
	if f.Kicker != "" && !protocol.ValidOniText(f.Kicker, MaxGuideKicker, false) {
		return errors.New("kicker is too long or carries control characters")
	}
	switch f.Phase {
	case GuideReading, GuideKeeping, GuideWinding:
		if f.Text != "" {
			return fmt.Errorf("the %s phase carries no text", f.Phase)
		}
	default:
		if !guideLine(f.Text) {
			return fmt.Errorf("the %s phase carries its line, at most %d characters on one line", f.Phase, MaxGuideText)
		}
	}
	switch f.Phase {
	case GuidePointing, GuideWaiting:
		if f.Target == 0 {
			return fmt.Errorf("the %s phase names the strand target it points with", f.Phase)
		}
	case GuideExplaining:
	default:
		if f.Target != 0 {
			return fmt.Errorf("the %s phase points at nothing", f.Phase)
		}
	}
	if f.Plan != nil {
		if f.Phase == GuideWinding || f.Phase == GuideNudging {
			return fmt.Errorf("the %s phase shows no checklist", f.Phase)
		}
		if err := f.Plan.validate(lim); err != nil {
			return err
		}
	}
	return f.validActions()
}

func (f GuideFrame) validActions() error {
	if f.Actions == nil {
		return nil
	}
	if len(f.Actions) == 0 {
		return errors.New("an empty action list is absent")
	}
	offered := guidePhaseActions[f.Phase]
	for i, a := range f.Actions {
		if !slices.Contains(offered, a) {
			return fmt.Errorf("the %s phase does not offer %q", f.Phase, a)
		}
		if slices.Contains(f.Actions[:i], a) {
			return fmt.Errorf("action %q is offered twice", a)
		}
		if a == GuideActionCircle && f.Target == 0 {
			return errors.New("circling needs a target")
		}
	}
	return nil
}

func (p GuidePlan) validate(lim guideLimits) error {
	if p.Done == nil && p.Current == "" && p.Next == nil {
		return errors.New("an empty plan is absent")
	}
	for _, list := range []struct {
		name  string
		items []string
		max   int
	}{{"done", p.Done, lim.done}, {"next", p.Next, lim.next}} {
		if list.items == nil {
			continue
		}
		if len(list.items) == 0 || len(list.items) > list.max {
			return fmt.Errorf("a plan lists 1 to %d %s steps, or none", list.max, list.name)
		}
		for _, item := range list.items {
			if !guideLine(item) {
				return fmt.Errorf("a %s step is empty, too long or carries control characters", list.name)
			}
		}
	}
	if p.Current != "" && !guideLine(p.Current) {
		return errors.New("the current step is too long or carries control characters")
	}
	return nil
}

func guideLine(s string) bool {
	return strings.TrimSpace(s) != "" && protocol.ValidOniText(s, MaxGuideText, false)
}

func (c guideCall) validate() error {
	switch c.Action {
	case protocol.OniGuideStart:
		if strings.TrimSpace(c.Goal) == "" || !protocol.ValidOniText(c.Goal, protocol.OniGuideGoalMax, false) {
			return fmt.Errorf("a start names its goal in 1 to %d characters on one line", protocol.OniGuideGoalMax)
		}
	case protocol.OniGuideNext, protocol.OniGuideStop, protocol.OniGuideCircle:
		if c.Goal != "" {
			return errors.New("only a start names a goal")
		}
	default:
		return fmt.Errorf("action %q is not start, next, stop or circle", c.Action)
	}
	return nil
}

func (c keepCall) validate() error {
	if !validCode(c.ID, 64) {
		return errors.New("the keep offer id is not a short code")
	}
	return nil
}

func (c pickCall) validate() error {
	switch c.Action {
	case protocol.OniPickArm:
		if c.Output != "" || c.X != 0 || c.Y != 0 {
			return errors.New("arming names no spot; the click does")
		}
		if c.Text != "" && (strings.TrimSpace(c.Text) == "" || !protocol.ValidOniText(c.Text, protocol.OniPickTextMax, true)) {
			return fmt.Errorf("the question is at most %d characters without control characters", protocol.OniPickTextMax)
		}
	case protocol.OniPickPoint:
		if c.Text != "" {
			return errors.New("the question comes with the arm, not the click")
		}
		if !validOutput(c.Output) {
			return errors.New("a click names its output")
		}
		for _, v := range []float64{c.X, c.Y} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > maxBoxPx {
				return errors.New("a click is finite logical pixels on its output")
			}
		}
	case protocol.OniPickCancel, protocol.OniPickWait:
		if c.Output != "" || c.X != 0 || c.Y != 0 || c.Text != "" {
			return fmt.Errorf("a %s carries nothing", c.Action)
		}
	default:
		return fmt.Errorf("action %q is not arm, point, cancel or wait", c.Action)
	}
	return nil
}
