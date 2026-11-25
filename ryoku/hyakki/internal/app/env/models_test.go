package env

import (
	"reflect"
	"slices"
	"testing"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/state"
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
			{ID: "ember/rashin-fast", Provider: protocol.ProviderEmber, Name: "Qwen3.8 Flash", Logo: "qwen", Tier: protocol.TierPaid, Available: true},
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
		{ID: protocol.ProviderEmber, Configured: true},
	}
	return models, providers
}

func TestAskModelsAreGroupedEmberLocalFreePaid(t *testing.T) {
	models, providers := offeredModels()
	options, ids := modelRows(models, providers, "", false, nil)
	want := []string{
		protocol.RouteAuto, protocol.RouteFast, protocol.RouteSmart,
		"ember/rashin-fast",
		"ollama-local/llama",
		"kilo/a", "kilo/b", "openrouter/x",
		"openai/gpt", "anthropic/claude",
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("listed %v, want %v", ids, want)
	}
	ember, local, free, paid := "Ryoku Ember", i18n.T("On this computer"), i18n.T("Free"), i18n.T("Paid")
	groups := []string{"", "", "", ember, local, free, free, free, paid, paid}
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

// A provider's own listing joins its group: a free one that answers can be picked, and says when its
// tool support is unknown; what a key would open is too long a list, so only Hyakki's picks stand for it.
func TestAskModelsCarryTheListedModels(t *testing.T) {
	models, providers := offeredModels()
	models.Models = append(models.Models,
		protocol.ModelInfo{ID: "kilo/listed:free", Provider: "kilo", Name: "Listed", Tier: protocol.TierFree, Available: true, Listed: true, ToolsKnown: true},
		protocol.ModelInfo{ID: "kilo/mystery:free", Provider: "kilo", Name: "Mystery", Tier: protocol.TierFree, Available: true, Listed: true},
		protocol.ModelInfo{ID: "anthropic/listed", Provider: "anthropic", Name: "Listed Claude", Tier: protocol.TierPaid, Listed: true, ToolsKnown: true},
	)
	options, ids := modelRows(models, providers, "", false, nil)
	if slices.Contains(ids, "anthropic/listed") {
		t.Fatal("a listed model of a provider without a key was offered")
	}
	for index, id := range ids {
		switch id {
		case "kilo/listed:free":
			if options[index].Group != i18n.T("Free") || options[index].Detail != id {
				t.Fatalf("listed free model = %#v", options[index])
			}
		case "kilo/mystery:free":
			if options[index].Detail != i18n.Tf("%s, tools unknown", id) {
				t.Fatalf("unknown tools model = %#v", options[index])
			}
		}
	}
	if !slices.Contains(ids, "kilo/listed:free") || !slices.Contains(ids, "kilo/mystery:free") {
		t.Fatalf("listed models missing: %v", ids)
	}
}

func TestAskModelsKeepAPinTheyNoLongerOffer(t *testing.T) {
	models, providers := offeredModels()
	for _, test := range []struct {
		pin, group string
		at         int
	}{
		// a free model whose provider stopped answering stays in its own group
		{pin: "groq/y", group: i18n.T("Free"), at: 8},
		{pin: "gone/model", group: i18n.T("Unavailable"), at: 10},
	} {
		options, ids := modelRows(models, providers, test.pin, false, nil)
		at := slices.Index(ids, test.pin)
		if at != test.at {
			t.Fatalf("pin %s listed at %d in %v, want %d", test.pin, at, ids, test.at)
		}
		if !options[at].Disabled || options[at].Group != test.group {
			t.Fatalf("pin %s shown as %+v, want it unpickable in %q", test.pin, options[at], test.group)
		}
	}
	if _, ids := modelRows(models, providers, "kilo/a", false, nil); len(ids) != 10 {
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

// Signed out, Ryoku Ember leads the list as one row that opens the account
// popover; it is a way there, never a model a select can take. Signed in, its
// models stand in its place and the row is gone.
func TestSignedOutEmberIsARowThatOpensTheAccount(t *testing.T) {
	models, providers := offeredModels()
	models.Models = slices.DeleteFunc(models.Models, func(m protocol.ModelInfo) bool { return m.Provider == protocol.ProviderEmber })
	providers = slices.DeleteFunc(providers, func(p protocol.ProviderInfo) bool { return p.ID == protocol.ProviderEmber })
	opened := 0
	options, ids := modelRows(models, providers, "", false, func() { opened++ })
	at := slices.Index(ids, emberSignInID)
	if at != len(models.Routes) {
		t.Fatalf("the sign-in row is at %d in %v, want first after the routes", at, ids)
	}
	row := options[at]
	if row.Label != "Ryoku Ember — sign in to use" || row.Group != "Ryoku Ember" || row.Act == nil || row.Disabled {
		t.Fatalf("the sign-in row is %+v", row)
	}
	row.Act()
	if opened != 1 {
		t.Fatalf("picking the row opened the account %d times", opened)
	}
	if protocol.IsHostedModel(ids[at]) || protocol.IsRoute(ids[at]) {
		t.Fatalf("the row stands for %q, which names a model or a route", ids[at])
	}

	snapshot := state.Snapshot{Providers: providers, AccountKnown: true, Account: protocol.AccountState{State: protocol.AccountSignedOut}}
	if !SignInToEmber(snapshot) {
		t.Fatal("a signed-out account offers no way to Ryoku Ember")
	}
	snapshot.Account.State = protocol.AccountSignedIn
	if SignInToEmber(snapshot) {
		t.Fatal("a signed-in account is still asked to sign in")
	}

	models, providers = offeredModels()
	if _, ids := modelRows(models, providers, "", false, func() {}); slices.Contains(ids, emberSignInID) {
		t.Fatalf("Ryoku Ember's models and the sign-in row are both listed: %v", ids)
	}
	if SignInToEmber(state.Snapshot{Providers: providers}) {
		t.Fatal("an offered Ryoku Ember provider still asks for a sign-in")
	}
}
