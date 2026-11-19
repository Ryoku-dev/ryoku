package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWayTooAdjective_CorrectsWayToFast(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You drive way to fast.", NewWayTooAdjective(), "You drive way too fast.")
}
func TestWayTooAdjective_CorrectsWayToComplicated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think this would be way to complicated to implement.", NewWayTooAdjective(), "I think this would be way too complicated to implement.")
}
func TestWayTooAdjective_CorrectsWayToMuch(t *testing.T) {
	linttest.AssertSuggestionResult(t, "…and ate way to much.", NewWayTooAdjective(), "…and ate way too much.")
}
func TestWayTooAdjective_AllowsFastWayToTest(t *testing.T) {
	linttest.AssertLintCount(t, "Fast way to test daily defence teams?", NewWayTooAdjective(), 0)
}
