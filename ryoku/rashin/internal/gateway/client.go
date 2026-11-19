package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

var (
	streamIdleGap = 60 * time.Second
	errStreamIdle = errors.New("provider stream was idle for 60 seconds")
)

type routeCandidate struct {
	provider *provider
	model    *model
	keyID    string
	stat     successStat
	hasStat  bool
	// place is the provider's position in the provider order: Rashin's own
	// models, the user's llama.cpp, Ollama, then the catalogue
	place int
}

func (c routeCandidate) local() bool { return c.provider.Tier == protocol.TierLocal }

type upstreamError struct {
	status     int
	body       string
	retryAfter time.Duration
	cause      error
}

func (e *upstreamError) Error() string {
	if e.body != "" {
		return safeProviderError(e.status, e.body, e.cause)
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return fmt.Sprintf("provider returned HTTP %d", e.status)
}

func (e *upstreamError) Unwrap() error   { return e.cause }
func (e *upstreamError) StatusCode() int { return e.status }

func (e *upstreamError) ContextTooLong() bool {
	lower := strings.ToLower(e.body)
	if lower == "" && e.cause != nil {
		lower = strings.ToLower(e.cause.Error())
	}
	return e.status == http.StatusRequestEntityTooLarge || strings.Contains(lower, "context length") ||
		strings.Contains(lower, "context window") || strings.Contains(lower, "too many tokens") ||
		strings.Contains(lower, "exceeds the available context size")
}

// Stream starts a routed OpenAI-compatible chat completion.
func (g *Gateway) Stream(ctx context.Context, request llm.Request) (llm.Stream, error) {
	candidates, err := g.candidates(ctx, request)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 && userImages(request) {
		return nil, &llm.NoRoute{Reason: g.noPictureModel(), NoVision: true}
	}
	if len(candidates) == 0 {
		return nil, &llm.NoRoute{Reason: i18n.T("No free model is available. Add a free key in Rashin > Settings > Providers, or start a model on this computer.")}
	}
	// a request that cannot be rendered fails the same for every model
	if _, err := renderOpenAI(request, candidates[0].provider, candidates[0].model); err != nil {
		return nil, err
	}
	stream := &routedStream{gateway: g, ctx: ctx, request: request, candidates: candidates, started: g.now()}
	stream.strict = request.Strict && pinnedRoute(request.Model)
	return stream, nil
}

// Window is the context window of the model request would go to first, 0
// when nothing would take it or its window is unknown. A managed model may
// need to load first; its advertised training window is not its allocation.
func (g *Gateway) Window(ctx context.Context, request llm.Request) int {
	candidates, err := g.candidates(ctx, request)
	if err != nil {
		return 0
	}
	for _, candidate := range candidates {
		if candidate.provider.ID != protocol.ProviderRashinModels {
			return candidate.model.Context
		}
		local := g.managed()
		if local == nil {
			continue
		}
		if leased, ok := local.(interface{ Acquire() func() }); ok {
			defer leased.Acquire()()
		}
		endpoint, bearer, err := local.Ensure(ctx, candidate.model.ID)
		if err != nil {
			continue
		}
		window := g.managedWindow(ctx, endpoint, bearer, candidate.model.ID)
		if len(request.Tools) > 0 && window < 8192 {
			if request.Strict && pinnedRoute(request.Model) {
				return 0
			}
			continue
		}
		return window
	}
	return 0
}

// noPictureModel says that nothing in use reads pictures, and names the
// catalogue's providers that would.
func (g *Gateway) noPictureModel() string {
	var names []string
	for i := range g.catalog.Providers {
		for _, m := range g.catalog.Providers[i].Models {
			if m.Vision {
				names = append(names, g.catalog.Providers[i].Name)
				break
			}
		}
	}
	if len(names) == 0 {
		return i18n.T("None of the providers reads pictures.")
	}
	return i18n.Tf("None of the providers in use reads pictures. These can, with a key added in Rashin > Settings > Providers: %s.", strings.Join(names, ", "))
}

// candidates are the models a request may go to, in the order they are
// tried. Auto, Fast and Smart never reach a paid model: one answers only
// when it is the model asked for.
func (g *Gateway) candidates(ctx context.Context, request llm.Request) ([]routeCandidate, error) {
	modelRoute := strings.TrimSpace(request.Model)
	pinned := pinnedRoute(modelRoute)
	strictPinned := pinned && request.Strict
	// A model without vision answers a picture it never saw, or refuses the
	// whole request, so the user's pictures only ever go to one that reads
	// them. A tool's pictures prefer such a model and are left out for one
	// that cannot, when no other is in use or it is the model asked for.
	pictures := userImages(request)
	toolPictures := !pictures && toolImages(request)
	var candidates, blind []routeCandidate
	for place, p := range g.providers(ctx, false) {
		if request.LocalOnly && p.Tier != protocol.TierLocal {
			continue
		}
		if enabled, err := g.store.providerEnabled(p.ID); err != nil {
			return nil, err
		} else if !enabled {
			continue
		}
		if _, cooling, err := g.store.cooldown(providerScope(p.ID), g.now()); err != nil {
			return nil, err
		} else if cooling {
			continue
		}
		keys, err := g.routeKeys(p)
		if err != nil {
			return nil, err
		}
		if len(keys) == 0 {
			continue
		}
		cooling, err := g.store.modelCooldowns(p.ID, g.now())
		if err != nil {
			return nil, err
		}
		for j := range p.Models {
			m := &p.Models[j]
			picked := modelRoute == p.ID+"/"+m.ID
			if p.ID == protocol.ProviderRashinModels && len(request.Tools) > 0 {
				if !m.Tools || m.Context < 8192 || m.Params < 500_000_000 {
					if strictPinned && picked {
						return nil, &llm.NoRoute{Reason: managedModelReason(m.Name, m.Tools, m.Context, m.Params)}
					}
					continue
				}
			}
			// a model whose tool support nobody knows answers only when picked by name
			if len(request.Tools) > 0 && m.toolsUnknown && !picked {
				continue
			}
			if len(request.Tools) > 0 && !m.Tools && !m.toolsUnknown {
				continue
			}
			if strictPinned && !picked {
				continue
			}
			if p.tierOf(m) == protocol.TierPaid && !picked {
				continue
			}
			if pictures && !m.Vision {
				if strictPinned {
					return nil, &llm.NoRoute{Reason: i18n.Tf("%s cannot read pictures. Choose Auto, or a model that can.", m.Name), NoVision: true}
				}
				continue
			}
			if _, ok := cooling[m.ID]; ok {
				continue
			}
			stat, hasStat, err := g.store.stat(p.ID, m.ID)
			if err != nil {
				return nil, err
			}
			for _, keyID := range keys {
				candidate := routeCandidate{provider: p, model: m, keyID: keyID, stat: stat, hasStat: hasStat, place: place}
				if toolPictures && !m.Vision && !strictPinned {
					blind = append(blind, candidate)
					continue
				}
				candidates = append(candidates, candidate)
			}
		}
	}
	if len(candidates) == 0 {
		candidates = blind
	}
	if strictPinned && len(candidates) == 0 {
		if pictures {
			return nil, &llm.NoRoute{Reason: g.noPictureModel(), NoVision: true}
		}
		if strings.HasPrefix(modelRoute, protocol.ProviderRashinModels+"/") {
			return nil, &llm.NoRoute{Reason: g.managedUnavailable(ctx, strings.TrimPrefix(modelRoute, protocol.ProviderRashinModels+"/"), request)}
		}
		return nil, &llm.NoRoute{Reason: i18n.Tf("The model you chose is not available now: %s. Choose Auto, or wait for it.", modelRoute)}
	}
	models := g.settings().Providers.Models
	sortCandidates(candidates, routeOrder{
		route:       modelRoute,
		remoteFirst: models.RemoteFirst,
		light:       request.Light && !pinned,
		lightModel:  strings.TrimSpace(models.Light),
	})
	if pinned && !request.Strict {
		preferPinnedCandidates(candidates, modelRoute)
	}
	return candidates, nil
}

func managedModelReason(name string, tools bool, context int, params int64) string {
	switch {
	case !tools:
		return i18n.Tf("%s has no tool calling. Choose a model with tool calling and at least 8K context for Ask or Agents.", name)
	case context == 0:
		return i18n.Tf("%s has no verified context window. Choose a model with at least 8K context for Ask or Agents.", name)
	case context < 8192:
		return i18n.Tf("%s was trained for only %d context tokens. Choose a model with at least 8K context for Ask or Agents.", name, context)
	case params < 500_000_000:
		return i18n.Tf("%s has fewer than 0.5B parameters and cannot serve Ask or Agents. Choose a larger model.", name)
	}
	return ""
}

func (g *Gateway) managedUnavailable(ctx context.Context, id string, request llm.Request) string {
	local := g.managed()
	if local == nil {
		return i18n.T("Rashin's model runtime is missing. Set up llama.cpp in Settings > Providers > Local models.")
	}
	result := local.Models(ctx)
	for _, model := range result.Models {
		if model.ID != id {
			continue
		}
		if model.State != protocol.LocalInstalled {
			return i18n.Tf("%s is not downloaded. Download it in Settings > Providers > Local models, or choose Auto.", model.Name)
		}
		if len(request.Tools) > 0 {
			if reason := managedModelReason(model.Name, model.Tools, model.Context, model.Params); reason != "" {
				return reason
			}
		}
		if model.Error != "" {
			return i18n.Tf("%s failed to load: %s. Open the llama.cpp log in Settings > Providers > Local models, or choose Auto.", model.Name, model.Error)
		}
		if result.Runtime.State == protocol.RuntimeMissing || result.Runtime.State == protocol.RuntimeTooOld {
			return i18n.Tf("%s cannot start: %s. Install or update llama.cpp in Settings > Providers > Local models.", model.Name, result.Runtime.Error)
		}
		return i18n.Tf("%s is temporarily unavailable. Check its llama.cpp log in Settings > Providers > Local models, or choose Auto.", model.Name)
	}
	return i18n.Tf("%s is not on this computer. Download it in Settings > Providers > Local models, or choose Auto.", id)
}

// pinnedRoute says the request names one model rather than Auto, Fast or Smart.
func pinnedRoute(route string) bool {
	route = strings.TrimSpace(route)
	return route != "" && route != protocol.RouteAuto && route != protocol.RouteFast && route != protocol.RouteSmart
}

func (g *Gateway) routeKeys(p *provider) ([]string, error) {
	if p.Keyless {
		return []string{""}, nil
	}
	records, err := g.store.listKeys(p.ID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, record := range records {
		if record.Status == protocol.KeyError {
			continue
		}
		if _, cooling, err := g.store.cooldown(keyScope(record.ID), g.now()); err != nil {
			return nil, err
		} else if !cooling {
			ids = append(ids, record.ID)
		}
	}
	return ids, nil
}

// routeOrder is how a request's candidates are ranked.
type routeOrder struct {
	route string
	// remoteFirst puts free remote models ahead of the ones on this
	// computer (providers.models.remoteFirst)
	remoteFirst bool
	// light is internal work with no pin: lightModel (provider/model) when
	// it is set and can answer, then the smallest local models, then the
	// quickest free remote ones
	light      bool
	lightModel string
}

func sortCandidates(candidates []routeCandidate, order routeOrder) {
	sort.SliceStable(candidates, func(i, j int) bool { return order.before(candidates[i], candidates[j]) })
}

func (o routeOrder) before(a, b routeCandidate) bool {
	if o.light {
		if ar, br := o.lightRank(a), o.lightRank(b); ar != br {
			return ar < br
		}
		if a.local() {
			if ap, bp := paramsKey(a, true), paramsKey(b, true); ap != bp {
				return ap < bp
			}
			return a.place < b.place
		}
		return remoteBefore(a, b, protocol.RouteFast)
	}
	// a model on this computer answers with no account and no quota
	if a.local() != b.local() {
		return a.local() != o.remoteFirst
	}
	if a.local() {
		// the user's choice of where models run comes first: Rashin's own,
		// then llama.cpp, then Ollama; within one, the size suits the route
		if a.place != b.place {
			return a.place < b.place
		}
		if o.route == protocol.RouteFast && a.hasStat && b.hasStat && a.stat.TTFBMs != b.stat.TTFBMs {
			return a.stat.TTFBMs < b.stat.TTFBMs
		}
		fewest := o.route == protocol.RouteFast
		return paramsKey(a, fewest) < paramsKey(b, fewest)
	}
	return remoteBefore(a, b, o.route)
}

// lightRank orders light work: the configured light model, local models of
// class light, other local models, then remote ones.
func (o routeOrder) lightRank(c routeCandidate) int {
	switch {
	case o.lightModel != "" && c.provider.ID+"/"+c.model.ID == o.lightModel:
		return 0
	case c.local() && c.model.Light:
		return 1
	case c.local():
		return 2
	}
	return 3
}

// paramsKey sorts on parameter count, fewest or most first; an unknown
// count goes after every known one.
func paramsKey(c routeCandidate, fewest bool) int64 {
	switch {
	case c.model.Params <= 0:
		return math.MaxInt64
	case fewest:
		return c.model.Params
	}
	return math.MaxInt64 - 1 - c.model.Params
}

// remoteBefore ranks two hosted models for a route: Fast by measured time to
// first token then speed, Smart by quality, Auto by quality then latency.
func remoteBefore(a, b routeCandidate, route string) bool {
	if route == protocol.RouteFast {
		if a.hasStat != b.hasStat {
			return a.hasStat
		}
		if a.hasStat && a.stat.TTFBMs != b.stat.TTFBMs {
			return a.stat.TTFBMs < b.stat.TTFBMs
		}
		if a.model.SpeedRank != b.model.SpeedRank {
			return a.model.SpeedRank > b.model.SpeedRank
		}
		return a.model.QualityRank > b.model.QualityRank
	}
	if a.model.QualityRank != b.model.QualityRank {
		return a.model.QualityRank > b.model.QualityRank
	}
	if route == protocol.RouteSmart {
		return false
	}
	if route == protocol.RouteAuto || route == "" {
		if a.hasStat != b.hasStat {
			return a.hasStat
		}
		if a.hasStat && a.stat.LatencyMs != b.stat.LatencyMs {
			return a.stat.LatencyMs < b.stat.LatencyMs
		}
	}
	return false
}

func preferPinnedCandidates(candidates []routeCandidate, modelRoute string) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i].provider.ID+"/"+candidates[i].model.ID == modelRoute
		right := candidates[j].provider.ID+"/"+candidates[j].model.ID == modelRoute
		return left && !right
	})
}

