package demo

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// one model server running on this computer and one that is not started, as a reader first finds them
func demoLocalProviders(settings protocol.ProvidersSettings) []protocol.ProviderInfo {
	return []protocol.ProviderInfo{
		{
			ID: protocol.ProviderOllamaLocal, Name: "Ollama", Tier: protocol.TierLocal, Keyless: true, Enabled: true, Configured: true,
			Models: 3, Available: 3, Signup: "https://ollama.com/download", Note: i18n.T("Install Ollama and run ollama serve."),
			Local: true, Address: settings.Local[protocol.ProviderOllamaLocal], Running: true,
		},
		{
			ID: protocol.ProviderLlamaCpp, Name: "llama.cpp", Tier: protocol.TierLocal, Keyless: true, Enabled: true, Configured: true,
			Signup: "https://github.com/ggml-org/llama.cpp", Note: i18n.T("Run llama-server -m with your model file, such as model.gguf."),
			Local: true, Address: settings.Local[protocol.ProviderLlamaCpp],
		},
	}
}

// a paid provider the reader added a key for
func demoAnthropic() protocol.ProviderInfo {
	return protocol.ProviderInfo{
		ID: "anthropic", Name: "Anthropic", Tier: protocol.TierPaid, Enabled: true, Configured: true, Models: 4, Available: 4,
		Signup: "https://platform.claude.com/settings/keys", KeyHint: "sk-ant-...",
		Keys: []protocol.KeyInfo{{ID: "anthropic-demo", Label: i18n.T("Main"), Status: protocol.KeyHealthy, Masked: "sk-ant-••••7c2e"}},
	}
}

// a paid provider with no key yet: its models show, and cannot be picked
func demoOpenAI() protocol.ProviderInfo {
	return protocol.ProviderInfo{
		ID: "openai", Name: "OpenAI", Tier: protocol.TierPaid, Enabled: true, Models: 3,
		Signup: "https://platform.openai.com/api-keys", KeyHint: "sk-proj-...",
	}
}

// what models.list answers in the demo: free and credits rows, the running local server's models,
// a paid provider with a key and one without, and what OpenRouter's and NVIDIA's own listings add
func demoAskModels() []protocol.ModelInfo {
	model := func(provider, id, name, tier string, available bool) protocol.ModelInfo {
		return protocol.ModelInfo{ID: provider + "/" + id, Provider: provider, Name: name, Tier: tier, Tools: true, ToolsKnown: true, Available: available}
	}
	models := []protocol.ModelInfo{
		model(protocol.ProviderOllamaLocal, "llama3.2", "llama3.2", protocol.TierLocal, true),
		model(protocol.ProviderOllamaLocal, "qwen3:14b", "qwen3:14b", protocol.TierLocal, true),
		model(protocol.ProviderOllamaLocal, "gemma3:12b", "gemma3:12b", protocol.TierLocal, true),
		model("puter", "gpt-oss-120b", "GPT OSS 120B", protocol.TierFree, true),
		model("openrouter", "free", i18n.T("Free router"), protocol.TierCredits, true),
		model("openrouter", "nvidia/nemotron-3.5-lightning:free", "Nemotron 3.5 Lightning", protocol.TierCredits, true),
		model("openrouter", "nex-agi/nex-n2.5-pro:free", "Nex N2.5 Pro", protocol.TierCredits, true),
		model("anthropic", "claude-opus-5-5", "Claude Opus 5.5", protocol.TierPaid, true),
		model("anthropic", "claude-sonnet-5-5", "Claude Sonnet 5.5", protocol.TierPaid, true),
		model("anthropic", "claude-haiku-5-5", "Claude Haiku 5.5", protocol.TierPaid, true),
		model("anthropic", "claude-fable-5-2", "Claude Fable 5.2", protocol.TierPaid, true),
		model("openai", "gpt-5.6-sol", "GPT-5.6 Sol", protocol.TierPaid, false),
		model("openai", "gpt-5.6-terra", "GPT-5.6 Terra", protocol.TierPaid, false),
		model("openai", "gpt-5.6-luna", "GPT-5.6 Luna", protocol.TierPaid, false),
		model("nvidia", "z-ai/glm-5.2", "GLM 5.2", protocol.TierFree, true),
		model("nvidia", "z-ai/glm-5.1", "GLM 5.1", protocol.TierFree, true),
	}
	// OpenRouter lists a few free routes among hundreds it bills; NVIDIA lists every model free and says nothing of tools
	models = append(models, demoListedModels("openrouter", 60, 12, true)...)
	return append(models, demoListedModels("nvidia", 71, 71, false)...)
}

