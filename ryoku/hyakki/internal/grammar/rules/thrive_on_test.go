package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThriveOn_IgnoreThriveOff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I thrive off open source projects.", NewThriveOn(), "I thrive on open source projects.")
}

func TestThriveOn_FixThriveOffOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I thrive off of criticism and constructive feedback.", NewThriveOn(), "I thrive on criticism and constructive feedback.")
}

func TestThriveOn_IgnoreThrivedOff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The decision hurt the emulator community who thrived off AVX512 chips at the low to medium end, not for the large registers, but for the masking capabilities.", NewThriveOn(), "The decision hurt the emulator community who thrived on AVX512 chips at the low to medium end, not for the large registers, but for the masking capabilities.")
}

func TestThriveOn_IgnoreThrivedOffOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "All my life I have been determined to succeed in my education and have thrived off of an overwhelming amount of creativity.", NewThriveOn(), "All my life I have been determined to succeed in my education and have thrived on an overwhelming amount of creativity.")
}

func TestThriveOn_IgnoreThrivesOffCollaboration(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Open source thrives off collaboration.", NewThriveOn(), "Open source thrives on collaboration.")
}

func TestThriveOn_IgnoreThrivesOffOfCommunity(t *testing.T) {
	linttest.AssertSuggestionResult(t, "An all-in-one technical interview prep platform which fosters and thrives off of community engagement.", NewThriveOn(), "An all-in-one technical interview prep platform which fosters and thrives on community engagement.")
}

func TestThriveOn_FixThrivingOff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So the government granted monopolies to those companies willing to risk the investment, and our teleco's have been thriving off it ever since.", NewThriveOn(), "So the government granted monopolies to those companies willing to risk the investment, and our teleco's have been thriving on it ever since.")
}

func TestThriveOn_IgnoreThriveOffTheLand(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A great collection of resources to thrive off the land.", NewThriveOn(), "A great collection of resources to thrive off the land.")
}

func TestThriveOn_IgnoreThrivingOffOfHisBack(t *testing.T) {
	t.Skip("Ignored upstream: Analogy with 'live off his back'")
	linttest.AssertNoLints(t, "We're getting free software, that we ALL use, and the person who has worked tirelessly on it is struggling financially, while we are all thriving off of his back.", NewThriveOn())
}
