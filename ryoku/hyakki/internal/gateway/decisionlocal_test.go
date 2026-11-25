package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/localmodels"
	"ryoku-rashin/internal/protocol"
)

// localDecisionGateway is a test gateway whose local decision route is
// pinned to address with a model chosen
func localDecisionGateway(t *testing.T, address string) *Gateway {
	t.Helper()
	g := newTestGateway(t)
	g.settings = func() protocol.Settings {
		return protocol.Settings{Providers: protocol.ProvidersSettings{Decisions: protocol.DecisionRoutingSettings{Route: protocol.DecisionLocal, LocalAddress: address, LocalModel: "fixture-local"}}}
	}
	return g
}

// The server the user installed is detected by one GET of its model list,
// carrying no key, body or decision data, and with no second request.
func TestLocalDecisionServerIsDetectedByOneLookAtItsModelList(t *testing.T) {
	var asks atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks.Add(1)
		if r.Header.Get("Authorization") != "" || r.ContentLength > 0 {
			t.Errorf("the look carried a key or a body: %v %d", r.Header, r.ContentLength)
		}
		if !decisionListing(w, r) {
			t.Errorf("the look asked %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	g := localDecisionGateway(t, server.URL+"/v1")
	status := g.LocalDecisions(context.Background())
	if status.State != protocol.DecisionLocalDetected || status.Address != server.URL+"/v1" || status.Reason != "" || status.RAMMiB != 16<<10 || status.Cores != 8 || asks.Load() != 1 {
		t.Fatalf("detected server: %+v, asks=%d", status, asks.Load())
	}
}

// Nothing listening, or something that is not a decision server, is not
// found, and the reason names the address.
func TestLocalDecisionServerNotFound(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL + "/v1"
	closed.Close()
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model":"something else"}`))
	}))
	defer other.Close()
	missing := httptest.NewServer(http.NotFoundHandler())
	defer missing.Close()
	for _, c := range []struct {
		name, address, says string
	}{
		{"nothing listening", closedURL, "No local decision server answers at " + closedURL},
		{"no model list", missing.URL + "/v1", "but not as a decision server"},
		{"not a model list", other.URL + "/v1", "but not as a decision server"},
	} {
		status := localDecisionGateway(t, c.address).LocalDecisions(context.Background())
		if status.State != protocol.DecisionLocalAbsent || !strings.Contains(status.Reason, c.says) {
			t.Errorf("%s: %+v", c.name, status)
		}
	}
}

// An address off this computer is refused before anything is asked, by the
// look and by a decision alike.
func TestLocalDecisionLookRefusesAnAddressOffThisComputer(t *testing.T) {
	for _, address := range []string{"http://192.168.1.20:11435/v1", "http://example.com/v1", "http://[::ffff:10.0.0.1]:11435/v1", "http://user:pw@127.0.0.1:11435/v1", "unix:///run/decide.sock"} {
		g := localDecisionGateway(t, address)
		g.local.client.Transport = decisionTransport(func(r *http.Request) (*http.Response, error) {
			t.Errorf("%s: asked %s", address, r.URL)
			return nil, errDecisionRoute
		})
		status := g.LocalDecisions(context.Background())
		if status.State != protocol.DecisionLocalRefused || !strings.Contains(status.Reason, "not a loopback address") {
			t.Errorf("%s: %+v", address, status)
		}
		result, err := g.Decide(context.Background(), fastDecisionRequest(""))
		if err != nil || result.Resolved || result.Route != protocol.DecisionRules || !strings.Contains(result.Notice, "not a loopback address") {
			t.Errorf("%s: decision %#v, %v", address, result, err)
		}
	}
}

// A server that does not answer its model list in time is given up on
// within the look's budget, once, and is not found.
func TestLocalDecisionLookGivesUpOnASlowServer(t *testing.T) {
	var asks atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks.Add(1)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	g := localDecisionGateway(t, server.URL+"/v1")
	started := time.Now()
	status := g.LocalDecisions(context.Background())
	elapsed := time.Since(started)
	if status.State != protocol.DecisionLocalAbsent || !strings.Contains(status.Reason, "did not answer within 750 ms") || asks.Load() != 1 {
		t.Fatalf("slow server: %+v, asks=%d", status, asks.Load())
	}
	if elapsed < decisionProbeBudget || elapsed > decisionProbeBudget+time.Second {
		t.Fatalf("the look took %v, want about %v", elapsed, decisionProbeBudget)
	}
}

// The hardware gate reads the injected computer: one too small is refused
// with what it has and what it needs, before the server is asked, by the
// look and by a decision; one that fits goes on to the look.
func TestLocalDecisionHardwareGate(t *testing.T) {
	var looks, decisions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if decisionListing(w, r) {
			looks.Add(1)
			return
		}
		decisions.Add(1)
		fastDecisionAnswer(w)
	}))
	defer server.Close()
	g := localDecisionGateway(t, server.URL+"/v1")
	g.decisionHardware = func() localmodels.Hardware { return localmodels.Hardware{RAMMiB: 4 << 10, Cores: 2} }
	status := g.LocalDecisions(context.Background())
	if status.State != protocol.DecisionLocalSmall || !strings.Contains(status.Reason, "too small") || status.RAMMiB != 4<<10 || status.Cores != 2 {
		t.Fatalf("small computer: %+v", status)
	}
	result, err := g.Decide(context.Background(), fastDecisionRequest(""))
	if err != nil || result.Resolved || result.Route != protocol.DecisionRules || !strings.Contains(result.Notice, "too small for a local decision model") || !strings.Contains(result.Notice, "No decision data was sent") {
		t.Fatalf("small computer decided: %#v, %v", result, err)
	}
	if looks.Load() != 0 || decisions.Load() != 0 {
		t.Fatalf("a computer under the gate asked the server: looks=%d decisions=%d", looks.Load(), decisions.Load())
	}

	g.decisionHardware = func() localmodels.Hardware {
		return localmodels.Hardware{RAMMiB: localmodels.DecisionMinRAMMiB, Cores: localmodels.DecisionMinCores}
	}
	if status := g.LocalDecisions(context.Background()); status.State != protocol.DecisionLocalDetected {
		t.Fatalf("a computer at the gate: %+v", status)
	}
	result, err = g.Decide(context.Background(), fastDecisionRequest(""))
	if err != nil || !result.Resolved || result.Route != protocol.DecisionLocal || looks.Load() != 2 || decisions.Load() != 1 {
		t.Fatalf("a computer at the gate decided: %#v, %v, looks=%d decisions=%d", result, err, looks.Load(), decisions.Load())
	}
}

// A decision on the local route with no server there is refused with the
// plain reason, and the decision itself is never posted.
func TestLocalDecisionRefusedWhenNoServerIsFound(t *testing.T) {
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	g := localDecisionGateway(t, server.URL+"/v1")
	result, err := g.Decide(context.Background(), fastDecisionRequest(""))
	if err != nil || result.Resolved || result.Decision != nil || !strings.Contains(result.Notice, "but not as a decision server") || posts.Load() != 0 {
		t.Fatalf("absent server decided: %#v, %v, posts=%d", result, err, posts.Load())
	}
}
