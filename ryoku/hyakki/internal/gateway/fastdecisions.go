package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/localmodels"
	"ryoku-rashin/internal/protocol"
)

var (
	errDecisionCredits = errors.New("decision credits unavailable")
	errDecisionRoute   = errors.New("decision route unavailable")
)

// Decide is a bounded classifier call, not a thinking-model prompt or an input
// executor. Rules are owned by its consumer; unresolved means ask or use rules.
func (g *Gateway) Decide(ctx context.Context, p protocol.FastDecisionParams) (protocol.FastDecisionResult, error) {
	started := g.now()
	result := protocol.FastDecisionResult{Route: protocol.DecisionRules, Notice: i18n.T("No model was used. Continue with built-in rules or ask for clarification.")}
	finish := func() protocol.FastDecisionResult {
		result.ElapsedMS = max(0, g.now().Sub(started).Milliseconds())
		return result
	}
	var cfg protocol.DecisionRoutingSettings
	if g.settings != nil {
		cfg = g.settings().Providers.Decisions
	}
	if p.Feature != "" && !protocol.ValidDecisionFeature(p.Feature) {
		return result, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The decision feature identifier is invalid."))
	}
	route := p.Route
	if route == "" {
		route = cfg.Features[p.Feature]
		if route == "" {
			route = cfg.Route
		}
	}
	if route == "" {
		route = protocol.DecisionRules
	}
	if !protocol.ValidDecisionRoute(route) {
		return result, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a known decision route."))
	}
	if p.Model != "" && (route == protocol.DecisionAuto || route == protocol.DecisionRules) {
		return result, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Pin the local or Ember route when choosing a decision model."))
	}
	requestedModel := p.Model
	if p.Model == "" {
		p.Model = protocol.EmberDecisionModel
	}
	spec, err := protocol.CompileDecisionRequest(p.DecisionRequest)
	if err != nil {
		return result, protocol.Errorf(protocol.CodeBadRequest, "%s", err.Error())
	}
	if route == protocol.DecisionRules {
		return finish(), nil
	}
	budget := cfg.BudgetMS
	if budget == 0 {
		budget = protocol.DefaultDecisionBudgetMS
	}
	if p.BudgetMS != 0 {
		budget = p.BudgetMS
	}
	if budget < 100 || budget > 5000 {
		return result, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The decision time budget must be between 100 and 5000 milliseconds."))
	}
	if p.IdempotencyKey != "" && !decisionIdempotency(p.IdempotencyKey) {
		return result, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("The decision idempotency key is malformed."))
	}
	call, cancel := context.WithTimeout(ctx, time.Duration(budget)*time.Millisecond)
	defer cancel()
	stop := context.AfterFunc(g.ctx, cancel)
	defer stop()
	var routes [2]string
	count := 1
	switch route {
	case protocol.DecisionAuto:
		count = 0
		if cfg.LocalModel != "" {
			routes[count] = protocol.DecisionLocal
			count++
		}
		if cfg.AllowHosted {
			routes[count] = protocol.DecisionEmber
			count++
		}
	case protocol.DecisionEmber:
		routes[0] = route
		if cfg.LocalModel != "" {
			routes[1] = protocol.DecisionLocal
			count = 2
		}
	default:
		routes[0] = route
	}
	var previous, refusal string
	for index := 0; index < count; index++ {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if call.Err() != nil {
			break
		}
		chosen := routes[index]
		model := cfg.LocalModel
		if route == protocol.DecisionLocal && requestedModel != "" {
			model = requestedModel
		}
		if chosen == protocol.DecisionEmber {
			model = protocol.EmberDecisionModel
			if route == protocol.DecisionEmber {
				model = p.Model
			}
		}
		var answer protocol.DecisionResponse
		var units int64
		var attemptErr error
		attemptCtx := call
		var attemptCancel context.CancelFunc
		if index+1 < count {
			remaining := time.Duration(budget) * time.Millisecond
			if deadline, ok := call.Deadline(); ok {
				remaining = time.Until(deadline)
			}
			attemptCtx, attemptCancel = context.WithTimeout(call, remaining/2)
		}
		if chosen == protocol.DecisionLocal {
			if model == "" {
				attemptErr = errDecisionRoute
			} else if status := g.localDecisionStatus(attemptCtx, cfg.LocalAddress); status.State != protocol.DecisionLocalDetected {
				// absent, too small or not on this computer: refused with
				// the reason, never asked
				attemptErr, refusal = errDecisionRoute, status.Reason
			} else {
				answer, units, attemptErr = g.fetchDecision(attemptCtx, g.local.client.Transport, status.Address+"/systemone", "local", spec, model, "", false)
			}
		} else {
			answer, units, attemptErr = g.hostedDecision(attemptCtx, spec, model, p.IdempotencyKey, p.AccountID)
		}
		if attemptCancel != nil {
			attemptCancel()
		}
		if attemptErr != nil {
			if chosen == protocol.DecisionEmber {
				if errors.Is(attemptErr, errDecisionCredits) {
					result.CreditsExhausted = true
					previous = i18n.T("Ember credits are insufficient; no hosted decision was dispatched.")
				} else {
					previous = i18n.T("Ryoku Ember did not return a verified decision. Any already-dispatched usage is settled by its server.")
				}
			} else if refusal != "" {
				previous = refusal + " " + i18n.T("No decision data was sent to the local route.")
			} else {
				previous = i18n.T("The local decision model did not return a verified decision.")
			}
			continue
		}
		provider := "decision-local"
		result.Notice = i18n.T("This decision stayed on this computer. Model output does not grant permission.")
		if chosen == protocol.DecisionEmber {
			provider = protocol.ProviderEmber
			result.Notice = i18n.T("This decision went to Ryoku Ember. Its state and typed questions left this device for the hosted inference provider and spent the same finite Ember credits as other hosted work. Model output does not grant permission.")
		}
		if previous != "" {
			result.Notice = previous + " " + result.Notice
		}
		result.Route, result.Resolved, result.Decision, result.EmberUnits = chosen, true, &answer, units
		_ = g.store.recordUsage(g.now(), provider, answer.Model, true, protocol.UsageRow{InputTokens: answer.Usage.Input, OutputTokens: answer.Usage.Output})
		return finish(), nil
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if previous != "" {
		result.Notice = previous + " " + result.Notice
	}
	return finish(), nil
}

