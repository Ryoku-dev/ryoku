package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDidnt_CorrectsIDint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I dint lock the gate.", NewDidnt(), "I didn't lock the gate.")
}

func TestDidnt_CorrectsYouDint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You dint look this way.", NewDidnt(), "You didn't look this way.")
}

func TestDidnt_CorrectsHeDint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He dint see the sign.", NewDidnt(), "He didn't see the sign.")
}

func TestDidnt_CorrectsSheDint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She dint call me back.", NewDidnt(), "She didn't call me back.")
}

func TestDidnt_CorrectsWeDint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We dint sleep much.", NewDidnt(), "We didn't sleep much.")
}

func TestDidnt_CorrectsTheyDint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They dint enjoy the show.", NewDidnt(), "They didn't enjoy the show.")
}

func TestDidnt_CorrectsItDint(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It dint rain today.", NewDidnt(), "It didn't rain today.")
}

func TestDidnt_DoesNotFlagDintNoun(t *testing.T) {
	linttest.AssertNoLints(t, "The blow left a small dint in the metal.", NewDidnt())
}

func TestDidnt_DoesNotFlagQuotedDint(t *testing.T) {
	linttest.AssertNoLints(t, "He muttered 'dint' under his breath.", NewDidnt())
}

func TestDidnt_DoesNotFlagPastTenseWithNot(t *testing.T) {
	linttest.AssertLintCount(t, "I did not lock the gate.", NewDidnt(), 0)
}
