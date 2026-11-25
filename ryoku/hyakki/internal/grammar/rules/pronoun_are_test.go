package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPronounAre_FixesYouR(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You r absolutely right.", NewPronounAre(), "You are absolutely right.")
}

func TestPronounAre_OffersContractionOption(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You r absolutely right.", NewPronounAre(), "You're absolutely right.")
}

func TestPronounAre_KeepsUppercasePronoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "YOU r welcome here.", NewPronounAre(), "YOU ARE welcome here.")
}

func TestPronounAre_FixesTheyRWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They r, of course, arriving tomorrow.", NewPronounAre(), "They are, of course, arriving tomorrow.")
}

func TestPronounAre_FixesWeRLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we r ready now.", NewPronounAre(), "we are ready now.")
}

func TestPronounAre_FixesTheyRSentenceStart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They r planning ahead.", NewPronounAre(), "They are planning ahead.")
}

func TestPronounAre_FixesLowercaseSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "they r late again.", NewPronounAre(), "they are late again.")
}

func TestPronounAre_HandlesLineBreak(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We r\nready to go.", NewPronounAre(), "We are\nready to go.")
}

func TestPronounAre_DoesNotFlagContraction(t *testing.T) {
	linttest.AssertLintCount(t, "You're looking great.", NewPronounAre(), 0)
}

func TestPronounAre_DoesNotFlagFullForm(t *testing.T) {
	linttest.AssertLintCount(t, "They are excited about it.", NewPronounAre(), 0)
}

func TestPronounAre_IgnoresSimilarWord(t *testing.T) {
	linttest.AssertLintCount(t, "Your results impressed everyone.", NewPronounAre(), 0)
}
