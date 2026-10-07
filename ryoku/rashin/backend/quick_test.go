package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func quickTestEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("RYOKU_CONFIG_BASE", filepath.Join(home, "packaged-config-absent"))
	t.Setenv("RYOKU_PROWL_PORT", "1")
	t.Setenv("RYOKU_RASHIN_VAULT", filepath.Join(home, "vault"))
	return home
}

func quickGateway(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("RYOKU_PROWL_PORT", strings.TrimPrefix(srv.URL, "http://127.0.0.1:"))
	tokenPath := prowlGatewayTokenPath()
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte("machine-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return srv
}

func writeSSE(w http.ResponseWriter, deltas ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, delta := range deltas {
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", delta)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func TestQuickCompleteUsesGatewayRouteAndBearer(t *testing.T) {
	quickTestEnv(t)
	var gotModel, gotAuth, gotPath string
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/setup/harnesses":
			fmt.Fprint(w, `{"routable":true,"harnesses":[]}`)
		case "/v1/chat/completions":
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			var body struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			gotModel = body.Model
			writeSSE(w, "Routed answer.")
		default:
			http.NotFound(w, r)
		}
	})
	cfg := defaultConfig()
	cfg.Quick.Route = "auto:fast"
	target, err := resolveQuickTarget(cfg)
	if err != nil {
		t.Fatal(err)
	}
	answer, err := quickComplete(context.Background(), target, "hello", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if answer != "Routed answer." {
		t.Fatalf("answer = %q", answer)
	}
	if gotPath != "/v1/chat/completions" || gotModel != "auto:fast" {
		t.Fatalf("path = %q model = %q", gotPath, gotModel)
	}
	if gotAuth != "Bearer machine-token" {
		t.Fatalf("authorization = %q", gotAuth)
	}
}

func TestResolveQuickTargetReportsUnroutableGateway(t *testing.T) {
	quickTestEnv(t)
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"routable":false,"reason":"no models","harnesses":[]}`)
	})
	_, err := resolveQuickTarget(defaultConfig())
	if err == nil || err.Error() != "Prowl has no provider connected; open Prowl > Providers in Rashin" {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateQuickRoute(t *testing.T) {
	quickTestEnv(t)
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/profiles":
			fmt.Fprint(w, `[{"id":7,"name":"Work","modelCount":2},{"id":8,"name":"Personal","modelCount":1},{"id":9,"name":"Empty","modelCount":0}]`)
		case "/api/profiles/active":
			fmt.Fprint(w, `{"activeProfileId":7}`)
		default:
			http.NotFound(w, r)
		}
	})
	for _, route := range []string{"auto", "AUTO:SMART", "auto:work"} {
		got, err := validateQuickRoute(context.Background(), route)
		if err != nil {
			t.Fatalf("%s: %v", route, err)
		}
		if got != strings.ToLower(route) {
			t.Fatalf("%s normalized to %q", route, got)
		}
	}
	routes, err := quickRoutes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 9 || routes[0] != (quickRouteJSON{ID: "auto", Label: "Active set", Sub: "Work"}) ||
		routes[7] != (quickRouteJSON{ID: "auto:personal", Label: "Personal", Sub: "Routing set"}) ||
		routes[8] != (quickRouteJSON{ID: "auto:work", Label: "Work", Sub: "Routing set"}) {
		t.Fatalf("routes = %+v", routes)
	}
	if _, err := validateQuickRoute(context.Background(), "auto:missing"); err == nil {
		t.Fatal("missing routing set must be rejected")
	}
}

func TestLoadConfigMigratesLegacyQuickShape(t *testing.T) {
	quickTestEnv(t)
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"enabled":true,"port":3600,"quick":{"provider":"deepseek","model":"deepseek-chat","baseUrl":"https://api.deepseek.com/v1","keyEnv":"DEEPSEEK_API_KEY"}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadConfig()
	if cfg.Quick.Route != "auto" {
		t.Fatalf("route = %q", cfg.Quick.Route)
	}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"provider", "model", "baseUrl", "keyEnv"} {
		if strings.Contains(string(body), `"`+retired+`"`) {
			t.Fatalf("saved config retained %q: %s", retired, body)
		}
	}
	if !strings.Contains(string(body), `"route": "auto"`) {
		t.Fatalf("saved config missing route: %s", body)
	}
}

func TestQuickCompleteStreamsAnswer(t *testing.T) {
	quickTestEnv(t)
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, "The kernel ", "is 7.0.12.")
	})
	var streamed strings.Builder
	got, err := quickComplete(context.Background(),
		quickTarget{Route: "auto"}, "kernel?",
		func(delta string) { streamed.WriteString(delta) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "The kernel is 7.0.12." || streamed.String() != got {
		t.Fatalf("answer %q streamed %q", got, streamed.String())
	}
}

func TestQuickCompleteSentinelEscalates(t *testing.T) {
	quickTestEnv(t)
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		writeSSE(w, "TOOLS_", "REQUIRED")
	})
	streamedAny := false
	_, err := quickComplete(context.Background(),
		quickTarget{Route: "auto"}, "summarize this video",
		func(string) { streamedAny = true }, nil)
	if err != errNeedsTools {
		t.Fatalf("err = %v, want errNeedsTools", err)
	}
	if streamedAny {
		t.Fatal("sentinel must never stream to the transcript")
	}
}

func TestQuickCompleteToolLoop(t *testing.T) {
	quickTestEnv(t)
	var calls int
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 0 {
			calls++
			fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"system_query","arguments":"{\"topic\":\"kernel\"}"}}]}}]}`+"\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		writeSSE(w, "Kernel is current.")
	})
	var tools []string
	got, err := quickComplete(context.Background(),
		quickTarget{Route: "auto"}, "what kernel?", nil,
		func(_ string, title string, status string) {
			if status == "in_progress" {
				tools = append(tools, title)
			}
		})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Kernel is current." {
		t.Fatalf("answer %q", got)
	}
	if len(tools) != 1 || !strings.Contains(tools[0], "kernel") {
		t.Fatalf("tool events %v", tools)
	}
}

func TestMigrateQuickProviderKeysOnce(t *testing.T) {
	quickTestEnv(t)
	envPath := filepath.Join(filepath.Dir(ConfigPath()), "rashin.env")
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envPath, []byte("GROQ_API_KEY=groq-secret\nOPENAI_API_KEY=skip-me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var posts int
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/keys":
			fmt.Fprint(w, `[]`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/keys":
			posts++
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["platform"] != "groq" || body["key"] != "groq-secret" || body["label"] != "imported from rashin" {
				t.Fatalf("body = %#v", body)
			}
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	})
	if err := migrateQuickProviderKeys(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := migrateQuickProviderKeys(context.Background()); err != nil {
		t.Fatal(err)
	}
	if posts != 1 {
		t.Fatalf("posts = %d, want one", posts)
	}
}
