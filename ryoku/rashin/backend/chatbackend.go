package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// chatbackend.go lets the Super+S chat run an agent other than Hermes for its
// interactive ACP session. Hermes is first-class and the recommended default;
// any other agent needs its own Agent Client Protocol adapter on PATH. A coding
// CLI with no ACP adapter is not a chat backend -- it is wired for terminal use
// instead (see agents.go), which is the path most agents take.

// chatBackend is an agent that can drive the chat's ACP session.
type chatBackend struct {
	ID          string
	Name        string
	Argv        []string // the ACP command to spawn
	Recommended bool
}

type chatAgentRouting struct {
	Active  bool   `json:"active"`
	Pending bool   `json:"pending"`
	Reason  string `json:"reason"`
}

type chatAgentSelection struct {
	Agents  []ChatBackendInfo `json:"agents"`
	Routing chatAgentRouting  `json:"routing"`
}

// chatBackends lists the agents Rashin can run as the chat's ACP session, in
// preference order (Hermes leads). Every backend here must route its models
// through Prowl.
func chatBackends() []chatBackend {
	return []chatBackend{
		{ID: "hermes", Name: "Hermes", Argv: hermesACPArgv(), Recommended: true},
		{ID: "omp", Name: "Oh My Pi", Argv: []string{"omp", "acp"}},
		{ID: "opencode", Name: "opencode", Argv: []string{"opencode", "acp"}},
		{ID: "claude", Name: "Claude Code", Argv: []string{"claude-code-acp"}},
	}
}

func hermesACPArgv() []string {
	if bin, ok := FindHermes(); ok {
		return []string{bin, "acp"}
	}
	return []string{"hermes", "acp"}
}

// chatBackendAvailable reports whether a backend's ACP command is runnable now.
func chatBackendAvailable(b chatBackend) bool {
	if len(b.Argv) == 0 {
		return false
	}
	if b.ID == "hermes" {
		_, ok := FindHermes()
		return ok
	}
	_, err := exec.LookPath(b.Argv[0])
	return err == nil
}

// resolveChatBackend picks the ACP command to spawn for the given config.
func resolveChatBackend(cfg Config) (chatBackend, bool) {
	return pickBackend(cfg.ChatAgent, chatBackends(), chatBackendAvailable)
}

// pickBackend is the pure selection core: the wanted agent when available, else
// the first available (Hermes leads), else none. Availability is injected so it
// is unit-testable without real adapters on PATH.
func pickBackend(want string, backs []chatBackend, avail func(chatBackend) bool) (chatBackend, bool) {
	if want != "" {
		for _, b := range backs {
			if b.ID == want && avail(b) {
				return b, true
			}
		}
	}
	for _, b := range backs {
		if avail(b) {
			return b, true
		}
	}
	return chatBackend{}, false
}

// lookupChatBackend finds a backend by id.
func lookupChatBackend(id string) (chatBackend, bool) {
	for _, b := range chatBackends() {
		if b.ID == id {
			return b, true
		}
	}
	return chatBackend{}, false
}

// setChatAgent connects the requested backend to Prowl before persisting it.
// "" and "auto" both select the recommended backend, Hermes.
func setChatAgent(ctx context.Context, id string) (chatAgentRouting, error) {
	id = strings.TrimSpace(id)
	selected := id
	if id == "" || id == "auto" {
		id = ""
		selected = "hermes"
	}
	if _, ok := lookupChatBackend(selected); !ok {
		return chatAgentRouting{}, fmt.Errorf("unknown chat agent %q", selected)
	}
	pending, reason, err := connectHarness(ctx, selected)
	if err != nil {
		return chatAgentRouting{}, err
	}
	cfg := LoadConfig()
	cfg.ChatAgent = id
	if err := SaveConfig(cfg); err != nil {
		return chatAgentRouting{}, err
	}
	return chatAgentRouting{Active: !pending, Pending: pending, Reason: reason}, nil
}

func activeChatAgentID(cfg Config) string {
	if backend, ok := resolveChatBackend(cfg); ok {
		return backend.ID
	}
	if cfg.ChatAgent != "" && cfg.ChatAgent != "auto" {
		return cfg.ChatAgent
	}
	return "hermes"
}

func chatAgentInUse(id string, cfg Config) bool {
	selected := cfg.ChatAgent
	if selected == "" || selected == "auto" {
		selected = "hermes"
	}
	return id == selected || id == activeChatAgentID(cfg)
}

// chatModelProwlAlias identifies the duplicate auto entries a routed provider
// advertises without treating the harness's direct models as Prowl routes.
func chatModelProwlAlias(active bool, model ModelInfo, startedOn string) bool {
	id := strings.TrimSpace(model.ID)
	segment := id
	if cut := strings.LastIndexAny(segment, ":/"); cut >= 0 {
		segment = segment[cut+1:]
	}
	if !strings.EqualFold(segment, "auto") {
		return false
	}
	for _, prefix := range []string{"prowl:", "custom:prowl:", "prowl/"} {
		if len(id) >= len(prefix) && strings.EqualFold(id[:len(prefix)], prefix) {
			return true
		}
	}
	const provider = "provider: prowl"
	for i := 0; i+len(provider) <= len(model.Description); i++ {
		if strings.EqualFold(model.Description[i:i+len(provider)], provider) {
			return true
		}
	}
	return active && id == strings.TrimSpace(startedOn)
}
