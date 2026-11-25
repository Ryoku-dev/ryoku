package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// countedServer answers every chat with the model it was asked for and
// counts the chats; fail makes it refuse each one with that status.
type countedServer struct {
	*httptest.Server
	chats atomic.Int32
}

func newCountedServer(t *testing.T, fail int) *countedServer {
	t.Helper()
	server := &countedServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.chats.Add(1)
		if fail != 0 {
			http.Error(w, `{"error":{"message":"overloaded"}}`, fail)
			return
		}
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sse(w, `{"choices":[{"delta":{"content":"`+body.Model+`"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	t.Cleanup(server.Close)
	return server
}

// newLocalVisionServer is an Ollama on this computer whose one model reads
// pictures.
func newLocalVisionServer(t *testing.T) *countedServer {
	t.Helper()
	server := &countedServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"eyes:latest","object":"model"}]}`)
		case "/api/show":
			_, _ = io.WriteString(w, `{"capabilities":["completion","vision"],"details":{"parameter_size":"4B"},"model_info":{"general.architecture":"gemma3","gemma3.context_length":32768},"parameters":"num_ctx 8192"}`)
		case "/v1/chat/completions":
			server.chats.Add(1)
			var body struct {
				Model string `json:"model"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			sse(w, `{"choices":[{"delta":{"content":"`+body.Model+`"},"finish_reason":"stop"}]}`, `[DONE]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func sightedProvider(id, baseURL string, quality int) provider {
	p := testProvider(id, baseURL, quality, false)
	p.Models[0].Vision = true
	return p
}

// sendPlanned sends request pinned strictly to plan, the way a disclosed
// screen ask goes out, and returns the model that answered.
func sendPlanned(t *testing.T, gateway *Gateway, request llm.Request, plan llm.Plan) (llm.Final, error) {
	t.Helper()
	request.Model, request.Strict, request.LocalOnly, request.Light = plan.Route(), true, plan.Local, false
	stream, err := gateway.Stream(context.Background(), request)
	if err != nil {
		return llm.Final{}, err
	}
	if _, err := collect(t, stream); err != nil {
		return llm.Final{}, err
	}
	return stream.Final(), nil
}

// The plan names the model that answers: under Auto, a pin, a light glance
// and local-only, the request sent pinned to the plan reaches that model and
// no other, and planning itself sends nothing.
func TestPlanIsTheRouteUsed(t *testing.T) {
	local := newLocalVisionServer(t)
	best := newCountedServer(t, 0)
	other := newCountedServer(t, 0)
	blind := newCountedServer(t, 0)
	gateway := newLocalTestGateway(t, map[string]string{protocol.ProviderOllamaLocal: local.URL + "/v1", protocol.ProviderLlamaCpp: "http://127.0.0.1:1/v1"})
	paid := sightedProvider("paid", other.URL, 120)
	paid.Tier = protocol.TierPaid
	setTestProviders(gateway, testProvider("blind", blind.URL, 200, true), sightedProvider("best", best.URL, 100), sightedProvider("other", other.URL, 60), paid)
	gateway.settings = func() protocol.Settings {
		return protocol.Settings{Providers: protocol.ProvidersSettings{
			Local:  map[string]string{protocol.ProviderOllamaLocal: local.URL + "/v1", protocol.ProviderLlamaCpp: "http://127.0.0.1:1/v1"},
			Models: protocol.LocalModelsSettings{RemoteFirst: true},
		}}
	}

	for _, test := range []struct {
		name    string
		request llm.Request
		want    llm.Plan
		server  *countedServer
	}{
		{"auto", pictureRequest(protocol.RouteAuto, false), llm.Plan{Provider: "best", Model: "chat", Vision: true}, best},
		{"pinned", pictureRequest("other/chat", true), llm.Plan{Provider: "other", Model: "chat", Vision: true}, other},
		{"loose pin", pictureRequest("other/chat", false), llm.Plan{Provider: "other", Model: "chat", Vision: true}, other},
		{"paid pin", pictureRequest("paid/chat", true), llm.Plan{Provider: "paid", Model: "chat", Vision: true, Paid: true}, other},
		{"local only", func() llm.Request {
			request := pictureRequest(protocol.RouteAuto, false)
			request.LocalOnly = true
			return request
		}(), llm.Plan{Provider: protocol.ProviderOllamaLocal, Model: "eyes:latest", Local: true, Vision: true}, local},
		{"light glance", func() llm.Request {
			request := pictureRequest("", false)
			request.Light = true
			return request
		}(), llm.Plan{Provider: protocol.ProviderOllamaLocal, Model: "eyes:latest", Local: true, Vision: true}, local},
	} {
		t.Run(test.name, func(t *testing.T) {
			chats := func() int32 { return local.chats.Load() + best.chats.Load() + other.chats.Load() + blind.chats.Load() }
			before := chats()
			plan, err := gateway.Plan(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}
			if plan != test.want {
				t.Fatalf("plan = %#v, want %#v", plan, test.want)
			}
			if chats() != before {
				t.Fatalf("planning sent %d requests", chats()-before)
			}
			// unpinned, the stream's first try is the plan as well
			stream, err := gateway.Stream(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := collect(t, stream); err != nil {
				t.Fatal(err)
			}
			if final := stream.Final(); final.Provider != plan.Provider || final.Model != plan.Route() || final.Attempts != 1 {
				t.Fatalf("the request went to %s (%d attempts), the plan named %s", final.Model, final.Attempts, plan.Route())
			}
			served := test.server.chats.Load()
			final, err := sendPlanned(t, gateway, test.request, plan)
			if err != nil {
				t.Fatal(err)
			}
			if final.Provider != plan.Provider || final.Model != plan.Route() || test.server.chats.Load() != served+1 {
				t.Fatalf("the pinned request went to %s, the plan named %s", final.Model, plan.Route())
			}
		})
	}
	if blind.chats.Load() != 0 {
		t.Fatalf("the model that cannot read pictures was sent them %d times", blind.chats.Load())
	}
}

// With nothing in use that reads pictures, the plan refuses as the stream
// does, with the same words and the no-vision mark, and so does a pin to a
// model that cannot read them.
func TestPlanRefusesAPictureNothingReads(t *testing.T) {
	gateway := newTestGateway(t)
	setTestProviders(gateway, testProvider("blind", "http://127.0.0.1:1", 100, true))
	for _, request := range []llm.Request{pictureRequest(protocol.RouteAuto, false), pictureRequest("blind/chat", true)} {
		_, planErr := gateway.Plan(context.Background(), request)
		_, streamErr := gateway.Stream(context.Background(), request)
		var planned, streamed *llm.NoRoute
		if !errors.As(planErr, &planned) || !errors.As(streamErr, &streamed) || !planned.NoVision || planned.Reason != streamed.Reason {
			t.Fatalf("plan refused %v, stream refused %v", planErr, streamErr)
		}
	}
	// a picture is no part of a text plan's refusal
	setTestProviders(gateway)
	_, err := gateway.Plan(context.Background(), llm.Request{Model: protocol.RouteAuto, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}})
	var refused *llm.NoRoute
	if !errors.As(err, &refused) || refused.NoVision {
		t.Fatalf("an empty router planned %v", err)
	}
}

// A request pinned strictly to its plan fails with that model rather than
// going to another, where the same request unpinned fails over.
func TestStrictPlanNeverFailsOver(t *testing.T) {
	setup := func() (*Gateway, *countedServer, *countedServer) {
		failing := newCountedServer(t, http.StatusServiceUnavailable)
		spare := newCountedServer(t, 0)
		gateway := newTestGateway(t)
		setTestProviders(gateway, sightedProvider("failing", failing.URL, 100), sightedProvider("spare", spare.URL, 60))
		return gateway, failing, spare
	}

	gateway, failing, spare := setup()
	plan, err := gateway.Plan(context.Background(), pictureRequest(protocol.RouteAuto, false))
	if err != nil || plan.Route() != "failing/chat" {
		t.Fatalf("plan = %#v, %v", plan, err)
	}
	if _, err := sendPlanned(t, gateway, pictureRequest(protocol.RouteAuto, false), plan); err == nil {
		t.Fatal("a pinned request whose model failed reported an answer")
	}
	if failing.chats.Load() == 0 || spare.chats.Load() != 0 {
		t.Fatalf("pinned: the planned model was tried %d times, another %d times", failing.chats.Load(), spare.chats.Load())
	}

	// the same failure unpinned goes elsewhere, so the pin is what held it
	gateway, failing, spare = setup()
	stream, err := gateway.Stream(context.Background(), pictureRequest(protocol.RouteAuto, false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collect(t, stream); err != nil || failing.chats.Load() == 0 || spare.chats.Load() != 1 {
		t.Fatalf("unpinned: failing %d, spare %d, %v", failing.chats.Load(), spare.chats.Load(), err)
	}
}

// Ryoku Ember is planned only when it is the model asked for, as a paid,
// hosted route the daemon keeps for the app, and planning it reaches no
// part of the Account.
func TestPlanNamesEmberOnlyWhenPicked(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		sse(w, `[DONE]`)
	}))
	defer server.Close()
	hosted := newFakeHosted(server.URL, true)
	gateway := hostedGateway(t, hosted)
	plan, err := gateway.Plan(context.Background(), hostedRequest(hostedModel))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Route() != hostedModel || plan.Provider != protocol.ProviderEmber || !plan.Paid || plan.Local || !protocol.IsHostedModel(plan.Route()) {
		t.Fatalf("hosted plan = %#v", plan)
	}
	if _, err := gateway.Plan(context.Background(), hostedRequest(protocol.RouteAuto)); !asNoRoute(err) {
		t.Fatalf("Auto planned the Account's model: %v", err)
	}
	if hits.Load() != 0 || hosted.endpoints.Load() != 0 || hosted.used.Load() != 0 {
		t.Fatalf("planning reached the Account: %d requests, %d endpoints, %d uses", hits.Load(), hosted.endpoints.Load(), hosted.used.Load())
	}
}
