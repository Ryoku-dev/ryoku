package env

import (
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