type routedStream struct {
	gateway    *Gateway
	ctx        context.Context
	request    llm.Request
	candidates []routeCandidate
	started    time.Time

	// strict is a request pinned to one model, which never goes elsewhere, so
	// a failure is told as that model's and failure is the last one's cause.
	strict  bool
	failure error

	attempts       int
	next           int
	current        attempt
	candidate      routeCandidate
	attemptStarted time.Time
	// deadline is when the current attempt must have said something. A
	// model on this computer may first have to load, so it waits longer;
	// localSpent is that waiting, which the remote budget does not count.
	deadline   time.Time
	localSpent time.Duration
	committed  bool
	finished   bool
	closed     bool
	firstAt    time.Time
	output     int
	final      llm.Final
	mu         sync.Mutex
}

func (s *routedStream) Next() (llm.Delta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.finished {
		return llm.Delta{}, io.EOF
	}
	for {
		if s.current == nil {
			if s.attempts >= maxAttempts || s.next >= len(s.candidates) {
				s.finished = true
				if s.strict {
					return llm.Delta{}, s.pinFailed(s.failure)
				}
				message := errors.New(i18n.T("Every available model failed. Check Providers for cooldowns and key errors"))
				if s.failure != nil {
					return llm.Delta{}, &routedFailure{message: message, cause: s.failure}
				}
				return llm.Delta{}, message
			}
			next := s.candidates[s.next]
			if !s.committed && !next.local() && !s.gateway.now().Before(s.remoteDeadline()) {
				s.finished = true
				if s.strict {
					return llm.Delta{}, &llm.NoRoute{Reason: i18n.Tf("%s did not answer within 45 seconds. Pick another model or Auto.", s.pinnedName())}
				}
				return llm.Delta{}, errors.New(i18n.T("No provider produced a response within 45 seconds"))
			}
			s.candidate = next
			s.next++
			s.attempts++
			s.attemptStarted = s.gateway.now()
			s.deadline = s.remoteDeadline()
			if next.local() {
				s.deadline = s.attemptStarted.Add(localFirstByteBudget)
			}
			wire, err := renderOpenAI(s.request, next.provider, next.model)
			if err != nil {
				s.finished = true
				return llm.Delta{}, err
			}
			attempt, status, body, err := s.startCandidate(wire)
			if err != nil {
				s.endAttempt()
				if !s.handleFailure(status, body, err) {
					s.finished = true
					if s.strict {
						return llm.Delta{}, s.pinFailed(err)
					}
					return llm.Delta{}, err
				}
				continue
			}
			s.current = wrapDialect(attempt, next.provider, next.model, s.request)
		}

		delta, err := s.nextAttemptDelta()
		if err == nil {
			if delta.Text == "" && delta.Reasoning == "" && delta.Signature == "" && delta.ToolStart == nil && len(delta.ToolCalls) == 0 {
				continue
			}
			if !s.committed {
				s.committed = true
				s.firstAt = s.gateway.now()
			}
			s.output += len(delta.Text) + len(delta.Reasoning)
			for _, call := range delta.ToolCalls {
				s.output += len(call.Name) + len(call.Arguments)
			}
			return delta, nil
		}
		if errors.Is(err, io.EOF) {
			if !s.committed && s.current.outcome().emitted == 0 {
				_ = s.current.Close()
				s.current = nil
				s.endAttempt()
				if !s.handleFailure(0, "", errors.New("provider returned an empty completion")) {
					s.finished = true
					return llm.Delta{}, errors.New(i18n.T("Provider returned an empty completion"))
				}
				continue
			}
			s.finishSuccess()
			return llm.Delta{}, io.EOF
		}
		if s.committed {
			s.recordFailure()
		}
		_ = s.current.Close()
		s.current = nil
		if s.committed {
			s.finished = true
			return llm.Delta{}, err
		}
		s.endAttempt()
		if !s.handleFailure(0, "", err) {
			s.finished = true
			if s.strict {
				return llm.Delta{}, s.pinFailed(err)
			}
			return llm.Delta{}, err
		}
	}
}

