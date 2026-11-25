package feature

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// HandsCardPrefix starts the id of every Hands handoff card.
const HandsCardPrefix = "h-"

// HandsStepCardPrefix starts the id of every Hands step's disclosure card,
// the Inspect question each step's picture, page text and control names go
// out behind; a screen ask's cards start otherwise.
const HandsStepCardPrefix = "hs-"

var handsSpec = sync.OnceValue(func() spec.Hands {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s.Hands
})

// HandoffTTL is how long a Hands handoff card waits for the user, the
// spec's hands.consentTtlS; it then lapses and the task ends.
func HandoffTTL() time.Duration { return handsSpec().ConsentTTL() }

// HandsMaxSteps is the most steps a Hands task may take, the spec's
// hands.budget steps and extra together.
func HandsMaxSteps() int { return handsSpec().Budget.Max() }

// HandoffCard is the card Hands shows when it hands a step to the user: it
// points at the control and waits, with left the time the card has before
// it lapses. It has no yes: the user presses the control themselves, and
// nothing they say or answer makes Hands press it. Nothing leaves the
// computer for it, so it names no route. feature is the Hands row of the
// stage the task runs at.
func HandoffCard(feature, id, question string, left time.Duration) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: feature, Class: ConsentFirm, Destination: EgressLocal, Question: question, ExpiresMs: left.Milliseconds()}
}

// HandsStageFeature is the Hands row of stage, or "" for off or no stage.
func HandsStageFeature(stage string) string {
	switch stage {
	case protocol.OniHandsBrowser:
		return HandsBrowser
	case protocol.OniHandsApp:
		return HandsApp
	case protocol.OniHandsDesktop:
		return HandsDesktop
	}
	return ""
}

// a handoff card points at a control on the page or the screen; it sends
// nothing and waits no longer than the spec says
func validHandoffCard(f ConsentFrame) error {
	if !strings.HasPrefix(f.ID, HandsCardPrefix) || len(f.ID) == len(HandsCardPrefix) {
		return fmt.Errorf("a Hands card's id starts %q", HandsCardPrefix)
	}
	if f.Destination != EgressLocal || f.Route != "" {
		return errors.New("a Hands card points at the page and sends nothing")
	}
	if ttl := HandoffTTL().Milliseconds(); f.ExpiresMs > ttl {
		return fmt.Errorf("a Hands card waits at most %d ms", ttl)
	}
	return nil
}

