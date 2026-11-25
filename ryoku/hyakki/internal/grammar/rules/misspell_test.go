package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMisspell_BaseForm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They often miss spell names in the log.", NewMisspell(), "They often misspell names in the log.")
}

func TestMisspell_PastTense(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She miss spelled the answer on the quiz.", NewMisspell(), "She misspelled the answer on the quiz.")
}

func TestMisspell_PastTenseHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She miss-spelled the answer on the quiz.", NewMisspell(), "She misspelled the answer on the quiz.")
}

func TestMisspell_GerundForm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "His constant miss spelling frustrated the team.", NewMisspell(), "His constant misspelling frustrated the team.")
}
