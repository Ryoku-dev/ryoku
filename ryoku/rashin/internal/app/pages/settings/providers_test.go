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
	}
	for _, test := range cases {
		if got, _ := providerState(test.provider); got != test.want {
			t.Fatalf("providerState(%+v) = %q, want %q", test.provider, got, test.want)
		}
	}
}
