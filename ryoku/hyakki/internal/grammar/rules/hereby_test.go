package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHereby_Declare(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I here by declare this state to be free.", NewHereby(), "I hereby declare this state to be free.")
}

func TestHereby_AllowsHereByNoun(t *testing.T) {
	linttest.AssertNoLints(t, "I got here by skill.", NewHereby())
}