// remoteDeadline is when a hosted model must have answered: the first-byte
// budget from the start, not counting time spent waiting on local models.
func (s *routedStream) remoteDeadline() time.Time {
	return s.started.Add(s.localSpent + firstByteBudget)
}

// endAttempt closes the books on an attempt that gave nothing.
func (s *routedStream) endAttempt() {
	if s.candidate.local() {
		s.localSpent += s.gateway.now().Sub(s.attemptStarted)
	}
}

// timedOut says how long a model that gave nothing was waited for.
func (s *routedStream) timedOut(err error) error {
	if !errors.Is(err, context.DeadlineExceeded) || !s.candidate.local() {
		return err
	}
	return errors.New(i18n.Tf("no answer within %d minutes", int(localFirstByteBudget/time.Minute)))
}

// pinnedName is the pinned model's name, as the user picked it.
func (s *routedStream) pinnedName() string {
	candidate := s.candidate
	if candidate.model == nil {
		candidate = s.candidates[0]
	}
	if candidate.model.Name != "" {
		return candidate.model.Name
	}
	return candidate.model.ID
}

type routedFailure struct {
	message error
	cause   error
}

func (e *routedFailure) Error() string { return e.message.Error() }
func (e *routedFailure) Unwrap() []error {
	return []error{e.message, e.cause}
}

