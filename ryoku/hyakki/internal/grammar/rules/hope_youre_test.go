package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHopeYoure_CorrectsExcitedHere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hope your excited here.", NewHopeYoure(), "I hope you're excited here.")
}

func TestHopeYoure_CorrectsSafeThere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hope your safe there.", NewHopeYoure(), "I hope you're safe there.")
}

func TestHopeYoure_CorrectsSafeOverThere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hope your safe over there.", NewHopeYoure(), "I hope you're safe over there.")
}

func TestHopeYoure_CorrectsFineHere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hope your fine here.", NewHopeYoure(), "I hope you're fine here.")
}

func TestHopeYoure_CorrectsHappyThere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hope your happy there.", NewHopeYoure(), "I hope you're happy there.")
}

func TestHopeYoure_CorrectsHealthyOutHere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hope your healthy out here.", NewHopeYoure(), "I hope you're healthy out here.")
}

func TestHopeYoure_CorrectsStrongThere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We hope your strong there.", NewHopeYoure(), "We hope you're strong there.")
}

func TestHopeYoure_CorrectsSorryHere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hope your sorry here.", NewHopeYoure(), "Hope you're sorry here.")
}

func TestHopeYoure_NoLintWithContraction(t *testing.T) {
	linttest.AssertLintCount(t, "I hope you're excited here.", NewHopeYoure(), 0)
}

func TestHopeYoure_NoLintWithoutAdjective(t *testing.T) {
	linttest.AssertLintCount(t, "I hope your trip went well.", NewHopeYoure(), 0)
}

func TestHopeYoure_NoLintWithFollowingClause(t *testing.T) {
	linttest.AssertLintCount(t, "I hope your friends are well there.", NewHopeYoure(), 0)
}

func TestHopeYoure_NoLintWhenPossessiveContext(t *testing.T) {
	linttest.AssertLintCount(t, "I hope your advice on punctuation is helpful.", NewHopeYoure(), 0)
}
