package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDashes_CatchesEnDash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "pre--Industrial Revolution", NewDashes(), "pre–Industrial Revolution")
}

func TestDashes_CatchesEmDash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "'There is no box' --- Scott", NewDashes(), "'There is no box' — Scott")
}

func TestDashes_NoOverlaps(t *testing.T) {
	linttest.AssertSuggestionCount(t, "'There is no box' --- Scott", NewDashes(), 1)
}

func TestDashes_NoLintForLongHyphenSequences(t *testing.T) {
	linttest.AssertSuggestionCount(t, "'There is no box' ------ Scott", NewDashes(), 0)
}
