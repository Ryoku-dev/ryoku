package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCorrectNumberSuffix_PassesCorrectCases(t *testing.T) {
	linttest.AssertLintCount(t, "2nd", NewCorrectNumberSuffix(), 0)
	linttest.AssertLintCount(t, "101st", NewCorrectNumberSuffix(), 0)
	linttest.AssertLintCount(t, "1012th", NewCorrectNumberSuffix(), 0)
}

func TestCorrectNumberSuffix_DetectsIncorrectCases(t *testing.T) {
	linttest.AssertLintCount(t, "2st", NewCorrectNumberSuffix(), 1)
	linttest.AssertLintCount(t, "101nd", NewCorrectNumberSuffix(), 1)
	linttest.AssertLintCount(t, "1012rd", NewCorrectNumberSuffix(), 1)
}
