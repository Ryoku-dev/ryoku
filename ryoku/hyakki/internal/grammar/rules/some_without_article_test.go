package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSomeWithoutArticle_FixesSimpleLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We interviewed the some candidates today.", NewSomeWithoutArticle(), "We interviewed some candidates today.")
}

func TestSomeWithoutArticle_FixesSentenceCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Some volunteers arrived early.", NewSomeWithoutArticle(), "Some volunteers arrived early.")
}

func TestSomeWithoutArticle_PreservesUppercaseBlock(t *testing.T) {
	linttest.AssertSuggestionResult(t, "THE SOME OPTIONS WERE LISTED.", NewSomeWithoutArticle(), "SOME OPTIONS WERE LISTED.")
}

func TestSomeWithoutArticle_SecondSuggestionProducesTheSame(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We kept the some approach from last year.", NewSomeWithoutArticle(), "We kept the same approach from last year.")
}

func TestSomeWithoutArticle_IgnoresAlreadyCorrectSome(t *testing.T) {
	linttest.AssertLintCount(t, "We interviewed some candidates today.", NewSomeWithoutArticle(), 0)
}

func TestSomeWithoutArticle_IgnoresTheSame(t *testing.T) {
	linttest.AssertLintCount(t, "We kept the same approach from last year.", NewSomeWithoutArticle(), 0)
}

func TestSomeWithoutArticle_IgnoresTheSomething(t *testing.T) {
	linttest.AssertLintCount(t, "We interviewed the something else entirely.", NewSomeWithoutArticle(), 0)
}

func TestSomeWithoutArticle_WorksBeforeComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They reviewed the some, then finalized the list.", NewSomeWithoutArticle(), "They reviewed some, then finalized the list.")
}

func TestSomeWithoutArticle_WorksBeforePossessiveNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The report praised the some team's effort.", NewSomeWithoutArticle(), "The report praised some team's effort.")
}

func TestSomeWithoutArticle_HandlesLineBreakSpacing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We invited the some\nartists to perform.", NewSomeWithoutArticle(), "We invited some\nartists to perform.")
}
