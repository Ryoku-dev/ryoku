package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSaveToSafe_FixIgnore(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is save to ignore trivial code.", NewSaveToSafe(), "It is safe to ignore trivial code.")
}

func TestSaveToSafe_FixTravel(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is it save to travel abroad now?", NewSaveToSafe(), "Is it safe to travel abroad now?")
}

func TestSaveToSafe_IgnoreCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "It is safe to assume nothing.", NewSaveToSafe(), 0)
}
