package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func fastDecisionRequest(route string) protocol.FastDecisionParams {
	return protocol.FastDecisionParams{Route: route, AccountID: "ra_fixture", DecisionRequest: protocol.DecisionRequest{
		State:     json.RawMessage(`"Read the selected fixture"`),
		Questions: map[string]protocol.DecisionQuestion{"intent": {Type: "choice", Criteria: json.RawMessage(`{"read":"Read it","stop":"Do not proceed"}`)}},
	}}
}

func fastDecisionAnswer(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"model":"fixture:resolved","answers":{"intent":{"type":"choice","choice":"read","confidence":0.8,"probabilities":{"read":0.9,"stop":0.1}}},"usage":{"input_tokens":43,"output_tokens":0}}`))
}

// decisionListing answers the local decision server's model list, as a
// server the user installed does, and reports whether it did
func decisionListing(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"fixture-local","object":"model"}]}`))
	return true
}

type decisionTransport func(*http.Request) (*http.Response, error)

func (f decisionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestLocalDecisionUsesItsConfiguredModelWithoutAccountOrRemoteTransport(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if decisionListing(w, r) {
			return
		}
		hits.Add(1)
		if r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer local" {
			t.Errorf("unexpected local request: %s", r.URL.Path)
		}
		var request protocol.DecisionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Model != "fixture-local" {
			t.Errorf("local model was replaced: %#v, %v", request, err)
		}
		fastDecisionAnswer(w)
	}))
	defer server.Close()
	g := newTestGateway(t)
	g.settings = func() protocol.Settings {
		return protocol.Settings{Providers: protocol.ProvidersSettings{Decisions: protocol.DecisionRoutingSettings{Route: protocol.DecisionLocal, LocalAddress: server.URL + "/v1", LocalModel: "fixture-local"}}}
	}
	g.client.Transport = decisionTransport(func(*http.Request) (*http.Response, error) {
		t.Error("local data reached the remote/proxied transport")
		return nil, errDecisionRoute
	})
	result, err := g.Decide(context.Background(), fastDecisionRequest(""))
	if err != nil || !result.Resolved || result.Route != protocol.DecisionLocal || result.Decision == nil || result.Decision.Answers["intent"].Choice != "read" || result.EmberUnits != 0 || hits.Load() != 1 {
		t.Fatalf("local decision: %#v, %v, hits=%d", result, err, hits.Load())
	}
}

func TestAutomaticDecisionFailureCannotSpendWithoutExplicitHostedOptIn(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer local.Close()
	var hostedHits atomic.Int32
	hosted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hostedHits.Add(1); fastDecisionAnswer(w) }))
	defer hosted.Close()
	h := newFakeHosted(hosted.URL+"/v1", true)
	g := hostedGateway(t, h)
	g.settings = func() protocol.Settings {
		return protocol.Settings{Providers: protocol.ProvidersSettings{Decisions: protocol.DecisionRoutingSettings{Route: protocol.DecisionAuto, LocalAddress: local.URL + "/v1", LocalModel: "fixture-local"}}}
	}
	result, err := g.Decide(context.Background(), fastDecisionRequest(""))
	if err != nil || result.Resolved || result.Decision != nil || result.Route != protocol.DecisionRules || h.endpoints.Load() != 0 || hostedHits.Load() != 0 {
		t.Fatalf("automatic failure spent hosted authority: %#v, %v", result, err)
	}
}

func TestDecisionInsufficientCreditsFallsBackLocallyWithoutHidingTheRoute(t *testing.T) {
	var localHits, hostedHits atomic.Int32
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if decisionListing(w, r) {
			return
		}
		localHits.Add(1)
		fastDecisionAnswer(w)
	}))
	defer local.Close()
	hosted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hostedHits.Add(1)
		w.WriteHeader(http.StatusPaymentRequired)
	}))
	defer hosted.Close()
	h := newFakeHosted(hosted.URL+"/v1", true)
	g := hostedGateway(t, h)
	g.settings = func() protocol.Settings {
		return protocol.Settings{Providers: protocol.ProvidersSettings{Decisions: protocol.DecisionRoutingSettings{LocalAddress: local.URL + "/v1", LocalModel: "fixture-local"}}}
	}
	result, err := g.Decide(context.Background(), fastDecisionRequest(protocol.DecisionEmber))
	if err != nil || !result.Resolved || result.Route != protocol.DecisionLocal || result.EmberUnits != 0 || !result.CreditsExhausted || localHits.Load() != 1 || hostedHits.Load() != 1 || h.used.Load() != 1 {
		t.Fatalf("credit fallback: %#v, %v", result, err)
	}
}

