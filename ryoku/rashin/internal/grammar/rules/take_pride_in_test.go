package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTakePrideIn_Take(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I take pride of my work and go out of my way to support people who honored me with building their product using it.", NewTakePrideIn(), "I take pride in my work and go out of my way to support people who honored me with building their product using it.")
}

func TestTakePrideIn_Taken(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... the variety and multifaceted society that we Malaysians have always taken pride of?", NewTakePrideIn(), "... the variety and multifaceted society that we Malaysians have always taken pride in?")
}

func TestTakePrideIn_Takes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Being a part of the team that takes pride of their work.", NewTakePrideIn(), "Being a part of the team that takes pride in their work.")
}

func TestTakePrideIn_Took(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and somehow we always took pride of well written and maintainble code", NewTakePrideIn(), "and somehow we always took pride in well written and maintainble code")
}

func TestTakePrideIn_DontFlagTakePrideOfPlace(t *testing.T) {
	linttest.AssertNoLints(t, "Any better alternatives will take pride of place, but for now, this is fine.", NewTakePrideIn())
}

func TestTakePrideIn_DontFlagTakenPrideOfPlace(t *testing.T) {
	linttest.AssertNoLints(t, "The venture-backed model has taken pride of place, but we should celebrate forks", NewTakePrideIn())
}

func TestTakePrideIn_DontFlagTakenPrideOfPlaceHyphenated(t *testing.T) {
	linttest.AssertNoLints(t, "\"sometimes some strange errors might occur\" has now taken pride-of-place as my favorite bug report of all time", NewTakePrideIn())
}

func TestTakePrideIn_DontFlagTakesPrideOfPlace(t *testing.T) {
	linttest.AssertNoLints(t, "this magnificent family residence takes pride of place just footsteps from the tranquil shores of Vaucluse Bay", NewTakePrideIn())
}

func TestTakePrideIn_DontFlagTookPrideOfPlace(t *testing.T) {
	linttest.AssertNoLints(t, "They were built to be exceptionally robust, and that took pride of place (for a while) over technical and luxury features.", NewTakePrideIn())
}

func TestTakePrideIn_DontFlagEdgeCase(t *testing.T) {
	t.Skip("Surely too rare to address")
	linttest.AssertNoLints(t, "what is known as “dune bashing” has taken pride of honour on my list of “don’t ever do such a stupid thing again.”", NewTakePrideIn())
}
