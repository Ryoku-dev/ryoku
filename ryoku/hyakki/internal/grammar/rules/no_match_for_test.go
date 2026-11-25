package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNoMatchFor_FixAgainst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Erlang was no match against my sweeping gale.", NewNoMatchFor(), "Erlang was no match for my sweeping gale.")
}

func TestNoMatchFor_FixTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My BW5 was no match to his BW7.", NewNoMatchFor(), "My BW5 was no match for his BW7.")
}

func TestNoMatchFor_FixOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This Attack Plane Was No Match Of Me So I Did This To Him", NewNoMatchFor(), "This Attack Plane Was No Match For Me So I Did This To Him")
}

func TestNoMatchFor_FixItsTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "cuz AI is bull crap and its no match to human voice", NewNoMatchFor(), "cuz AI is bull crap and its no match for human voice")
}

func TestNoMatchFor_FixImTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Im no match to you but like let me no what u think", NewNoMatchFor(), "Im no match for you but like let me no what u think")
}

func TestNoMatchFor_TheyreTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Theyre no match to late 60s early 70s sansuis.", NewNoMatchFor(), "Theyre no match for late 60s early 70s sansuis.")
}

func TestNoMatchFor_FixHesTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Even ouki on drinks with renpa said hes no match to him.", NewNoMatchFor(), "Even ouki on drinks with renpa said hes no match for him.")
}

func TestNoMatchFor_FixShesTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Izma tries to struggle but she's no match to your superior strength", NewNoMatchFor(), "Izma tries to struggle but she's no match for your superior strength")
}

func TestNoMatchFor_DontFixFor(t *testing.T) {
	linttest.AssertLintCount(t, "Type to search appears even there is no match for search term when autoFocus is true.", NewNoMatchFor(), 0)
}
