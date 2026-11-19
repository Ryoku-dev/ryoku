package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestEllipsisLength_AllowsCorrectEllipsis(t *testing.T) {
	linttest.AssertLintCount(t, "...", NewEllipsisLength(), 0)
}

func TestEllipsisLength_CorrectsLongEllipsis(t *testing.T) {
	linttest.AssertLintCount(t, ".....", NewEllipsisLength(), 1)
	linttest.AssertSuggestionResult(t, ".....", NewEllipsisLength(), "...")
}

func TestEllipsisLength_CorrectsShortEllipsis(t *testing.T) {
	linttest.AssertLintCount(t, "..", NewEllipsisLength(), 1)
	linttest.AssertSuggestionResult(t, "..", NewEllipsisLength(), "...")
}
