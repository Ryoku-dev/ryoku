package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCurrencyPlacement_Eof(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It was my last bill worth more than 4$.", NewCurrencyPlacement(), "It was my last bill worth more than $4.")
}

func TestCurrencyPlacement_BlogTitleAllowsCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "The Best $25 I Ever Spent", NewCurrencyPlacement(), 0)
}

func TestCurrencyPlacement_BlogTitle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Best 25$ I Ever Spent", NewCurrencyPlacement(), "The Best $25 I Ever Spent")
}

func TestCurrencyPlacement_BlogTitleCents(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Best ¢25 I Ever Spent", NewCurrencyPlacement(), "The Best 25¢ I Ever Spent")
}

func TestCurrencyPlacement_BlogTitleWithSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Best 25   $ I Ever Spent", NewCurrencyPlacement(), "The Best $25 I Ever Spent")
}

func TestCurrencyPlacement_MultipleDollarPlainEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They were either 25$ 24$ or 23$.", NewCurrencyPlacement(), "They were either $25 $24 or $23.")
}

func TestCurrencyPlacement_MultiplePound(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They were either 25£ 24£ or 23£.", NewCurrencyPlacement(), "They were either £25 £24 or £23.")
}

func TestCurrencyPlacement_YenAllowsCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "It cost ¥380.", NewCurrencyPlacement(), 0)
}

func TestCurrencyPlacement_MultipleYen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They were either 380¥ 24¥ or 23¥.", NewCurrencyPlacement(), "They were either ¥380 ¥24 or ¥23.")
}

func TestCurrencyPlacement_YenWithSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It cost ¥ 380.", NewCurrencyPlacement(), "It cost ¥380.")
}

func TestCurrencyPlacement_YenAfterWithSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It cost 380 ¥.", NewCurrencyPlacement(), "It cost ¥380.")
}

func TestCurrencyPlacement_Suffix(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It was my 20th$.", NewCurrencyPlacement(), "It was my $20th.")
}

func TestCurrencyPlacement_SevenEvenTwoDecimalClean(t *testing.T) {
	linttest.AssertLintCount(t, "$7.00", NewCurrencyPlacement(), 0)
}
