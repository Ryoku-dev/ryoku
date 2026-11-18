package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestViceVersa_CorrectsBasicHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We swapped the arguments vice-versa this time.", NewViceVersa(), "We swapped the arguments vice versa this time.")
}

func TestViceVersa_CorrectsLeadingCapitalization(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Vice-Versa, the movie, was interesting.", NewViceVersa(), "Vice versa, the movie, was interesting.")
}

func TestViceVersa_CorrectsAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They agreed VICE-VERSA on the clause.", NewViceVersa(), "They agreed VICE VERSA on the clause.")
}

func TestViceVersa_CorrectsWithExtraA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The logic works vice-a-versa as well.", NewViceVersa(), "The logic works vice versa as well.")
}

func TestViceVersa_CorrectsViseVariant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The rule applies vise-versa too.", NewViceVersa(), "The rule applies vice versa too.")
}

func TestViceVersa_CorrectsViseExtraAVariant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The rule applies Vise-A-Versa too.", NewViceVersa(), "The rule applies Vice versa too.")
}

func TestViceVersa_CorrectsWithTrailingSuffix(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That was a vice-versa-like transformation.", NewViceVersa(), "That was a vice versa-like transformation.")
}

func TestViceVersa_AllowsCorrectSpelling(t *testing.T) {
	linttest.AssertLintCount(t, "We swapped the arguments vice versa this time.", NewViceVersa(), 0)
}

func TestViceVersa_AllowsSentenceCase(t *testing.T) {
	linttest.AssertLintCount(t, "Vice versa, the movie, was interesting.", NewViceVersa(), 0)
}

func TestViceVersa_DoesNotFlagUnrelatedWords(t *testing.T) {
	linttest.AssertLintCount(t, "Their service-versa mapping was custom.", NewViceVersa(), 0)
}
