package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSoonToBe_HyphenatesPossessivePhrase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We met his soon to be boss at lunch.", NewSoonToBe(), "We met his soon-to-be boss at lunch.")
}

func TestSoonToBe_HyphenatesArticlePhrase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They toasted the soon to be couple.", NewSoonToBe(), "They toasted the soon-to-be couple.")
}

func TestSoonToBe_HyphenatesSentenceStart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Soon to be parents filled the classroom.", NewSoonToBe(), "Soon-to-be parents filled the classroom.")
}

func TestSoonToBe_AllowsExistingHyphens(t *testing.T) {
	linttest.AssertNoLints(t, "We met his soon-to-be boss yesterday.", NewSoonToBe())
}

func TestSoonToBe_KeepsNonAdjectivalUse(t *testing.T) {
	linttest.AssertNoLints(t, "The concert is soon to be over.", NewSoonToBe())
}

func TestSoonToBe_HyphenatesWithAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Our soon to be newly married friends visited.", NewSoonToBe(), "Our soon-to-be newly married friends visited.")
}

func TestSoonToBe_HyphenatesHyphenatedNumberPhrase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Our soon to be 5-year-old son starts school.", NewSoonToBe(), "Our soon-to-be 5-year-old son starts school.")
}

func TestSoonToBe_HyphenatesInLawPhrase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She thanked her soon to be in-laws for hosting.", NewSoonToBe(), "She thanked her soon-to-be in-laws for hosting.")
}

func TestSoonToBe_HyphenatesFutureEvent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We reserved space for our soon to be celebration.", NewSoonToBe(), "We reserved space for our soon-to-be celebration.")
}

func TestSoonToBe_IgnoresMisalignedVerbChain(t *testing.T) {
	linttest.AssertLintCount(t, "They will soon to be moving overseas.", NewSoonToBe(), 0)
}

func TestSoonToBe_HyphenatesGuestExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I cooked for my soon to be guests.", NewSoonToBe(), "I cooked for my soon-to-be guests.")
}

func TestSoonToBe_IgnoresRearrangedPhrase(t *testing.T) {
	linttest.AssertNoLints(t, "We hope to soon be home.", NewSoonToBe())
}
