package protocol

import "testing"

// A member's task runs on the first model the chain names: the member's own,
// the teams defaults', the Agents model, else auto. The app shows the same.
func TestTeamTaskEngineFollowsTheChain(t *testing.T) {
	var settings Settings
	if model, options := ResolveEngine(Member{}, settings); model != RouteAuto || options != nil {
		t.Fatalf("empty chain = %q %v", model, options)
	}
	settings.Agent.Model = "openrouter/qwen3-coder:free"
	settings.Agent.Options = map[string]string{OptionEffort: "low"}
	if model, options := ResolveEngine(Member{}, settings); model != "openrouter/qwen3-coder:free" || options[OptionEffort] != "low" {
		t.Fatalf("agents = %q %v", model, options)
	}
	settings.Teams.Defaults.Model = RouteFast
	settings.Teams.Defaults.Options = map[string]string{OptionEffort: "high"}
	if model, options := ResolveEngine(Member{}, settings); model != RouteFast || options[OptionEffort] != "high" {
		t.Fatalf("defaults = %q %v", model, options)
	}
	member := Member{Model: "rashin-models/lfm3-omni-2b"}
	if model, options := ResolveEngine(member, settings); model != "rashin-models/lfm3-omni-2b" || options[OptionEffort] != "high" {
		t.Fatalf("member = %q %v", model, options)
	}
}

// A model id only an external harness understood is never a member's engine.
func TestEngineModelIsARouteOrAProvidersModel(t *testing.T) {
	for model, want := range map[string]bool{
		"":                           true,
		RouteAuto:                    true,
		RouteSmart:                   true,
		"rashin-models/lfm3-omni-2b": true,
		"kilo/nvidia/nemotron:free":  true,
		"haiku":                      false,
		"gpt-5.6":                    false,
		"/model":                     false,
		"provider/":                  false,
		"two words/x":                false,
	} {
		if got := ValidEngineModel(model); got != want {
			t.Fatalf("ValidEngineModel(%q) = %v, want %v", model, got, want)
		}
	}
}
