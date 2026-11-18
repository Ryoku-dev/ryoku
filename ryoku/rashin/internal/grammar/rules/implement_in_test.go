package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestImplementIn_FixImplementSomethingLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How to implement overlay into your code?", NewImplementIn(), "How to implement overlay in your code?")
}

func TestImplementIn_FixImplementSomethingTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Implement Connectors into the Testbed", NewImplementIn(), "Implement Connectors in the Testbed")
}

func TestImplementIn_FixImplementSomething(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So why not implement it into links or w3m?", NewImplementIn(), "So why not implement it in links or w3m?")
}

func TestImplementIn_FixImplementInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Use ngx-daterangepicker-material as Storybook Angular (Issue about Ranges Clicked and Dates Updated) when implement into other project", NewImplementIn(), "Use ngx-daterangepicker-material as Storybook Angular (Issue about Ranges Clicked and Dates Updated) when implement in other project")
}

func TestImplementIn_FixImplementedInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Demo of MCTS AI implemented into a platformer with a level editor.", NewImplementIn(), "Demo of MCTS AI implemented in a platformer with a level editor.")
}

func TestImplementIn_FixImplementedSomething(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Trained and implemented autoencoder into a pipeline with Google Cloud transcription API", NewImplementIn(), "Trained and implemented autoencoder in a pipeline with Google Cloud transcription API")
}

func TestImplementIn_FixImplementingSomething(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Problem with implementing library into the Vue.js webpack-based project", NewImplementIn(), "Problem with implementing library in the Vue.js webpack-based project")
}

func TestImplementIn_FixImplementingSomethingTitleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Implementing breadcrumbs into site with AngularJS v1.2.16.", NewImplementIn(), "Implementing breadcrumbs in site with AngularJS v1.2.16.")
}

func TestImplementIn_FixImplementsSomething(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A plugin that implements latex into discord.", NewImplementIn(), "A plugin that implements latex in discord.")
}

func TestImplementIn_DontFlagImplementsIntoTrait(t *testing.T) {
	linttest.AssertNoLints(t, "I suppose there is a concern that somebody may define a struct which implements Into", NewImplementIn())
}

func TestImplementIn_DontFlagImplementingIntoTrait(t *testing.T) {
	linttest.AssertNoLints(t, "There might still be cases where one cannot implement From but implementing Into is possible.", NewImplementIn())
}

func TestImplementIn_DontFlagImplementingTheIntoTrait(t *testing.T) {
	linttest.AssertNoLints(t, "I'm having trouble implementing the Into trait for a generic struct in Rust.", NewImplementIn())
}
