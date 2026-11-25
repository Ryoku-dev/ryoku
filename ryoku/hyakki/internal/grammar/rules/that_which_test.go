package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThatWhich_CatchesLowercase(t *testing.T) {
	linttest.AssertLintCount(t, "To reiterate, that that is cool is not uncool.", NewThatWhich(), 1)
}

func TestThatWhich_CatchesDifferentCases(t *testing.T) {
	linttest.AssertLintCount(t, "That that is cool is not uncool.", NewThatWhich(), 1)
}

func TestThatWhich_LikesCorrection(t *testing.T) {
	linttest.AssertLintCount(t, "To reiterate, that which is cool is not uncool.", NewThatWhich(), 0)
}
