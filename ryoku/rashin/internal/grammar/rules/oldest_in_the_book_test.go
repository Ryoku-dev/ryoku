package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOldestInTheBook_FixDelphiMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is the oldest Delphi mistake in the books and I'm sure you've made it before (we all have), and I'm sure you recognise it when you see it.", NewOldestInTheBook(), "This is the oldest Delphi mistake in the book and I'm sure you've made it before (we all have), and I'm sure you recognise it when you see it.")
}

func TestOldestInTheBook_FixTrick(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... oldest trick in the books, a restart and it works all the times(for now).", NewOldestInTheBook(), "... oldest trick in the book, a restart and it works all the times(for now).")
}

func TestOldestInTheBook_FixVirusTrick(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Once the OS is started the MBR is typically protected for virus reasons - this is one of the oldest virus tricks in the books - goes back to ...", NewOldestInTheBook(), "Once the OS is started the MBR is typically protected for virus reasons - this is one of the oldest virus tricks in the book - goes back to ...")
}

func TestOldestInTheBook_FixMistake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ok, I realized now that I was making the oldest mistake in the books with my code, dividing my v by 2 instead of dividing it by 5.", NewOldestInTheBook(), "Ok, I realized now that I was making the oldest mistake in the book with my code, dividing my v by 2 instead of dividing it by 5.")
}

func TestOldestInTheBook_FixTricks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He enables the oldest tricks in the books, create fear from thing like prosperity (we really don't need Foxconn?)", NewOldestInTheBook(), "He enables the oldest tricks in the book, create fear from thing like prosperity (we really don't need Foxconn?)")
}

func TestOldestInTheBook_FixMilitaryPlays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Isnt that like one of the oldest military plays in the books?", NewOldestInTheBook(), "Isnt that like one of the oldest military plays in the book?")
}

func TestOldestInTheBook_IsOldestTrickInTheBooksRefToIdom(t *testing.T) {
	linttest.AssertLintMessage(t, "This is one of the oldest trick in the books", NewOldestInTheBook(), "This idiom should use singular `book` instead of plural `books`.")
}

func TestOldestInTheBook_IsChromaticAlterationsRefToIdom(t *testing.T) {
	linttest.AssertLintMessage(t, "One of the oldest chromatic alterations in the books is the raising of the leading tone", NewOldestInTheBook(), "If this is a play on the idiom `oldest trick in the book`, it should use singular `book` instead of plural `books`.")
}
