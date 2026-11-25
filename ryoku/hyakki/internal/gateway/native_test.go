package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/gateway/textcalls"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

type managedForTest struct {
	core.Local
	url       string
	ensure    atomic.Int32
	leases    atomic.Int32
	inventory []protocol.LocalModel
}

func (m *managedForTest) Inventory() []protocol.LocalModel { return m.inventory }
func (m *managedForTest) Ensure(_ context.Context, id string) (string, string, error) {
	m.ensure.Add(1)
	if id != "lfm3-omni-2b" {
		return "", "", fmt.Errorf("unexpected model %s", id)
	}
	return m.url, "test-key", nil
}
func (m *managedForTest) Acquire() func() {
	m.leases.Add(1)
	return func() { m.leases.Add(-1) }
}
func (m *managedForTest) Models(context.Context) protocol.LocalResult {
	panic("provider listing must not inspect the managed runtime")
}
func (m *managedForTest) RuntimeState() (bool, string) { return false, "" }

type managedDetailedForTest struct {
	managedForTest
	result protocol.LocalResult
}

func (m *managedDetailedForTest) Models(context.Context) protocol.LocalResult { return m.result }
func (m *managedDetailedForTest) Ensure(ctx context.Context, id string) (string, string, error) {
	if m.result.Runtime.State == protocol.RuntimeMissing {
		return "", "", errors.New("llama.cpp is not on this computer. Install or update llama.cpp in Settings > Providers > Local models.")
	}
	for _, model := range m.result.Models {
		if model.ID == id && model.Error != "" {
			return "", "", errors.New(model.Error + ". Open the llama.cpp log in Settings > Providers > Local models.")
		}
	}
	return m.managedForTest.Ensure(ctx, id)
}

func TestPinnedManagedModelExplainsWhyItCannotAnswer(t *testing.T) {
	for _, test := range []struct {
		name    string
		model   protocol.LocalModel
		runtime protocol.LocalRuntime
		want    string
	}{
		{"no tools", protocol.LocalModel{ID: "tiny", Name: "Supra 65M", State: protocol.LocalInstalled, Context: 512, Params: 65_000_000}, protocol.LocalRuntime{}, "no tool calling"},
		{"short context", protocol.LocalModel{ID: "tiny", Name: "Short model", State: protocol.LocalInstalled, Context: 512, Params: 2e9, Tools: true}, protocol.LocalRuntime{}, "512 context tokens"},
		{"too few parameters", protocol.LocalModel{ID: "tiny", Name: "Small model", State: protocol.LocalInstalled, Context: 8192, Params: 65_000_000, Tools: true}, protocol.LocalRuntime{}, "0.5B"},
		{"not downloaded", protocol.LocalModel{ID: "tiny", Name: "Not here", State: protocol.LocalAvailable}, protocol.LocalRuntime{}, "Download it"},
		{"missing runtime", protocol.LocalModel{ID: "tiny", Name: "Needs llama", State: protocol.LocalInstalled, Context: 8192, Params: 2e9, Tools: true}, protocol.LocalRuntime{State: protocol.RuntimeMissing, Error: "llama-server missing"}, "Install or update"},
		{"failed to load", protocol.LocalModel{ID: "tiny", Name: "Bad weights", State: protocol.LocalInstalled, Context: 8192, Params: 2e9, Tools: true, Error: "failed to load: unsupported tensor"}, protocol.LocalRuntime{}, "unsupported tensor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := &managedDetailedForTest{result: protocol.LocalResult{Models: []protocol.LocalModel{test.model}, Runtime: test.runtime}}
			if test.model.State == protocol.LocalInstalled {
				manager.inventory = []protocol.LocalModel{test.model}
			}
			g := newTestGateway(t)
			g.SetLocal(manager)
			req := question()
			req.Model, req.Strict = "rashin-models/tiny", true
			stream, err := g.Stream(context.Background(), req)
			if err == nil {
				_, err = stream.Next()
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("pinned model returned %v, want %q", err, test.want)
			}
		})
	}
}

