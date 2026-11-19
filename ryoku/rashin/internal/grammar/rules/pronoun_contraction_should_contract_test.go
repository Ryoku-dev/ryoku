package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPronounContractionShouldContract_ContractsYourCorrectly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "your the best", NewShouldContract(), "you're the best")
}

func TestPronounContractionShouldContract_ContractsWereComplexCorrectly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "were a good team", NewShouldContract(), "we're a good team")
}

func TestPronounContractionShouldContract_CaseInsensitiveHandling(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your the best", NewShouldContract(), "You're the best")
}

func TestPronounContractionShouldContract_NoMatchWithoutThe(t *testing.T) {
	linttest.AssertLintCount(t, "your best", NewShouldContract(), 0)
	linttest.AssertLintCount(t, "were best", NewShouldContract(), 0)
}

func TestPronounContractionShouldContract_NoMatchWithPunctuation(t *testing.T) {
	linttest.AssertLintCount(t, "your, the best", NewShouldContract(), 0)
}

func TestPronounContractionShouldContract_AllowNorm(t *testing.T) {
	linttest.AssertLintCount(t, "Let's start this story by going back to the dark ages before internet applications were the norm.", NewShouldContract(), 0)
}

func TestPronounContractionShouldContract_AllowIssue1508(t *testing.T) {
	linttest.AssertNoLints(t, "Were any other toys fun?", NewShouldContract())
	linttest.AssertNoLints(t, "You were his closest friend.", NewShouldContract())
}

func TestPronounContractionShouldContract_AllowsIssue1673(t *testing.T) {
	linttest.AssertNoLints(t, "What were the action items?", NewShouldContract())
}
