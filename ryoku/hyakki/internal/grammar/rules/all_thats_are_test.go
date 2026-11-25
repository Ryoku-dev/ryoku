package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAllThatsAre_ThatIsCommunicable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That is, all that is communicable are differences that make a difference, or information states.", NewAllThatsAre(), "That is, all that is communicable is differences that make a difference, or information states.")
}

func TestAllThatsAre_ThatIsMissing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "All that is missing are the washerwomen", NewAllThatsAre(), "All that is missing is the washerwomen")
}

func TestAllThatsAre_ThatIsNeeded(t *testing.T) {
	linttest.AssertSuggestionResult(t, "All that is needed are angle brackets.", NewAllThatsAre(), "All that is needed is angle brackets.")
}

func TestAllThatsAre_ThatsNeeded(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we now have a mechanically sound Golf GTI and all that's needed are some important finishing touches", NewAllThatsAre(), "we now have a mechanically sound Golf GTI and all that's needed is some important finishing touches")
}

func TestAllThatsAre_ThatsRequired(t *testing.T) {
	linttest.AssertSuggestionResult(t, "All that's required are 7 ice cream sandwiches and a cup of milk.", NewAllThatsAre(), "All that's required is 7 ice cream sandwiches and a cup of milk.")
}
