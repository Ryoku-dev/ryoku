package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCriteriaPhenomenaSingularCriteriaPhenomena_CanDetectIncorrectCriteria(t *testing.T) {
	linttest.AssertSuggestionResult(t, "...One criteria is essential...", NewSingularCriteriaPhenomena(), "...One criterion is essential...")
}

func TestCriteriaPhenomenaSingularCriteriaPhenomena_CanDetectIncorrectPhenomena(t *testing.T) {
	linttest.AssertSuggestionResult(t, "...I would like to see that phenomena.", NewSingularCriteriaPhenomena(), "...I would like to see that phenomenon.")
}

func TestCriteriaPhenomenaSingularCriteriaPhenomena_AllowsCorrectCriteria(t *testing.T) {
	linttest.AssertLintCount(t, "...She disagrees with those criteria.", NewSingularCriteriaPhenomena(), 0)
}

func TestCriteriaPhenomenaSingularCriteriaPhenomena_AllowsCorrectPhenomena(t *testing.T) {
	linttest.AssertLintCount(t, "...Many phenomena were on display.", NewSingularCriteriaPhenomena(), 0)
}
