package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestItWouldBe_FlagsSimpleShame(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think I would be a shame if this happened.", NewItWouldBe(), "I think it would be a shame if this happened.")
}
func TestItWouldBe_FlagsBelieveBummer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We believe I might not be a bummer after all.", NewItWouldBe(), "We believe it might not be a bummer after all.")
}
func TestItWouldBe_FlagsDoubtGoodIdea(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They doubt I will be a good idea for the team.", NewItWouldBe(), "They doubt it will be a good idea for the team.")
}
func TestItWouldBe_IgnoresCorrectIt(t *testing.T) {
	linttest.AssertLintCount(t, "I think it would be a shame if this happened.", NewItWouldBe(), 0)
}
func TestItWouldBe_IgnoresFirstPersonStatement(t *testing.T) {
	linttest.AssertLintCount(t, "I would be a good fit for the role.", NewItWouldBe(), 0)
}
