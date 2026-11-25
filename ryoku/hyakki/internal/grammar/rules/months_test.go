package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMonths_FixInAugust(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I worked for WebstaurantStore doing Quality Assurance Automation and am now transitioning to a new graduate developer role at BNY Mellon, starting in august.", NewMonths(), "I worked for WebstaurantStore doing Quality Assurance Automation and am now transitioning to a new graduate developer role at BNY Mellon, starting in August.")
}

func TestMonths_FixInMarch(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This game was originally written by me in march 2000.", NewMonths(), "This game was originally written by me in March 2000.")
}

func TestMonths_FixInMay(t *testing.T) {
	linttest.AssertSuggestionResult(t, "typo in may 2024 updates", NewMonths(), "typo in May 2024 updates")
}

func TestMonths_FixLastAugust(t *testing.T) {
	linttest.AssertSuggestionResult(t, "since last august smart has been leading talks to open up japan", NewMonths(), "since last August smart has been leading talks to open up japan")
}

func TestMonths_FixLastMay(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have a 2019 mini countryman that i purchased last may.", NewMonths(), "I have a 2019 mini countryman that i purchased last May.")
}

func TestMonths_FixOfAugust(t *testing.T) {
	linttest.AssertSuggestionResult(t, "change abbreviation of august for Indonesian locale", NewMonths(), "change abbreviation of August for Indonesian locale")
}

func TestMonths_FixMarch2019(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How to disable drop cap today (late march 2019)", NewMonths(), "How to disable drop cap today (late March 2019)")
}

func TestMonths_FixMay2022(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That will be ende from 30 may 2022.", NewMonths(), "That will be ende from 30 May 2022.")
}

func TestMonths_FixDays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Between march 15 and august 27.", NewMonths(), "Between March 15 and August 27.")
}
