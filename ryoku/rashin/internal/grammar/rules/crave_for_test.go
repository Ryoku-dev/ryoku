package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCraveFor_FixCraveFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We crave for this inference code.", NewCraveFor(), "We crave this inference code.")
}

func TestCraveFor_FixEverCravedFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ever craved for a tool to monitor what files are growing faster than other ones?", NewCraveFor(), "Ever craved a tool to monitor what files are growing faster than other ones?")
}

func TestCraveFor_FixCravesFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Defending coders is the main talent everyone craves for.", NewCraveFor(), "Defending coders is the main talent everyone craves.")
}

func TestCraveFor_FixAmCravingFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but hopefully it's the avenue that leads me to what am craving for !", NewCraveFor(), "but hopefully it's the avenue that leads me to what am craving !")
}

func TestCraveFor_FixAreCravingFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Many of my users are craving for a timepicker in dropdown format :)", NewCraveFor(), "Many of my users are craving a timepicker in dropdown format :)")
}

func TestCraveFor_FixBeCravingFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It offers you the no-frills experience for Hugo that you might be craving for.", NewCraveFor(), "It offers you the no-frills experience for Hugo that you might be craving.")
}

func TestCraveFor_FixBeenCravingFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And this is something the community has been craving for and splitting up the ecosystem", NewCraveFor(), "And this is something the community has been craving and splitting up the ecosystem")
}

func TestCraveFor_FixIMCravingFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm craving for Hazel engine devlog man...", NewCraveFor(), "I'm craving Hazel engine devlog man...")
}

func TestCraveFor_FixIsCravingFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Now everyone is craving for this level of abstraction running on GPUs.", NewCraveFor(), "Now everyone is craving this level of abstraction running on GPUs.")
}

func TestCraveFor_FixWasCravingFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yeah, I was craving for some splinter cell action, did a playthrough of CT", NewCraveFor(), "Yeah, I was craving some splinter cell action, did a playthrough of CT")
}

func TestCraveFor_FixWereCravingFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "is this the \"freedom\" and the \"flexibility\" we were craving for compared to the Arduino folks way of doing things uC?", NewCraveFor(), "is this the \"freedom\" and the \"flexibility\" we were craving compared to the Arduino folks way of doing things uC?")
}

func TestCraveFor_DontFlagOneOfThemBeingCraving(t *testing.T) {
	linttest.AssertNoLints(t, "The Second Noble Truth identifies three aspects of craving, one of them being craving for non-being.", NewCraveFor())
}
