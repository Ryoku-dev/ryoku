package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCompoundSubjectI_CorrectsMyMotherAndMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My mother and me went to California.", NewCompoundSubjectI(), "My mother and I went to California.")
}

func TestCompoundSubjectI_CorrectsMyBrotherAndMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My brother and me would often go to the cinema.", NewCompoundSubjectI(), "My brother and I would often go to the cinema.")
}

func TestCompoundSubjectI_CorrectsYourFriendAndMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your friend and me are heading out.", NewCompoundSubjectI(), "Your friend and I are heading out.")
}

func TestCompoundSubjectI_CorrectsHerManagerAndMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Her manager and me have talked about it.", NewCompoundSubjectI(), "Her manager and I have talked about it.")
}

func TestCompoundSubjectI_CorrectsHisCatAndMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "His cat and me were inseparable.", NewCompoundSubjectI(), "His cat and I were inseparable.")
}

func TestCompoundSubjectI_CorrectsTheirKidsAndMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Their kids and me will play outside.", NewCompoundSubjectI(), "Their kids and I will play outside.")
}

func TestCompoundSubjectI_CorrectsOurNeighborAndMe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Our neighbor and me can help tomorrow.", NewCompoundSubjectI(), "Our neighbor and I can help tomorrow.")
}

func TestCompoundSubjectI_CorrectsWithQuotePrefix(t *testing.T) {
	linttest.AssertSuggestionResult(t, "\"My mother and me went to California,\" she said.", NewCompoundSubjectI(), "\"My mother and I went to California,\" she said.")
}

func TestCompoundSubjectI_CorrectsAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "MY BROTHER AND ME WILL HANDLE IT.", NewCompoundSubjectI(), "MY BROTHER AND I WILL HANDLE IT.")
}

func TestCompoundSubjectI_IgnoresBetweenYouAndMe(t *testing.T) {
	linttest.AssertLintCount(t, "Between you and me, this stays here.", NewCompoundSubjectI(), 0)
}

func TestCompoundSubjectI_IgnoresCommaAfterMe(t *testing.T) {
	linttest.AssertLintCount(t, "My mother and me, as usual, went to the park.", NewCompoundSubjectI(), 0)
}
