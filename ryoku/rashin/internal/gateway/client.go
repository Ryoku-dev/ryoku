package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

type wireMessage struct {
	Role       string         `json:"role"`
	Content    any            `json:"content,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type wireToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Arguments   string          `json:"arguments,omitempty"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatRequest struct {
	Model         string            `json:"model"`
	Messages      []wireMessage     `json:"messages"`
	Tools         []wireTool        `json:"tools,omitempty"`
	ToolChoice    string            `json:"tool_choice,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Temperature   *float64          `json:"temperature,omitempty"`
	Stream        bool              `json:"stream"`
	StreamOptions wireStreamOptions `json:"stream_options"`
	// Extra are a caller's own parameters, sent as they came.
	Extra map[string]json.RawMessage `json:"-"`
}

// MarshalJSON adds Extra beside the fields the gateway sets itself, which win.
func (r chatRequest) MarshalJSON() ([]byte, error) {
	type plain chatRequest
	body, err := json.Marshal(plain(r))
	if err != nil || len(r.Extra) == 0 {
		return body, err
	}
	fields := make(map[string]json.RawMessage, len(r.Extra)+8)
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	for name, value := range r.Extra {
		if _, set := fields[name]; !set {
			fields[name] = value
		}
	}
	return json.Marshal(fields)
}

type routeCandidate struct {
	provider *provider
	model    *model
	keyID    string
	stat     successStat
	hasStat  bool
}

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

func (e *upstreamError) Unwrap() error { return e.cause }

// Stream starts a routed OpenAI-compatible chat completion.
func (g *Gateway) Stream(ctx context.Context, request llm.Request) (llm.Stream, error) {
	wire, err := renderRequest(request)
	if err != nil {
		return nil, err
	}
	candidates, err := g.candidates(request)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 && hasImages(request) {
		return nil, &llm.NoRoute{Reason: g.noPictureModel()}
	}
	if len(candidates) == 0 {
		return nil, &llm.NoRoute{Reason: i18n.T("No free model is available. Add a free key in Rashin > Settings > Providers.")}
	}
	return &routedStream{gateway: g, ctx: ctx, request: wire, candidates: candidates, started: g.now()}, nil
}

func renderRequest(request llm.Request) (chatRequest, error) {
	wire := chatRequest{
		Model: request.Model, MaxTokens: request.MaxTokens, Temperature: request.Temperature,
		Stream: true, StreamOptions: wireStreamOptions{IncludeUsage: true}, Extra: request.Extra,
	}
	for _, message := range request.Messages {
		m := wireMessage{Role: string(message.Role), ToolCallID: message.ToolCallID, Name: message.Name}
		switch message.Role {
		case llm.RoleUser:
			if len(message.Images) == 0 {
				m.Content = message.Content
			} else {
				parts := []map[string]any{{"type": "text", "text": message.Content}}
				for _, image := range message.Images {
					mime := strings.TrimSpace(image.Mime)
					if mime == "" {
						mime = "application/octet-stream"
					}
					parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(image.Data)}})
				}
				m.Content = parts
			}
		case llm.RoleAssistant:
			m.Content = message.Content
			for _, call := range message.ToolCalls {
				m.ToolCalls = append(m.ToolCalls, wireToolCall{ID: call.ID, Type: "function", Function: wireFunction{Name: call.Name, Arguments: call.Arguments}})
			}
		case llm.RoleSystem, llm.RoleTool:
			m.Content = message.Content
		default:
			return chatRequest{}, fmt.Errorf(i18n.T("Unsupported chat role: %s"), message.Role)
		}
		wire.Messages = append(wire.Messages, m)
	}
	for _, tool := range request.Tools {
		parameters := tool.Parameters
		if len(parameters) == 0 {
			parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		if !json.Valid(parameters) {
			return chatRequest{}, fmt.Errorf(i18n.T("Tool schema for %s is not valid JSON"), tool.Name)
		}
		wire.Tools = append(wire.Tools, wireTool{Type: "function", Function: wireFunction{Name: tool.Name, Description: tool.Description, Parameters: parameters}})
	}
	// a caller that says how tools are chosen is passed on as is
	if _, chosen := request.Extra["tool_choice"]; len(wire.Tools) > 0 && !chosen {
		wire.ToolChoice = "auto"
	}
	return wire, nil
}

// hasImages reports a request that only a model with vision can answer.
func hasImages(request llm.Request) bool {
	for _, message := range request.Messages {
		if len(message.Images) > 0 {
			return true
		}
	}
	return false
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

func (g *Gateway) candidates(request llm.Request) ([]routeCandidate, error) {
	modelRoute := strings.TrimSpace(request.Model)
	pinned := modelRoute != "" && modelRoute != protocol.RouteAuto && modelRoute != protocol.RouteFast && modelRoute != protocol.RouteSmart
	strictPinned := pinned && request.Strict
	// A model without vision answers a picture it never saw, or refuses the
	// whole request, so one is only ever sent to a model that can read it.
	pictures := hasImages(request)
	var candidates []routeCandidate
	for i := range g.catalog.Providers {
		p := &g.catalog.Providers[i]
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
		for j := range p.Models {
			m := &p.Models[j]
			if len(request.Tools) > 0 && !m.Tools {
				continue
			}
			if strictPinned && modelRoute != p.ID+"/"+m.ID {
				continue
			}
			if pictures && !m.Vision {
				if strictPinned {
					return nil, &llm.NoRoute{Reason: i18n.Tf("%s cannot read pictures. Choose Auto, or a model that can.", m.Name)}
				}
				continue
			}
			if _, cooling, err := g.store.cooldown(modelScope(p.ID, m.ID), g.now()); err != nil {
				return nil, err
			} else if cooling {
				continue
			}
			stat, hasStat, err := g.store.stat(p.ID, m.ID)
			if err != nil {
				return nil, err
			}
			for _, keyID := range keys {
				candidates = append(candidates, routeCandidate{provider: p, model: m, keyID: keyID, stat: stat, hasStat: hasStat})
			}
		}
	}
	if strictPinned && len(candidates) == 0 {
		return nil, &llm.NoRoute{Reason: i18n.Tf("The model you chose is not available now: %s. Choose Auto, or wait for it.", modelRoute)}
	}
	sortCandidates(candidates, modelRoute)
	if pinned && !request.Strict {
		preferPinnedCandidates(candidates, modelRoute)
	}
	return candidates, nil
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

func sortCandidates(candidates []routeCandidate, route string) {
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
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
	})
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
	request    chatRequest
	candidates []routeCandidate
	started    time.Time

	attempts       int
	next           int
	current        *sseAttempt
	candidate      routeCandidate
	attemptStarted time.Time
	committed      bool
	finished       bool
	closed         bool
	firstAt        time.Time
	output         int
	final          llm.Final
	mu             sync.Mutex
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
				return llm.Delta{}, errors.New(i18n.T("Every available free model failed. Check Providers for cooldowns and key errors"))
			}
			if !s.committed && s.gateway.now().Sub(s.started) >= firstByteBudget {
				s.finished = true
				return llm.Delta{}, errors.New(i18n.T("No provider produced a response within 45 seconds"))
			}
			s.candidate = s.candidates[s.next]
			s.next++
			s.attempts++
			s.attemptStarted = s.gateway.now()
			req := s.request
			req.Model = s.candidate.model.ID
			attempt, status, body, err := s.startCandidate(req)
			if err != nil {
				if !s.handleFailure(status, body, err) {
					s.finished = true
					return llm.Delta{}, err
				}
				continue
			}
			s.current = attempt
		}

		delta, err := s.nextAttemptDelta()
		if err == nil {
			if delta.Text == "" && delta.Reasoning == "" && len(delta.ToolCalls) == 0 {
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
			if !s.committed && s.current.emitted == 0 {
				_ = s.current.Close()
				s.current = nil
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
		if !s.handleFailure(0, "", err) {
			s.finished = true
			return llm.Delta{}, err
		}
	}
}

func (s *routedStream) startCandidate(request chatRequest) (*sseAttempt, int, string, error) {
	remaining := firstByteBudget - s.gateway.now().Sub(s.started)
	if remaining <= 0 {
		return nil, 0, "", context.DeadlineExceeded
	}
	attemptCtx, cancel := context.WithCancel(s.ctx)
	type result struct {
		attempt *sseAttempt
		status  int
		body    string
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		attempt, status, body, err := s.gateway.startAttempt(attemptCtx, s.candidate, request)
		ch <- result{attempt: attempt, status: status, body: body, err: err}
	}()
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case result := <-ch:
		if result.attempt != nil {
			result.attempt.cancel = cancel
		} else {
			cancel()
		}
		return result.attempt, result.status, result.body, result.err
	case <-s.ctx.Done():
		cancel()
		return nil, 0, "", s.ctx.Err()
	case <-timer.C:
		cancel()
		return nil, 0, "", context.DeadlineExceeded
	}
}

func (s *routedStream) nextAttemptDelta() (llm.Delta, error) {
	cur := s.current
	wait := streamIdleGap
	timeoutErr := error(errStreamIdle)
	if !s.committed {
		wait = firstByteBudget - s.gateway.now().Sub(s.started)
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

func (s *routedStream) finishSuccess() {
	usage := s.current.usage
	if !s.current.usageKnown {
		usage = llm.Usage{Input: estimateRequestTokens(s.request), Output: bytesToTokens(s.output), Estimated: true}
	}
	finish := s.current.finish
	if finish == "" {
		finish = llm.FinishStop
	}
	s.final = llm.Final{Model: s.candidate.provider.ID + "/" + s.candidate.model.ID, Provider: s.candidate.provider.ID, Finish: finish, Usage: usage, Attempts: s.attempts}
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
	usage := s.current.usage
	if !s.current.usageKnown {
		usage = llm.Usage{Input: estimateRequestTokens(s.request), Output: bytesToTokens(s.output), Estimated: true}
	}
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

func (g *Gateway) startAttempt(ctx context.Context, candidate routeCandidate, request chatRequest) (*sseAttempt, int, string, error) {
	secret := ""
	if candidate.keyID != "" {
		var err error
		secret, _, err = g.store.reveal(candidate.keyID)
		if err != nil {
			return nil, 0, "", err
		}
	}
	endpoint, bearer, err := resolveEndpoint(candidate.provider, secret)
	if err != nil {
		return nil, 0, "", err
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
	body, err := json.Marshal(request)
	if err != nil {
		return nil, 0, "", err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, 0, "", err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "text/event-stream")
	if bearer != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+bearer)
	}
	for key, value := range candidate.provider.ExtraHeaders {
		httpRequest.Header.Set(key, value)
	}
	response, err := g.client.Do(httpRequest)
	if err != nil {
		return nil, 0, "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if readErr != nil {
			return nil, response.StatusCode, "", readErr
		}
		bodyText := string(data)
		retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), g.now())
		return nil, response.StatusCode, bodyText, &upstreamError{status: response.StatusCode, body: bodyText, retryAfter: retryAfter}
	}
	return newSSEAttempt(response), 0, "", nil
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

type toolAccumulator struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

type sseAttempt struct {
	response   *http.Response
	cancel     context.CancelFunc
	scanner    *bufio.Scanner
	queue      []llm.Delta
	tools      map[int]*toolAccumulator
	done       bool
	emitted    int
	finish     string
	usage      llm.Usage
	usageKnown bool
	close      sync.Once
}

func newSSEAttempt(response *http.Response) *sseAttempt {
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	return &sseAttempt{response: response, scanner: scanner, tools: make(map[int]*toolAccumulator)}
}

func (s *sseAttempt) Next() (llm.Delta, error) {
	if len(s.queue) > 0 {
		delta := s.queue[0]
		s.queue = s.queue[1:]
		s.emitted++
		return delta, nil
	}
	if s.done {
		return llm.Delta{}, io.EOF
	}
	for {
		data, err := s.nextEvent()
		if err != nil {
			if errors.Is(err, io.EOF) {
				s.flushTools()
				s.done = true
				if len(s.queue) > 0 {
					return s.Next()
				}
			}
			return llm.Delta{}, err
		}
		if data == "[DONE]" {
			s.flushTools()
			s.done = true
			if len(s.queue) > 0 {
				return s.Next()
			}
			return llm.Delta{}, io.EOF
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return llm.Delta{}, fmt.Errorf("decode provider stream: %w", err)
		}
		if chunk.Error != nil && len(chunk.Choices) == 0 {
			return llm.Delta{}, errors.New(chunk.Error.message())
		}
		if chunk.Usage != nil {
			s.usage = llm.Usage{Input: firstNonZero(chunk.Usage.PromptTokens, chunk.Usage.InputTokens), Output: firstNonZero(chunk.Usage.CompletionTokens, chunk.Usage.OutputTokens)}
			s.usageKnown = true
		}
		var delta llm.Delta
		for _, choice := range chunk.Choices {
			delta.Text += textFromJSON(choice.Delta.Content)
			delta.Reasoning += firstText(choice.Delta.ReasoningContent, choice.Delta.Reasoning, choice.Delta.Thinking)
			for _, call := range choice.Delta.ToolCalls {
				acc := s.tools[call.Index]
				if acc == nil {
					acc = &toolAccumulator{}
					s.tools[call.Index] = acc
				}
				if call.ID != "" {
					acc.ID = call.ID
				}
				if call.Function.Name != "" {
					acc.Name += call.Function.Name
				}
				acc.Arguments.WriteString(call.Function.Arguments)
			}
			if choice.FinishReason != "" {
				s.finish = normalizeFinish(choice.FinishReason)
				s.flushTools()
			}
		}
		if delta.Text != "" || delta.Reasoning != "" {
			s.emitted++
			return delta, nil
		}
		if len(s.queue) > 0 {
			return s.Next()
		}
		return llm.Delta{}, nil
	}
}

func (s *sseAttempt) nextEvent() (string, error) {
	var lines []string
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" {
			if len(lines) > 0 {
				return strings.Join(lines, "\n"), nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			lines = append(lines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := s.scanner.Err(); err != nil {
		return "", err
	}
	if len(lines) > 0 {
		return strings.Join(lines, "\n"), nil
	}
	return "", io.EOF
}

func (s *sseAttempt) flushTools() {
	if len(s.tools) == 0 {
		return
	}
	indices := make([]int, 0, len(s.tools))
	for index := range s.tools {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	calls := make([]llm.ToolCall, 0, len(indices))
	for _, index := range indices {
		tool := s.tools[index]
		calls = append(calls, llm.ToolCall{ID: tool.ID, Name: tool.Name, Arguments: tool.Arguments.String()})
	}
	if s.finish == "" {
		s.finish = llm.FinishToolCalls
	}
	s.queue = append(s.queue, llm.Delta{ToolCalls: calls})
	s.tools = make(map[int]*toolAccumulator)
}

func (s *sseAttempt) Final() llm.Final { return llm.Final{Finish: s.finish, Usage: s.usage} }

func (s *sseAttempt) Close() error {
	var err error
	s.close.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		err = s.response.Body.Close()
	})
	return err
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          json.RawMessage `json:"content"`
			ReasoningContent string          `json:"reasoning_content"`
			Reasoning        string          `json:"reasoning"`
			Thinking         string          `json:"thinking"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		InputTokens      int `json:"input_tokens"`
		OutputTokens     int `json:"output_tokens"`
	} `json:"usage"`
	Error *streamError `json:"error"`
}

type streamError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

func (e *streamError) message() string {
	if e.Message != "" {
		return e.Message
	}
	return "provider stream returned an error"
}

func textFromJSON(raw json.RawMessage) string {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var parts []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		for _, part := range parts {
			text += part.Text
		}
	}
	return text
}

func firstText(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func normalizeFinish(reason string) string {
	switch reason {
	case "stop", "eos":
		return llm.FinishStop
	case "tool_calls", "function_call":
		return llm.FinishToolCalls
	case "length", "max_tokens":
		return llm.FinishLength
	case "content_filter":
		return llm.FinishFilter
	default:
		return reason
	}
}

func estimateRequestTokens(request chatRequest) int {
	data, _ := json.Marshal(request.Messages)
	tools, _ := json.Marshal(request.Tools)
	return bytesToTokens(len(data) + len(tools))
}

func bytesToTokens(n int) int {
	if n <= 0 {
		return 0
	}
	return (n + 3) / 4
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
	if status == http.StatusRequestEntityTooLarge || strings.Contains(lower, "context length") || strings.Contains(lower, "context window") || strings.Contains(lower, "too many tokens") {
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