// pinFailed tells a pinned request's failure as its model's, with the
// provider's reason, and offers the way out; Every available model failed
// would be wrong for a request that only ever had one. A cancelled request
// stays cancelled.
func (s *routedStream) pinFailed(cause error) error {
	if errors.Is(cause, context.Canceled) {
		return cause
	}
	cause = s.timedOut(cause)
	reason := i18n.T("no answer")
	if cause != nil {
		if text := strings.TrimRight(strings.TrimSpace(cause.Error()), "."); text != "" {
			// a provider may quote the key it refused
			reason = s.gateway.redactKeyMessage(s.candidate.keyID, text)
		}
	}
	route := &llm.NoRoute{Reason: i18n.Tf("%s did not answer: %s. Pick another model or Auto.", s.pinnedName(), reason)}
	if cause != nil {
		return &routedFailure{message: route, cause: cause}
	}
	return route
}

func (s *routedStream) startCandidate(request chatRequest) (*sseAttempt, int, string, error) {
	remaining := s.deadline.Sub(s.gateway.now())
	if remaining <= 0 {
		return nil, 0, "", context.DeadlineExceeded
	}
	attemptCtx, cancel := context.WithCancel(s.ctx)
	// The managed router may idle out while a long response is streaming;
	// keep it leased until the attempt's body closes.
	release := func() {}
	if s.candidate.provider.ID == protocol.ProviderRashinModels {
		if local := s.gateway.managed(); local != nil {
			if leased, ok := local.(interface{ Acquire() func() }); ok {
				release = leased.Acquire()
			}
		}
	}
	var released sync.Once
	finish := func() { cancel(); released.Do(release) }
	// attemptCtx lives as long as the attempt.
	type result struct {
		attempt *sseAttempt
		status  int
		body    string
		window  int
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		attempt, status, body, window, err := s.gateway.startAttempt(attemptCtx, s.candidate, request)
		if attemptCtx.Err() != nil {
			if attempt != nil {
				_ = attempt.Close()
			}
			return
		}
		ch <- result{attempt: attempt, status: status, body: body, window: window, err: err}
	}()
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case result := <-ch:
		if s.candidate.provider.ID == protocol.ProviderRashinModels {
			// The candidate's model came from a fresh inventory value. Keep
			// the loaded allocation on this attempt, not on that inventory.
			model := *s.candidate.model
			model.Context = result.window
			s.candidate.model = &model
		}
		if result.attempt != nil {
			result.attempt.cancel = finish
		} else {
			finish()
		}
		return result.attempt, result.status, result.body, result.err
	case <-s.ctx.Done():
		finish()
		return nil, 0, "", s.ctx.Err()
	case <-timer.C:
		finish()
		return nil, 0, "", context.DeadlineExceeded
	}
}

