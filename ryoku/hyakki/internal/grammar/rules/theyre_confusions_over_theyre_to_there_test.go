package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTheyreConfusionsOverTheyreToThere_CorrectsLocativeAsciiApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Put the chairs over they're by the window.", NewOverTheyreToThere(), "Put the chairs over there by the window.")
}

func TestTheyreConfusionsOverTheyreToThere_CorrectsLocativeSmartApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is that their car parked over they’re?", NewOverTheyreToThere(), "Is that their car parked over there?")
}

func TestTheyreConfusionsOverTheyreToThere_IgnoresCorrectForm(t *testing.T) {
	linttest.AssertLintCount(t, "Put the chairs over there by the window.", NewOverTheyreToThere(), 0)
}
