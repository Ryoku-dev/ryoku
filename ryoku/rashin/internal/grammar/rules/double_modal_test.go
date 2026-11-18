package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDoubleModal_DetectsMightCould(t *testing.T) {
	linttest.AssertLintCount(t, "They might could finish the project by Friday.", NewDoubleModal(), 1)
}

func TestDoubleModal_DetectsShouldOught(t *testing.T) {
	linttest.AssertLintCount(t, "You should ought to apologize.", NewDoubleModal(), 1)
}

func TestDoubleModal_AllowsSingleModal(t *testing.T) {
	linttest.AssertLintCount(t, "She must leave early.", NewDoubleModal(), 0)
}

func TestDoubleModal_DetectsTwoDoubleModals(t *testing.T) {
	linttest.AssertLintCount(t, "He may can join us, and you might could too.", NewDoubleModal(), 2)
}

func TestDoubleModal_SuggestsRemovingSecondModalKeepsFirst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They might could finish the project by Friday.", NewDoubleModal(), "They might finish the project by Friday.")
}

func TestDoubleModal_SuggestsRemovingSecondModalKeepsFirstVariantOrder(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You could might want to double-check that.", NewDoubleModal(), "You could want to double-check that.")
}

func TestDoubleModal_SuggestsRemovingSecondModalKeepsFirstCapitalised(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We Must Should be consistent.", NewDoubleModal(), "We Must be consistent.")
}

func TestDoubleModal_AllowsWillNeed(t *testing.T) {
	linttest.AssertNoLints(t, "You will need administrator or editor level access", NewDoubleModal())
}