func (s *routedStream) nextAttemptDelta() (llm.Delta, error) {
	cur := s.current
	wait := streamIdleGap
	timeoutErr := error(errStreamIdle)
	if !s.committed {
		wait = s.deadline.Sub(s.gateway.now())
		timeoutErr = context.DeadlineExceeded
		if wait <= 0 {
			return llm.Delta{}, timeoutErr
		}
	}
	type result struct {
		delta llm.Delta
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		delta, err := cur.Next()
		ch <- result{delta: delta, err: err}
	}()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case result := <-ch:
		return result.delta, result.err
	case <-s.ctx.Done():
		_ = cur.Close()
		return llm.Delta{}, s.ctx.Err()
	case <-timer.C:
		_ = cur.Close()
		return llm.Delta{}, timeoutErr
	}
}

func (s *routedStream) handleFailure(status int, body string, err error) bool {
	s.failure = err
	_ = s.gateway.store.recordUsage(s.gateway.now(), s.candidate.provider.ID, s.candidate.model.ID, false, protocol.UsageRow{})
	if s.candidate.keyID != "" {
		_ = s.gateway.applyHealth(s.candidate.keyID, status, body, false)
	}
	now := s.gateway.now()
	class := classifyFailure(status, body, err, now)
	switch class.kind {
	case failureAuth:
		if s.candidate.keyID != "" {
			s.skipKey(s.candidate.keyID)
		} else {
			s.skipProvider(s.candidate.provider.ID)
		}
		return true
	case failureRate:
		until := now.Add(class.duration)
		_ = s.gateway.store.setCooldown(modelScope(s.candidate.provider.ID, s.candidate.model.ID), until, class.reason)
		if s.candidate.keyID != "" {
			_ = s.gateway.store.setCooldown(keyScope(s.candidate.keyID), until, class.reason)
		}
		s.skipRateLimited(s.candidate.provider.ID, s.candidate.model.ID, s.candidate.keyID)
		return true
	case failureCredit, failureProvider:
		_ = s.gateway.store.setCooldown(providerScope(s.candidate.provider.ID), now.Add(class.duration), class.reason)
		s.skipProvider(s.candidate.provider.ID)
		return true
	case failureModel:
		_ = s.gateway.store.setCooldown(modelScope(s.candidate.provider.ID, s.candidate.model.ID), now.Add(class.duration), class.reason)
		s.skipModel(s.candidate.provider.ID, s.candidate.model.ID)
		return true
	case failureContext:
		s.skipContextsAtMost(s.candidate.model.Context)
		return true
	default:
		return false
	}
}

