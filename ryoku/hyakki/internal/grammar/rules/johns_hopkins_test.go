package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestJohnsHopkins_CorrectsUniversityReference(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I applied to John Hopkins University last fall.", NewJohnsHopkins(), "I applied to Johns Hopkins University last fall.")
}

func TestJohnsHopkins_CorrectsHospitalReference(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She works at the John Hopkins hospital.", NewJohnsHopkins(), "She works at the Johns Hopkins hospital.")
}

func TestJohnsHopkins_CorrectsStandaloneName(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We toured John Hopkins yesterday.", NewJohnsHopkins(), "We toured Johns Hopkins yesterday.")
}

func TestJohnsHopkins_CorrectsLowercaseUsage(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I studied at john hopkins online.", NewJohnsHopkins(), "I studied at johns hopkins online.")
}

func TestJohnsHopkins_CorrectsAcrossNewlineWhitespace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We met at John\nHopkins for lunch.", NewJohnsHopkins(), "We met at Johns\nHopkins for lunch.")
}

func TestJohnsHopkins_CorrectsWithTrailingPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I toured John Hopkins, and it was great.", NewJohnsHopkins(), "I toured Johns Hopkins, and it was great.")
}

func TestJohnsHopkins_CorrectsBeforeHyphenatedUnit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She joined the John Hopkins-affiliated lab.", NewJohnsHopkins(), "She joined the Johns Hopkins-affiliated lab.")
}

func TestJohnsHopkins_AllowsCorrectSpelling(t *testing.T) {
	linttest.AssertLintCount(t, "Johns Hopkins University has a great program.", NewJohnsHopkins(), 0)
}

func TestJohnsHopkins_AllowsApostrophizedForm(t *testing.T) {
	linttest.AssertLintCount(t, "John Hopkins's novel won awards.", NewJohnsHopkins(), 0)
}

func TestJohnsHopkins_AllowsReversedNameOrder(t *testing.T) {
	linttest.AssertLintCount(t, "Hopkins, John is a contact.", NewJohnsHopkins(), 0)
}
