package feature

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// The id prefixes of the Phase 16 cards. A card's id starts with its
// feature's prefix, so the daemon routes an answer by the id alone and no
// card is ever taken for another feature's.
const (
	ForgetCardPrefix     = "forget-"
	GuardCardPrefix      = "guard-"
	RecallCardPrefix     = "recall-"
	RewriteCardPrefix    = "rewrite-"
	AssignmentCardPrefix = "assign-"
)

var phase16Spec = sync.OnceValue(func() *spec.OniSpec {
	s, err := spec.Load()
	if err != nil {
		panic(err)
	}
	return s
})

// ForgetCard is the card that names the facts a spoken forget found and
// forgets them only on a yes: per-action, on this computer, so it names no
// route. The question names the facts, never a sensitive number.
func ForgetCard(id, question string) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: MemoryForget, Class: ConsentPerAction, Destination: EgressLocal, Question: question, ExpiresMs: ConsentTTL.Milliseconds()}
}

// GuardCard is the focus guard's card before each close: it names the feed
// and the app, and the window closes only on a yes. Nothing leaves the
// computer for it.
func GuardCard(id, question string) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: FeedGuard, Class: ConsentPerAction, Destination: EgressLocal, Question: question, ExpiresMs: ConsentTTL.Milliseconds()}
}

// RecallCard is the disclosure card shown before recalled passages go to a
// hosted route, which route names; the question names the passages'
// titles. A local route sends nothing off the computer and asks nothing.
func RecallCard(id, route, question string) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: Recall, Class: ConsentDisclosure, Destination: "hosted", Route: route, Question: question, ExpiresMs: ConsentTTL.Milliseconds()}
}

// RewriteCard is the disclosure card shown before a selection is sent to
// be rewritten: destination is local or hosted, and a hosted one names its
// route. The selection itself goes only in the call's result to the app or
// the CLI, never on the card.
func RewriteCard(id, destination, route, question string) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: Rewrite, Class: ConsentDisclosure, Destination: destination, Route: route, Question: question, ExpiresMs: ConsentTTL.Milliseconds()}
}

// AssignmentCard is the disclosure card shown before a problem's picture is
// sent; the picture itself goes only in the call's result.
func AssignmentCard(id, destination, route, question string) *ConsentFrame {
	return &ConsentFrame{ID: id, Feature: Assignment, Class: ConsentDisclosure, Destination: destination, Route: route, Question: question, ExpiresMs: ConsentTTL.Milliseconds()}
}

// cardPrefixes maps each Phase 16 card's feature to its id prefix.
var cardPrefixes = map[string]string{
	MemoryForget: ForgetCardPrefix,
	FeedGuard:    GuardCardPrefix,
	Recall:       RecallCardPrefix,
	Rewrite:      RewriteCardPrefix,
	Assignment:   AssignmentCardPrefix,
}

// a Phase 16 card starts with its feature's prefix; a recall card discloses
// a hosted route, as a local recall asks nothing
func validPhase16Card(f ConsentFrame) error {
	prefix := cardPrefixes[f.Feature]
	if !strings.HasPrefix(f.ID, prefix) || len(f.ID) == len(prefix) {
		return fmt.Errorf("a %s card's id starts %q", f.Feature, prefix)
	}
	if f.Feature == Recall && f.Destination != "hosted" {
		return errors.New("a recall card discloses a hosted route; a local recall asks nothing")
	}
	return nil
}

// KindFocus is the focus guard's frame kind.
const KindFocus = "focus"

// Focus phases: the guard is not armed, it is armed, or it just ended with
// the user kept off the feeds. A frame never names a feed, a window or an
// app.
const (
	FocusOff   = "off"
	FocusArmed = "armed"
	FocusDone  = "done"
)

// FocusPhases are every focus phase.
var FocusPhases = []string{FocusOff, FocusArmed, FocusDone}

// focusClosedMax bounds the closes one guard reports.
const focusClosedMax = 10000

// FocusFrame is the focus guard's state for the presenter: its phase, the
// time an armed guard has left, at most the spec's focus.maxMinutes, and
// how many windows or tabs it closed. It carries no title, word or app.
type FocusFrame struct {
	Phase  string `json:"phase"`
	LeftMs int64  `json:"leftMs"`
	Closed int    `json:"closed"`
}

func (FocusFrame) Kind() string { return KindFocus }

func (f FocusFrame) validate() error {
	if f.Closed < 0 || f.Closed > focusClosedMax {
		return fmt.Errorf("closed is outside 0..%d", focusClosedMax)
	}
	switch f.Phase {
	case FocusOff:
		if f.LeftMs != 0 || f.Closed != 0 {
			return errors.New("an unarmed guard has no time left and closed nothing")
		}
	case FocusArmed:
		if max := phase16Spec().Focus.Max().Milliseconds(); f.LeftMs < 1 || f.LeftMs > max {
			return fmt.Errorf("an armed guard has 1..%d ms left", max)
		}
	case FocusDone:
		if f.LeftMs != 0 {
			return errors.New("a finished guard has no time left")
		}
	default:
		return fmt.Errorf("unknown focus phase %q", f.Phase)
	}
	return nil
}