// count models as a provider's own listing names them, the first free ones of them free
func demoListedModels(provider string, count, free int, toolsKnown bool) []protocol.ModelInfo {
	families := []string{"Qwen", "Gemma", "Llama", "Mistral", "DeepSeek", "Kimi", "MiniMax", "Phi", "Granite", "Nemotron", "Falcon", "Yi"}
	models := make([]protocol.ModelInfo, 0, count)
	for index := range count {
		family := families[index%len(families)]
		size := strconv.Itoa(4 + (index*7)%120)
		id := strings.ToLower(family) + "/" + strings.ToLower(family) + "-" + size + "b"
		name := family + " " + size + "B"
		tier := protocol.TierPaid
		if index < free {
			tier = protocol.TierFree
			if provider == "openrouter" {
				id, name, tier = id+":free", name+" (free)", protocol.TierCredits
			}
		}
		models = append(models, protocol.ModelInfo{
			ID: provider + "/" + id, Provider: provider, Name: name, Tier: tier, Context: 32_768 << (index % 6),
			Tools: toolsKnown && index%5 != 4, ToolsKnown: toolsKnown, Vision: index%4 == 1, Available: true, Listed: true,
		})
	}
	return models
}

// a provider's real count and how many are free follow what models.list answers, read two hours before the demo's now
func withDemoListings(providers []protocol.ProviderInfo, models []protocol.ModelInfo, now time.Time) []protocol.ProviderInfo {
	read := now.Add(-2 * time.Hour)
	for index := range providers {
		provider := &providers[index]
		if provider.Local {
			continue
		}
		total, free, listed := 0, 0, false
		for _, m := range models {
			if m.Provider != provider.ID {
				continue
			}
			total++
			if m.Tier != protocol.TierPaid {
				free++
			}
			listed = listed || m.Listed
		}
		if total == 0 {
			continue
		}
		provider.Models, provider.Free = total, free
		if listed {
			provider.Listed = &read
		}
	}
	return providers
}

// models.list for one provider answers that provider's only, as the daemon's does
func (s *server) listModels(raw json.RawMessage) (protocol.ModelsResult, error) {
	var params protocol.ModelsParams
	if err := decode(raw, &params); err != nil {
		return protocol.ModelsResult{}, err
	}
	if params.Provider == "" {
		return s.data.models, nil
	}
	result := protocol.ModelsResult{Routes: s.data.models.Routes}
	for _, m := range s.data.models.Models {
		if m.Provider == params.Provider {
			result.Models = append(result.Models, m)
		}
	}
	return result, nil
}

// the local rows follow the addresses in the settings, refused and kept off the internet as the daemon does
func (s *server) syncLocalProviders() error {
	local := s.data.snapshot.Settings.Providers.Local
	if local == nil {
		local = make(map[string]string, len(protocol.LocalProviders))
		s.data.snapshot.Settings.Providers.Local = local
	}
	defaults := protocol.DefaultLocalAddresses()
	for index := range s.data.snapshot.Providers {
		provider := &s.data.snapshot.Providers[index]
		if !provider.Local {
			continue
		}
		address, ok := defaults[provider.ID], true
		if typed := local[provider.ID]; typed != "" {
			address, ok = protocol.ParseLocalAddress(typed)
		}
		if !ok {
			local[provider.ID] = provider.Address
			return badRequest(i18n.Tf("Use an address on this computer or your local network, such as %s.", defaults[provider.ID]))
		}
		local[provider.ID], provider.Address = address, address
	}
	return nil
}

// a test of a server that is not started finds nothing there, as the daemon's does
func (s *server) testProvider(id string) protocol.ProviderTestResult {
	for _, provider := range s.data.snapshot.Providers {
		if provider.ID == id && provider.Local && !provider.Running {
			return protocol.ProviderTestResult{Provider: id, Message: i18n.Tf("Not running at %s.", provider.Address)}
		}
	}
	return protocol.ProviderTestResult{Provider: id, OK: true, Model: "free", LatencyMs: 84, Message: i18n.T("Ready")}
}
