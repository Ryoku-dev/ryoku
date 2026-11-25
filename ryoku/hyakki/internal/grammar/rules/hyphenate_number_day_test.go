package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHyphenateNumberDay_CorrectsThreeDayTraining(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The company offers a 3 day training program.", NewHyphenateNumberDay(), "The company offers a 3-day training program.")
}

func TestHyphenateNumberDay_CorrectsFiveDayChallenge(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Join the 5 day challenge to improve your skills.", NewHyphenateNumberDay(), "Join the 5-day challenge to improve your skills.")
}

func TestHyphenateNumberDay_CorrectsSevenDayPlan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She followed a strict 7 day meal plan.", NewHyphenateNumberDay(), "She followed a strict 7-day meal plan.")
}

func TestHyphenateNumberDay_DoesNotCorrectWhenNotAdjective(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The seminar lasts for 2 days.", NewHyphenateNumberDay(), "The seminar lasts for 2 days.")
}

func TestHyphenateNumberDay_CorrectsVariedPhrases(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They implemented a new 6 day work schedule.", NewHyphenateNumberDay(), "They implemented a new 6-day work schedule.")
	linttest.AssertSuggestionResult(t, "Enroll in our 10 day fitness bootcamp!", NewHyphenateNumberDay(), "Enroll in our 10-day fitness bootcamp!")
}

func TestHyphenateNumberDay_EdgeCaseDayLong(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The 4 day-long seminar was insightful.", NewHyphenateNumberDay(), "The 4-day-long seminar was insightful.")
}

func TestHyphenateNumberDay_EdgeCasePluralDays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The trip was a fun 5 day experience.", NewHyphenateNumberDay(), "The trip was a fun 5-day experience.")
}

func TestHyphenateNumberDay_IgnoresSpelledOutNumbers(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We had a three day holiday.", NewHyphenateNumberDay(), "We had a three day holiday.")
}
