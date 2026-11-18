package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWrongApostrophe_FixDontWithSemicolonToApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's better if you don;t type like this.", NewWrongApostrophe(), "It's better if you don't type like this.")
}

func TestWrongApostrophe_IgnoreCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "I don't doubt it.", NewWrongApostrophe(), 0)
}

func TestWrongApostrophe_FixTitleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Don;t type like this.", NewWrongApostrophe(), "Don't type like this.")
}

func TestWrongApostrophe_FixAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "DON;T TRY THIS AT HOME.", NewWrongApostrophe(), "DON'T TRY THIS AT HOME.")
}

func TestWrongApostrophe_FixIllAndMonkeys(t *testing.T) {
	t.Skip("replace_with_match_case has a bug turning `I'll` into `I'LL`")
	linttest.AssertSuggestionResult(t, "Well I;ll be a monkey;s uncle!", NewWrongApostrophe(), "Well I'll be a monkey's uncle!")
}

func TestWrongApostrophe_FixOtherContractionsAndPossessives(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Let;s see if we;ve fixed patrakov;s bug. Fun wasn;t it?", NewWrongApostrophe(), "Let's see if we've fixed patrakov's bug. Fun wasn't it?")
}

func TestWrongApostrophe_CorrectsIveWithCorrectCapitalization(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I;ve", NewWrongApostrophe(), "I've")
}

func TestWrongApostrophe_FixAcuteDont(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To see the list of available bikes for a location, you don´t need any authentication.", NewWrongApostrophe(), "To see the list of available bikes for a location, you don't need any authentication.")
}

func TestWrongApostrophe_FixAcuteIm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In my research, I´m applying the latest generation of quantitative methods in epidemiology", NewWrongApostrophe(), "In my research, I'm applying the latest generation of quantitative methods in epidemiology")
}

func TestWrongApostrophe_FixAcuteIts(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and it´s auto-updated if that project is hosted here on github", NewWrongApostrophe(), "and it's auto-updated if that project is hosted here on github")
}

func TestWrongApostrophe_FixAcuteLets(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Let´s now visit the main functionalities provided by GrimoireLab.", NewWrongApostrophe(), "Let's now visit the main functionalities provided by GrimoireLab.")
}

func TestWrongApostrophe_FixAcuteMicrosofts(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Windows 11 Upgrade tool that bypasses new Microsoft´s requirements", NewWrongApostrophe(), "Windows 11 Upgrade tool that bypasses new Microsoft's requirements")
}

func TestWrongApostrophe_FixAcuteYoure(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You´re looking for clues, but you´re missing all the signs", NewWrongApostrophe(), "You're looking for clues, but you're missing all the signs")
}

func TestWrongApostrophe_FixBackticksUsedAsApostrophes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And It Won`T Make One Bit Of Difference If I Answer Right Or Wrong. When You`Re Rich, They Think You Really Know", NewWrongApostrophe(), "And It Won'T Make One Bit Of Difference If I Answer Right Or Wrong. When You'Re Rich, They Think You Really Know")
}
