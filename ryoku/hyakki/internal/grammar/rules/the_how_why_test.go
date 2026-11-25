package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTheHowWhy_BasicTheHow(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is the how it all started.", NewTheHowWhy(), "This is how it all started.")
}

func TestTheHowWhy_TheWhy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The important part is the why it matters.", NewTheHowWhy(), "The important part is why it matters.")
}

func TestTheHowWhy_SkipHowTo(t *testing.T) {
	linttest.AssertLintCount(t, "I'd like to explain the how to install this properly.", NewTheHowWhy(), 0)
}

func TestTheHowWhy_SkipWhosWho(t *testing.T) {
	linttest.AssertLintCount(t, "We covered the who's who of corporate leadership last time.", NewTheHowWhy(), 0)
}

func TestTheHowWhy_TheWho(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We must identify the who is responsible.", NewTheHowWhy(), "We must identify who is responsible.")
}

func TestTheHowWhy_TheWhen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He outlined the when the new phase will start.", NewTheHowWhy(), "He outlined when the new phase will start.")
}

func TestTheHowWhy_TheWhat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The presentation clarifies the what we intend to build.", NewTheHowWhy(), "The presentation clarifies what we intend to build.")
}

func TestTheHowWhy_NoFalsePositive(t *testing.T) {
	linttest.AssertLintCount(t, "These tips examine the how to fix your code quickly, plus the what's next.", NewTheHowWhy(), 0)
}
