package ask

import (
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/protocol"
)

func installed(runtime string, loaded ...string) protocol.LocalResult {
	return protocol.LocalResult{
		Models:  []protocol.LocalModel{{ID: "lfm3", Name: "LFM3 Omni 2B", Context: 32768, State: protocol.LocalInstalled}},
		Runtime: protocol.LocalRuntime{State: runtime, Loaded: loaded},
	}
}

// the line under a new ask promises who answers, so it must follow the order the routes take: this computer first
func TestWhatAnswersSaysThisComputerBeforeAProvider(t *testing.T) {
	free := protocol.ProviderInfo{ID: "groq", Name: "Groq", Enabled: true, Available: 3}
	ollama := protocol.ProviderInfo{ID: protocol.ProviderOllamaLocal, Name: "Ollama", Enabled: true, Local: true, Running: true, Available: 2}
	own := protocol.ProviderInfo{ID: protocol.ProviderRashinModels, Name: "Rashin models", Enabled: true, Local: true, OnDemand: true}
	cases := []struct {
		name        string
		local       protocol.LocalResult
		providers   []protocol.ProviderInfo
		remoteFirst bool
		model       string
		word        string
		tone        palette.Tone
		act         int
		leads       bool
	}{
		{"a loaded local model answers", installed(protocol.RuntimeReady, "lfm3"), []protocol.ProviderInfo{free}, false, protocol.RouteAuto, "Ready on this computer", palette.Alive, actProviders, false},
		{"a resting one offers to start", installed(protocol.RuntimeStopped), []protocol.ProviderInfo{free}, false, "", "On this computer", palette.Idle, actStart, true},
		{"one coming up says so", installed(protocol.RuntimeStarting), nil, false, protocol.RouteFast, "Starting", palette.Becoming, actProviders, false},
		{"the reader's own server counts as this computer", protocol.LocalResult{}, []protocol.ProviderInfo{free, ollama}, false, protocol.RouteAuto, "Ready on this computer", palette.Alive, actProviders, false},
		{"free providers answer when nothing is local", protocol.LocalResult{}, []protocol.ProviderInfo{free}, false, protocol.RouteAuto, "Ready", palette.Alive, actProviders, false},
		{"a model with no llama.cpp leaves it to the providers", installed(protocol.RuntimeMissing), []protocol.ProviderInfo{free}, false, protocol.RouteAuto, "Ready", palette.Alive, actProviders, false},
		{"a failed local model is said even when a provider covers", installed(protocol.RuntimeFailed), []protocol.ProviderInfo{free}, false, protocol.RouteAuto, "The local model failed", palette.Failed, actProviders, true},
		{"nothing at all asks for a model or a key", protocol.LocalResult{}, nil, false, protocol.RouteAuto, "Nothing can answer", palette.Attention, actProviders, true},
		{"remote first is the reader's escape", installed(protocol.RuntimeReady, "lfm3"), []protocol.ProviderInfo{free}, true, protocol.RouteAuto, "Ready", palette.Alive, actProviders, false},
		{"a picked model that is offered", protocol.LocalResult{}, []protocol.ProviderInfo{free}, false, "groq/llama", "Ready", palette.Alive, actProviders, false},
		{"a picked local model rests until asked", installed(protocol.RuntimeStopped), []protocol.ProviderInfo{own}, false, protocol.ProviderRashinModels + "/lfm3", "On this computer", palette.Idle, actStart, true},
		{"a picked loaded local model is ready", installed(protocol.RuntimeReady, "lfm3"), []protocol.ProviderInfo{own}, false, protocol.ProviderRashinModels + "/lfm3", "Ready on this computer", palette.Alive, actProviders, false},
		{"a picked local model without llama.cpp cannot answer", installed(protocol.RuntimeMissing), []protocol.ProviderInfo{own}, false, protocol.ProviderRashinModels + "/lfm3", "Cannot answer now", palette.Attention, actProviders, true},
		{"a local model that is not downloaded cannot answer", protocol.LocalResult{}, []protocol.ProviderInfo{own}, false, protocol.ProviderRashinModels + "/missing", "Cannot answer now", palette.Attention, actProviders, true},
		{"a stopped external local server cannot answer", protocol.LocalResult{}, []protocol.ProviderInfo{{ID: protocol.ProviderOllamaLocal, Enabled: true, Local: true, Available: 1}}, false, protocol.ProviderOllamaLocal + "/model", "Cannot answer now", palette.Attention, actProviders, true},
		{"a picked model nothing offers", installed(protocol.RuntimeReady, "lfm3"), []protocol.ProviderInfo{free}, false, "openai/gpt", "Cannot answer now", palette.Attention, actProviders, true},
	}
	for _, c := range cases {
		got := whatAnswers(c.local, c.providers, c.remoteFirst, c.model, "the picked model")
		if got.word != c.word || got.tone != c.tone || got.act != c.act || got.leads != c.leads {
			t.Errorf("%s: got %q tone %v act %d leads %v, want %q tone %v act %d leads %v",
				c.name, got.word, got.tone, got.act, got.leads, c.word, c.tone, c.act, c.leads)
		}
		if got.detail == "" || got.label == "" {
			t.Errorf("%s: the line needs its caption and its way to change it, got %q and %q", c.name, got.detail, got.label)
		}
	}
	if started := whatAnswers(installed(protocol.RuntimeStopped), nil, false, "", ""); started.model != "lfm3" {
		t.Errorf("Start now would load %q, not the installed model", started.model)
	}
}
