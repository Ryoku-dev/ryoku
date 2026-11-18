package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestASomeTime_AfterASomeTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am connecting to a websocket and I get this error after a some time: SyntaxError: Unexpected end of JSON input", NewASomeTime(), "I am connecting to a websocket and I get this error after some time: SyntaxError: Unexpected end of JSON input")
}

func TestASomeTime_ForASomeMonths(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I haven't upgraded my system for at least a some months.", NewASomeTime(), "I haven't upgraded my system for at least some months.")
}

func TestASomeTime_ForASomeTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The problem with this code is that not receiving images for a some time causes the window to hang", NewASomeTime(), "The problem with this code is that not receiving images for some time causes the window to hang")
}

func TestASomeTime_ForASomeWeeks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This problem have been there for a some weeks so does it plans to ve fixed.", NewASomeTime(), "This problem have been there for some weeks so does it plans to ve fixed.")
}

func TestASomeTime_ASomeWeeksAgo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I reported some warnings from the log a some weeks ago.", NewASomeTime(), "I reported some warnings from the log some weeks ago.")
}

func TestASomeTime_ASomeHours(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I guess support people is was cheaper than just a some hours of an engineer who can actually fix the platform", NewASomeTime(), "I guess support people is was cheaper than just some hours of an engineer who can actually fix the platform")
}

func TestASomeTime_ASomeTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Spent a some time not using it, now the problem is back....", NewASomeTime(), "Spent some time not using it, now the problem is back....")
}