const (
	// decisionProbeBudget bounds one look for the local decision server. A
	// server on this computer lists its models at once; one that has not
	// within this long cannot answer a decision inside its budget either.
	decisionProbeBudget = 750 * time.Millisecond
	// decisionProbeMax bounds the model list read back
	decisionProbeMax = 256 << 10
)

// LocalDecisions looks once for the local decision server at the configured
// address. It is asked for on demand, by Settings or a decision, and never
// polled. Nothing is asked unless the address is a loopback one and this
// computer passes the hardware gate; then one GET of its model list, with
// no redirect, retry or proxy, says whether a server is there.
func (g *Gateway) LocalDecisions(ctx context.Context) protocol.DecisionLocalStatus {
	var address string
	if g.settings != nil {
		address = g.settings().Providers.Decisions.LocalAddress
	}
	return g.localDecisionStatus(ctx, address)
}

func (g *Gateway) localDecisionStatus(ctx context.Context, address string) protocol.DecisionLocalStatus {
	if address == "" {
		address = protocol.DefaultDecisionAddress
	}
	base, ok := protocol.ParseDecisionAddress(address)
	if !ok {
		return protocol.DecisionLocalStatus{State: protocol.DecisionLocalRefused, Reason: i18n.T("The local decision address is not a loopback address on this computer, so nothing was asked there.")}
	}
	status := protocol.DecisionLocalStatus{State: protocol.DecisionLocalAbsent, Address: base}
	var hardware localmodels.Hardware
	if g.decisionHardware != nil {
		hardware = g.decisionHardware()
	}
	status.RAMMiB, status.Cores = hardware.RAMMiB, hardware.Cores
	if fits, reason := localmodels.DecisionHardware(hardware); !fits {
		status.State, status.Reason = protocol.DecisionLocalSmall, reason
		return status
	}
	notFound := i18n.Tf("No local decision server answers at %s. Hyakki installs and starts none; start the one you installed, or choose Built-in rules.", base)
	probe, cancel := context.WithTimeout(ctx, decisionProbeBudget)
	defer cancel()
	slow := func() bool { return ctx.Err() == nil && probe.Err() != nil }
	request, err := http.NewRequestWithContext(probe, http.MethodGet, base+"/models", nil)
	if err != nil {
		status.Reason = notFound
		return status
	}
	// a single transport exchange: no redirect is followed and nothing is
	// tried twice
	response, err := g.local.client.Transport.RoundTrip(request)
	if err != nil {
		status.Reason = notFound
		if slow() {
			status.Reason = i18n.Tf("The local decision server at %s did not answer within %d ms.", base, decisionProbeBudget.Milliseconds())
		}
		return status
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, decisionProbeMax+1))
	if err != nil && slow() {
		status.Reason = i18n.Tf("The local decision server at %s did not answer within %d ms.", base, decisionProbeBudget.Milliseconds())
		return status
	}
	var listing struct {
		Data []json.RawMessage `json:"data"`
	}
	if err != nil || response.StatusCode != http.StatusOK || len(raw) > decisionProbeMax || json.Unmarshal(raw, &listing) != nil || listing.Data == nil {
		status.Reason = i18n.Tf("Something answers at %s, but not as a decision server.", base)
		return status
	}
	status.State = protocol.DecisionLocalDetected
	return status
}

