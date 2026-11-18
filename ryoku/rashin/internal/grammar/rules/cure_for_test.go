package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCureFor_CorrectsSimpleCureAgainst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Researchers sought a cure against the stubborn illness.", NewCureFor(), "Researchers sought a cure for the stubborn illness.")
}

func TestCureFor_CorrectsPluralCuresAgainst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Doctors insist this serum cures against the new variant.", NewCureFor(), "Doctors insist this serum cures for the new variant.")
}

func TestCureFor_CorrectsPastParticipleCuredAgainst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The remedy was cured against the infection last spring.", NewCureFor(), "The remedy was cured for the infection last spring.")
}

func TestCureFor_CorrectsUppercaseAgainst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We still trust the cure AGAINST the dreaded plague.", NewCureFor(), "We still trust the cure FOR the dreaded plague.")
}

func TestCureFor_CorrectsAtSentenceStart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Cure against that condition became the rallying cry.", NewCureFor(), "Cure for that condition became the rallying cry.")
}

func TestCureFor_DoesNotFlagCureFor(t *testing.T) {
	linttest.AssertLintCount(t, "They finally found a cure for the fever.", NewCureFor(), 0)
}

func TestCureFor_DoesNotFlagCureFrom(t *testing.T) {
	linttest.AssertLintCount(t, "A cure from this rare herb is on the horizon.", NewCureFor(), 0)
}

func TestCureFor_DoesNotFlagWithComma(t *testing.T) {
	linttest.AssertLintCount(t, "A cure, against all odds, appeared in the files.", NewCureFor(), 0)
}

func TestCureFor_DoesNotFlagUnrelatedAgainst(t *testing.T) {
	linttest.AssertLintCount(t, "Travelers stand against the roaring wind on the cliffs.", NewCureFor(), 0)
}

func TestCureFor_DoesNotFlagSecureAgainst(t *testing.T) {
	linttest.AssertLintCount(t, "The fortress stayed secure against the invaders.", NewCureFor(), 0)
}
