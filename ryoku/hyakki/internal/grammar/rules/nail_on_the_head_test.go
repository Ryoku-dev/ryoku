package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNailOnTheHead_FixHat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She hit the nail on the hat.", NewNailOnTheHead(), "She hit the nail on the head.")
}

func TestNailOnTheHead_FixHad(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You really put the nail on the had with that comment.", NewNailOnTheHead(), "You really put the nail on the head with that comment.")
}

func TestNailOnTheHead_FixHit(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They hit the nail on the hit regarding our problem.", NewNailOnTheHead(), "They hit the nail on the head regarding our problem.")
}

func TestNailOnTheHead_FixHid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The article nails the nail on the hid this time.", NewNailOnTheHead(), "The article nails the nail on the head this time.")
}

func TestNailOnTheHead_IgnoreCorrect(t *testing.T) {
	linttest.AssertLintCount(t, "She hit the nail on the head.", NewNailOnTheHead(), 0)
}
