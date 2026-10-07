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

// pickBackend is the chat-agent selection core. A wrong choice would either
// silently ignore the user's pick or drive the chat with an adapter that is not
// installed. Availability is injected so the test needs no real adapters.
func TestPickBackend(t *testing.T) {
	backs := []chatBackend{
		{ID: "hermes", Name: "Hermes", Recommended: true},
		{ID: "claude", Name: "Claude Code"},
	}
	only := func(ids ...string) func(chatBackend) bool {
		set := map[string]bool{}
		for _, id := range ids {
			set[id] = true
		}
		return func(b chatBackend) bool { return set[b.ID] }
	}
	cases := []struct {
		name   string
		want   string
		avail  func(chatBackend) bool
		expect string
		ok     bool
	}{
		{"default falls to hermes", "", only("hermes", "claude"), "hermes", true},
		{"explicit available pick wins", "claude", only("hermes", "claude"), "claude", true},
		{"unavailable pick falls back to hermes", "claude", only("hermes"), "hermes", true},
		{"retired gemini pick falls back to hermes", "gemini", only("hermes"), "hermes", true},
		{"nothing available", "claude", only(), "", false},
		{"default with only a non-hermes agent", "", only("claude"), "claude", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, ok := pickBackend(c.want, backs, c.avail)
			if ok != c.ok || b.ID != c.expect {
				t.Fatalf("pickBackend(%q) = (%q,%v), want (%q,%v)", c.want, b.ID, ok, c.expect, c.ok)
			}
		})
	}
	for _, backend := range chatBackends() {
		if backend.ID == "gemini" {
			t.Fatal("Gemini must not be offered as a chat backend")
		}
	}
}

func TestSetChatAgentConnectsAndPersistsPendingChoice(t *testing.T) {
	home := quickTestEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".hermes"), 0o755); err != nil {
		t.Fatal(err)
	}
	activated := map[string]bool{}
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/setup/harnesses":
			fmt.Fprint(w, `{"routable":false,"reason":"no provider","harnesses":[{"id":"claude","skills":"current"},{"id":"hermes","skills":"current"}]}`)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/setup/harnesses/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/setup/harnesses/")
			var body map[string]bool
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["activate"] {
				activated[id] = true
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{"error":{"type":"conflict","code":"not_routable","message":"no provider"}}`)
				return
			}
			fmt.Fprintf(w, `{"harness":{"id":%q,"injected":true,"skills":"current"}}`, id)
		case r.Method == http.MethodPost && r.URL.Path == "/api/setup/skills":
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	})

	if _, err := setChatAgent(context.Background(), "bogus"); err == nil {
		t.Fatal("unknown chat agent must error")
	}
	if _, err := setChatAgent(context.Background(), "gemini"); err == nil {
		t.Fatal("Gemini must not be accepted as a chat agent")
	}
	routing, err := setChatAgent(context.Background(), "claude")
	if err != nil {
		t.Fatalf("valid agent: %v", err)
	}
	if !routing.Pending || routing.Active || routing.Reason != "no provider" || !activated["claude"] {
		t.Fatalf("claude routing = %+v, activated=%v", routing, activated["claude"])
	}
	if cfg := LoadConfig(); cfg.ChatAgent != "claude" || !cfg.HasProwlHarness("claude") {
		t.Fatalf("config after selecting claude = %+v", cfg)
	}
	routing, err = setChatAgent(context.Background(), "auto")
	if err != nil {
		t.Fatalf("auto: %v", err)
	}
	if !routing.Pending || !activated["hermes"] {
		t.Fatalf("Hermes routing = %+v, activated=%v", routing, activated["hermes"])
	}
	if cfg := LoadConfig(); cfg.ChatAgent != "" || !cfg.HasProwlHarness("claude") || !cfg.HasProwlHarness("hermes") {
		t.Fatalf("config after selecting auto = %+v", cfg)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/chat/agent?id=claude", nil)
	response := httptest.NewRecorder()
	handleSetChatAgent(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var result chatAgentSelection
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.Routing.Pending || result.Routing.Active || result.Routing.Reason != "no provider" {
		t.Fatalf("response routing = %+v", result.Routing)
	}
}

func TestSetChatAgentDoesNotSaveGatewayFailure(t *testing.T) {
	home := quickTestEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/setup/harnesses":
			fmt.Fprint(w, `{"routable":true,"harnesses":[{"id":"claude","skills":"current"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/setup/harnesses/claude":
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprint(w, `{"error":{"type":"gateway","code":"upstream_failed","message":"activation failed"}}`)
		default:
			http.NotFound(w, r)
		}
	})
	if _, err := setChatAgent(context.Background(), "claude"); err == nil {
		t.Fatal("gateway failure must fail the selection")
	}
	if got := LoadConfig().ChatAgent; got != "" {
		t.Fatalf("chatAgent = %q after failed activation", got)
	}
	if LoadConfig().HasProwlHarness("claude") {
		t.Fatal("failed activation recorded the harness as connected")
	}
}