func (s *routedStream) skipProvider(id string) {
	filtered := s.candidates[:s.next]
	for _, candidate := range s.candidates[s.next:] {
		if candidate.provider.ID != id {
			filtered = append(filtered, candidate)
		}
	}
	s.candidates = filtered
}

func (s *routedStream) skipModel(providerID, modelID string) {
	filtered := s.candidates[:s.next]
	for _, candidate := range s.candidates[s.next:] {
		if candidate.provider.ID != providerID || candidate.model.ID != modelID {
			filtered = append(filtered, candidate)
		}
	}
	s.candidates = filtered
}

func (s *routedStream) skipKey(keyID string) {
	filtered := s.candidates[:s.next]
	for _, candidate := range s.candidates[s.next:] {
		if candidate.keyID != keyID {
			filtered = append(filtered, candidate)
		}
	}
	s.candidates = filtered
}

func (s *routedStream) skipRateLimited(providerID, modelID, keyID string) {
	filtered := s.candidates[:s.next]
	for _, candidate := range s.candidates[s.next:] {
		sameModel := candidate.provider.ID == providerID && candidate.model.ID == modelID
		sameKey := keyID != "" && candidate.keyID == keyID
		if !sameModel && !sameKey {
			filtered = append(filtered, candidate)
		}
	}
	s.candidates = filtered
}

func (s *routedStream) skipContextsAtMost(context int) {
	filtered := s.candidates[:s.next]
	for _, candidate := range s.candidates[s.next:] {
		if candidate.model.Context > context {
			filtered = append(filtered, candidate)
		}
	}
	s.candidates = filtered
}

// usage is what the attempt counted, or an estimate when the provider said
// nothing.
func (s *routedStream) usage(out attemptOutcome) llm.Usage {
	if out.usageKnown {
		return out.usage
	}
	return llm.Usage{Input: estimateTokens(s.request), Output: bytesToTokens(s.output),
		CacheRead: out.usage.CacheRead, CacheWrite: out.usage.CacheWrite, Estimated: true}
}

