package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAvoidCurses_DetectsShit(t *testing.T) {
	linttest.AssertLintCount(t, "He ate shit when he fell off the bike.", NewAvoidCurses(), 1)
}

func TestAvoidCurses_AllowsDickens1656(t *testing.T) {
	linttest.AssertLintCount(t, "Charles Dickens wrote many novels.", NewAvoidCurses(), 0)
}

func TestAvoidCurses_FixShit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "shit", NewAvoidCurses(), "crap")
}

func TestAvoidCurses_FixShitTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Shit", NewAvoidCurses(), "Crap")
}

func TestAvoidCurses_FixShitAllcaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SHIT", NewAvoidCurses(), "CRAP")
}

func TestAvoidCurses_FixFWordToAllAsterisks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "fuck those fucking fuckers", NewAvoidCurses(), "**** those ****ing ****ers")
}

func TestAvoidCurses_FixShitWithSingleAsterisk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "shit", NewAvoidCurses(), "sh*t")
}

func TestAvoidCurses_FixShiteAllCapsWithSingleAsterisk(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SHIT", NewAvoidCurses(), "SH*T")
}
