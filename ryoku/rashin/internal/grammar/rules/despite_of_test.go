package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDespiteOf_CatchesLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The team performed well, despite of the difficulties they faced.", NewDespiteOf(), "The team performed well, despite the difficulties they faced.")
}

func TestDespiteOf_CatchesDifferentCases(t *testing.T) {
	linttest.AssertLintCount(t, "Despite of the rain, we went for a walk.", NewDespiteOf(), 1)
}

func TestDespiteOf_LikesCorrection(t *testing.T) {
	linttest.AssertLintCount(t, "The team performed well, despite the difficulties they faced. In spite of the rain, we went for a walk.", NewDespiteOf(), 0)
}
