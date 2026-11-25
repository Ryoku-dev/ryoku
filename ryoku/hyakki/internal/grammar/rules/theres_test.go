package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTheres_CorrectsLowercaseBeforeThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We realized their's the clue we missed.", NewTheres(), "We realized there's the clue we missed.")
}

func TestTheres_CorrectsSentenceStart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Their's the solution on the table.", NewTheres(), "There's the solution on the table.")
}

func TestTheres_CorrectsBeforeNo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I promise their's no extra charge.", NewTheres(), "I promise there's no extra charge.")
}

func TestTheres_CorrectsBeforeAn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I suspect their's an error in the log.", NewTheres(), "I suspect there's an error in the log.")
}

func TestTheres_CorrectsBeforeA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Maybe their's a better route available.", NewTheres(), "Maybe there's a better route available.")
}

func TestTheres_CorrectsBeforeAnother(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Their's another round after this.", NewTheres(), "There's another round after this.")
}

func TestTheres_CorrectsBeforeEnough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Their's enough context in the report.", NewTheres(), "There's enough context in the report.")
}

func TestTheres_AllowsPossessivePronounForm(t *testing.T) {
	linttest.AssertLintCount(t, "Theirs is the final draft.", NewTheres(), 0)
}

func TestTheres_IgnoresWithoutDeterminerAfterward(t *testing.T) {
	linttest.AssertLintCount(t, "I think their's better already.", NewTheres(), 0)
}

func TestTheres_IgnoresCorrectContraction(t *testing.T) {
	linttest.AssertLintCount(t, "There's a bright sign ahead.", NewTheres(), 0)
}
