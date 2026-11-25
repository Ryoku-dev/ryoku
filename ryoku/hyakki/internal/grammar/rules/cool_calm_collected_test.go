package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCoolCalmCollected_FirstCommaOnly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One thing that annoys me the most is when people say “cool, calm and collective”.", NewCoolCalmCollected(), "One thing that annoys me the most is when people say “cool, calm and collected”.")
}

func TestCoolCalmCollected_BothCommas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "\"Cool, calm, and collective\" is apart of a list of my favorite blundered sayings.", NewCoolCalmCollected(), "\"Cool, calm, and collected\" is apart of a list of my favorite blundered sayings.")
}

func TestCoolCalmCollected_OnlyCommaIsInWrongPosition(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Your all cool calm and, collective With your shity paddles", NewCoolCalmCollected(), "Your all cool calm and, collected With your shity paddles")
}

func TestCoolCalmCollected_NoCommas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But being able to stay cool calm and collective in that situation is extremely important in this job.", NewCoolCalmCollected(), "But being able to stay cool calm and collected in that situation is extremely important in this job.")
}