func decisionIdempotency(value string) bool {
	if len(value) < 8 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == ':' || r == '-') {
			return false
		}
	}
	return true
}

func (g *Gateway) hostedDecision(ctx context.Context, spec protocol.DecisionSpec, model, idem, expectedAccount string) (protocol.DecisionResponse, int64, error) {
	var empty protocol.DecisionResponse
	h := g.account()
	if h == nil || expectedAccount == "" {
		return empty, 0, errDecisionRoute
	}
	base, token, bound, err := h.Endpoint(ctx, expectedAccount)
	if err != nil {
		return empty, 0, errDecisionRoute
	}
	defer h.Used()
	if idem == "" {
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return empty, 0, err
		}
		idem = "decision-" + hex.EncodeToString(nonce[:])
	}
	out, units, err := g.fetchDecision(bound, g.client.Transport, strings.TrimRight(base, "/")+"/decisions", token, spec, model, idem, true)
	var rejected *decisionHTTPError
	if errors.As(err, &rejected) && rejected.status == http.StatusUnauthorized {
		h.Rejected()
	}
	return out, units, err
}

type decisionHTTPError struct{ status int }

func (e *decisionHTTPError) Error() string { return "The decision endpoint refused the request." }

func (g *Gateway) fetchDecision(ctx context.Context, transport http.RoundTripper, endpoint, token string, spec protocol.DecisionSpec, model, idem string, hosted bool) (protocol.DecisionResponse, int64, error) {
	var empty protocol.DecisionResponse
	if transport == nil {
		return empty, 0, errDecisionRoute
	}
	body, err := json.Marshal(spec.Request(model))
	if err != nil || len(body) > protocol.DecisionBodyMax {
		return empty, 0, errDecisionRoute
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return empty, 0, errDecisionRoute
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	if idem != "" {
		request.Header.Set("Idempotency-Key", idem)
	}
	// A single transport exchange cannot follow an attacker-controlled redirect.
	response, err := transport.RoundTrip(request)
	if err != nil {
		return empty, 0, errDecisionRoute
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if hosted && response.StatusCode == http.StatusPaymentRequired {
			return empty, 0, errDecisionCredits
		}
		return empty, 0, &decisionHTTPError{status: response.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, protocol.DecisionResponseMax+1))
	if err != nil || len(raw) > protocol.DecisionResponseMax {
		return empty, 0, errDecisionRoute
	}
	var wire struct {
		protocol.DecisionResponse
		Ryoku struct {
			Units int64 `json:"units"`
		} `json:"ryoku"`
	}
	if json.Unmarshal(raw, &wire) != nil || wire.Ryoku.Units < 0 {
		return empty, 0, errDecisionRoute
	}
	if err := spec.Validate(wire.DecisionResponse); err != nil {
		return empty, 0, errDecisionRoute
	}
	if hosted {
		wire.Model = protocol.EmberDecisionModel
	} else {
		wire.Model = g.Redact(wire.Model)
	}
	return wire.DecisionResponse, wire.Ryoku.Units, nil
}
