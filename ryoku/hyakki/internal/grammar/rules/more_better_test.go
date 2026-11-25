package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMoreBetter_FlagMostBiggest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Most biggest", NewMoreBetter(), "Biggest")
}

func TestMoreBetter_FlagMoreBetterAndMoreBetter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "More bigger is more better", NewMoreBetter(), "Bigger is better")
}
