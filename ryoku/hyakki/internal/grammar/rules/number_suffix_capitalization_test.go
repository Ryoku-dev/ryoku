package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNumberSuffixCapitalization_DetectsUppercaseSuffix(t *testing.T) {
	linttest.AssertLintCount(t, "2ND", NewNumberSuffixCapitalization(), 1)
}

func TestNumberSuffixCapitalization_DetectsInconsistentSuffix(t *testing.T) {
	linttest.AssertLintCount(t, "2nD", NewNumberSuffixCapitalization(), 1)
}

func TestNumberSuffixCapitalization_PassesCorrectCase(t *testing.T) {
	linttest.AssertLintCount(t, "2nd", NewNumberSuffixCapitalization(), 0)
}
