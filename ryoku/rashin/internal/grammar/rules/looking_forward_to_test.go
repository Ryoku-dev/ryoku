package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLookingForwardTo_NotLintWithCorrectVerb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She was looking forward to see the grandchildren again.", NewLookingForwardTo(), "She was looking forward to seeing the grandchildren again.")
}

func TestLookingForwardTo_LintWithIncorrectVerb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She was looking forward to see the grandchildren again.", NewLookingForwardTo(), "She was looking forward to seeing the grandchildren again.")
}

func TestLookingForwardTo_LintWithIncorrectVerbEndingInE(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She was looking forward to make the grandchildren happy.", NewLookingForwardTo(), "She was looking forward to making the grandchildren happy.")
}

func TestLookingForwardTo_NotLintWithNonVerb(t *testing.T) {
	linttest.AssertLintCount(t, "She was looking forward to the grandchildren's visit.", NewLookingForwardTo(), 0)
}
