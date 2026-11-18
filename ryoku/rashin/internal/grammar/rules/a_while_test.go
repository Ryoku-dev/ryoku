package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAWhile_AllowIssue2144(t *testing.T) {
	linttest.AssertNoLints(t, "After thinking awhile, I decided to foo a bar.", NewAWhile())
	linttest.AssertNoLints(t, "After thinking for a while, I decided to foo a bar.", NewAWhile())
}

func TestAWhile_FixIssue2144(t *testing.T) {
	linttest.AssertSuggestionResult(t, "After thinking a while, I decided to foo a bar.", NewAWhile(), "After thinking awhile, I decided to foo a bar.")
}

func TestAWhile_CorrectInQuiteAWhile(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I haven't seen him in quite awhile.", NewAWhile(), "I haven't seen him in quite a while.")
}

func TestAWhile_CorrectInAWhile(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I haven't checked in awhile.", NewAWhile(), "I haven't checked in a while.")
}

func TestAWhile_CorrectForAwhile(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Video Element Error: MEDA_ERR_DECODE when chrome is left open for awhile", NewAWhile(), "Video Element Error: MEDA_ERR_DECODE when chrome is left open for a while")
}

func TestAWhile_CorrectAfterAwhile(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Links on portal stop working after awhile, requiring page refresh.", NewAWhile(), "Links on portal stop working after a while, requiring page refresh.")
}
