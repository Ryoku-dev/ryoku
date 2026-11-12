package settings

import (
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestOrderedProvidersPutsAnsweringKeyedThenKeylessFirst(t *testing.T) {
	providers := []protocol.ProviderInfo{
		{ID: "zulu", Name: "Zulu", Configured: true},
		{ID: "ovh", Name: "OVHcloud", Keyless: true, Enabled: true, Configured: true, Available: 2},
		{ID: "nvidia", Name: "NVIDIA", Enabled: true, Configured: true, Available: 5},
		{ID: "alpha", Name: "Alpha"},
		{ID: "xai", Name: "xAI"},
		{ID: "zhipu", Name: "Zhipu"},
		{ID: "groq", Name: "Groq", Enabled: true, Configured: true, Available: 4},
		{ID: "kilo", Name: "Kilo", Keyless: true, Enabled: true, Configured: true, Available: 1},
	}

	ordered := orderedProviders(providers)
	got := make([]string, len(ordered))
	for index, provider := range ordered {
		got[index] = provider.ID
	}
	want := []string{"groq", "nvidia", "kilo", "ovh", "alpha", "xai", "zhipu", "zulu"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ordered provider IDs = %v, want %v", got, want)
	}
	if providers[0].ID != "zulu" {
		t.Fatalf("orderedProviders mutated its input: first ID = %q", providers[0].ID)
	}
}

func TestProviderStateNamesWhatStandsBetweenTheReaderAndAnAnswer(t *testing.T) {
	cases := []struct {
		provider protocol.ProviderInfo
		want     string
	}{
		{protocol.ProviderInfo{Available: 3}, "Off"},
		{protocol.ProviderInfo{Enabled: true, Available: 3}, "Ready"},
		{protocol.ProviderInfo{Enabled: true}, "Needs a key"},
		{protocol.ProviderInfo{Enabled: true, Configured: true}, "Not answering"},
		{protocol.ProviderInfo{Enabled: true, Configured: true, Keyless: true, Local: true}, "Not running"},
		{protocol.ProviderInfo{Enabled: true, Configured: true, Keyless: true, Local: true, Running: true}, "No model yet"},
		{protocol.ProviderInfo{Enabled: true, Configured: true, Keyless: true, Local: true, Running: true, Models: 3, Available: 3}, "Ready"},
	}
	for _, test := range cases {
		if got, _ := providerState(test.provider); got != test.want {
			t.Fatalf("providerState(%+v) = %q, want %q", test.provider, got, test.want)
		}
	}
}

// A local provider reads as running with its models or as the one line that
// starts it; a paid provider says so before its keys. The week's usage follows
// the counts, never the how-to sentence.
func TestProviderMetaSaysRunningOrHowToStart(t *testing.T) {
	cases := []struct {
		provider protocol.ProviderInfo
		used     string
		want     string
	}{
		{protocol.ProviderInfo{Local: true, Keyless: true, Running: true, Models: 3}, "", "Running, 3 models"},
		{protocol.ProviderInfo{Local: true, Keyless: true, Running: true, Models: 1}, "2 answers this week", "Running, 1 model, 2 answers this week"},
		{protocol.ProviderInfo{Local: true, Keyless: true, Note: "Install Ollama and run ollama serve."}, "", "Install Ollama and run ollama serve."},
		{protocol.ProviderInfo{Local: true, Keyless: true, Note: "Install Ollama and run ollama serve."}, "2 answers this week", "Install Ollama and run ollama serve."},
		{protocol.ProviderInfo{Tier: protocol.TierPaid, Models: 4}, "", "Paid, no key yet, 4 models"},
		{protocol.ProviderInfo{Tier: protocol.TierPaid, Models: 4, Keys: []protocol.KeyInfo{{ID: "a"}}}, "", "Paid, 1 key, 4 models"},
		{protocol.ProviderInfo{Tier: protocol.TierFree, Models: 1, Keys: []protocol.KeyInfo{{ID: "a"}}}, "", "1 key, 1 model"},
		{protocol.ProviderInfo{Tier: protocol.TierFree, Keyless: true, Models: 2}, "", "Needs no key, 2 models"},
		{protocol.ProviderInfo{Tier: protocol.TierFree, Keyless: true, Models: 2}, "5 answers this week", "Needs no key, 2 models, 5 answers this week"},
	}
	for _, test := range cases {
		if got := providerMeta(test.provider, test.used); got != test.want {
			t.Fatalf("providerMeta(%+v, %q) = %q, want %q", test.provider, test.used, got, test.want)
		}
	}
}

func TestLocalProvidersAreGroupedApartInTheirOwnOrder(t *testing.T) {
	local, online := splitProviders([]protocol.ProviderInfo{
		{ID: "ollama-local", Local: true}, {ID: "groq"}, {ID: "llamacpp", Local: true}, {ID: "openai"},
	})
	if len(local) != 2 || local[0].ID != "ollama-local" || local[1].ID != "llamacpp" || len(online) != 2 || online[0].ID != "groq" {
		t.Fatalf("local = %v, online = %v", local, online)
	}
}
