package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMostNumber_CorrectsMostNumber(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Find artists that have been on Spotify the most number of times.", NewMostNumber(), "Find artists that have been on Spotify the highest number of times.")
}

func TestMostNumber_CorrectsMostAmountTitleCase(t *testing.T) {
	t.Skip("Upstream ignored: replace_with_match_case currently produces 'GreatEst'")
	linttest.AssertSuggestionResult(t, "Area of Container with the Most Amount of Water", NewMostNumber(), "Area of Container with the Greatest Amount of Water")
}

func TestMostNumber_CorrectsMostAmount(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I just wanted to make sure it's good for the most amount of people, not just what I like.", NewMostNumber(), "I just wanted to make sure it's good for the greatest amount of people, not just what I like.")
}

func TestMostNumber_DontCorrectMostNumberWithoutContext(t *testing.T) {
	linttest.AssertLintCount(t, "The random non-sequential nature should prevent most number gaming/sniping/lunging.", NewMostNumber(), 0)
}

func TestMostNumber_CorrectsMostAmountWithMaximum(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you want to support the most amount of different architectures ...", NewMostNumber(), "If you want to support the maximum amount of different architectures ...")
}
