package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOutOfDate_CorrectsOutOfDate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The software is out of date.", NewOutOfDate(), "The software is out-of-date.")
}

func TestOutOfDate_CorrectsOutOfDateWithVariation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This information is out of-date.", NewOutOfDate(), "This information is out-of-date.")
}

func TestOutOfDate_AllowsCorrectUsage(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The guidelines are out-of-date.", NewOutOfDate(), "The guidelines are out-of-date.")
}
