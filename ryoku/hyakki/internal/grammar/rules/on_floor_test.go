package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOnFloor_NotLintWithCorrectPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "I'm living on the 3rd floor of a building.", NewOnFloor(), 0)
}

func TestOnFloor_LintWithIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm living in the 3rd floor of a building.", NewOnFloor(), "I'm living on the 3rd floor of a building.")
}

func TestOnFloor_LintWithAt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm living at the second floor of a building.", NewOnFloor(), "I'm living on the second floor of a building.")
}

func TestOnFloor_InTheStartOfSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the 3rd floor of a building.", NewOnFloor(), "On the 3rd floor of a building.")
}

func TestOnFloor_AtTheStartOfSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "At the second floor of a building.", NewOnFloor(), "On the second floor of a building.")
}

func TestOnFloor_NoLintWithLookUpAt(t *testing.T) {
	linttest.AssertLintCount(t, "She looked up at the third floor.", NewOnFloor(), 0)
}

func TestOnFloor_NoLintWithStopAt(t *testing.T) {
	linttest.AssertLintCount(t, "The elevator stops at the 3rd floor of a building.", NewOnFloor(), 0)
}

func TestOnFloor_NoLintWithLookingUpAt(t *testing.T) {
	linttest.AssertLintCount(t, "The workers are looking up at the 3rd floor of a building.", NewOnFloor(), 0)
}
