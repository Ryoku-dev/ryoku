package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const (
	hostedToken = "acct-token-1"
	hostedModel = protocol.ProviderEmber + "/rashin-fast"
)

// fakeHosted is a Ryoku Account for the router: its server, its token, and a
// bound context the test ends to sign it out.
type fakeHosted struct {
	mu       sync.Mutex
	models   []protocol.AccountModel
	base     string
	token    string
	identity string
	cancel   context.CancelFunc
	onChange func()

	endpoints atomic.Int32
	used      atomic.Int32
	rejected  atomic.Int32
}

func newFakeHosted(base string, signedIn bool) *fakeHosted {
	h := &fakeHosted{base: base, token: hostedToken, identity: "ra_fixture"}
	if signedIn {
		h.models = []protocol.AccountModel{{ID: "rashin-fast", Name: "Qwen3.8 Flash", Logo: "qwen"}}
	}
	return h
}

func (h *fakeHosted) Models() []protocol.AccountModel {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]protocol.AccountModel(nil), h.models...)
}

func (h *fakeHosted) Endpoint(ctx context.Context, expectedAccount string) (string, string, context.Context, error) {
	h.endpoints.Add(1)
	bound, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	if expectedAccount != "" && expectedAccount != h.identity {
		h.mu.Unlock()
		cancel()
		return "", "", nil, errors.New("Account changed")
	}
	h.cancel = cancel
	h.mu.Unlock()
	return h.base, h.token, bound, nil
}

// signOut ends the context the last request was bound to.
func (h *fakeHosted) signOut() {
	h.mu.Lock()
	cancel := h.cancel
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (h *fakeHosted) Rejected()          { h.rejected.Add(1) }
func (h *fakeHosted) Used()              { h.used.Add(1) }
func (h *fakeHosted) OnChange(fn func()) { h.mu.Lock(); h.onChange = fn; h.mu.Unlock() }

// hostedGateway has no catalogue provider, so a hosted model is the only one
// that could answer.
func hostedGateway(t *testing.T, h Hosted) *Gateway {
	t.Helper()
	gateway := newTestGateway(t)
	setTestProviders(gateway)
	gateway.SetHosted(h)
	return gateway
}

func hostedRequest(route string) llm.Request {
	return llm.Request{Model: route, Strict: pinnedRoute(route), Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}}
}

func asNoRoute(err error) bool {
	var refused *llm.NoRoute
	return errors.As(err, &refused)
}

func TestHostedSignedOutHasNoProviderAndNoRoute(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sse(w, `[DONE]`)
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, false)
	gateway := hostedGateway(t, hosted)

	providers, err := gateway.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range providers {
		if p.ID == protocol.ProviderEmber {
			t.Fatalf("signed-out account lists provider %#v", p)
		}
	}
	models, err := gateway.Models(context.Background(), protocol.ModelsParams{})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models.Models {
		if m.Provider == protocol.ProviderEmber {
			t.Fatalf("signed-out account lists model %#v", m)
		}
	}
	_, err = gateway.Stream(context.Background(), hostedRequest(hostedModel))
	if !asNoRoute(err) {
		t.Fatalf("signed-out request = %v, want NoRoute", err)
	}
	if hits.Load() != 0 || hosted.endpoints.Load() != 0 {
		t.Fatalf("signed-out request reached the server: %d hits, %d endpoints", hits.Load(), hosted.endpoints.Load())
	}

	// no account at all behaves the same
	gateway.SetHosted(nil)
	if _, err := gateway.Stream(context.Background(), hostedRequest(hostedModel)); !asNoRoute(err) {
		t.Fatalf("no-account request = %v, want NoRoute", err)
	}
}

func TestHostedModelIsPaidAndNeverRoutedAutomatically(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"hosted"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway := hostedGateway(t, hosted)

	models, err := gateway.Models(context.Background(), protocol.ModelsParams{})
	if err != nil {
		t.Fatal(err)
	}
	var found *protocol.ModelInfo
	for i := range models.Models {
		if models.Models[i].ID == hostedModel {
			found = &models.Models[i]
		}
	}
	if found == nil || found.Tier != protocol.TierPaid || found.Provider != protocol.ProviderEmber || !found.Available {
		t.Fatalf("hosted model = %#v", found)
	}
	if found.Name != "Qwen3.8 Flash" || found.Logo != "qwen" {
		t.Fatalf("hosted model does not carry the server's name and mark: %#v", found)
	}
	providers, err := gateway.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	listed := false
	for _, p := range providers {
		if p.ID == protocol.ProviderEmber {
			listed = true
			if p.Tier != protocol.TierPaid || p.Models != 1 || p.Free != 0 {
				t.Fatalf("provider info = %#v", p)
			}
		}
	}
	if !listed {
		t.Fatal("signed-in account has no provider")
	}

	// the only model there is, and still no route may choose it
	for _, route := range []string{protocol.RouteAuto, protocol.RouteFast, protocol.RouteSmart} {
		stream, err := gateway.Stream(context.Background(), hostedRequest(route))
		if err == nil {
			_ = stream.Close()
			t.Fatalf("route %s reached a model with only the hosted one in use", route)
		}
		if !asNoRoute(err) {
			t.Fatalf("route %s = %v, want NoRoute", route, err)
		}
	}
	if hits.Load() != 0 || hosted.endpoints.Load() != 0 {
		t.Fatalf("a route reached the hosted server: %d hits, %d endpoints", hits.Load(), hosted.endpoints.Load())
	}

	// with a free provider in use, the routes answer from it
	var freeHits atomic.Int32
	free := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		freeHits.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"free"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer free.Close()
	setTestProviders(gateway, testProvider("free", free.URL, 10, true))
	for _, route := range []string{protocol.RouteAuto, protocol.RouteFast, protocol.RouteSmart} {
		stream, err := gateway.Stream(context.Background(), hostedRequest(route))
		if err != nil {
			t.Fatalf("route %s: %v", route, err)
		}
		deltas, err := collect(t, stream)
		if err != nil || len(deltas) == 0 || deltas[0].Text != "free" {
			t.Fatalf("route %s answered %#v, %v", route, deltas, err)
		}
		if got := stream.Final().Provider; got != "free" {
			t.Fatalf("route %s answered from %q", route, got)
		}
	}
	if hits.Load() != 0 || hosted.endpoints.Load() != 0 {
		t.Fatalf("a route reached the hosted server: %d hits, %d endpoints", hits.Load(), hosted.endpoints.Load())
	}
}

