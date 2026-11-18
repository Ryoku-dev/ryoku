package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCautionaryTale_CatchesCautionaryTail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It serves as a cautionary tail.", NewCautionaryTale(), "It serves as a cautionary tale.")
}

func TestCautionaryTale_CatchesInspirationalTail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Her journey is an inspirational tail of perseverance.", NewCautionaryTale(), "Her journey is an inspirational tale of perseverance.")
}

func TestCautionaryTale_CatchesCapitalizedCautionaryTail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The article discusses a Cautionary Tail about privacy.", NewCautionaryTale(), "The article discusses a Cautionary Tale about privacy.")
}

func TestCautionaryTale_CatchesUppercaseCautionaryTail(t *testing.T) {
	linttest.AssertSuggestionResult(t, "THE STORY IS A CAUTIONARY TAIL.", NewCautionaryTale(), "THE STORY IS A CAUTIONARY TALE.")
}

func TestCautionaryTale_CatchesMixedCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This serves as an inspirational Tail for all.", NewCautionaryTale(), "This serves as an inspirational Tale for all.")
}

func TestCautionaryTale_AllowsActualTail(t *testing.T) {
	linttest.AssertLintCount(t, "The dog wagged its tail happily.", NewCautionaryTale(), 0)
}

func TestCautionaryTale_AllowsDifferentAdjectiveWithTail(t *testing.T) {
	linttest.AssertLintCount(t, "The cat has a long tail.", NewCautionaryTale(), 0)
}

func TestCautionaryTale_AllowsCorrectTale(t *testing.T) {
	linttest.AssertLintCount(t, "It serves as a cautionary tale.", NewCautionaryTale(), 0)
}

func TestCautionaryTale_AllowsInspirationalTale(t *testing.T) {
	linttest.AssertLintCount(t, "Her story is an inspirational tale.", NewCautionaryTale(), 0)
}

func TestCautionaryTale_CatchesInLongerText(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The movie presents a cautionary tail about the dangers of AI. It's really scary.", NewCautionaryTale(), "The movie presents a cautionary tale about the dangers of AI. It's really scary.")
}

func TestCautionaryTale_CatchesMultipleOccurrences(t *testing.T) {
	linttest.AssertLintCount(t, "This cautionary tail is also an inspirational tail about overcoming adversity.", NewCautionaryTale(), 2)
}

func TestCautionaryTale_AllowsTailInDifferentContext(t *testing.T) {
	linttest.AssertLintCount(t, "The inspirational speaker told the tale of a dog's tail.", NewCautionaryTale(), 0)
}

func TestCautionaryTale_CatchesAtStartOfSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Cautionary tail: don't trust strangers.", NewCautionaryTale(), "Cautionary tale: don't trust strangers.")
}