func (s *routedStream) finishSuccess() {
	out := s.current.outcome()
	usage := s.usage(out)
	finish := out.finish
	if finish == "" {
		finish = llm.FinishStop
	}
	s.final = llm.Final{
		Model: s.candidate.provider.ID + "/" + s.candidate.model.ID, Provider: s.candidate.provider.ID,
		Finish: finish, Usage: usage, Attempts: s.attempts, Context: s.candidate.model.Context,
	}
	latency := s.gateway.now().Sub(s.attemptStarted)
	ttfb := latency
	if !s.firstAt.IsZero() {
		ttfb = s.firstAt.Sub(s.attemptStarted)
	}
	_ = s.gateway.store.recordSuccess(s.candidate.provider.ID, s.candidate.model.ID, latency, ttfb)
	_ = s.gateway.store.recordUsage(s.gateway.now(), s.candidate.provider.ID, s.candidate.model.ID, true, protocol.UsageRow{InputTokens: int64(usage.Input), OutputTokens: int64(usage.Output), Estimated: usage.Estimated})
	if s.candidate.keyID != "" {
		_ = s.gateway.applyHealth(s.candidate.keyID, 200, "", true)
	}
	_ = s.current.Close()
	s.finished = true
}

func (s *routedStream) recordFailure() {
	usage := s.usage(s.current.outcome())
	_ = s.gateway.store.recordUsage(s.gateway.now(), s.candidate.provider.ID, s.candidate.model.ID, false, protocol.UsageRow{InputTokens: int64(usage.Input), OutputTokens: int64(usage.Output), Estimated: usage.Estimated})
}

func (s *routedStream) Final() llm.Final {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.final
}

// answering names the model the answer comes from. It is settled once the
// first piece is out: the router does not change models after that.
func (s *routedStream) answering() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.candidate.provider.ID + "/" + s.candidate.model.ID
}

func (s *routedStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.current != nil {
		return s.current.Close()
	}
	return nil
}

// startAttempt sends one rendered request to a candidate. Rashin's own
// models are started first when they are down; a start that fails is a
// provider failure, so the router moves on to the next candidate.
func (g *Gateway) startAttempt(ctx context.Context, candidate routeCandidate, request chatRequest) (*sseAttempt, int, string, int, error) {
	var endpoint, bearer string
	window := candidate.model.Context
	if candidate.provider.ID == protocol.ProviderRashinModels {
		local := g.managed()
		if local == nil {
			return nil, http.StatusServiceUnavailable, "", 0, &upstreamError{status: http.StatusServiceUnavailable, cause: errors.New(i18n.T("Rashin's own models are not set up"))}
		}
		var err error
		endpoint, bearer, err = local.Ensure(ctx, candidate.model.ID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, 0, "", 0, ctx.Err()
			}
			return nil, http.StatusServiceUnavailable, "", 0, &upstreamError{status: http.StatusServiceUnavailable, cause: err}
		}
		window = g.managedWindow(ctx, endpoint, bearer, candidate.model.ID)
		if len(request.Tools) > 0 && window < 8192 {
			if window == 0 {
				return nil, http.StatusBadRequest, "", 0, &upstreamError{status: http.StatusBadRequest, cause: errors.New(i18n.Tf("%s has no verified loaded context window. Check the model runtime in Local models.", candidate.model.Name))}
			}
			return nil, http.StatusBadRequest, "", window, &upstreamError{status: http.StatusBadRequest, cause: errors.New(i18n.Tf("%s has only %d context window tokens allocated. Set at least 8K context in Local models for Ask or Agents.", candidate.model.Name, window))}
		}
	} else {
		secret := ""
		if candidate.keyID != "" {
			var err error
			secret, _, err = g.store.reveal(candidate.keyID)
			if err != nil {
				return nil, 0, "", window, err
			}
		}
		var err error
		endpoint, bearer, err = resolveEndpoint(candidate.provider, secret)
		if err != nil {
			return nil, 0, "", window, err
		}
	}
	if candidate.provider.CompletionTokens {
		request.MaxCompletionTokens, request.MaxTokens = request.MaxTokens, 0
	}
	if candidate.provider.ID == "cohere" {
		request.Tools = append([]wireTool(nil), request.Tools...)
		for i := range request.Tools {
			var schema any
			if json.Unmarshal(request.Tools[i].Function.Parameters, &schema) == nil {
				schema = stripSchemaKeywords(schema)
				if cleaned, err := json.Marshal(schema); err == nil {
					request.Tools[i].Function.Parameters = cleaned
				}
			}
		}
	}
	client := g.client
	if candidate.provider.Tier == protocol.TierLocal {
		client = g.local.client
	}
	attempt, status, body, err := g.postOpenAI(ctx, client, endpoint, bearer, candidate.provider, request)
	return attempt, status, body, window, err
}

func stripSchemaKeywords(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, child := range typed {
			if key != "additionalProperties" && key != "$schema" {
				cleaned[key] = stripSchemaKeywords(child)
			}
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(typed))
		for i := range typed {
			cleaned[i] = stripSchemaKeywords(typed[i])
		}
		return cleaned
	default:
		return value
	}
}

