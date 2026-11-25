package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCatch22_FixCache22Space(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So, i'm in a bit of a cache 22 situation.", NewCatch22(), "So, i'm in a bit of a catch 22 situation.")
}

func TestCatch22_FixCache22Hyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This leads to a cache-22 situation.", NewCatch22(), "This leads to a catch-22 situation.")
}

func TestCatch22_FixCash22(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's literally a cash 22 it's selectivity but also specific criteria", NewCatch22(), "It's literally a catch 22 it's selectivity but also specific criteria")
}

func TestCatch22_FixCashTwentyTwoSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It’s a bit of a cash twenty two situation. You see?", NewCatch22(), "It’s a bit of a catch twenty two situation. You see?")
}

func TestCatch22_FixCashTwentyTwoHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So, they're they're in a cash twenty-two but in the middle of all of that, we gotta deal with the presenting problem.", NewCatch22(), "So, they're they're in a catch twenty-two but in the middle of all of that, we gotta deal with the presenting problem.")
}

func TestCatch22_FixCacheTwentyTwoHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The club is in such a cache twenty-two situation given it past experiences with previous managers.", NewCatch22(), "The club is in such a catch twenty-two situation given it past experiences with previous managers.")
}
