package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThrowAway_CorrectsSimpleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We through away the old code.", NewThrowAway(), "We throw away the old code.")
}

func TestThrowAway_OffersPastTenseOption(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We through away the old code.", NewThrowAway(), "We threw away the old code.")
}

func TestThrowAway_CorrectsSentenceStartCapital(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Through away this document when you're done.", NewThrowAway(), "Throw away this document when you're done.")
}

func TestThrowAway_CorrectsAllCapsInstance(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Please THROUGH AWAY THE TRASH.", NewThrowAway(), "Please THROW AWAY THE TRASH.")
}

func TestThrowAway_CorrectsWithExtraWhitespace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We through  away the leftovers.", NewThrowAway(), "We throw  away the leftovers.")
}

func TestThrowAway_DoesNotFlagThrowAway(t *testing.T) {
	linttest.AssertNoLints(t, "They throw away the packaging every time.", NewThrowAway())
}

func TestThrowAway_DoesNotFlagThroughTunnel(t *testing.T) {
	linttest.AssertNoLints(t, "They walked through the tunnel away from danger.", NewThrowAway())
}

func TestThrowAway_FlagsMultipleOccurrences(t *testing.T) {
	linttest.AssertLintCount(t, "We through away the forks and through away the spoons.", NewThrowAway(), 2)
}

func TestThrowAway_DoesNotFlagThorough(t *testing.T) {
	linttest.AssertNoLints(t, "She gave the room a thorough, away-from-home cleaning.", NewThrowAway())
}

func TestThrowAway_CorrectsWithContraction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Don't through away your shot.", NewThrowAway(), "Don't throw away your shot.")
}

func TestThrowAway_DoesNotFlagSpreadWords(t *testing.T) {
	linttest.AssertNoLints(t, "They pushed through as the crowd moved away.", NewThrowAway())
}
