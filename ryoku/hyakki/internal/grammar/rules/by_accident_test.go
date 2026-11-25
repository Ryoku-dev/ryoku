package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestByAccident_FixOnAccident(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Snapshot revert feature is unintuitive and easy to use on accident.", NewByAccident(), "Snapshot revert feature is unintuitive and easy to use by accident.")
}

func TestByAccident_FixOnCompleteAccident(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I Came across this comment on complete accident, however, I did notice the same thing with a slowdown in chrome for android", NewByAccident(), "I Came across this comment by complete accident, however, I did notice the same thing with a slowdown in chrome for android")
}

func TestByAccident_FixOnHappyAccident(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just did this on happy accident the other day with my partner.", NewByAccident(), "Just did this by happy accident the other day with my partner.")
}

func TestByAccident_FixOnLiteralAccident(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I did this on literal accident, trying to prove someone wrong that its not that easy.", NewByAccident(), "I did this by literal accident, trying to prove someone wrong that its not that easy.")
}

func TestByAccident_FixOnMereAccident(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hated this challenge and nope I don't I completed it on mere accident.", NewByAccident(), "I hated this challenge and nope I don't I completed it by mere accident.")
}

func TestByAccident_FixOnPureAccident(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I got this on pure accident after forgetting to enable WebGL on LibreWolf", NewByAccident(), "I got this by pure accident after forgetting to enable WebGL on LibreWolf")
}

func TestByAccident_FixOnSheerAccident(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I more of think of things that got discovered on sheer accident, something no normal human would just do and expect results.", NewByAccident(), "I more of think of things that got discovered by sheer accident, something no normal human would just do and expect results.")
}

func TestByAccident_FixOnTotalAccident(t *testing.T) {
	linttest.AssertSuggestionResult(t, "On Total Accident, I Found Out Yona's True Title.", NewByAccident(), "By Total Accident, I Found Out Yona's True Title.")
}
