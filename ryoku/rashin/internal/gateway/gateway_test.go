package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func newTestGateway(t *testing.T) *Gateway {
	t.Helper()
	gateway, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	return gateway
}

func setTestProviders(gateway *Gateway, providers ...provider) {
	gateway.catalog.Providers = providers
	gateway.catalog.byID = make(map[string]*provider, len(providers))
	for i := range gateway.catalog.Providers {
		gateway.catalog.byID[providers[i].ID] = &gateway.catalog.Providers[i]
	}
}

func testProvider(id, baseURL string, quality int, tools bool) provider {
	return provider{ID: id, Name: id, Tier: "permanent-free", BaseURL: baseURL, Auth: "none", Keyless: true, Models: []model{{ID: "chat", Name: "chat", Context: 128000, Tools: tools, QualityRank: quality, SpeedRank: quality}}}
}

func sse(w http.ResponseWriter, frames ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, frame := range frames {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", frame)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

func collect(t *testing.T, stream llm.Stream) ([]llm.Delta, error) {
	t.Helper()
	defer stream.Close()
	var deltas []llm.Delta
	for {
		delta, err := stream.Next()
		if errors.Is(err, io.EOF) {
			return deltas, nil
		}
		if err != nil {
			return deltas, err
		}
		deltas = append(deltas, delta)
	}
}

func TestSSEParsesReasoningUsageAndSplitToolArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`{"choices":[{"delta":{"role":"assistant","reasoning_content":"think "}}]}`,
			`{"choices":[{"delta":{"content":"done"}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"search","arguments":"{\"q\":"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"ryoku\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":11,"completion_tokens":7}}`,
			`[DONE]`,
		)
	}))
	defer server.Close()
	gateway := newTestGateway(t)
	setTestProviders(gateway, testProvider("test", server.URL, 100, true))
	stream, err := gateway.Stream(context.Background(), llm.Request{Model: protocol.RouteAuto, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}, Tools: []llm.Tool{{Name: "search", Parameters: []byte(`{"type":"object"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	deltas, err := collect(t, stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 3 {
		t.Fatalf("deltas = %#v", deltas)
	}
	if deltas[0].Reasoning != "think " || deltas[1].Text != "done" {
		t.Fatalf("unexpected text deltas: %#v", deltas)
	}
	if got := deltas[2].ToolCalls; len(got) != 1 || got[0].ID != "call_1" || got[0].Name != "search" || got[0].Arguments != `{"q":"ryoku"}` {
		t.Fatalf("tool calls = %#v", got)
	}
	final := stream.Final()
	if final.Finish != llm.FinishToolCalls || final.Usage.Input != 11 || final.Usage.Output != 7 || final.Usage.Estimated {
		t.Fatalf("final = %#v", final)
	}
}

func TestFailoverClassifiedErrorsThenSuccess(t *testing.T) {
	statuses := []int{http.StatusTooManyRequests, http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError}
	providers := make([]provider, 0, len(statuses)+1)
	var hits atomic.Int32
	for i, status := range statuses {
		status := status
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if status == http.StatusTooManyRequests {
				w.Header().Set("Retry-After", "1")
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream rejected request"}}`))
		}))
		defer server.Close()
		providers = append(providers, testProvider(fmt.Sprintf("p%d", i), server.URL, 100-i, true))
	}
	success := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer success.Close()
	providers = append(providers, testProvider("success", success.URL, 90, true))
	providers[len(providers)-1].Models[0].QualityRank = 90
	providers[3].Models[0].QualityRank = 97
	providers[4].Models[0].QualityRank = 96
	gateway := newTestGateway(t)
	setTestProviders(gateway, providers...)
	stream, err := gateway.Stream(context.Background(), llm.Request{Model: protocol.RouteAuto, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	deltas, err := collect(t, stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 1 || deltas[0].Text != "ok" {
		t.Fatalf("deltas = %#v", deltas)
	}
	if stream.Final().Attempts != 5 || hits.Load() != 5 {
		t.Fatalf("final = %#v, hits = %d", stream.Final(), hits.Load())
	}
}

func TestNoFailoverAfterFirstContent(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"partial"}}]}`, `{not-json`)
	}))
	defer first.Close()
	var fallbackHits atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHits.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"wrong"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer fallback.Close()
	gateway := newTestGateway(t)
	setTestProviders(gateway, testProvider("first", first.URL, 100, true), testProvider("fallback", fallback.URL, 90, true))
	stream, err := gateway.Stream(context.Background(), llm.Request{Model: protocol.RouteAuto, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	delta, err := stream.Next()
	if err != nil || delta.Text != "partial" {
		t.Fatalf("first delta = %#v, %v", delta, err)
	}
	if _, err := stream.Next(); err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("second Next error = %v", err)
	}
	if fallbackHits.Load() != 0 {
		t.Fatalf("fallback received %d requests after commit", fallbackHits.Load())
	}
}

func TestDailyQuotaCooldownRunsUntilUTCMidnight(t *testing.T) {
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"daily quota allocation used up"}}`))
	}))
	defer limited.Close()
	success := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer success.Close()
	gateway := newTestGateway(t)
	now := time.Date(2026, 11, 10, 22, 30, 0, 0, time.FixedZone("local", 2*60*60))
	gateway.now = func() time.Time { return now }
	setTestProviders(gateway, testProvider("limited", limited.URL, 100, true), testProvider("success", success.URL, 90, true))
	stream, err := gateway.Stream(context.Background(), llm.Request{Model: protocol.RouteAuto, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collect(t, stream); err != nil {
		t.Fatal(err)
	}
	until, ok, err := gateway.store.cooldown(modelScope("limited", "chat"), now)
	if err != nil || !ok {
		t.Fatalf("cooldown = %v, %v", ok, err)
	}
	want := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	if !until.Equal(want) {
		t.Fatalf("cooldown until %s, want %s", until, want)
	}
}

func TestToolRequestSkipsModelsWithoutTools(t *testing.T) {
	var noToolHits atomic.Int32
	noTools := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		noToolHits.Add(1)
		sse(w, `[DONE]`)
	}))
	defer noTools.Close()
	tools := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer tools.Close()
	gateway := newTestGateway(t)
	setTestProviders(gateway, testProvider("no-tools", noTools.URL, 100, false), testProvider("tools", tools.URL, 90, true))
	stream, err := gateway.Stream(context.Background(), llm.Request{Model: protocol.RouteAuto, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}, Tools: []llm.Tool{{Name: "noop", Parameters: []byte(`{"type":"object"}`)}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collect(t, stream); err != nil {
		t.Fatal(err)
	}
	if noToolHits.Load() != 0 {
		t.Fatalf("non-tool model received %d requests", noToolHits.Load())
	}
}

func TestKeySealingMaskingAndHonestHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer server.Close()
	gateway := newTestGateway(t)
	p := testProvider("keyed", server.URL, 100, true)
	p.Keyless = false
	p.Auth = "bearer"
	setTestProviders(gateway, p)
	record, duplicate, err := gateway.store.addKey("keyed", "main", "gsk_secret_a1b2")
	if err != nil || duplicate {
		t.Fatalf("add key = %v, duplicate %v", err, duplicate)
	}
	secret, loaded, err := gateway.store.reveal(record.ID)
	if err != nil || secret != "gsk_secret_a1b2" {
		t.Fatalf("reveal = %q, %v", secret, err)
	}
	if strings.Contains(string(loaded.Sealed), secret) || maskSecret(secret) != "gsk_...a1b2" {
		t.Fatalf("secret was not sealed or masked: %q", maskSecret(secret))
	}
	response, err := gateway.client.Get(server.URL + "/models")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	_, loaded, err = gateway.store.reveal(record.ID)
	if err != nil || loaded.Status != protocol.KeyUnknown {
		t.Fatalf("model list changed health to %q", loaded.Status)
	}
	if err := gateway.applyHealth(record.ID, http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`, false); err != nil {
		t.Fatal(err)
	}
	_, loaded, _ = gateway.store.reveal(record.ID)
	if loaded.Status != protocol.KeyError {
		t.Fatalf("401 status = %q", loaded.Status)
	}
	if err := gateway.applyHealth(record.ID, http.StatusOK, "", true); err != nil {
		t.Fatal(err)
	}
	_, loaded, _ = gateway.store.reveal(record.ID)
	if loaded.Status != protocol.KeyHealthy {
		t.Fatalf("successful completion status = %q", loaded.Status)
	}
}

func TestCommittedStreamIdleEndsWithoutFailover(t *testing.T) {
	previous := streamIdleGap
	streamIdleGap = 25 * time.Millisecond
	defer func() { streamIdleGap = previous }()

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"partial"}}]}`)
		<-r.Context().Done()
	}))
	defer first.Close()
	var fallbackHits atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHits.Add(1)
		sse(w, `{"choices":[{"delta":{"content":"wrong"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer fallback.Close()

	gateway := newTestGateway(t)
	setTestProviders(gateway, testProvider("first", first.URL, 100, true), testProvider("fallback", fallback.URL, 90, true))
	stream, err := gateway.Stream(context.Background(), llm.Request{Model: protocol.RouteAuto, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	delta, err := stream.Next()
	if err != nil || delta.Text != "partial" {
		t.Fatalf("first delta = %#v, %v", delta, err)
	}
	if _, err := stream.Next(); !errors.Is(err, errStreamIdle) {
		t.Fatalf("idle error = %v", err)
	}
	if fallbackHits.Load() != 0 {
		t.Fatalf("fallback received %d requests after commit", fallbackHits.Load())
	}
}

func TestPinnedModelFallsBackUnlessStrict(t *testing.T) {
	for _, test := range []struct {
		name         string
		strict       bool
		wantFallback int32
		wantSuccess  bool
	}{
		{name: "non-strict", wantFallback: 1, wantSuccess: true},
		{name: "strict", strict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			pinned := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"message":"temporary failure"}}`))
			}))
			defer pinned.Close()
			var fallbackHits atomic.Int32
			fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fallbackHits.Add(1)
				sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
			}))
			defer fallback.Close()

			gateway := newTestGateway(t)
			setTestProviders(gateway, testProvider("pinned", pinned.URL, 1, true), testProvider("fallback", fallback.URL, 100, true))
			stream, err := gateway.Stream(context.Background(), llm.Request{Model: "pinned/chat", Strict: test.strict, Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}}})
			if err != nil {
				t.Fatal(err)
			}
			deltas, streamErr := collect(t, stream)
			if test.wantSuccess {
				if streamErr != nil || len(deltas) != 1 || deltas[0].Text != "ok" {
					t.Fatalf("deltas = %#v, error = %v", deltas, streamErr)
				}
				if stream.Final().Attempts != 2 {
					t.Fatalf("attempts = %d", stream.Final().Attempts)
				}
			} else if streamErr == nil {
				t.Fatal("strict pinned request unexpectedly failed over")
			}
			if fallbackHits.Load() != test.wantFallback {
				t.Fatalf("fallback hits = %d, want %d", fallbackHits.Load(), test.wantFallback)
			}
		})
	}
}

