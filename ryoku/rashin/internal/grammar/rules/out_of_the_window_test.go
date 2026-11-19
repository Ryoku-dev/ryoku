package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOutOfTheWindow_UsEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The whole Gnome session goes out of the window instantly, both upon explicit screen locking and upon automatic locking after a timeout.", NewOutOfTheWindow(dict.American), "The whole Gnome session goes out the window instantly, both upon explicit screen locking and upon automatic locking after a timeout.")
}

func TestOutOfTheWindow_UkEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "determinism went out the window, everything is terrible", NewOutOfTheWindow(dict.British), "determinism went out of the window, everything is terrible")
}

func TestOutOfTheWindow_InEnglishDoesntFlagWithOf(t *testing.T) {
	linttest.AssertNoLints(t, "errors and orchestration in general all go out of the window", NewOutOfTheWindow(dict.Indian))
}

func TestOutOfTheWindow_InEnglishDoesntFlagWithoutOf(t *testing.T) {
	linttest.AssertNoLints(t, "I was so excited to develop my new app in .NET MAUI but then all this excitement went out the window due to two things stability and tooling.", NewOutOfTheWindow(dict.Indian))
}

func TestOutOfTheWindow_GivesTheRightMessageForCanadianEnglish(t *testing.T) {
	linttest.AssertLintMessage(t, "This all works great unless I use middlware and then it all goes out of the window.", NewOutOfTheWindow(dict.Canadian), "If this is the idiom about abandoning a plan, it's more usual to leave out `of` in Canadian English")
}
