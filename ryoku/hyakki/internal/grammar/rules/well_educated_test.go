package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWellEducated_CorrectsSimpleSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She is good-educated.", NewWellEducated(), "She is well-educated.")
}

func TestWellEducated_CorrectsInClause(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The panel found him good-educated and articulate.", NewWellEducated(), "The panel found him well-educated and articulate.")
}

func TestWellEducated_CorrectsWithModifier(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They considered her very good-educated for her age.", NewWellEducated(), "They considered her very well-educated for her age.")
}

func TestWellEducated_CorrectsAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Their mentors are GOOD-EDUCATED leaders.", NewWellEducated(), "Their mentors are WELL-EDUCATED leaders.")
}

func TestWellEducated_CorrectsTitleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The report lauded Good-Educated Candidates.", NewWellEducated(), "The report lauded Well-Educated Candidates.")
}

func TestWellEducated_CorrectsWithQuotes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He called them \"good-educated\" professionals.", NewWellEducated(), "He called them \"well-educated\" professionals.")
}

func TestWellEducated_CorrectsSplitTokens(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Their children are good - educated despite the odds.", NewWellEducated(), "Their children are well-educated despite the odds.")
}

func TestWellEducated_AllowsWellEducated(t *testing.T) {
	linttest.AssertLintCount(t, "She is well-educated.", NewWellEducated(), 0)
}

func TestWellEducated_AllowsGoodEducationPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "They received a good education.", NewWellEducated(), 0)
}

func TestWellEducated_AllowsGoodToBeEducated(t *testing.T) {
	linttest.AssertLintCount(t, "It is good to be educated about local history.", NewWellEducated(), 0)
}
