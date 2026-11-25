package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDoublePossessiveNounPhrase_FixMyFirstPart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is my first part of my phrase.", NewDoublePossessiveNounPhrase(), "This is the first part of my phrase.")
}

func TestDoublePossessiveNounPhrase_FixYourSecondHalf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As you helpfully noted in your second half of your comment, high wealth, deliberately low income", NewDoublePossessiveNounPhrase(), "As you helpfully noted in the second half of your comment, high wealth, deliberately low income")
}

func TestDoublePossessiveNounPhrase_FixHisLatter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In his latter part of his life, he does talk of the thrill", NewDoublePossessiveNounPhrase(), "In the latter part of his life, he does talk of the thrill")
}

func TestDoublePossessiveNounPhrase_FixYourFirst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Use this bullet-point template to build out your first draft of your resume.", NewDoublePossessiveNounPhrase(), "Use this bullet-point template to build out the first draft of your resume.")
}

func TestDoublePossessiveNounPhrase_FixYourYear(t *testing.T) {
	linttest.AssertSuggestionResult(t, "benefits like 50% off your first year of your subscription", NewDoublePossessiveNounPhrase(), "benefits like 50% off the first year of your subscription")
}

func TestDoublePossessiveNounPhrase_DontFlagInformOf(t *testing.T) {
	linttest.AssertNoLints(t, "Nash can inform his former clients of his concerns", NewDoublePossessiveNounPhrase())
}

func TestDoublePossessiveNounPhrase_DontFlagNotifyOf(t *testing.T) {
	linttest.AssertNoLints(t, "it must notify your former spouse of his or her TCC rights", NewDoublePossessiveNounPhrase())
}

func TestDoublePossessiveNounPhrase_DontFlagTheirOfOur(t *testing.T) {
	linttest.AssertNoLints(t, "Ofgem published their final determination of our business plan.", NewDoublePossessiveNounPhrase())
}