func TestInstalledManagedModelRoutesWithoutStartingOnListing(t *testing.T) {
	var served atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("managed model missing bearer token")
		}
		if r.URL.Path == "/props" {
			if r.URL.Query().Get("model") != "lfm3-omni-2b" || r.URL.Query().Get("autoload") != "false" {
				t.Errorf("managed props query = %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"default_generation_settings":{"n_ctx":12288}}`)
			return
		}
		var req struct {
			Model string `json:"model"`
			Tools []any  `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Model != "lfm3-omni-2b" || len(req.Tools) == 0 {
			t.Errorf("managed model request = %#v: %v", req, err)
		}
		sse(w, `{"choices":[{"delta":{"content":"local"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer server.Close()
	manager := &managedForTest{url: server.URL, inventory: []protocol.LocalModel{{ID: "lfm3-omni-2b", Name: "LFM3 Omni 2B", State: protocol.LocalInstalled, Class: protocol.LocalClassLight, Params: 2e9, Context: 16384, Tools: true, Vision: true, Audio: true}}}
	g := newTestGateway(t)
	g.SetLocal(manager)
	remote := testProvider("free", "http://127.0.0.1:1", 100, true)
	setTestProviders(g, remote)
	listed, err := g.Models(context.Background(), protocol.ModelsParams{})
	if err != nil || manager.ensure.Load() != 0 || len(listed.Models) != 2 {
		t.Fatalf("listing started server or missed models: %#v, %v", listed, err)
	}
	managed := listed.Models[0]
	if managed.ID != "rashin-models/lfm3-omni-2b" || !managed.Tools || !managed.Vision || !managed.Audio || !managed.Light || managed.Context != 16384 || managed.Params != 2e9 {
		t.Fatalf("managed capabilities = %#v", managed)
	}
	for _, route := range []string{protocol.RouteAuto, protocol.RouteFast, protocol.RouteSmart} {
		request := question()
		request.Model = route
		text, final := answer(t, g, request)
		if text != "local" || final.Model != managed.ID || final.Context != 12288 {
			t.Fatalf("%s returned %q %#v", route, text, final)
		}
	}
	if served.Load() != 6 || manager.ensure.Load() != 3 || manager.leases.Load() != 0 {
		t.Fatalf("requests %d, starts %d, open leases %d", served.Load(), manager.ensure.Load(), manager.leases.Load())
	}
}

// A managed model advertises its training window, but a constrained machine
// may load it with less. Planning, eligibility, and final feedback must use
// the router's allocation rather than that advertised value.
func TestManagedWindowFollowsLoadedAllocationAndRoutesAroundSmallContexts(t *testing.T) {
	var allocation atomic.Int32
	allocation.Store(12288)
	var localChats atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("runtime request has no bearer token")
		}
		switch r.URL.Path {
		case "/props":
			if r.URL.Query().Get("model") != "lfm3-omni-2b" || r.URL.Query().Get("autoload") != "false" {
				t.Errorf("props queried %s", r.URL.RawQuery)
			}
			_, _ = fmt.Fprintf(w, `{"default_generation_settings":{"n_ctx":%d}}`, allocation.Load())
		case "/v1/chat/completions":
			localChats.Add(1)
			sse(w, `{"choices":[{"delta":{"content":"local"},"finish_reason":"stop"}]}`, `[DONE]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"remote"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer remote.Close()
	manager := &managedForTest{url: server.URL + "/v1", inventory: []protocol.LocalModel{{
		ID: "lfm3-omni-2b", Name: "LFM3 Omni 2B", State: protocol.LocalInstalled, Params: 2e9, Context: 32768, Tools: true,
	}}}
	g := newTestGateway(t)
	g.SetLocal(manager)
	setTestProviders(g, testProvider("free", remote.URL, 90, true))
	req := question()
	if window := g.Window(context.Background(), req); window != 12288 {
		t.Fatalf("planned window %d, want loaded 12288", window)
	}
	if text, final := answer(t, g, req); text != "local" || final.Context != 12288 {
		t.Fatalf("local response %q with context %d", text, final.Context)
	}
	allocation.Store(4096)
	if window := g.Window(context.Background(), req); window != 128000 {
		t.Fatalf("fallback planned window %d, want remote 128000", window)
	}
	if text, final := answer(t, g, req); text != "remote" || final.Context != 128000 || localChats.Load() != 1 {
		t.Fatalf("short local context routed %q with context %d, local chats %d", text, final.Context, localChats.Load())
	}
	req.Model, req.Strict = "rashin-models/lfm3-omni-2b", true
	if window := g.Window(context.Background(), req); window != 0 {
		t.Fatalf("strict short-context window %d, want 0", window)
	}
	stream, err := g.Stream(context.Background(), req)
	if err == nil {
		_, err = stream.Next()
	}
	if err == nil || !strings.Contains(err.Error(), "4096 context window tokens") {
		t.Fatalf("strict short-context route: %v", err)
	}
	allocation.Store(0)
	if window := g.Window(context.Background(), question()); window != 128000 {
		t.Fatalf("unverified local context planned %d, want remote 128000", window)
	}
	if text, _ := answer(t, g, question()); text != "remote" || localChats.Load() != 1 {
		t.Fatalf("unverified local context routed %q, local chats %d", text, localChats.Load())
	}
	noTools := question()
	noTools.Tools = nil
	noTools.Model, noTools.Strict = "rashin-models/lfm3-omni-2b", true
	if window := g.Window(context.Background(), noTools); window != 0 {
		t.Fatalf("unknown loaded context planned %d, want 0", window)
	}
	if text, final := answer(t, g, noTools); text != "local" || final.Context != 0 {
		t.Fatalf("unknown loaded context returned %q with claimed context %d", text, final.Context)
	}
}

func TestRouteOrderLocalRemoteAndLight(t *testing.T) {
	makeCandidate := func(id string, tier string, params int64, speed int, place int) routeCandidate {
		return routeCandidate{provider: &provider{ID: id, Tier: tier}, model: &model{ID: "chat", Params: params, SpeedRank: speed, Light: params > 0 && params <= protocol.LightParams}, place: place}
	}
	managed := makeCandidate("rashin-models", protocol.TierLocal, 2e9, 0, 0)
	localLarge := makeCandidate("llamacpp", protocol.TierLocal, 8e9, 0, 1)
	localSmall := makeCandidate("ollama-local", protocol.TierLocal, 1e9, 0, 2)
	remote := makeCandidate("free", protocol.TierFree, 0, 100, 3)
	for _, route := range []string{protocol.RouteAuto, protocol.RouteFast, protocol.RouteSmart} {
		c := []routeCandidate{remote, localSmall, localLarge, managed}
		sortCandidates(c, routeOrder{route: route})
		if c[0].provider.ID != "rashin-models" {
			t.Fatalf("%s order puts %s first", route, c[0].provider.ID)
		}
		sortCandidates(c, routeOrder{route: route, remoteFirst: true})
		if c[0].provider.ID != "free" {
			t.Fatalf("remoteFirst %s puts %s first", route, c[0].provider.ID)
		}
	}
	c := []routeCandidate{remote, localLarge, managed, localSmall}
	sortCandidates(c, routeOrder{light: true, lightModel: "rashin-models/chat"})
	if c[0].provider.ID != "rashin-models" || c[1].provider.ID != "ollama-local" || c[2].provider.ID != "llamacpp" || c[3].provider.ID != "free" {
		t.Fatalf("light order = %s, %s, %s, %s", c[0].provider.ID, c[1].provider.ID, c[2].provider.ID, c[3].provider.ID)
	}
}

func TestProbeNeedsAnActualToolCall(t *testing.T) {
	var callTool atomic.Bool
	callTool.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ToolChoice any    `json:"tool_choice"`
			Reasoning  string `json:"reasoning_effort"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ToolChoice != "required" || body.Reasoning != "high" {
			t.Errorf("probe request = %#v (%v)", body, err)
		}
		if callTool.Load() {
			sse(w, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call1","function":{"name":"noop","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`, `[DONE]`)
		} else {
			sse(w, `{"choices":[{"delta":{"content":"OK"},"finish_reason":"stop"}]}`, `[DONE]`)
		}
	}))
	defer server.Close()
	g := newTestGateway(t)
	p := testProvider("free", server.URL, 90, true)
	p.Models[0].Reasoning = true
	setTestProviders(g, p)
	request := protocol.ModelProbeParams{Model: "free/chat", Options: map[string]string{protocol.OptionEffort: llm.EffortHigh}}
	result, err := g.Probe(context.Background(), request)
	if err != nil || !result.OK || result.Model.ID != "free/chat" || result.Model.Name != "chat" || len(result.Options) != 1 || result.Options[0].Current != "high" {
		t.Fatalf("probe = %#v, %v", result, err)
	}
	callTool.Store(false)
	result, err = g.Probe(context.Background(), request)
	if err != nil || result.OK || !strings.Contains(result.Message, "did not call") {
		t.Fatalf("text-only probe = %#v, %v", result, err)
	}
}

func TestLocalProbeParsersMatchServerResponseShapes(t *testing.T) {
	var listing llamaListing
	if err := json.Unmarshal([]byte(`{"data":[{"id":"lfm3-omni-2b","architecture":{"input_modalities":["text","image","audio","video"]},"meta":{"n_ctx":16384,"n_ctx_train":32768,"n_params":2000000000}}]}`), &listing); err != nil {
		t.Fatal(err)
	}
	m := llamaModel(listing.Data[0])
	if !m.Tools || !m.Vision || !m.Audio || m.Context != 16384 || m.Params != 2e9 {
		t.Fatalf("llama model = %#v", m)
	}
	var props llamaProps
	if err := json.Unmarshal([]byte(`{"default_generation_settings":{"n_ctx":12288},"modalities":{"vision":true,"audio":true},"chat_template_caps":{"supports_tools":true,"supports_tool_calls":true}}`), &props); err != nil {
		t.Fatal(err)
	}
	applyProps(&m, props, true)
	if !m.Tools || !m.Vision || m.Context != 12288 {
		t.Fatalf("llama props = %#v", m)
	}
	for _, test := range []struct {
		response string
		window   int
	}{
		{`{"default_generation_settings":{"n_ctx":12288},"n_ctx":8192}`, 12288},
		{`{"n_ctx":8192}`, 8192},
		{`{"model_info":{"n_ctx":4096}}`, 4096},
		{`{"default_generation_settings":{"n_ctx":0}}`, 0},
	} {
		var response llamaProps
		if err := json.Unmarshal([]byte(test.response), &response); err != nil {
			t.Fatal(err)
		}
		if got := response.contextWindow(); got != test.window {
			t.Fatalf("props %s reported %d, want %d", test.response, got, test.window)
		}
	}
	var show ollamaShow
	if err := json.Unmarshal([]byte(`{"capabilities":["completion","vision","tools"],"details":{"parameter_size":"2B"},"model_info":{"general.architecture":"lfm","lfm.context_length":32768}}`), &show); err != nil {
		t.Fatal(err)
	}
	applyOllama(&m, show, 8192)
	if !m.Vision || !m.Tools || m.Context != 8192 || m.Params != 2e9 {
		t.Fatalf("ollama show = %#v", m)
	}
}

func TestUnloadedLlamaRouterListsWithoutLoading(t *testing.T) {
	var props atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"small","status":{"value":"unloaded"},"architecture":{"input_modalities":["text","image"]}}]}`))
		case "/props":
			props.Add(1)
			http.Error(w, "the listing loaded a model", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	g := newTestGateway(t)
	running, models, _ := g.localModels(context.Background(), protocol.ProviderLlamaCpp, server.URL+"/v1", true)
	if !running || len(models) != 1 || !models[0].Vision || props.Load() != 0 {
		t.Fatalf("router listing = %#v, running %v, props called %d times", models, running, props.Load())
	}
}

func TestExplicitDialectScansHostedTextToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `{"choices":[{"delta":{"content":"Checking <tool_ca"}}]}`, `{"choices":[{"delta":{"content":"ll><function=search><parameter=q>hello</parameter></function></tool_call>"},"finish_reason":"stop"}]}`, `[DONE]`)
	}))
	defer server.Close()
	g := newTestGateway(t)
	p := testProvider("free", server.URL, 90, true)
	p.Models[0].ToolDialect = textcalls.XMLFn
	setTestProviders(g, p)
	request := question()
	stream, err := g.Stream(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	deltas, err := collect(t, stream)
	if err != nil {
		t.Fatal(err)
	}
	var text string
	var calls []llm.ToolCall
	for _, delta := range deltas {
		text += delta.Text
		calls = append(calls, delta.ToolCalls...)
	}
	if strings.Contains(text, "<tool_call>") || len(calls) != 1 || calls[0].Name != "search" || !strings.Contains(calls[0].Arguments, "hello") || stream.Final().Finish != llm.FinishToolCalls {
		t.Fatalf("leak: text=%q calls=%#v final=%#v", text, calls, stream.Final())
	}
}

func TestUpstreamErrorKeepsStatusAndContextCauseBehindSafeRouteText(t *testing.T) {
	upstream := &upstreamError{status: http.StatusBadRequest, body: `{"error":"context length exceeded"}`}
	wrapped := &routedFailure{message: &llm.NoRoute{Reason: "safe route summary"}, cause: upstream}
	var contextError interface {
		StatusCode() int
		ContextTooLong() bool
	}
	if !errors.As(wrapped, &contextError) || contextError.StatusCode() != http.StatusBadRequest || !contextError.ContextTooLong() {
		t.Fatalf("upstream context status was lost: %v", wrapped)
	}
	var route *llm.NoRoute
	if !errors.As(wrapped, &route) || wrapped.Error() != "safe route summary" {
		t.Fatalf("safe route error was lost: %v", wrapped)
	}
}