// Running out of credits is a typed field on the result, set only by the
// credit route's own refusal: whatever the notice says, another failure
// never sets it, and a refusal sets it whether or not a local answer
// followed.
func TestDecisionCreditExhaustionIsATypedSignal(t *testing.T) {
	refusing := func(status int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
	}
	for _, c := range []struct {
		name      string
		status    int
		account   string
		exhausted bool
	}{
		{"out of credits", http.StatusPaymentRequired, "ra_fixture", true},
		{"server error", http.StatusInternalServerError, "ra_fixture", false},
		{"signed out", http.StatusUnauthorized, "ra_fixture", false},
		{"unreviewed account", http.StatusPaymentRequired, "", false},
	} {
		hosted := refusing(c.status)
		g := hostedGateway(t, newFakeHosted(hosted.URL+"/v1", true))
		request := fastDecisionRequest(protocol.DecisionEmber)
		request.AccountID = c.account
		result, err := g.Decide(context.Background(), request)
		hosted.Close()
		if err != nil || result.Resolved || result.Decision != nil || result.EmberUnits != 0 || result.CreditsExhausted != c.exhausted {
			t.Errorf("%s: %#v, %v", c.name, result, err)
		}
	}
	// the local route failing is no credit signal either
	local := refusing(http.StatusServiceUnavailable)
	defer local.Close()
	g := newTestGateway(t)
	g.settings = func() protocol.Settings {
		return protocol.Settings{Providers: protocol.ProvidersSettings{Decisions: protocol.DecisionRoutingSettings{Route: protocol.DecisionLocal, LocalAddress: local.URL + "/v1", LocalModel: "fixture-local"}}}
	}
	if result, err := g.Decide(context.Background(), fastDecisionRequest("")); err != nil || result.Resolved || result.CreditsExhausted {
		t.Fatalf("a local failure: %#v, %v", result, err)
	}
}

func TestHostedDecisionCannotFollowRedirectWithAccountBearer(t *testing.T) {
	var leaked atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { leaked.Add(1); fastDecisionAnswer(w) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	g := hostedGateway(t, newFakeHosted(source.URL+"/v1", true))
	result, err := g.Decide(context.Background(), fastDecisionRequest(protocol.DecisionEmber))
	if err != nil || result.Resolved || leaked.Load() != 0 {
		t.Fatalf("redirect exposed a decision bearer: %#v, %v, hits=%d", result, err, leaked.Load())
	}
}

func TestAccountChangeDropsInFlightDecisionInsteadOfPublishingToNextIdentity(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	h := newFakeHosted(server.URL+"/v1", true)
	g := hostedGateway(t, h)
	finished := make(chan protocol.FastDecisionResult, 1)
	go func() {
		result, _ := g.Decide(context.Background(), fastDecisionRequest(protocol.DecisionEmber))
		finished <- result
	}()
	<-started
	h.signOut()
	result := <-finished
	if result.Resolved || result.Decision != nil || h.used.Load() != 1 {
		t.Fatalf("late identity-bound decision was published: %#v", result)
	}
}

func TestHostedDecisionCannotSpendAnUnreviewedOrDifferentAccount(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); fastDecisionAnswer(w) }))
	defer server.Close()
	h := newFakeHosted(server.URL+"/v1", true)
	g := hostedGateway(t, h)
	for _, account := range []string{"", "ra_someone_else"} {
		request := fastDecisionRequest(protocol.DecisionEmber)
		request.AccountID = account
		result, err := g.Decide(context.Background(), request)
		if err != nil || result.Resolved || result.Decision != nil {
			t.Fatalf("an unreviewed Account produced a paid decision: %#v, %v", result, err)
		}
	}
	if hits.Load() != 0 || h.used.Load() != 0 {
		t.Fatalf("wrong-account selection reached paid inference: hits=%d, refreshes=%d", hits.Load(), h.used.Load())
	}
}