func TestProviderTestRedactsSecretFromErrors(t *testing.T) {
	const secret = "gsk_private_value_a1b2"
	for _, test := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "http error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprintf(w, `{"error":{"message":"invalid key %s"}}`, secret)
			},
		},
		{
			name: "stream error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				sse(w, fmt.Sprintf(`{"error":{"message":"provider rejected %s"}}`, secret))
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()
			gateway := newTestGateway(t)
			p := testProvider("keyed", server.URL, 100, true)
			p.Keyless = false
			p.Auth = "bearer"
			setTestProviders(gateway, p)
			if _, _, err := gateway.store.addKey("keyed", "main", secret); err != nil {
				t.Fatal(err)
			}
			result, err := gateway.Test(context.Background(), "keyed")
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(result.Message, secret) || !strings.Contains(result.Message, "[redacted]") {
				t.Fatalf("test message exposed secret: %q", result.Message)
			}
		})
	}
}

func TestOnChangeCoversHealthAndProviderTests(t *testing.T) {
	gateway := newTestGateway(t)
	var changes atomic.Int32
	gateway.OnChange(func() { changes.Add(1) })

	record, _, err := gateway.store.addKey("keyed", "main", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := gateway.applyHealth(record.ID, http.StatusOK, "", true); err != nil {
		t.Fatal(err)
	}
	if changes.Load() != 1 {
		t.Fatalf("health change callbacks = %d", changes.Load())
	}
	if err := gateway.applyHealth(record.ID, http.StatusOK, "", true); err != nil {
		t.Fatal(err)
	}
	if changes.Load() != 1 {
		t.Fatalf("unchanged health callbacks = %d", changes.Load())
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer server.Close()
	setTestProviders(gateway, testProvider("keyless", server.URL, 100, true))
	result, err := gateway.Test(context.Background(), "keyless")
	if err != nil || !result.OK {
		t.Fatalf("provider test = %#v, %v", result, err)
	}
	if changes.Load() != 2 {
		t.Fatalf("provider test callbacks = %d", changes.Load())
	}
}

func TestModelsUseSingleQualifiedID(t *testing.T) {
	gateway := newTestGateway(t)
	p := testProvider("ovh", "https://example.invalid", 100, true)
	p.Models[0].ID = "meta/llama"
	setTestProviders(gateway, p)

	result, err := gateway.Models(context.Background(), "ovh")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Models) != 1 {
		t.Fatalf("models = %#v", result.Models)
	}
	if result.Models[0].ID != "ovh/meta/llama" || result.Models[0].Provider != "ovh" {
		t.Fatalf("model identity = %#v", result.Models[0])
	}
}
