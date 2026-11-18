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

func TestLocalVisionCapabilitiesAreProbedAndCached(t *testing.T) {
	for _, providerID := range protocol.LocalProviders {
		t.Run(providerID, func(t *testing.T) {
			var probes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/models":
					_, _ = io.WriteString(w, `{"data":[{"id":"capture-model"}]}`)
				case "/api/show":
					var body struct {
						Model string `json:"model"`
					}
					if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&body) != nil || body.Model != "capture-model" {
						t.Errorf("bad Ollama probe: %s %#v", r.Method, body)
					}
					probes.Add(1)
					_, _ = io.WriteString(w, `{"capabilities":["completion","vision"]}`)
				case "/props":
					if r.Method != http.MethodGet {
						t.Errorf("bad llama.cpp probe: %s", r.Method)
					}
					probes.Add(1)
					_, _ = io.WriteString(w, `{"modalities":{"vision":true}}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			gateway := newTestGateway(t)
			for range 2 {
				running, models, _ := gateway.localModels(context.Background(), providerID, server.URL+"/v1", false)
				if !running || len(models) != 1 || !models[0].Vision || models[0].Tools {
					t.Fatalf("local models = %#v, running %v", models, running)
				}
			}
			if probes.Load() != 1 {
				t.Fatalf("capabilities probed %d times within the TTL", probes.Load())
			}
		})
	}
}

func TestLocalVisionProbeFailureDoesNotInventVision(t *testing.T) {
	for _, response := range []string{`{`, `{"capabilities":["completion"]}`, `{"modalities":{"vision":false}}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, response)
		}))
		gateway := newTestGateway(t)
		for _, id := range protocol.LocalProviders {
			vision, _ := gateway.localCapabilities(context.Background(), id, server.URL+"/v1", "model")
			if vision {
				t.Fatalf("%s invented vision from %q", id, response)
			}
		}
		server.Close()
	}
}

func TestLocalOnlyNeverRoutesToRemoteProviders(t *testing.T) {
	server := newLocalServer(t)
	gateway := newLocalTestGateway(t, map[string]string{protocol.ProviderOllamaLocal: server.URL + "/v1", protocol.ProviderLlamaCpp: "http://127.0.0.1:1/v1"})
	remote := testProvider("remote", "http://127.0.0.1:1", 100, true)
	setTestProviders(gateway, remote)
	request := question()
	request.LocalOnly = true
	request.Model = protocol.RouteSmart
	candidates, err := gateway.candidates(context.Background(), request)
	if err != nil || len(candidates) == 0 {
		t.Fatalf("local routing = %#v, %v", candidates, err)
	}
	for _, candidate := range candidates {
		if candidate.provider.Tier != protocol.TierLocal {
			t.Fatalf("local-only routed remotely: %#v", candidate.provider)
		}
	}
	request.Model, request.Strict = "remote/model", true
	candidates, err = gateway.candidates(context.Background(), request)
	var unavailable *llm.NoRoute
	if !errors.As(err, &unavailable) || len(candidates) != 0 {
		t.Fatalf("a remote pin escaped local-only: %#v, %v", candidates, err)
	}
}

func TestNoPictureRouteHasNoVisionCode(t *testing.T) {
	gateway := newTestGateway(t)
	textOnly := testProvider("text", "http://127.0.0.1:1", 100, true)
	textOnly.Models[0].Vision = false
	setTestProviders(gateway, textOnly)
	_, err := gateway.Stream(context.Background(), llm.Request{Model: protocol.RouteFast, Messages: []llm.Message{{Role: llm.RoleUser, Images: []llm.Image{{Mime: "image/png", Data: []byte("image")}}}}})
	var noRoute *llm.NoRoute
	if !errors.As(err, &noRoute) || !noRoute.NoVision || noRoute.Reason != gateway.noPictureModel() {
		t.Fatalf("picture refusal = %#v", err)
	}
}
