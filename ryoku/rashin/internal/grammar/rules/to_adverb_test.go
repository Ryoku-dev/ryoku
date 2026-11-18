package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestToAdverb_CorrectsToNeverTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Tom has decided to never to do that again.", NewToAdverb(), "Tom has decided to never do that again.")
}

func TestToAdverb_AlternativeMovesAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Tom has decided to never to do that again.", NewToAdverb(), "Tom has decided never to do that again.")
}

func TestToAdverb_CorrectsToMaybeTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The next step is to maybe to take a language class.", NewToAdverb(), "The next step is to maybe take a language class.")
}

func TestToAdverb_CorrectsToNotTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He tells the monitor to not to collect anything.", NewToAdverb(), "He tells the monitor to not collect anything.")
}

func TestToAdverb_CorrectsToJustTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She told me to just to keep the peace.", NewToAdverb(), "She told me to just keep the peace.")
}

func TestToAdverb_CorrectsToReallyTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They plan to really to push the release.", NewToAdverb(), "They plan to really push the release.")
}

func TestToAdverb_OffersTwoSuggestions(t *testing.T) {
	linttest.AssertSuggestionCount(t, "He agreed to probably to lead the effort.", NewToAdverb(), 2)
}

func TestToAdverb_AllowsSingleToWithAdverb(t *testing.T) {
	linttest.AssertLintCount(t, "He wants to always win the match.", NewToAdverb(), 0)
}

func TestToAdverb_CorrectsToQuicklyTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They hoped to quickly to solve it.", NewToAdverb(), "They hoped to quickly solve it.")
}

func TestToAdverb_IgnoresMissingVerbAfterSecondTo(t *testing.T) {
	linttest.AssertLintCount(t, "We tried to eventually to.", NewToAdverb(), 0)
}

func TestToAdverb_HandlesCapitalizedTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To Always to succeed is the goal.", NewToAdverb(), "To Always succeed is the goal.")
}
