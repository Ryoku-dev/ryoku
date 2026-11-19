package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestQuoteSpacing_FlagsMissingSpaceBeforeQuote(t *testing.T) {
	linttest.AssertLintCount(t, "He said\"hello\" to me.", NewQuoteSpacing(), 1)
}

func TestQuoteSpacing_FlagsMissingSpaceAfterQuote(t *testing.T) {
	linttest.AssertLintCount(t, "She whispered \"hurry\"and left.", NewQuoteSpacing(), 1)
}

func TestQuoteSpacing_AllowsQuotesWithSpacing(t *testing.T) {
	linttest.AssertNoLints(t, "They shouted \"charge\" together.", NewQuoteSpacing())
}

func TestQuoteSpacing_AllowsQuotesAtEndOfSentence(t *testing.T) {
	linttest.AssertNoLints(t, "They shouted \"charge.\"", NewQuoteSpacing())
}

func TestQuoteSpacing_FixByInsertingSpaceAfter(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's not a complete sentence since it lacks a verb but it would be valid as a title or answer to a question since \"dog runs\"are things which exist", NewQuoteSpacing(), "It's not a complete sentence since it lacks a verb but it would be valid as a title or answer to a question since \"dog runs\" are things which exist")
}

func TestQuoteSpacing_EnsureFirstSuggestionIsSpaceThenQuote(t *testing.T) {
	linttest.AssertFirstSuggestionResult(t, "foo\"bar\" bar", NewQuoteSpacing(), "foo \"bar\" bar")
}

func TestQuoteSpacing_EnsureFirstSuggestionIsQuoteThenSpace(t *testing.T) {
	linttest.AssertFirstSuggestionResult(t, "foo \"bar\"bar", NewQuoteSpacing(), "foo \"bar\" bar")
}

func TestQuoteSpacing_EnsureFirstSuggestionIsNotQuoteThenSpace(t *testing.T) {
	defer func() {
		if _, ok := recover().(quoteSpacingFailingAssertion); !ok {
			t.Fatal("expected assertion failure")
		}
	}()
	linttest.AssertFirstSuggestionResult(quoteSpacingFailingAssertion{}, "foo \"bar\"bar", NewQuoteSpacing(), "foo\" bar\" bar")
}

func TestQuoteSpacing_EnsureQuoteThenSpaceIsSuggestedEvenWhenWrong(t *testing.T) {
	linttest.AssertSuggestionResult(t, "foo\"bar\" bar", NewQuoteSpacing(), "foo\" bar\" bar")
}

type quoteSpacingFailingAssertion struct{}

func (quoteSpacingFailingAssertion) Helper()               {}
func (quoteSpacingFailingAssertion) Fatalf(string, ...any) { panic(quoteSpacingFailingAssertion{}) }
