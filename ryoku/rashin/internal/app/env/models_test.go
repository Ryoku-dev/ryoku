package env

import (
	"reflect"
	"slices"
	"testing"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

func offeredModels() (protocol.ModelsResult, []protocol.ProviderInfo) {
	models := protocol.ModelsResult{
		Routes: []protocol.Choice{{ID: protocol.RouteAuto, Name: "Auto"}, {ID: protocol.RouteFast, Name: "Fast"}, {ID: protocol.RouteSmart, Name: "Smart"}},
		// the gateway's ids already carry the provider, as models.list sends them
		Models: []protocol.ModelInfo{
			{ID: "openai/gpt", Provider: "openai", Name: "GPT", Tier: protocol.TierPaid, Available: true},
			{ID: "anthropic/claude", Provider: "anthropic", Name: "Claude", Tier: protocol.TierPaid},
			{ID: "openai/cooling", Provider: "openai", Name: "Cooling", Tier: protocol.TierPaid},
			{ID: "ollama-local/llama", Provider: "ollama-local", Name: "Llama", Tier: protocol.TierLocal, Available: true},
			{ID: "kilo/b", Provider: "kilo", Name: "Beta", Tier: protocol.TierFree, Available: true},
			{ID: "kilo/a", Provider: "kilo", Name: "Alpha", Tier: protocol.TierFree, Available: true},
			{ID: "openrouter/x", Provider: "openrouter", Name: "Router", Tier: protocol.TierCredits, Available: true},
			{ID: "groq/y", Provider: "groq", Name: "Groq", Tier: protocol.TierFree},
		},
	}
	providers := []protocol.ProviderInfo{
		{ID: "kilo", Keyless: true},
		{ID: "openrouter", Configured: true},
		{ID: "groq"},
		{ID: "openai", Configured: true},
		{ID: "anthropic"},
		{ID: "ollama-local", Keyless: true},
	}
	return models, providers
}

func TestAskModelsAreGroupedFreeLocalPaid(t *testing.T) {
	models, providers := offeredModels()
	options, ids := modelRows(models, providers, "")
	want := []string{
		protocol.RouteAuto, protocol.RouteFast, protocol.RouteSmart,
		"kilo/a", "kilo/b", "openrouter/x",
		"ollama-local/llama",
		"openai/gpt", "anthropic/claude",
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("listed %v, want %v", ids, want)
	}
	free, local, paid := i18n.T("Free"), i18n.T("Local"), i18n.T("Paid")
	groups := []string{"", "", "", free, free, free, local, paid, paid}
	for index, option := range options {
		if option.Group != groups[index] {
			t.Errorf("%s is in group %q, want %q", ids[index], option.Group, groups[index])
		}
		if wantDisabled := ids[index] == "anthropic/claude"; option.Disabled != wantDisabled {
			t.Errorf("%s disabled = %v, want %v", ids[index], option.Disabled, wantDisabled)
		}
	}
	if keyless := options[len(options)-1]; keyless.Detail != i18n.T("Add a key in Providers") {
		t.Fatalf("a paid model without a key says %q", keyless.Detail)
	}
}

func TestAskModelsKeepAPinTheyNoLongerOffer(t *testing.T) {
	models, providers := offeredModels()
	for _, test := range []struct {
		pin, group string
		at         int
	}{
		// a free model whose provider stopped answering stays in its own group
		{pin: "groq/y", group: i18n.T("Free"), at: 6},
		{pin: "gone/model", group: i18n.T("Unavailable"), at: 9},
	} {
		options, ids := modelRows(models, providers, test.pin)
		at := slices.Index(ids, test.pin)
		if at != test.at {
			t.Fatalf("pin %s listed at %d in %v, want %d", test.pin, at, ids, test.at)
		}
		if !options[at].Disabled || options[at].Group != test.group {
			t.Fatalf("pin %s shown as %+v, want it unpickable in %q", test.pin, options[at], test.group)
		}
	}
	if _, ids := modelRows(models, providers, "kilo/a"); len(ids) != 9 {
		t.Fatalf("a pin the list offers was added again: %v", ids)
	}
}

func TestAskModelsStartFromTheSettingsModel(t *testing.T) {
	models, providers := offeredModels()
	menu := ModelMenu{models: models, providers: providers, loaded: true}
	options, ids, selected := menu.Choices("kilo/b")
	if ids[selected] != "kilo/b" || options[selected].Label != "Beta" {
		t.Fatalf("the select shows %q for the model Settings names", ids[selected])
	}
	// Settings and Ask share the menu, so a model gone from every provider still reads as itself, never as Auto
	options, ids, selected = menu.Choices("gone/model")
	if ids[selected] != "gone/model" || !options[selected].Disabled {
		t.Fatalf("a pinned model nothing offers shows as %q", ids[selected])
	}
}
