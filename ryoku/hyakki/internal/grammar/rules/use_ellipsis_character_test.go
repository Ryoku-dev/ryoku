package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestUseEllipsisCharacter_CorrectsBasicEllipsis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "...", NewUseEllipsisCharacter(), "…")
}

func TestUseEllipsisCharacter_CorrectsSentenceFinalEllipsis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wait...", NewUseEllipsisCharacter(), "Wait…")
}

func TestUseEllipsisCharacter_CorrectsEllipsisWithTrailingSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wait... now", NewUseEllipsisCharacter(), "Wait… now")
}

func TestUseEllipsisCharacter_CorrectsEllipsisInQuotes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "\"...\"", NewUseEllipsisCharacter(), "\"…\"")
}

func TestUseEllipsisCharacter_CorrectsEllipsisAfterWord(t *testing.T) {
	linttest.AssertSuggestionResult(t, "maybe...", NewUseEllipsisCharacter(), "maybe…")
}

func TestUseEllipsisCharacter_CorrectsEllipsisBeforeWord(t *testing.T) {
	linttest.AssertSuggestionResult(t, "...maybe", NewUseEllipsisCharacter(), "…maybe")
}

func TestUseEllipsisCharacter_CorrectsMultipleEllipses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... and ...", NewUseEllipsisCharacter(), "… and …")
}

func TestUseEllipsisCharacter_CorrectsAdjacentToPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wait...!", NewUseEllipsisCharacter(), "Wait…!")
}

func TestUseEllipsisCharacter_CorrectsParentheticalEllipsis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "(...) ", NewUseEllipsisCharacter(), "(…) ")
}

func TestUseEllipsisCharacter_AllowsUnicodeEllipsis(t *testing.T) {
	linttest.AssertNoLints(t, "…", NewUseEllipsisCharacter())
}

func TestUseEllipsisCharacter_AllowsUnicodeEllipsisInSentence(t *testing.T) {
	linttest.AssertNoLints(t, "Wait…", NewUseEllipsisCharacter())
}

func TestUseEllipsisCharacter_AllowsFourPeriods(t *testing.T) {
	linttest.AssertNoLints(t, "....", NewUseEllipsisCharacter())
}

func TestUseEllipsisCharacter_AllowsTwoPeriods(t *testing.T) {
	linttest.AssertNoLints(t, "..", NewUseEllipsisCharacter())
}

func TestUseEllipsisCharacter_AllowsLongPeriodRun(t *testing.T) {
	linttest.AssertNoLints(t, ".....", NewUseEllipsisCharacter())
}

func TestUseEllipsisCharacter_FlagsSingleThreePeriodEllipsisOnce(t *testing.T) {
	linttest.AssertLintCount(t, "Wait... now", NewUseEllipsisCharacter(), 1)
}
