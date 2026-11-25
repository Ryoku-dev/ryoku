package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestQuantifierNeedsOf_FixesACoupleMonths(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A couple months ago...", NewQuantifierNeedsOf(), "A couple of months ago...")
}

func TestQuantifierNeedsOf_FixesACoupleWeeks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A couple weeks ago...", NewQuantifierNeedsOf(), "A couple of weeks ago...")
}

func TestQuantifierNeedsOf_FixesACoupleDays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A couple days ago...", NewQuantifierNeedsOf(), "A couple of days ago...")
}

func TestQuantifierNeedsOf_FixesACoupleSeconds(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A couple seconds ago...", NewQuantifierNeedsOf(), "A couple of seconds ago...")
}

func TestQuantifierNeedsOf_FixesACoupleMinutes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A couple minutes ago...", NewQuantifierNeedsOf(), "A couple of minutes ago...")
}

func TestQuantifierNeedsOf_FixesACoupleHouses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A couple houses ago...", NewQuantifierNeedsOf(), "A couple of houses ago...")
}

func TestQuantifierNeedsOf_FixesACoupleCenturies(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A couple centuries ago...", NewQuantifierNeedsOf(), "A couple of centuries ago...")
}

func TestQuantifierNeedsOf_FixesACouplePeople(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I saw a couple people walk by a few minutes ago.", NewQuantifierNeedsOf(), "I saw a couple of people walk by a few minutes ago.")
}
