package settings

import (
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestOrderedProvidersPrioritizesAvailableKeyedThenKeyless(t *testing.T) {
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

func TestProviderMetaReportsKeyAndModelCounts(t *testing.T) {
	provider := protocol.ProviderInfo{Models: 7, Keys: []protocol.KeyInfo{{}, {}}}
	if got, want := providerMeta(provider), "2 keys · 7 models"; got != want {
		t.Fatalf("providerMeta() = %q, want %q", got, want)
	}

	provider.Models = 1
	provider.Keys = provider.Keys[:1]
	if got, want := providerMeta(provider), "1 key · 1 model"; got != want {
		t.Fatalf("providerMeta() = %q, want %q", got, want)
	}
}
