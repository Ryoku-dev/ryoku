// Package decide is the code-owned policy over the typed decision contract
// for reflex. The tier-0 rules answer first; what they miss may be classified
// by the optional local loopback route or the credit route, and only an answer
// that clears the spec thresholds and names an offered registry intent becomes
// a planned intent. Model output never grants a Press, a Press-only intent or
// hosted spend; every miss, failure or timeout falls through to the slow path.
package decide

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

const (
	// Feature is the decision feature key reflex asks under, so the route
	// follows providers.decisions.features["oni.reflex"].
	Feature = "oni.reflex"
	// QuestionID is the one typed question every route answers.
	QuestionID = "intent"
	// None is the label for "no reflex here"; it never acts.
	None = "none"
)

// creditLine is said once when the credit route runs dry and reflex is back
// on its own rules. One sentence of at most five words.
const creditLine = "Credits ran out; rules only."

// Decider classifies one utterance through the typed contract. The gateway
// (core.FastDecider) is the local loopback and credit route; Rules adapts the
// tier-0 table. None of them can authorize or execute anything.
type Decider interface {
	Decide(ctx context.Context, p protocol.FastDecisionParams) (protocol.FastDecisionResult, error)
}

// Labels are the registry intents a decision may name, sorted, followed by
// None. Press-only intents are never offered: only the rules plan them, and
// only from a Press.
func Labels() []string {
	var labels []string
	for _, info := range reflex.Registry() {
		if !info.PressOnly && string(info.ID) != None {
			labels = append(labels, string(info.ID))
		}
	}
	slices.Sort(labels)
	return append(slices.Compact(labels), None)
}

// offered reports whether label is a registry intent a decision may name.
func offered(label string) bool {
	return label != None && slices.Contains(Labels(), label)
}

// Question is the typed question: a choice over Labels with no instructions.
func Question() map[string]protocol.DecisionQuestion {
	criteria, err := json.Marshal(Labels())
	if err != nil {
		panic(err)
	}
	return map[string]protocol.DecisionQuestion{QuestionID: {Type: "choice", Criteria: criteria}}
}

// Request builds the call for one utterance. The body carries only the
// utterance, as a JSON string fenced as data, and the registry labels: no
// screen, window title, history or instruction rides it. The model is left
// to the route; the account is attached by the caller, never here.
func Request(utterance string) (protocol.FastDecisionParams, error) {
	text := strings.TrimSpace(utterance)
	if text == "" {
		return protocol.FastDecisionParams{}, errors.New("empty utterance")
	}
	state, err := json.Marshal(text)
	if err != nil {
		return protocol.FastDecisionParams{}, err
	}
	body := protocol.DecisionRequest{State: state, Questions: Question()}
	check := body
	check.Model = protocol.EmberDecisionModel
	if _, err := protocol.CompileDecisionRequest(check); err != nil {
		return protocol.FastDecisionParams{}, err
	}
	return protocol.FastDecisionParams{DecisionRequest: body, Feature: Feature}, nil
}

func probability(p float64) bool { return !math.IsNaN(p) && !math.IsInf(p, 0) && p >= 0 && p <= 1 }

// Accept is the one threshold policy every route goes through. It acts only
// when the result is resolved, answers exactly the intent question as a
// choice over exactly the offered labels with a consistent distribution, and
// names an offered intent whose probability is the strongest, at least min,
// and ahead of the runner-up by at least margin. Thresholds outside (0, 1]
// refuse everything. The reported confidence field is not trusted; the
// returned confidence is the chosen option's probability.
func Accept(res protocol.FastDecisionResult, min, margin float64) (reflex.Intent, float64, bool) {
	if !(min > 0 && min <= 1) || !(margin > 0 && margin <= 1) {
		return "", 0, false
	}
	if !res.Resolved || res.Decision == nil || len(res.Decision.Answers) != 1 {
		return "", 0, false
	}
	a, ok := res.Decision.Answers[QuestionID]
	if !ok || a.Type != "choice" || !offered(a.Choice) {
		return "", 0, false
	}
	labels := Labels()
	if len(a.Probabilities) != len(labels) {
		return "", 0, false
	}
	sum, top := 0.0, 0.0
	for _, label := range labels {
		p, ok := a.Probabilities[label]
		if !ok || !probability(p) {
			return "", 0, false
		}
		sum += p
		top = math.Max(top, p)
	}
	if math.Abs(sum-1) > float64(len(labels))*0.00006+0.000001 {
		return "", 0, false
	}
	confidence := a.Probabilities[a.Choice]
	if confidence+0.0001 < top || confidence < min || protocol.DecisionMargin(a) < margin {
		return "", 0, false
	}
	return reflex.Intent(a.Choice), confidence, true
}

// Rules adapts the tier-0 rules to the typed result, so their answer passes
// the same Accept with the same thresholds as the local and credit routes.
// A rule's confidence goes to its intent and the rest to None.
type Rules func(utterance string) (reflex.Intent, float64, bool)