func TestHostedStrictRequestStreamsWithTokenAndFreshKeys(t *testing.T) {
	var mu sync.Mutex
	var auths, keys, paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		sse(w,
			`{"choices":[{"delta":{"content":"hello "}}]}`,
			`{"choices":[{"delta":{"content":"from Ryoku"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":3}}`,
			`[DONE]`)
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway := hostedGateway(t, hosted)

	for i := 1; i <= 2; i++ {
		stream, err := gateway.Stream(context.Background(), hostedRequest(hostedModel))
		if err != nil {
			t.Fatal(err)
		}
		deltas, err := collect(t, stream)
		if err != nil {
			t.Fatal(err)
		}
		var text strings.Builder
		for _, d := range deltas {
			text.WriteString(d.Text)
		}
		if text.String() != "hello from Ryoku" {
			t.Fatalf("text = %q", text.String())
		}
		final := stream.Final()
		if final.Provider != protocol.ProviderEmber || final.Model != hostedModel {
			t.Fatalf("final = %#v", final)
		}
		if got := hosted.used.Load(); got != int32(i) {
			t.Fatalf("Used called %d times after %d successes", got, i)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(keys) != 2 {
		t.Fatalf("server saw %d requests", len(keys))
	}
	for i := range keys {
		if auths[i] != "Bearer "+hostedToken {
			t.Fatalf("Authorization = %q", auths[i])
		}
		if !strings.HasPrefix(keys[i], "rashin-") || len(keys[i]) <= len("rashin-") {
			t.Fatalf("Idempotency-Key = %q", keys[i])
		}
		if paths[i] != "/chat/completions" {
			t.Fatalf("path = %q", paths[i])
		}
	}
	if keys[0] == keys[1] {
		t.Fatalf("two requests shared the key %q", keys[0])
	}
	if hosted.rejected.Load() != 0 {
		t.Fatalf("Rejected called %d times on success", hosted.rejected.Load())
	}
}

// A 402 is told as the credits it lacks, with the code the app answers with
// Add Ember credits; it is never sent again by itself.
func TestHostedInsufficientCreditsIsToldAndNeverCoolsDown(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"type":"insufficient_credits","message":"Not enough Ember credits for this request.","available":0,"needed":1200}}`))
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway := hostedGateway(t, hosted)

	for i := 1; i <= 2; i++ {
		stream, err := gateway.Stream(context.Background(), hostedRequest(hostedModel))
		if err != nil {
			t.Fatal(err)
		}
		_, err = collect(t, stream)
		if refused := protocol.AsError(err); err == nil || refused.Code != protocol.CodeEmberCredits || err.Error() != "Not enough Ember credits for this request." {
			t.Fatalf("attempt %d error = %v (%+v), want the credits code and plain words", i, err, refused)
		}
		// the engine reads the status to decide on sending the same request again
		var status interface{ StatusCode() int }
		if !errors.As(err, &status) || status.StatusCode() != http.StatusPaymentRequired {
			t.Fatalf("attempt %d: the refusal lost its 402", i)
		}
		if got := hits.Load(); got != int32(i) {
			t.Fatalf("attempt %d: server saw %d requests, want %d (a cooldown kept the request away)", i, got, i)
		}
		if got := hosted.used.Load(); got != int32(i) {
			t.Fatalf("attempt %d: Used called %d times", i, got)
		}
	}
	if hosted.rejected.Load() != 0 {
		t.Fatalf("Rejected called %d times for a 402", hosted.rejected.Load())
	}
	assertNoHostedCooldown(t, gateway)
}

// A 403 for terms not yet accepted is told plainly with the code the app
// answers with the account page's terms; any other 403 keeps the server's
// words and no such code.
func TestHostedTermsRefusalIsToldPlainlyAndOtherRefusalsKeepTheirWords(t *testing.T) {
	const hold = "This account is on hold after a refund or a payment dispute. Contact Ryoku to have it restored."
	var reply atomic.Value
	reply.Store(`{"error":{"type":"forbidden","message":"Accept the current Ryoku Account terms and hosted-inference privacy disclosure before using Ryoku Ember.","reason":"account_consent_required"}}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(reply.Load().(string)))
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway := hostedGateway(t, hosted)

	stream, err := gateway.Stream(context.Background(), hostedRequest(hostedModel))
	if err != nil {
		t.Fatal(err)
	}
	_, err = collect(t, stream)
	if refused := protocol.AsError(err); err == nil || refused.Code != protocol.CodeAccountTerms || err.Error() != "Accept the Ryoku Account terms first." {
		t.Fatalf("terms refusal = %v (%+v), want the terms code and plain words", err, refused)
	}
	var status interface{ StatusCode() int }
	if !errors.As(err, &status) || status.StatusCode() != http.StatusForbidden {
		t.Fatal("the terms refusal lost its 403")
	}

	reply.Store(`{"error":{"type":"forbidden","message":"` + hold + `"}}`)
	stream, err = gateway.Stream(context.Background(), hostedRequest(hostedModel))
	if err != nil {
		t.Fatal(err)
	}
	_, err = collect(t, stream)
	if err == nil || !strings.Contains(err.Error(), hold) {
		t.Fatalf("hold refusal = %v, want the server's words", err)
	}
	if code := protocol.AsError(err).Code; code == protocol.CodeAccountTerms || code == protocol.CodeEmberCredits {
		t.Fatalf("a hold was told as %q", code)
	}
	if hosted.rejected.Load() != 0 {
		t.Fatalf("Rejected called %d times for a 403", hosted.rejected.Load())
	}
	assertNoHostedCooldown(t, gateway)
}

