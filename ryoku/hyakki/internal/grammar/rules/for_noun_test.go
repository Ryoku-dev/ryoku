package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestForNoun_CorrectsFroBasicCorrection(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I got a text fro Sarah.", NewForNoun(), "I got a text for Sarah.")
}

func TestForNoun_AllowsForClean(t *testing.T) {
	linttest.AssertLintCount(t, "I got a text for Sarah.", NewForNoun(), 0)
}

func TestForNoun_CorrectsFroSure(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He was away fro sure!", NewForNoun(), "He was away for sure!")
}
