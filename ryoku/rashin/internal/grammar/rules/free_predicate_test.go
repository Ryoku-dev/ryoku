package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFreePredicate_CorrectsIsFeeFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The trial is fee for new members.", NewFreePredicate(), "The trial is free for new members.")
}

func TestFreePredicate_CorrectsTotallyFee(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Customer support is totally fee.", NewFreePredicate(), "Customer support is totally free.")
}

func TestFreePredicate_CorrectsReallyFeeTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The workshop is really fee to attend.", NewFreePredicate(), "The workshop is really free to attend.")
}

func TestFreePredicate_CorrectsFeeWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Our platform is fee, and always available.", NewFreePredicate(), "Our platform is free, and always available.")
}

func TestFreePredicate_CorrectsFeePeriod(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Access is fee.", NewFreePredicate(), "Access is free.")
}

func TestFreePredicate_CorrectsFeePastTense(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The program was fee for nonprofits.", NewFreePredicate(), "The program was free for nonprofits.")
}

func TestFreePredicate_AllowsFeeBased(t *testing.T) {
	linttest.AssertNoLints(t, "The pricing model is fee-based.", NewFreePredicate())
}

func TestFreePredicate_AllowsFeePaying(t *testing.T) {
	linttest.AssertNoLints(t, "The membership is fee-paying.", NewFreePredicate())
}

func TestFreePredicate_AllowsFeeScheduleStatement(t *testing.T) {
	linttest.AssertNoLints(t, "This plan has a fee for standard support.", NewFreePredicate())
}

func TestFreePredicate_AllowsFeeFreePhrase(t *testing.T) {
	linttest.AssertNoLints(t, "Our service is fee-free for students.", NewFreePredicate())
}

func TestFreePredicate_CountsSingleLint(t *testing.T) {
	linttest.AssertLintCount(t, "The upgrade is fee for existing users.", NewFreePredicate(), 1)
}
