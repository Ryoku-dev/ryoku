package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// listingServer serves a listing on /models and a chat on /chat/completions
// that answers with the model it was asked for.
type listingServer struct {
	*httptest.Server
	listed atomic.Int32
	fail   atomic.Bool
	body   string
}

func newListingServer(t *testing.T, body string) *listingServer {
	t.Helper()
	server := &listingServer{body: body}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			server.listed.Add(1)
			if server.fail.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(server.body))
			return
		}
		var request struct{ Model string }
		_ = json.NewDecoder(r.Body).Decode(&request)
		sse(w, `{"choices":[{"delta":{"content":"`+request.Model+`"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	t.Cleanup(server.Close)
	return server
}

// awaitListing waits for the read the gateway started in the background.
func awaitListing(t *testing.T, gateway *Gateway, id string) {
	t.Helper()
	gateway.listings.mu.Lock()
	reading := gateway.listings.entries[id].reading
	gateway.listings.mu.Unlock()
	if reading != nil {
		select {
		case <-reading:
		case <-time.After(5 * time.Second):
			t.Fatal("the listing was never read")
		}
	}
}

// forgetListings undoes markListingsRead for one provider.
func forgetListings(gateway *Gateway, id string) {
	gateway.listings.mu.Lock()
	delete(gateway.listings.entries, id)
	gateway.listings.mu.Unlock()
}

// A listing is read in the background, trusted for listingTTL, read again
// after; one that failed waits listingRetry; and what was read survives a
// restart.
func TestListingIsCachedForItsTTL(t *testing.T) {
	server := newListingServer(t, `{"data":[{"id":"extra","supported_parameters":["tools"]}]}`)
	dir := t.TempDir()
	open := func() *Gateway {
		gateway, err := Open(dir, func() protocol.Settings { return protocol.Settings{} })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = gateway.Close() })
		return gateway
	}
	gateway := open()
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	gateway.now = func() time.Time { return now }
	setTestProviders(gateway, testProvider("listed", server.URL, 50, true))
	forgetListings(gateway, "listed")

	providers, err := gateway.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if providers[len(providers)-1].Models != 1 {
		t.Fatalf("the first answer waited on the listing: %#v", providers[len(providers)-1])
	}
	awaitListing(t, gateway, "listed")
	providers, _ = gateway.Providers(context.Background())
	listed := providers[len(providers)-1]
	if listed.Models != 2 || listed.Free != 2 || listed.Listed == nil || !listed.Listed.Equal(now) {
		t.Fatalf("after the read: %#v", listed)
	}
	now = now.Add(listingTTL - time.Minute)
	_, _ = gateway.Providers(context.Background())
	awaitListing(t, gateway, "listed")
	if server.listed.Load() != 1 {
		t.Fatalf("a fresh listing was read again: %d reads", server.listed.Load())
	}
	now = now.Add(2 * time.Minute)
	server.fail.Store(true)
	_, _ = gateway.Providers(context.Background())
	awaitListing(t, gateway, "listed")
	if server.listed.Load() != 2 {
		t.Fatalf("a stale listing was not read again: %d reads", server.listed.Load())
	}
	providers, _ = gateway.Providers(context.Background())
	if providers[len(providers)-1].Models != 2 {
		t.Fatalf("a failed read dropped what was listed: %#v", providers[len(providers)-1])
	}
	now = now.Add(listingRetry - time.Minute)
	_, _ = gateway.Providers(context.Background())
	awaitListing(t, gateway, "listed")
	if server.listed.Load() != 2 {
		t.Fatalf("a failed listing was asked again within listingRetry: %d reads", server.listed.Load())
	}
	_ = gateway.Close()

	again := open()
	setTestProviders(again, testProvider("listed", server.URL, 50, true))
	forgetListings(again, "listed")
	again.listings.loaded = false
	again.now = func() time.Time { return now }
	if read := again.listingRead("listed"); read.IsZero() {
		t.Fatal("the listing was not kept across a restart")
	}
}

// Read the list again reads at once and waits for it, but at most once a
// minute per provider, whoever asks.
func TestFreshReadsArePaced(t *testing.T) {
	server := newListingServer(t, `{"data":[{"id":"extra"}]}`)
	gateway := newTestGateway(t)
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	gateway.now = func() time.Time { return now }
	setTestProviders(gateway, testProvider("listed", server.URL, 50, true))

	fresh := protocol.ModelsParams{Provider: "listed", Fresh: true}
	models, err := gateway.Models(context.Background(), fresh)
	if err != nil {
		t.Fatal(err)
	}
	if len(models.Models) != 2 || server.listed.Load() != 1 {
		t.Fatalf("fresh read: %d models, %d reads", len(models.Models), server.listed.Load())
	}
	now = now.Add(30 * time.Second)
	if _, err := gateway.Models(context.Background(), protocol.ModelsParams{Fresh: true}); err != nil {
		t.Fatal(err)
	}
	if server.listed.Load() != 1 {
		t.Fatalf("a second fresh read within the minute reached the provider: %d reads", server.listed.Load())
	}
	now = now.Add(31 * time.Second)
	if _, err := gateway.Models(context.Background(), fresh); err != nil {
		t.Fatal(err)
	}
	if server.listed.Load() != 2 {
		t.Fatalf("a fresh read after the minute was not made: %d reads", server.listed.Load())
	}
}

// A listing read again that lists the same models still moves Listed and is
// announced, so Settings never shows an old read time after Read the list again.
func TestARereadOfTheSameListingIsAnnounced(t *testing.T) {
	server := newListingServer(t, `{"data":[{"id":"extra"}]}`)
	gateway := newTestGateway(t)
	now := time.Date(2026, 11, 12, 3, 54, 0, 0, time.UTC)
	gateway.now = func() time.Time { return now }
	setTestProviders(gateway, testProvider("listed", server.URL, 50, true))
	announced := make(chan struct{}, 4)
	gateway.OnChange(func() { announced <- struct{}{} })
	read := func() {
		t.Helper()
		if _, err := gateway.Models(context.Background(), protocol.ModelsParams{Provider: "listed", Fresh: true}); err != nil {
			t.Fatal(err)
		}
		select {
		case <-announced:
		case <-time.After(5 * time.Second):
			t.Fatalf("the read at %s was not announced", now.Format("15:04"))
		}
	}
	read()
	now = now.Add(5 * time.Minute)
	read()
	if server.listed.Load() != 2 {
		t.Fatalf("%d reads, want 2", server.listed.Load())
	}
	providers, err := gateway.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if listed := providers[len(providers)-1].Listed; listed == nil || !listed.Equal(now) {
		t.Fatalf("Listed = %v, want %s", listed, now)
	}
}

// Auto, Fast and Smart reach a listed model only when it is free and its
// tool support is known; a model with unknown tool support still answers
// when it is picked by name, and a paid one only then.
func TestAutoNeverPicksAPaidOrUnknownToolsModel(t *testing.T) {
	server := newListingServer(t, `{"data":[
		{"id":"paid-best","pricing":{"prompt":"0.1","completion":"0.1"},"supported_parameters":["tools"]},
		{"id":"mystery:free","pricing":{"prompt":"0","completion":"0"}},
		{"id":"known:free","pricing":{"prompt":"0","completion":"0"},"supported_parameters":["tools"]}
	]}`)
	gateway := newTestGateway(t)
	// a free provider with a key: what its listing prices is paid
	setTestProviders(gateway, provider{ID: "router", Name: "Router", Tier: protocol.TierFree, BaseURL: server.URL, Auth: "bearer"})
	if _, _, err := gateway.store.addKey("router", "", "router-test-key"); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.Models(context.Background(), protocol.ModelsParams{Provider: "router", Fresh: true}); err != nil {
		t.Fatal(err)
	}
	merged, err := gateway.findProvider(context.Background(), "router", false)
	if err != nil {
		t.Fatal(err)
	}
	if paid, _ := merged.findModel("paid-best"); paid == nil || merged.tierOf(paid) != protocol.TierPaid {
		t.Fatalf("the priced model of a free provider is not paid: %#v", merged.Models)
	}

	for _, route := range []string{protocol.RouteAuto, protocol.RouteFast, protocol.RouteSmart} {
		request := question()
		request.Model = route
		if text, _ := answer(t, gateway, request); text != "known:free" {
			t.Fatalf("%s answered from %q", route, text)
		}
	}
	for _, picked := range []string{"router/mystery:free", "router/paid-best"} {
		request := question()
		request.Model, request.Strict = picked, true
		if _, final := answer(t, gateway, request); final.Model != picked {
			t.Fatalf("a pick of %s answered from %s", picked, final.Model)
		}
	}
	models, err := gateway.Models(context.Background(), protocol.ModelsParams{Provider: "router"})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models.Models {
		if !m.Listed || m.ToolsKnown == (m.ID == "router/mystery:free") {
			t.Fatalf("model info = %#v", m)
		}
	}
}
