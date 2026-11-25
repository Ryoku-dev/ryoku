package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOxymorons_DetectsAmateurExpert(t *testing.T) {
	linttest.AssertLintCount(t, "The amateur expert gave his opinion.", NewOxymorons(), 1)
}

func TestOxymorons_DetectsIncreasinglyLess(t *testing.T) {
	linttest.AssertLintCount(t, "The solution was increasingly less effective.", NewOxymorons(), 1)
}

func TestOxymorons_DetectsAdvancingBackwards(t *testing.T) {
	linttest.AssertLintCount(t, "The project is advancing backwards?", NewOxymorons(), 1)
}

func TestOxymorons_DetectsAlludesExplicitlyTo(t *testing.T) {
	linttest.AssertLintCount(t, "The report alludes explicitly to several issues.", NewOxymorons(), 1)
}

func TestOxymorons_DetectsExplicitlyAlludesTo(t *testing.T) {
	linttest.AssertLintCount(t, "The report explicitly alludes to several issues.", NewOxymorons(), 1)
}

func TestOxymorons_DoesNotFlagCleanText(t *testing.T) {
	linttest.AssertLintCount(t, "The expert provided clear advice.", NewOxymorons(), 0)
}

func TestOxymorons_LowercaseMatch(t *testing.T) {
	linttest.AssertLintCount(t, "the amateur expert is often unreliable.", NewOxymorons(), 1)
}

func TestOxymorons_PhraseWithExtraWhitespace(t *testing.T) {
	linttest.AssertLintCount(t, "An organized    mess was found.", NewOxymorons(), 1)
}

func TestOxymorons_PhraseSplitByLineBreak(t *testing.T) {
	linttest.AssertLintCount(t, "nonworking\nmother is not a term to be used.", NewOxymorons(), 1)
}

func TestOxymorons_DetectsStandardOption(t *testing.T) {
	linttest.AssertLintCount(t, "and that probably correlates with CD players becoming a standard option in cars", NewOxymorons(), 1)
}
