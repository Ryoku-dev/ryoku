package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHavePronoun_CorrectsHasWe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Has we finished the report?", NewHavePronoun(), "Have we finished the report?")
}

func TestHavePronoun_CorrectsHasYou(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Has you misunderstood?", NewHavePronoun(), "Have you misunderstood?")
}

func TestHavePronoun_CorrectsHasI(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Has I misunderstood?", NewHavePronoun(), "Have I misunderstood?")
}

func TestHavePronoun_CorrectsHasThey(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Has they arrived yet?", NewHavePronoun(), "Have they arrived yet?")
}

func TestHavePronoun_AllowsHasHe(t *testing.T) {
	linttest.AssertLintCount(t, "Has he arrived yet?", NewHavePronoun(), 0)
}

func TestHavePronoun_IgnoresNonInitialUsage(t *testing.T) {
	linttest.AssertLintCount(t, "The system has we confused for a moment.", NewHavePronoun(), 0)
}
