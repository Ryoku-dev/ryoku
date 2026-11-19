package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWrongNegative_FixInsecured(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Problem with insecured chart repositories", NewWrongNegative(dict.Curated()), "Problem with unsecured chart repositories")
}

func TestWrongNegative_FixUnsecure(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Our one and only host is showing up as unsecure, but connected.", NewWrongNegative(dict.Curated()), "Our one and only host is showing up as insecure, but connected.")
}

func TestWrongNegative_FixUnpossible(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This System makes a individual design unpossible.", NewWrongNegative(dict.Curated()), "This System makes a individual design impossible.")
}

func TestWrongNegative_FixUnvisible(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sure, I'm using \"deleted\" here to mean a third state more unvisible than \"yanked\"", NewWrongNegative(dict.Curated()), "Sure, I'm using \"deleted\" here to mean a third state more invisible than \"yanked\"")
}

func TestWrongNegative_DontFlagDefo(t *testing.T) {
	linttest.AssertNoLints(t, "I defo used MSVC back then too", NewWrongNegative(dict.Curated()))
}

func TestWrongNegative_DontFlagDeus(t *testing.T) {
	linttest.AssertNoLints(t, "That deus ex machine stuff seems like enormous cope to me", NewWrongNegative(dict.Curated()))
}
