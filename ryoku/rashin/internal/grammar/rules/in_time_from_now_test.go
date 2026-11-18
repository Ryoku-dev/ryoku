package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestInTimeFromNow_InThreeYearsFromNow(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Closing this issue now to prevent it from still being open in three years from now.", NewInTimeFromNow(), []string{
		"Closing this issue now to prevent it from still being open in three years.",
		"Closing this issue now to prevent it from still being open three years from now.",
	}, []string{})
}

func TestInTimeFromNow_In2SecondsFromNow(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "The task will be executed in 2 seconds from now.", NewInTimeFromNow(), []string{
		"The task will be executed 2 seconds from now.",
		"The task will be executed in 2 seconds.",
	}, []string{})
}

func TestInTimeFromNow_InThreeWeeksFromNow(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I have created a pull request, which can be merged in three weeks from now", NewInTimeFromNow(), []string{
		"I have created a pull request, which can be merged in three weeks",
		"I have created a pull request, which can be merged three weeks from now",
	}, []string{})
}

func TestInTimeFromNow_In2HoursFromNow(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "send a notification every 30 minutes, starting in 2 hours from now", NewInTimeFromNow(), []string{
		"send a notification every 30 minutes, starting 2 hours from now",
		"send a notification every 30 minutes, starting in 2 hours",
	}, []string{})
}

func TestInTimeFromNow_InTwoWeeksFromNow(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "That problem will be solved in two weeks from now by Bintray people.", NewInTimeFromNow(), []string{
		"That problem will be solved two weeks from now by Bintray people.",
		"That problem will be solved in two weeks by Bintray people.",
	}, []string{})
}