func (f HandsFrame) validate() error {
	switch f.Stage {
	case protocol.OniHandsOff, protocol.OniHandsBrowser, protocol.OniHandsApp, protocol.OniHandsDesktop:
	default:
		return fmt.Errorf("unknown hands stage %q", f.Stage)
	}
	if f.End != "" && !slices.Contains(protocol.OniHandsEnds, f.End) {
		return fmt.Errorf("unknown hands end %q", f.End)
	}
	if !f.Active {
		if f.Scope != "" || f.Phase != "" || f.Step != 0 || f.Max != 0 || f.Pause != "" || f.App != "" || f.Output != "" {
			return errors.New("inactive hands carry no scope, phase, step, pause, app or output")
		}
		switch f.Reason {
		case ReasonOniOff, ReasonFeatureOff:
			return nil
		case ReasonAllowed:
			if f.Stage == protocol.OniHandsOff {
				return errors.New("hands that are off are never allowed")
			}
			return nil
		}
		return fmt.Errorf("inactive hands report reason %q, not %s, %s or %s", f.Reason, ReasonOniOff, ReasonFeatureOff, ReasonAllowed)
	}
	if f.End != "" {
		return errors.New("only the frame that ends a task says how it ended")
	}
	switch {
	case f.Stage == protocol.OniHandsOff:
		return fmt.Errorf("hands at stage %q are never active", f.Stage)
	case f.Reason != ReasonAllowed:
		return fmt.Errorf("active hands report reason %q, not %s", f.Reason, ReasonAllowed)
	case !slices.Contains(protocol.OniHandsScopes, f.Scope):
		return fmt.Errorf("unknown hands scope %q", f.Scope)
	case !protocol.OniHandsStageScope(f.Stage, f.Scope):
		return fmt.Errorf("hands at stage %q never work in scope %q", f.Stage, f.Scope)
	case !slices.Contains(protocol.OniHandsPhases, f.Phase):
		return fmt.Errorf("hands phase %q is not working, waiting or paused", f.Phase)
	case f.Max < 1 || f.Max > HandsMaxSteps():
		return fmt.Errorf("a task takes 1 to %d steps", HandsMaxSteps())
	case f.Step < 0 || f.Step > f.Max:
		return fmt.Errorf("step %d is not between 0 and %d", f.Step, f.Max)
	}
	if f.Scope != protocol.OniHandsScopeWindow && (f.App != "" || f.Output != "") {
		return errors.New("only a task in a picked window names its app and output")
	}
	if f.App != "" && !protocol.ValidOniHandsApp(f.App) || f.Output != "" && !protocol.ValidOniHandsApp(f.Output) {
		return errors.New("a window's app and output are named by their codes")
	}
	if f.Phase != protocol.OniHandsStatePaused {
		if f.Pause != "" {
			return errors.New("only a paused task names a pause")
		}
		return nil
	}
	switch {
	case slices.Contains(protocol.OniHandsScopePauses(f.Scope), f.Pause):
		return nil
	case slices.Contains(protocol.OniHandsPauses, f.Pause):
		return fmt.Errorf("pause %q is not scope %s's", f.Pause, f.Scope)
	}
	return fmt.Errorf("unknown pause %q", f.Pause)
}

func (c handsCall) validate() error {
	if c.Window != "" && c.Action != protocol.OniHandsSelect {
		return errors.New("only a select names a window")
	}
	if (c.Scope != "" || c.Task != "") && c.Action != protocol.OniHandsStart {
		return errors.New("only a start names a scope or a task")
	}
	switch c.Action {
	case protocol.OniHandsOpen, protocol.OniHandsTabs, protocol.OniHandsStop, protocol.OniHandsResume, protocol.OniHandsStatus,
		protocol.OniHandsForget, protocol.OniHandsWindows, protocol.OniHandsTasks, protocol.OniHandsClear:
		if c.Tab != "" || c.Goal != "" {
			return fmt.Errorf("%s names no tab and no goal", c.Action)
		}
	case protocol.OniHandsSelect:
		if c.Tab != "" || c.Goal != "" {
			return errors.New("a select names only its window")
		}
		if !protocol.ValidOniHandsWindowID(c.Window) {
			return fmt.Errorf("a select names one window by its id, 1 to %d letters and digits", protocol.OniHandsTabIDMax)
		}
	case protocol.OniHandsAuthorize:
		if c.Goal != "" {
			return errors.New("an authorize names no goal")
		}
		if !protocol.ValidOniHandsTabID(c.Tab) {
			return fmt.Errorf("an authorize names one tab by its id, 1 to %d letters and digits", protocol.OniHandsTabIDMax)
		}
	case protocol.OniHandsStart:
		if c.Tab != "" {
			return errors.New("a start names no tab: Hands works in the one authorized")
		}
		if strings.TrimSpace(c.Goal) == "" || !protocol.ValidOniText(c.Goal, protocol.OniHandsGoalMax, false) {
			return fmt.Errorf("a start names its goal in 1 to %d characters on one line", protocol.OniHandsGoalMax)
		}
		if c.Scope != "" && !slices.Contains(protocol.OniHandsScopes, c.Scope) {
			return fmt.Errorf("scope %q is not one of %s", c.Scope, strings.Join(protocol.OniHandsScopes, ", "))
		}
		if c.Task != "" && !protocol.ValidOniHandsTaskID(c.Task) {
			return errors.New("a start names a journal task by its id")
		}
	default:
		return fmt.Errorf("action %q is not one of %s", c.Action, strings.Join(protocol.OniHandsActions, ", "))
	}
	return nil
}
