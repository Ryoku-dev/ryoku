package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestExpandTimeShorthands_DetectsSingularHour(t *testing.T) {
	linttest.AssertSuggestionResult(t, "5 hr", NewExpandTimeShorthands(), "5 hours")
}

func TestExpandTimeShorthands_DetectsSingularMinute(t *testing.T) {
	linttest.AssertSuggestionResult(t, "10 min", NewExpandTimeShorthands(), "10 minutes")
}

func TestExpandTimeShorthands_DetectsSingularSecond(t *testing.T) {
	linttest.AssertSuggestionResult(t, "30 sec", NewExpandTimeShorthands(), "30 seconds")
}

func TestExpandTimeShorthands_DetectsPluralHours(t *testing.T) {
	linttest.AssertSuggestionResult(t, "5 hrs", NewExpandTimeShorthands(), "5 hours")
}

func TestExpandTimeShorthands_DetectsPluralMinutes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "10 mins", NewExpandTimeShorthands(), "10 minutes")
}

func TestExpandTimeShorthands_DetectsPluralSeconds(t *testing.T) {
	linttest.AssertSuggestionResult(t, "30 secs", NewExpandTimeShorthands(), "30 seconds")
}

func TestExpandTimeShorthands_DetectsMillisecond(t *testing.T) {
	linttest.AssertSuggestionResult(t, "5 ms", NewExpandTimeShorthands(), "5 milliseconds")
}

func TestExpandTimeShorthands_DetectsMilliseconds(t *testing.T) {
	linttest.AssertSuggestionResult(t, "10 msecs", NewExpandTimeShorthands(), "10 milliseconds")
}

func TestExpandTimeShorthands_HandlesPunctuationHour(t *testing.T) {
	linttest.AssertSuggestionResult(t, "5 hr.", NewExpandTimeShorthands(), "5 hours.")
}

func TestExpandTimeShorthands_HandlesPunctuationMinute(t *testing.T) {
	linttest.AssertSuggestionResult(t, "10 min,", NewExpandTimeShorthands(), "10 minutes,")
}

func TestExpandTimeShorthands_HandlesPunctuationSecond(t *testing.T) {
	linttest.AssertSuggestionResult(t, "30 sec!", NewExpandTimeShorthands(), "30 seconds!")
}

func TestExpandTimeShorthands_HandlesAdjacentNumberHour(t *testing.T) {
	linttest.AssertSuggestionResult(t, "5hr", NewExpandTimeShorthands(), "5 hours")
}

func TestExpandTimeShorthands_HandlesAdjacentNumberMinute(t *testing.T) {
	linttest.AssertSuggestionResult(t, "10-min", NewExpandTimeShorthands(), "10-minutes")
}

func TestExpandTimeShorthands_HandlesAdjacentNumberSecond(t *testing.T) {
	linttest.AssertSuggestionResult(t, "30sec", NewExpandTimeShorthands(), "30 seconds")
}
