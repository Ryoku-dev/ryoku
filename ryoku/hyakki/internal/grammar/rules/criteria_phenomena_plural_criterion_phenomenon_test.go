package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCriteriaPhenomenaPluralCriterionPhenomenon_DetectsReportedNamesakeExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To embody this original goal, I wanted to make the project a namesake of an author known for their literary devices and prose. I also had three other (practical) criterion:", NewPluralCriterionPhenomenon(), "To embody this original goal, I wanted to make the project a namesake of an author known for their literary devices and prose. I also had three other (practical) criteria:")
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_DetectsPluralNumberWithSingularCriterion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I also had three other (practical) criterion.", NewPluralCriterionPhenomenon(), "I also had three other (practical) criteria.")
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_DetectsDigitWithSingularCriterion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We used 3 criterion.", NewPluralCriterionPhenomenon(), "We used 3 criteria.")
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_DetectsTheseCriterion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "These criterion matter.", NewPluralCriterionPhenomenon(), "These criteria matter.")
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_DetectsThoseCriterion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Those criterion failed.", NewPluralCriterionPhenomenon(), "Those criteria failed.")
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_DetectsSeveralPhenomenon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Several strange phenomenon appeared.", NewPluralCriterionPhenomenon(), "Several strange phenomena appeared.")
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_DetectsMultipleAdjectivesBeforeSingularNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Two other practical review criterion failed.", NewPluralCriterionPhenomenon(), "Two other practical review criteria failed.")
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_DetectsParentheticalPhenomenon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Many other (rare) phenomenon appeared.", NewPluralCriterionPhenomenon(), "Many other (rare) phenomena appeared.")
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_DetectsHyphenatedInterveningWords(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Both long-term criterion changed.", NewPluralCriterionPhenomenon(), "Both long-term criteria changed.")
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_AllowsCorrectPluralCriteria(t *testing.T) {
	linttest.AssertLintCount(t, "Three other practical criteria were considered.", NewPluralCriterionPhenomenon(), 0)
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_AllowsCorrectPluralPhenomena(t *testing.T) {
	linttest.AssertLintCount(t, "Many rare phenomena were on display.", NewPluralCriterionPhenomenon(), 0)
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_AllowsCorrectSingularCriterion(t *testing.T) {
	linttest.AssertLintCount(t, "That criterion is important.", NewPluralCriterionPhenomenon(), 0)
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_AllowsCorrectSingularPhenomenon(t *testing.T) {
	linttest.AssertLintCount(t, "One strange phenomenon appeared.", NewPluralCriterionPhenomenon(), 0)
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_AllowsTheOtherCriterion(t *testing.T) {
	linttest.AssertLintCount(t, "The other criterion was practical.", NewPluralCriterionPhenomenon(), 0)
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_AllowsSingularNumberWithSingularCriterion(t *testing.T) {
	linttest.AssertLintCount(t, "1 criterion was enough.", NewPluralCriterionPhenomenon(), 0)
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_AllowsComparativeModifierWithSingularPhenomenon(t *testing.T) {
	linttest.AssertLintCount(t, "To the wingless a more interesting phenomenon is their dissimilarity.", NewPluralCriterionPhenomenon(), 0)
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_AllowsAmbiguousQuantifierWithSingularCriterion(t *testing.T) {
	linttest.AssertLintCount(t, "No criterion was enough.", NewPluralCriterionPhenomenon(), 0)
}

func TestCriteriaPhenomenaPluralCriterionPhenomenon_AllowsPluralModifierTooFarAway(t *testing.T) {
	linttest.AssertLintCount(t, "Three teams reviewed the policy before the criterion changed.", NewPluralCriterionPhenomenon(), 0)
}