func resolveEndpoint(p *provider, secret string) (string, string, error) {
	switch p.Auth {
	case "none":
		return p.BaseURL, "", nil
	case "bearer":
		if secret == "" {
			return "", "", errors.New("provider key is empty")
		}
		return p.BaseURL, secret, nil
	case "cloudflare":
		account, token, ok := strings.Cut(secret, ":")
		if !ok || strings.TrimSpace(account) == "" || strings.TrimSpace(token) == "" {
			return "", "", errors.New(i18n.T("Cloudflare keys use account_id:token"))
		}
		return "https://api.cloudflare.com/client/v4/accounts/" + strings.TrimSpace(account) + "/ai/v1", strings.TrimSpace(token), nil
	default:
		return "", "", fmt.Errorf("unsupported provider auth %q", p.Auth)
	}
}

type failureKind int

const (
	failureFatal failureKind = iota
	failureAuth
	failureRate
	failureCredit
	failureModel
	failureContext
	failureProvider
)

type failureClass struct {
	kind     failureKind
	duration time.Duration
	reason   string
}

func classifyFailure(status int, body string, err error, now time.Time) failureClass {
	lower := strings.ToLower(body)
	if lower == "" && err != nil {
		lower = strings.ToLower(err.Error())
	}
	if invalidCredential(status, body) {
		return failureClass{kind: failureAuth, reason: "invalid key"}
	}
	if status == http.StatusTooManyRequests || dailyQuota(lower) || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") {
		duration := time.Minute
		var upstream *upstreamError
		if errors.As(err, &upstream) && upstream.retryAfter > 0 {
			duration = upstream.retryAfter
		}
		if dailyQuota(lower) {
			utcNow := now.UTC()
			next := utcNow.AddDate(0, 0, 1)
			next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, time.UTC)
			duration = next.Sub(utcNow)
		}
		return failureClass{kind: failureRate, duration: duration, reason: "rate limit"}
	}
	if status == http.StatusPaymentRequired || strings.Contains(lower, "credit") && (strings.Contains(lower, "exhaust") || strings.Contains(lower, "insufficient") || strings.Contains(lower, "balance")) {
		return failureClass{kind: failureCredit, duration: 24 * time.Hour, reason: "credit exhausted"}
	}
	if status == http.StatusNotFound || status == http.StatusGone || strings.Contains(lower, "model not found") || strings.Contains(lower, "model_not_found") {
		return failureClass{kind: failureModel, duration: 24 * time.Hour, reason: "model unavailable"}
	}
	if status == http.StatusRequestEntityTooLarge || strings.Contains(lower, "context length") || strings.Contains(lower, "context window") || strings.Contains(lower, "too many tokens") || strings.Contains(lower, "exceeds the available context size") {
		return failureClass{kind: failureContext, reason: "context too large"}
	}
	if status >= 500 || transportFailure(err) || strings.Contains(lower, "empty completion") {
		return failureClass{kind: failureProvider, duration: 2 * time.Minute, reason: "provider unavailable"}
	}
	return failureClass{kind: failureFatal, reason: "request rejected"}
}

func invalidCredential(status int, body string) bool {
	if status == http.StatusUnauthorized {
		return true
	}
	if status != http.StatusForbidden {
		return false
	}
	lower := strings.ToLower(body)
	for _, marker := range []string{"invalid key", "invalid api key", "invalid_api_key", "invalid-api-key", "api key is invalid", "api_key_invalid", "invalid token", "authentication failed", "unauthorized key"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func dailyQuota(body string) bool {
	daily := strings.Contains(body, "daily") || strings.Contains(body, "per day") || strings.Contains(body, "per-day") || strings.Contains(body, "today")
	quota := strings.Contains(body, "quota") || strings.Contains(body, "limit") || strings.Contains(body, "allocation") || strings.Contains(body, "exhaust") || strings.Contains(body, "used up")
	return daily && quota
}

func transportFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	lower := strings.ToLower(err.Error())
	for _, marker := range []string{"timeout", "connection reset", "connection refused", "broken pipe", "unexpected eof", "no such host", "network is unreachable", "tls:", "server sent goaway"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

func safeProviderError(status int, body string, err error) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal([]byte(body), &envelope) == nil {
		for _, value := range []string{envelope.Error.Message, envelope.Message, envelope.Detail} {
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	if strings.TrimSpace(body) != "" {
		body = strings.Join(strings.Fields(body), " ")
		if len(body) > 300 {
			body = body[:300]
		}
		return body
	}
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("provider returned HTTP %d", status)
}
