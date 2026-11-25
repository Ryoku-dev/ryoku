package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestElsePossessive_FixesNoOneElses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's no one elses problem.", NewElsePossessive(), "It's no one else's problem.")
}

func TestElsePossessive_FixesSomeoneElses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's someone elses problem.", NewElsePossessive(), "It's someone else's problem.")
}

func TestElsePossessive_FixesAnybodyElses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Was that anybody elses idea?", NewElsePossessive(), "Was that anybody else's idea?")
}

func TestElsePossessive_FixesEveryoneElses(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He echoed everyone elses concerns.", NewElsePossessive(), "He echoed everyone else's concerns.")
}

func TestElsePossessive_IgnoresCorrectForm(t *testing.T) {
	linttest.AssertLintCount(t, "She borrowed someone else's notes.", NewElsePossessive(), 0)
}
