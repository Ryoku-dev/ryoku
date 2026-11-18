package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestByTheBook_FixPlayItByTheBooks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As a result, we can conclude that organizations do \"play it by the books\" but individually define what is inside their playbooks ...", NewByTheBook(), "As a result, we can conclude that organizations do \"play it by the book\" but individually define what is inside their playbooks ...")
}

func TestByTheBook_FixEverythingByTheBooks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The finalizer used in Replace AsyncProcess exit handler by weakref.finalize #4184 is doing everything by the books.", NewByTheBook(), "The finalizer used in Replace AsyncProcess exit handler by weakref.finalize #4184 is doing everything by the book.")
}

func TestByTheBook_DontFlagWhenNotAtEnd(t *testing.T) {
	linttest.AssertNoLints(t, "The order of books within the breadcrumb dropdowns (i.e. when you click the arrow) is done by the books' id, rather than by name", NewByTheBook())
}

func TestByTheBook_FixPlayByTheBooks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We essentially play by the books and let the actual Pod with identity make the requests.", NewByTheBook(), "We essentially play by the book and let the actual Pod with identity make the requests.")
}

func TestByTheBook_FixPlayingByTheBooks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If we're playing by the books, we should also implement something that tells a user when they're trying to make a table.", NewByTheBook(), "If we're playing by the book, we should also implement something that tells a user when they're trying to make a table.")
}

func TestByTheBook_FixGoByTheBooks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you wanted to go by the books, I would agree that removing such consistently unmethylated positions is a good option.", NewByTheBook(), "If you wanted to go by the book, I would agree that removing such consistently unmethylated positions is a good option.")
}

func TestByTheBook_FixConfiguredByTheBooks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a new media folder was created under config/addons_config/paperless_ng even though I configured it by the books", NewByTheBook(), "a new media folder was created under config/addons_config/paperless_ng even though I configured it by the book")
}

func TestByTheBook_FixAllByTheBooks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we did all by the books and we did not need the options at the end", NewByTheBook(), "we did all by the book and we did not need the options at the end")
}
