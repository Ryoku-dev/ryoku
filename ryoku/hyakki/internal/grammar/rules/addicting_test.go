package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAddicting_FixAddicting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is addicting like heroin.", NewAddicting(), "It is addictive like heroin.")
}

func TestAddicting_DontFlagAddictingObjectPronoun(t *testing.T) {
	linttest.AssertLintCount(t, "It is addicting me.", NewAddicting(), 0)
}

func TestAddicting_DontFlagAddictingReflexivePronoun(t *testing.T) {
	linttest.AssertLintCount(t, "He is addicting himself.", NewAddicting(), 0)
}

func TestAddicting_FixYetHighlyAddicting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The objective of the game is simple yet highly addicting, you start out with the four basic elements.", NewAddicting(), "The objective of the game is simple yet highly addictive, you start out with the four basic elements.")
}

func TestAddicting_DontFlagAddictingThemOn(t *testing.T) {
	linttest.AssertNoLints(t, "Helping humans on their daily tasks instead of addicting them on social networks of all sorts.", NewAddicting())
}

func TestAddicting_FixFindThingsAddictingMyself(t *testing.T) {
	t.Skip("False positive since `myself` is not an object pronoun in this construction")
	linttest.AssertSuggestionResult(t, "Yeah, I find taking the functional approach for these kinds of problems rather addicting myself :)", NewAddicting(), "Yeah, I find taking the functional approach for these kinds of problems rather addictive myself :)")
}

func TestAddicting_DontFixCoercedIntoAddictingThemselves(t *testing.T) {
	linttest.AssertNoLints(t, "The British, in another display of gunboat diplomacy, coerced countless innocent people into addicting themselves to opium.", NewAddicting())
}

func TestAddicting_FixAtEnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is addicting", NewAddicting(), "It is addictive")
}

func TestAddicting_FixBeforeComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "An addicting, side-scrolling, bouncing platform game - mzmousa/fruit-catcher.", NewAddicting(), "An addictive, side-scrolling, bouncing platform game - mzmousa/fruit-catcher.")
}

func TestAddicting_FixAtEndWithBang(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Name of the Game is \"Pappu Pakia\". It is super fun and addicting!", NewAddicting(), "Name of the Game is \"Pappu Pakia\". It is super fun and addictive!")
}

func TestAddicting_FixAtEndWithPeriod(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This game is kind of addicting. The fast pace and small initial tail size puts emphasis on control and reaction time.", NewAddicting(), "This game is kind of addictive. The fast pace and small initial tail size puts emphasis on control and reaction time.")
}

func TestAddicting_FixAtEndWithSemicolon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Powerful emotionally, Extremely addicting; Best game played in a long time, lost hours of time playing game without realizing", NewAddicting(), "Powerful emotionally, Extremely addictive; Best game played in a long time, lost hours of time playing game without realizing")
}

func TestAddicting_FixAtEndWithSemicolonNoCommasDueToChunkMode(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Extremely addicting; Best game played in a long time", NewAddicting(), "Extremely addictive; Best game played in a long time")
}
