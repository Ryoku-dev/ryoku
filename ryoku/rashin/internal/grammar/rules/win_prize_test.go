package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWinPrize_FixPriceSingular(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lena won a price in the coding marathon.", NewWinPrize(), "Lena won a prize in the coding marathon.")
}

func TestWinPrize_FixPricePlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Our team won the prices announced yesterday.", NewWinPrize(), "Our team won the prizes announced yesterday.")
}

func TestWinPrize_FixPriseSingular(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He finally won the prise he'd dreamed of.", NewWinPrize(), "He finally won the prize he'd dreamed of.")
}

func TestWinPrize_FixPrisePlural(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The inventors won several prises at the expo.", NewWinPrize(), "The inventors won several prizes at the expo.")
}

func TestWinPrize_IgnoreCorrectPrize(t *testing.T) {
	linttest.AssertLintCount(t, "Miranda won the grand prize last year.", NewWinPrize(), 0)
}

func TestWinPrize_FixNoDet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I won prices!", NewWinPrize(), "I won prizes!")
}
