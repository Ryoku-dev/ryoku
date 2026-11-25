package feature

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

var routineChip = sync.OnceValue(func() time.Duration {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return time.Duration(math.Round(s.Reflex.Routines.ChipS * float64(time.Second)))
})

// RoutineOfferTTL is how long a routine offer waits, the spec's
// reflex.routines.chipS; it then lapses to skip and nothing is kept.
func RoutineOfferTTL() time.Duration { return routineChip() }

// RoutineOffer is the consent frame that offers to keep a run of quick
// commands as a routine: per-action, answered by oni.keep or oni.skip within
// RoutineOfferTTL. Keeping writes only on this computer, so it names no
// route.
func RoutineOffer(id, question string) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: Routines, Class: ConsentPerAction, Destination: EgressLocal, Question: question, ExpiresMs: RoutineOfferTTL().Milliseconds()}
}

// DecisionCard is the disclosure card shown before a reflex decision goes to
// the hosted credit route, which route names; oni.consent.answer answers it.
// A decision on the local route sends nothing off the computer and needs no
// card.
func DecisionCard(id, route, question string) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: Decisions, Class: ConsentDisclosure, Destination: "hosted", Route: route, Question: question, ExpiresMs: ConsentTTL.Milliseconds()}
}

// a decision card discloses a hosted decision and names its route
func validDecisionCard(f ConsentFrame) error {
	if f.Destination != "hosted" {
		return fmt.Errorf("a %s card discloses a hosted decision; a local one asks nothing", f.Feature)
	}
	return nil
}

func (c remindersCall) validate() error {
	if c.Action != protocol.OniRemindersAdd && (c.Kind != "" || !c.Due.IsZero() || c.Text != "") {
		return fmt.Errorf("%s carries no kind, due time or words", c.Action)
	}
	switch c.Action {
	case protocol.OniRemindersList, protocol.OniRemindersCancelAll:
		if c.ID != "" {
			return fmt.Errorf("%s names no reminder", c.Action)
		}
	case protocol.OniRemindersCancel:
		if !protocol.ValidOniReminderID(c.ID) {
			return fmt.Errorf("a cancel names its reminder by its %d-digit id", protocol.OniReminderIDLen)
		}
	case protocol.OniRemindersAdd:
		switch {
		case c.ID != "":
			return errors.New("an add names no reminder")
		case c.Kind != protocol.OniReminderKindReminder && c.Kind != protocol.OniReminderKindTimer:
			return fmt.Errorf("kind %q is not reminder or timer", c.Kind)
		case c.Due.IsZero():
			return errors.New("an add says when it is due")
		case c.Text == "" && c.Kind == protocol.OniReminderKindReminder:
			return errors.New("a reminder carries its words")
		case c.Text != "" && !protocol.ValidOniText(c.Text, protocol.OniReminderTextMax, false):
			return fmt.Errorf("a reminder's words are one line of at most %d characters", protocol.OniReminderTextMax)
		}
	default:
		return fmt.Errorf("action %q is not list, add, cancel or cancel_all", c.Action)
	}
	return nil
}

func (c routinesCall) validate() error {
	switch c.Action {
	case protocol.OniRoutinesList, protocol.OniRoutinesForgetAll:
		if c.Name != "" {
			return fmt.Errorf("%s names no routine", c.Action)
		}
	case protocol.OniRoutinesRun:
		if c.Name != "" && !protocol.ValidOniRoutineName(c.Name) {
			return fmt.Errorf("a routine's name is one trimmed line of at most %d characters", protocol.OniRoutineNameMax)
		}
	case protocol.OniRoutinesForget:
		if !protocol.ValidOniRoutineName(c.Name) {
			return errors.New("a forget names its routine")
		}
	default:
		return fmt.Errorf("action %q is not list, run, forget or forget_all", c.Action)
	}
	return nil
}
