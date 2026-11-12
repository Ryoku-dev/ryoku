package env

import (
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// omp lists Claude Haiku 4.5 from Anthropic twice and from OpenRouter once, all under one name
func TestModelsOfOneNameAreToldApartByTheirIDs(t *testing.T) {
	choices := []protocol.Choice{
		{ID: "anthropic/claude-haiku-4-5", Name: "Claude Haiku 4.5"},
		{ID: "anthropic/claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5"},
		{ID: "openrouter/anthropic/claude-haiku-4.5", Name: "Claude Haiku 4.5"},
	}
	seen := map[string]bool{}
	for _, choice := range choices {
		option := ChoiceOption(choice)
		if option.Label != "Claude Haiku 4.5" || option.Detail != choice.ID {
			t.Errorf("%s reads %q, %q; want its name over its id", choice.ID, option.Label, option.Detail)
		}
		seen[option.Label+option.Detail] = true
	}
	if len(seen) != len(choices) {
		t.Error("two rows of the list read the same")
	}
	if option := ChoiceOption(protocol.Choice{ID: "opus", Name: "Opus", Description: "For the hardest work"}); option.Detail != "For the hardest work" {
		t.Errorf("a described model reads %q, want what the harness says of it", option.Detail)
	}
	if option := ChoiceOption(protocol.Choice{ID: "gpt-5.6"}); option.Label != "gpt-5.6" || option.Detail != "" {
		t.Errorf("a model with only an id reads %q, %q; want the id once", option.Label, option.Detail)
	}
}

// the board's facts line names the model a session starts on: the pick over what the harness runs by itself
func TestStartFactsNameThePick(t *testing.T) {
	setup := []protocol.HarnessFact{{Label: "Model", Value: "anthropic/claude-opus-5-5"}, {Label: "Thinking", Value: "Auto"}}
	harness := protocol.HarnessInfo{
		ID:    "omp",
		Setup: setup,
		Capabilities: &protocol.HarnessCapabilities{ModelList: []protocol.Choice{
			{ID: "anthropic/claude-haiku-4-5", Name: "Claude Haiku 4.5"},
		}},
	}
	got := StartFacts(harness, "anthropic/claude-haiku-4-5")
	want := []protocol.HarnessFact{{Label: "Model", Value: "Claude Haiku 4.5"}, {Label: "Thinking", Value: "Auto"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("with a pick the facts read %v, want %v", got, want)
	}
	if setup[0].Value != "anthropic/claude-opus-5-5" {
		t.Fatal("naming the pick changed the snapshot's facts")
	}
	if got := StartFacts(harness, ""); !reflect.DeepEqual(got, setup) {
		t.Fatalf("without a pick the facts read %v, want the harness's own %v", got, setup)
	}
	harness.Setup = []protocol.HarnessFact{{Label: "Thinking", Value: "Auto"}}
	if got := StartFacts(harness, "gpt-x"); !reflect.DeepEqual(got, []protocol.HarnessFact{{Label: "Model", Value: "gpt-x"}, {Label: "Thinking", Value: "Auto"}}) {
		t.Fatalf("a harness that names no model reads %v, want the pick first", got)
	}
}