func TestChatModelRouted(t *testing.T) {
	cases := []struct {
		name      string
		agent     string
		active    bool
		model     ModelInfo
		startedOn string
		want      bool
	}{
		{"Claude routes every model", "claude", true, ModelInfo{ID: "anthropic:opus"}, "", true},
		{"Prowl provider id", "hermes", true, ModelInfo{ID: "prowl:auto"}, "", true},
		{"custom Prowl provider id", "omp", true, ModelInfo{ID: "custom:prowl:auto"}, "", true},
		{"Prowl slash id", "opencode", true, ModelInfo{ID: "prowl/auto"}, "", true},
		{"Prowl provider description", "hermes", true, ModelInfo{ID: "custom:auto", Description: "Provider: PrOwL • current"}, "", true},
		{"session starting model", "omp", true, ModelInfo{ID: "custom:auto"}, "custom:auto", true},
		{"other Hermes provider", "hermes", true, ModelInfo{ID: "openai-codex:gpt-5.6"}, "custom:auto", false},
		{"other OMP provider", "omp", true, ModelInfo{ID: "openai:gpt-5.6"}, "prowl:auto", false},
		{"other opencode provider", "opencode", true, ModelInfo{ID: "anthropic/opus"}, "prowl:auto", false},
		{"pending route stays available", "hermes", false, ModelInfo{ID: "openai-codex:gpt-5.6"}, "custom:auto", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := chatModelRouted(test.agent, test.active, test.model, test.startedOn); got != test.want {
				t.Fatalf("chatModelRouted() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestUnwireActiveChatAgentIsConflict(t *testing.T) {
	quickTestEnv(t)
	cfg := LoadConfig()
	cfg.ChatAgent = "claude"
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/agents/unwire", strings.NewReader(`{"id":"claude"}`))
	response := httptest.NewRecorder()
	handleUnwireAgent(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "pick another Rashin chat agent") {
		t.Fatalf("response = %q", response.Body.String())
	}
}

// The paste snippet is the whole point of "point any agent at Ryoku": it must
// name the vault, prowl, and the skill so an unsupported agent gets the
// same power. A regression that drops one silently weakens every such agent.
func TestManifestSnippet(t *testing.T) {
	h := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(h, "data"))
	skill := filepath.Join(h, "skills", "ryoku")
	snip := manifestSnippet(skill)
	for _, want := range []string{"prowl", "AGENTS.md", filepath.Join(skill, "SKILL.md"), "ryoku-rashin"} {
		if !strings.Contains(snip, want) {
			t.Fatalf("snippet missing %q:\n%s", want, snip)
		}
	}
}

// vaultManifest lists the read-order docs; the index (AGENTS.md) must lead so an
// agent reads the map before anything else.
func TestVaultManifestOrder(t *testing.T) {
	items := vaultManifest()
	if len(items) == 0 || items[0].Label != "AGENTS.md" {
		t.Fatalf("vault manifest must lead with AGENTS.md, got %+v", items)
	}
	var haveDesktop bool
	for _, it := range items {
		if it.Label == "desktop.md" {
			haveDesktop = true
		}
	}
	if !haveDesktop {
		t.Fatal("vault manifest missing desktop.md")
	}
}