func TestHostedUnauthorizedRejectsTokenAndNeverCoolsDown(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_token","message":"Your sign-in has ended."}}`))
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway := hostedGateway(t, hosted)

	for i := 1; i <= 2; i++ {
		stream, err := gateway.Stream(context.Background(), hostedRequest(hostedModel))
		if err != nil {
			t.Fatal(err)
		}
		_, err = collect(t, stream)
		if err == nil {
			t.Fatalf("attempt %d: a 401 ended without an error", i)
		}
		if got := hits.Load(); got != int32(i) {
			t.Fatalf("attempt %d: server saw %d requests", i, got)
		}
		if got := hosted.rejected.Load(); got != int32(i) {
			t.Fatalf("attempt %d: Rejected called %d times", i, got)
		}
		if got := hosted.used.Load(); got != int32(i) {
			t.Fatalf("attempt %d: Used called %d times", i, got)
		}
	}
	assertNoHostedCooldown(t, gateway)
}

func assertNoHostedCooldown(t *testing.T, gateway *Gateway) {
	t.Helper()
	now := gateway.now()
	for _, scope := range []string{providerScope(protocol.ProviderEmber), modelScope(protocol.ProviderEmber, "rashin-fast")} {
		if _, cooling, err := gateway.store.cooldown(scope, now); err != nil || cooling {
			t.Fatalf("cooldown %s = %v, %v", scope, cooling, err)
		}
	}
	cooling, err := gateway.store.modelCooldowns(protocol.ProviderEmber, now)
	if err != nil || len(cooling) != 0 {
		t.Fatalf("model cooldowns = %v, %v", cooling, err)
	}
	models, err := gateway.Models(context.Background(), protocol.ModelsParams{})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models.Models {
		if m.ID == hostedModel && (!m.Available || m.CooldownUntil != nil) {
			t.Fatalf("hosted model shows as cooling down: %#v", m)
		}
	}
}

func TestHostedSignOutEndsAStreamPromptly(t *testing.T) {
	first := make(chan struct{})
	serverDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		sse(w, `{"choices":[{"delta":{"content":"one "}}]}`)
		close(first)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				sse(w, `{"choices":[{"delta":{"content":"more "}}]}`)
			}
		}
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway := hostedGateway(t, hosted)

	stream, err := gateway.Stream(context.Background(), hostedRequest(hostedModel))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	delta, err := stream.Next()
	if err != nil || delta.Text != "one " {
		t.Fatalf("first delta = %#v, %v", delta, err)
	}
	select {
	case <-first:
	case <-time.After(2 * time.Second):
		t.Fatal("server never started streaming")
	}

	signedOut := time.Now()
	hosted.signOut()
	ended := make(chan error, 1)
	go func() {
		for {
			if _, err := stream.Next(); err != nil {
				ended <- err
				return
			}
		}
	}()
	select {
	case err := <-ended:
		if errors.Is(err, io.EOF) {
			t.Fatalf("signing out ended the stream as a clean finish")
		}
		if took := time.Since(signedOut); took > 2*time.Second {
			t.Fatalf("stream took %s to end after sign-out", took)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream kept running after sign-out")
	}
	select {
	case <-serverDone:
	case <-time.After(2 * time.Second):
		t.Fatal("the server's request context never ended")
	}
}