// the Phase 16 calls decode into protocol's params through these
type (
	rewriteCall      protocol.OniRewriteParams
	recallCall       protocol.OniRecallParams
	recallImportCall protocol.OniRecallImportParams
	focusCall        protocol.OniFocusParams
	assignmentCall   protocol.OniAssignmentParams
)

func validCardID(id string) bool { return validCode(id, protocol.OniCardIDMax) }

func (c rewriteCall) validate() error {
	switch c.Action {
	case protocol.OniRewriteStart:
		if c.ID != "" {
			return errors.New("a start names no card")
		}
		switch {
		case c.Style == protocol.OniRewriteCustom:
			if strings.TrimSpace(c.Instruction) != c.Instruction || !protocol.ValidOniText(c.Instruction, protocol.OniRewriteInstructionMax, false) {
				return fmt.Errorf("a custom rewrite gives one trimmed line of at most %d characters", protocol.OniRewriteInstructionMax)
			}
		case c.Style != "" && !slices.Contains(protocol.OniRewriteStyles, c.Style):
			return fmt.Errorf("style %q is not a rewrite style", c.Style)
		case c.Instruction != "":
			return errors.New("only a custom rewrite gives an instruction")
		}
	case protocol.OniRewriteApply, protocol.OniRewriteUndo, protocol.OniRewriteCancel:
		if !validCardID(c.ID) {
			return fmt.Errorf("%s names its card or record by a short code", c.Action)
		}
		if c.Style != "" || c.Instruction != "" {
			return fmt.Errorf("%s takes no style or instruction", c.Action)
		}
	default:
		return fmt.Errorf("action %q is not start, apply, undo or cancel", c.Action)
	}
	return nil
}

func (c recallCall) validate() error {
	switch c.Action {
	case protocol.OniRecallAsk:
		if strings.TrimSpace(c.Text) == "" || !protocol.ValidOniText(c.Text, protocol.OniRecallTextMax, false) {
			return fmt.Errorf("an ask gives a question of at most %d characters on one line", protocol.OniRecallTextMax)
		}
	case protocol.OniRecallKnow:
		if c.Text != "" && (strings.TrimSpace(c.Text) == "" || !protocol.ValidOniText(c.Text, protocol.OniRecallTopicMax, false)) {
			return fmt.Errorf("a topic is one line of at most %d characters", protocol.OniRecallTopicMax)
		}
	default:
		return fmt.Errorf("action %q is not ask or know", c.Action)
	}
	return nil
}

func (c recallImportCall) validate() error {
	if len(c.Paths) == 0 || len(c.Paths) > protocol.OniImportPathsMax {
		return fmt.Errorf("an import names 1 to %d files", protocol.OniImportPathsMax)
	}
	seen := map[string]bool{}
	for _, p := range c.Paths {
		if !protocol.ValidOniImportPath(p) {
			return errors.New("an import names absolute, clean .json or .zip paths")
		}
		if seen[p] {
			return errors.New("an import names each file once")
		}
		seen[p] = true
	}
	return nil
}

func (c focusCall) validate() error {
	switch c.Action {
	case protocol.OniFocusStatus, protocol.OniFocusStop, protocol.OniFocusBack:
		if len(c.Words) != 0 || c.Minutes != 0 || c.UntilApp != "" {
			return fmt.Errorf("%s takes no words, minutes or app", c.Action)
		}
	case protocol.OniFocusArm:
		if (c.Minutes != 0) == (c.UntilApp != "") {
			return errors.New("an arm names minutes or an app to last until, not both")
		}
		if max := phase16Spec().Focus.MaxMinutes; c.Minutes < 0 || c.Minutes > max {
			return fmt.Errorf("a guard lasts 1 to %d minutes", max)
		}
		if c.UntilApp != "" && (strings.TrimSpace(c.UntilApp) != c.UntilApp || !protocol.ValidOniText(c.UntilApp, protocol.OniFocusAppMax, false)) {
			return fmt.Errorf("the app is one trimmed line of at most %d characters", protocol.OniFocusAppMax)
		}
		if len(c.Words) != 0 {
			if clean, ok := protocol.CleanOniFeedWords(c.Words); !ok || len(clean) != len(c.Words) {
				return fmt.Errorf("feed words are at most %d distinct words of up to %d characters each", protocol.OniFeedWordsMax, protocol.OniFeedWordMaxRunes)
			}
		}
	default:
		return fmt.Errorf("action %q is not status, arm, stop or back", c.Action)
	}
	return nil
}

func (c assignmentCall) validate() error {
	if !slices.Contains(protocol.OniAssignmentActions, c.Action) {
		return fmt.Errorf("action %q is not start, next, back, done, stop or status", c.Action)
	}
	if c.Style == "" {
		return nil
	}
	if c.Action != protocol.OniAssignmentStart {
		return fmt.Errorf("%s takes no style", c.Action)
	}
	if !slices.Contains(protocol.OniAssignmentStyles, c.Style) {
		return fmt.Errorf("style %q is not walkthrough or answers", c.Style)
	}
	return nil
}
