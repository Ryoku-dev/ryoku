package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestGoMissing_FixBecomeMissing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the step where chroma/saturation is checked and hue might become missing", NewGoMissing(), "the step where chroma/saturation is checked and hue might go missing")
}

func TestGoMissing_FixBecomeMissingTitleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How Do Quilts Become Missing?", NewGoMissing(), "How Do Quilts Go Missing?")
}

func TestGoMissing_FixBecomesMissing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One of them becomes missing, but since the new one was already in the library", NewGoMissing(), "One of them goes missing, but since the new one was already in the library")
}

func TestGoMissing_FixBecomingMissing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... could have confusing semantics of flows becoming missing arbitrarily", NewGoMissing(), "... could have confusing semantics of flows going missing arbitrarily")
}

func TestGoMissing_FixBecameMissing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the 'data-target' attribute from the link became missing", NewGoMissing(), "the 'data-target' attribute from the link went missing")
}

func TestGoMissing_FixHasBecomeMissing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Consider it correct that a reference to an asset that has become missing once is not restored.", NewGoMissing(), "Consider it correct that a reference to an asset that has become missing once is not restored.")
}

func TestGoMissing_FixHaveBecomeMissing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... and values that may have become missing ...", NewGoMissing(), "... and values that may have become missing ...")
}

func TestGoMissing_DontFlagMissingPersons(t *testing.T) {
	linttest.AssertNoLints(t, "Prevent Them Becoming Missing Persons", NewGoMissing())
}

func TestGoMissing_DontFlagMissingChildren(t *testing.T) {
	linttest.AssertNoLints(t, "and unaccompanied children who become missing children", NewGoMissing())
}

func TestGoMissing_DontFlagMissingChild(t *testing.T) {
	linttest.AssertNoLints(t, "She became Missing Child Number 1201258.", NewGoMissing())
}
