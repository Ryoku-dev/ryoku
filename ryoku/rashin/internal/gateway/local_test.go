package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// localServer is an Ollama-shaped server: its /v1/models lists a chat model
// and an embedding model, and its chat answers with the model it was sent.
type localServer struct {
	*httptest.Server
	listed atomic.Int32
	chats  atomic.Int32
}

func newLocalServer(t *testing.T) *localServer {
	t.Helper()
	server := &localServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			server.listed.Add(1)
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"llama3.2:latest","object":"model"},{"id":"nomic-embed-text:latest","object":"model"}]}`)
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

func question() llm.Request {
	return llm.Request{Model: protocol.RouteAuto, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}, Tools: []llm.Tool{{Name: "search", Parameters: []byte(`{"type":"object"}`)}}}
}

func answer(t *testing.T, gateway *Gateway, request llm.Request) (string, llm.Final) {
	t.Helper()
	stream, err := gateway.Stream(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	deltas, err := collect(t, stream)
	if err != nil {
		t.Fatal(err)
	}
	text := ""
	for _, delta := range deltas {
		text += delta.Text
	}
	return text, stream.Final()
}

// A running Ollama is found through /v1/models, its chat models are routed
// with tools and ahead of the catalogue under Auto, and a llama.cpp that is
// not running reads as such without an error and is never routed.
func TestALocalServerIsFoundAndAutoPrefersIt(t *testing.T) {
	server := newLocalServer(t)
	gateway := newLocalTestGateway(t, map[string]string{protocol.ProviderOllamaLocal: server.URL + "/v1", protocol.ProviderLlamaCpp: "http://127.0.0.1:1/v1"})
	var catalogue atomic.Int32
	free := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		catalogue.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"catalogue"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer free.Close()
	setTestProviders(gateway, testProvider("free", free.URL, 99, true))

	providers, err := gateway.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 3 || providers[0].ID != protocol.ProviderOllamaLocal || providers[1].ID != protocol.ProviderLlamaCpp {
		t.Fatalf("providers = %#v", providers)
	}
	ollama, llamacpp := providers[0], providers[1]
	if !ollama.Local || !ollama.Running || ollama.Tier != protocol.TierLocal || !ollama.Keyless || ollama.Models != 1 || ollama.Available != 1 || ollama.Address != server.URL+"/v1" {
		t.Fatalf("ollama = %#v", ollama)
	}
	if !llamacpp.Local || llamacpp.Running || llamacpp.Models != 0 || llamacpp.Available != 0 || llamacpp.Address != "http://127.0.0.1:1/v1" {
		t.Fatalf("llama.cpp = %#v", llamacpp)
	}

	models, err := gateway.Models(context.Background(), protocol.ModelsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(models.Models) != 2 {
		t.Fatalf("models = %#v", models.Models)
	}
	local := models.Models[0]
	if local.ID != "ollama-local/llama3.2:latest" || local.Name != "llama3.2" || local.Tier != protocol.TierLocal || !local.Tools || !local.Available {
		t.Fatalf("local model = %#v", local)
	}
	if models.Models[1].Tier != protocol.TierFree {
		t.Fatalf("catalogue model = %#v", models.Models[1])
	}

	text, final := answer(t, gateway, question())
	if text != "llama3.2:latest" || final.Provider != protocol.ProviderOllamaLocal || catalogue.Load() != 0 {
		t.Fatalf("auto answered %q from %#v, catalogue asked %d times", text, final, catalogue.Load())
	}
	smart := question()
	smart.Model = protocol.RouteSmart
	if text, _ := answer(t, gateway, smart); text != "catalogue" {
		t.Fatalf("smart answered %q, want the catalogue's best", text)
	}
	result, err := gateway.Test(context.Background(), protocol.ProviderLlamaCpp)
	if err != nil || result.OK || result.Message == "" {
		t.Fatalf("testing a stopped llama.cpp = %#v, %v", result, err)
	}
}

// A server's answer is trusted for localTTL; a test asks again, and a server
// that stops is announced as a change.
func TestALocalServerIsAskedAgainOnlyWhenItsAnswerIsOld(t *testing.T) {
	server := newLocalServer(t)
	gateway := newLocalTestGateway(t, map[string]string{protocol.ProviderOllamaLocal: server.URL + "/v1", protocol.ProviderLlamaCpp: "http://127.0.0.1:1/v1"})
	setTestProviders(gateway)
	now := time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
	gateway.now = func() time.Time { return now }
	changes := make(chan struct{}, 4)
	gateway.OnChange(func() { changes <- struct{}{} })
	for range 3 {
		if _, err := gateway.Providers(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if server.listed.Load() != 1 {
		t.Fatalf("asked %d times within the TTL", server.listed.Load())
	}
	now = now.Add(localTTL + time.Second)
	if _, err := gateway.Models(context.Background(), protocol.ModelsParams{}); err != nil {
		t.Fatal(err)
	}
	if server.listed.Load() != 2 {
		t.Fatalf("asked %d times after the TTL", server.listed.Load())
	}
	if _, err := gateway.Test(context.Background(), protocol.ProviderOllamaLocal); err != nil {
		t.Fatal(err)
	}
	if server.listed.Load() != 3 || server.chats.Load() != 1 {
		t.Fatalf("a test asked %d times and chatted %d times", server.listed.Load(), server.chats.Load())
	}
	for len(changes) > 0 {
		<-changes
	}
	server.Close()
	now = now.Add(localTTL + time.Second)
	providers, err := gateway.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if providers[0].Running {
		t.Fatal("a stopped server still reads as running")
	}
	select {
	case <-changes:
	case <-time.After(5 * time.Second):
		t.Fatal("a server that stopped was not announced")
	}
}

// The gateway connects to a local provider only on this computer or the
// local network, whatever address the settings carry.
func TestALocalProviderOffTheNetworkIsNeverReached(t *testing.T) {
	for address, allowed := range map[string]bool{
		"127.0.0.1:11434": true, "[::1]:8080": true, "192.168.1.20:11434": true, "10.0.0.5:80": true, "[fd00::7]:80": true,
		"203.0.113.9:11434": false, "169.254.169.254:80": false, "[2001:db8::1]:80": false, "8.8.8.8:53": false,
	} {
		if err := onlyPrivate("tcp", address, nil); (err == nil) != allowed {
			t.Fatalf("%s: %v, allowed %v", address, err, allowed)
		}
	}
	gateway := newLocalTestGateway(t, map[string]string{protocol.ProviderOllamaLocal: "http://203.0.113.9:11434/v1", protocol.ProviderLlamaCpp: "http://127.0.0.1:1/v1"})
	setTestProviders(gateway)
	started := time.Now()
	providers, err := gateway.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if providers[0].Running || time.Since(started) > time.Second {
		t.Fatalf("a public address was reached: %#v after %s", providers[0], time.Since(started))
	}
}

// Auto, Fast and Smart never send a question to a paid model, even the best
// one or a paid model of a free provider, and a pin to a free model does not
// fail over to one. A paid model answers when it is the model asked for.
func TestAutoNeverRoutesToAPaidModel(t *testing.T) {
	var paidAsked atomic.Int32
	paid := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paidAsked.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"paid"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer paid.Close()
	var freeFails atomic.Bool
	free := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if freeFails.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		sse(w, `{"choices":[{"delta":{"content":"free"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer free.Close()
	gateway := newTestGateway(t)
	billed := testProvider("billed", paid.URL, 100, true)
	billed.Tier = protocol.TierPaid
	mixed := testProvider("mixed", paid.URL, 99, true)
	mixed.Models[0].Tier = protocol.TierPaid
	setTestProviders(gateway, billed, mixed, testProvider("free", free.URL, 40, true))

	for _, route := range []string{protocol.RouteAuto, protocol.RouteFast, protocol.RouteSmart, ""} {
		request := question()
		request.Model = route
		if text, _ := answer(t, gateway, request); text != "free" {
			t.Fatalf("%q answered %q", route, text)
		}
	}
	if paidAsked.Load() != 0 {
		t.Fatalf("a route reached a paid model %d times", paidAsked.Load())
	}
	models, err := gateway.Models(context.Background(), protocol.ModelsParams{})
	if err != nil {
		t.Fatal(err)
	}
	tiers := map[string]string{}
	for _, m := range models.Models {
		tiers[m.ID] = m.Tier
	}
	if tiers["billed/chat"] != protocol.TierPaid || tiers["mixed/chat"] != protocol.TierPaid || tiers["free/chat"] != protocol.TierFree {
		t.Fatalf("tiers = %#v", tiers)
	}

	freeFails.Store(true)
	pinned := question()
	pinned.Model = "free/chat"
	stream, err := gateway.Stream(context.Background(), pinned)
	if err == nil {
		_, err = collect(t, stream)
	}
	if err == nil || paidAsked.Load() != 0 {
		t.Fatalf("a failing free pin fell over to a paid model: %v, asked %d", err, paidAsked.Load())
	}

	pinned.Model = "billed/chat"
	if text, final := answer(t, gateway, pinned); text != "paid" || final.Model != "billed/chat" {
		t.Fatalf("the paid pin answered %q from %#v", text, final)
	}
}

// A provider whose reasoning models take max_completion_tokens gets it in
// place of max_tokens; the others keep max_tokens.
func TestCompletionTokensReplaceMaxTokensWhereAsked(t *testing.T) {
	bodies := make(chan map[string]any, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies <- body
		sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer server.Close()
	gateway := newTestGateway(t)
	reasoning := testProvider("reasoning", server.URL, 90, true)
	reasoning.CompletionTokens = true
	setTestProviders(gateway, reasoning)
	request := question()
	request.MaxTokens = 4096
	answer(t, gateway, request)
	body := <-bodies
	if body["max_completion_tokens"] != float64(4096) || body["max_tokens"] != nil {
		t.Fatalf("reasoning provider body = %#v", body)
	}
	setTestProviders(gateway, testProvider("plain", server.URL, 90, true))
	answer(t, gateway, request)
	body = <-bodies
	if body["max_tokens"] != float64(4096) || body["max_completion_tokens"] != nil {
		t.Fatalf("plain provider body = %#v", body)
	}
}

// A provider is tested on a model it serves without billing when it has
// one, and a reasoning model that spends the one token thinking passes.
func TestAProviderIsTestedOnAFreeModel(t *testing.T) {
	var asked atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		asked.Store(body.Model)
		sse(w, `{"choices":[{"delta":{},"finish_reason":"length"}]}`, `[DONE]`)
	}))
	defer server.Close()
	gateway := newTestGateway(t)
	p := testProvider("google", server.URL, 80, true)
	p.Models = append(p.Models, model{ID: "pro", Name: "Pro", Tier: protocol.TierPaid, Tools: true, QualityRank: 99})
	setTestProviders(gateway, p)
	result, err := gateway.Test(context.Background(), "google")
	if err != nil || !result.OK {
		t.Fatalf("test = %#v, %v", result, err)
	}
	if asked.Load() != "chat" {
		t.Fatalf("tested on %v, want the free model", asked.Load())
	}
}
