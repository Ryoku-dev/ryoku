package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestModalSeem_CorrectsBasicCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It may seen impossible to finish.", NewModalSeem(), "It may seem impossible to finish.")
}

func TestModalSeem_CorrectsWithAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That might seen utterly ridiculous.", NewModalSeem(), "That might seem utterly ridiculous.")
}

func TestModalSeem_OffersBeOption(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It may seen impossible to finish.", NewModalSeem(), "It may be impossible to finish.")
}

func TestModalSeem_RespectsUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "THIS COULD SEEN TERRIBLE.", NewModalSeem(), "THIS COULD SEEM TERRIBLE.")
}

func TestModalSeem_CorrectsBeforePunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Still, it may seen absurd, but we will continue.", NewModalSeem(), "Still, it may seem absurd, but we will continue.")
}

func TestModalSeem_CorrectsAcrossNewline(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It may seen\n impossible to pull off.", NewModalSeem(), "It may seem\n impossible to pull off.")
}

func TestModalSeem_IgnoresCorrectSeem(t *testing.T) {
	linttest.AssertNoLints(t, "It may seem impossible to finish.", NewModalSeem())
}

func TestModalSeem_IgnoresModalWithBeSeen(t *testing.T) {
	linttest.AssertNoLints(t, "It may be seen as unfair.", NewModalSeem())
}

func TestModalSeem_IgnoresModalSeenNoun(t *testing.T) {
	linttest.AssertNoLints(t, "It may seen results sooner than expected.", NewModalSeem())
}

func TestModalSeem_IgnoresModalSeenClause(t *testing.T) {
	linttest.AssertLintCount(t, "It may seen that we are improving.", NewModalSeem(), 0)
}