func (r Rules) Decide(_ context.Context, p protocol.FastDecisionParams) (protocol.FastDecisionResult, error) {
	result := protocol.FastDecisionResult{Route: protocol.DecisionRules}
	var text string
	if r == nil || json.Unmarshal(p.State, &text) != nil {
		return result, nil
	}
	intent, confidence, ok := r(text)
	if !ok || !probability(confidence) {
		return result, nil
	}
	probabilities := make(map[string]float64, len(Labels())+1)
	for _, label := range Labels() {
		probabilities[label] = 0
	}
	probabilities[string(intent)] += confidence
	probabilities[None] += 1 - confidence
	result.Resolved = true
	result.Decision = &protocol.DecisionResponse{
		Model: protocol.DecisionRules,
		Answers: map[string]protocol.DecisionAnswer{QuestionID: {
			Type: "choice", Choice: string(intent), Confidence: &confidence, Probabilities: probabilities,
		}},
	}
	return result, nil
}

// Ask is one utterance the rules may have missed. AccountID is the reviewed
// account the caller bound at the disclosure card after a Press; empty means
// the credit route cannot spend.
type Ask struct {
	Utterance string
	AccountID string
}

// Outcome is what the policy decided. Act false means fall through to the
// slow path with the sentence intact. Line, when set, is a plain line to say.
type Outcome struct {
	Intent     reflex.Intent
	Confidence float64
	Act        bool
	Route      string
	Line       string
	ElapsedMS  int64
}

// Chooser runs tier 0, then the decision route, through Accept with the spec
// thresholds. Once the credit route reports exhausted credits, later calls
// are pinned to the local route with no account until Restore.
type Chooser struct {
	Tier0  Decider
	Route  Decider
	Limits spec.ReflexDecisions

	mu        sync.Mutex
	exhausted bool
}

// Restore lifts the out-of-credit pin, for example after the reviewed
// account or its credits changed.
func (c *Chooser) Restore() {
	c.mu.Lock()
	c.exhausted = false
	c.mu.Unlock()
}

// Exhausted reports whether the credit route is pinned off.
func (c *Chooser) Exhausted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.exhausted
}

type answer struct {
	res protocol.FastDecisionResult
	err error
}

// call bounds one Decide by ctx even when the decider ignores it.
func call(ctx context.Context, d Decider, p protocol.FastDecisionParams) (protocol.FastDecisionResult, error) {
	done := make(chan answer, 1)
	go func() {
		res, err := d.Decide(ctx, p)
		done <- answer{res, err}
	}()
	select {
	case got := <-done:
		return got.res, got.err
	case <-ctx.Done():
		return protocol.FastDecisionResult{}, ctx.Err()
	}
}

// Choose decides one utterance. It never returns an error: a malformed
// utterance, route failure, timeout, unresolved answer or refused answer is
// an Outcome with Act false.
func (c *Chooser) Choose(ctx context.Context, ask Ask) Outcome {
	started := time.Now()
	finish := func(out Outcome) Outcome {
		out.ElapsedMS = max(0, time.Since(started).Milliseconds())
		return out
	}
	p, err := Request(ask.Utterance)
	if err != nil {
		return finish(Outcome{})
	}
	min, margin := c.Limits.MinConfidence, c.Limits.MinMargin
	if c.Tier0 != nil {
		if res, err := call(ctx, c.Tier0, p); err == nil {
			if intent, confidence, ok := Accept(res, min, margin); ok {
				return finish(Outcome{Intent: intent, Confidence: confidence, Act: true, Route: protocol.DecisionRules})
			}
		}
	}
	if c.Route == nil || ctx.Err() != nil {
		return finish(Outcome{})
	}
	c.mu.Lock()
	pinned := c.exhausted
	c.mu.Unlock()
	if pinned {
		p.Route = protocol.DecisionLocal
	} else {
		p.AccountID = ask.AccountID
	}
	budget := c.Limits.BudgetMs
	if budget <= 0 {
		budget = protocol.DefaultDecisionBudgetMS
	}
	p.BudgetMS = budget
	bounded, cancel := context.WithTimeout(ctx, time.Duration(budget)*time.Millisecond)
	defer cancel()
	res, err := call(bounded, c.Route, p)
	if err != nil {
		return finish(Outcome{})
	}
	var out Outcome
	if !pinned && res.CreditsExhausted {
		c.mu.Lock()
		if !c.exhausted {
			c.exhausted = true
			out.Line = i18n.T(creditLine)
		}
		c.mu.Unlock()
	}
	// An answer that claims the credit route without a bound account, or
	// after the pin, is a spend nobody allowed: refuse it.
	if res.Route == protocol.DecisionEmber && (pinned || p.AccountID == "") {
		return finish(out)
	}
	if res.Route != protocol.DecisionLocal && res.Route != protocol.DecisionEmber {
		return finish(out)
	}
	if intent, confidence, ok := Accept(res, min, margin); ok {
		out.Intent, out.Confidence, out.Act, out.Route = intent, confidence, true, res.Route
	}
	return finish(out)
}
