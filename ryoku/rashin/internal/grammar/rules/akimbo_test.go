package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAkimbo_FixLegsAllSpaces(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm also a fan of the “ on the back and legs all a kimbo”.", NewAkimbo(), "I'm also a fan of the “ on the back and legs all akimbo”.")
}

func TestAkimbo_FixLegsHyphens(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I thought I'd have legs-a-kimbo, but not so.", NewAkimbo(), "I thought I'd have legs-akimbo, but not so.")
}

func TestAkimbo_FixLimbs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just lying there, limbs a kimbo.", NewAkimbo(), "Just lying there, limbs akimbo.")
}

func TestAkimbo_FixLetters(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Them syllables are all a-kimbo....", NewAkimbo(), "Them syllables are all akimbo....")
}
