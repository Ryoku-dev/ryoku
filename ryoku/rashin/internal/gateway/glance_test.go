package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func TestLocalVisionAndToolsAreIndependent(t *testing.T) {
	for _, providerID := range protocol.AddressedLocalProviders {
		t.Run(providerID, func(t *testing.T) {
			var probes atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/models", "/v1/models":
					_, _ = io.WriteString(w, `{"data":[{"id":"capture-model","architecture":{"input_modalities":["text","image","audio"]},"meta":{"n_ctx":16384,"n_ctx_train":32768,"n_params":2000000000}}]}`)
				case "/api/show":
					probes.Add(1)
					_, _ = io.WriteString(w, `{"capabilities":["completion","vision","tools","audio"],"model_info":{"general.architecture":"lfm","lfm.context_length":32768,"general.parameter_count":2000000000},"parameters":"num_ctx 8192"}`)
				case "/props":
					probes.Add(1)
					_, _ = io.WriteString(w, `{"default_generation_settings":{"n_ctx":12288},"chat_template_caps":{"supports_tool_calls":true},"modalities":{"vision":true,"audio":true}}`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			gateway := newTestGateway(t)
			for range 2 {
				running, models, _ := gateway.localModels(context.Background(), providerID, server.URL+"/v1", false)
				if !running || len(models) != 1 || !models[0].Vision || !models[0].Tools || !models[0].Audio || models[0].Params != 2e9 {
					t.Fatalf("local models = %#v, running %v", models, running)
				}
				wantContext := 12288
				if providerID == protocol.ProviderOllamaLocal {
					wantContext = 8192
				}
				if models[0].Context != wantContext {
					t.Fatalf("%s context = %d, want %d", providerID, models[0].Context, wantContext)
				}
			}
			if probes.Load() != 1 {
				t.Fatalf("capabilities probed %d times within the TTL", probes.Load())
			}
		})
	}
}

func TestLlamaVisionProbeFailureDoesNotInventVision(t *testing.T) {
	for _, response := range []string{`{`, `{"modalities":{"vision":false}}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/models":
				_, _ = io.WriteString(w, `{"data":[{"id":"model"}]}`)
			case "/props":
				_, _ = io.WriteString(w, response)
			default:
				http.NotFound(w, r)
			}
		}))
		gateway := newTestGateway(t)
		running, models, _ := gateway.localModels(context.Background(), protocol.ProviderLlamaCpp, server.URL+"/v1", false)
		if !running || len(models) != 1 || models[0].Vision {
			t.Fatalf("invented vision from %q: %#v", response, models)
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
