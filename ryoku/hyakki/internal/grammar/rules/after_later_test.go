package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAfterLater_After90DaysLater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Try to rename your organization after 90 days later because of GitHub official documentation it said.", NewAfterLater(), "Try to rename your organization after 90 days because of GitHub official documentation it said.")
}

func TestAfterLater_AfterAbout30MinutesLater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It plays like 1 minute of the song and then stops, and after about 30 minutes later, the bot disconnects an throws DisTubeError", NewAfterLater(), "It plays like 1 minute of the song and then stops, and about 30 minutes later, the bot disconnects an throws DisTubeError")
}

func TestAfterLater_After14DaysLater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "After 14 days later, the cache expired.", NewAfterLater(), "After 14 days, the cache expired.")
}

func TestAfterLater_AfterExactly5MinutesLater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "After exactly 5 minutes later, they try again and the cluster is formed then.", NewAfterLater(), "Exactly 5 minutes later, they try again and the cluster is formed then.")
}

func TestAfterLater_After22YearsLater1(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Completed YR campaign for 2nd time after 22 years later.", NewAfterLater(), "Completed YR campaign for 2nd time after 22 years.")
}

func TestAfterLater_AfterAlmost2YearsLater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This buyer contacted me after almost 2 years later.", NewAfterLater(), "This buyer contacted me almost 2 years later.")
}

func TestAfterLater_After2YearsLater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is Jedi Survivor better now after 2 years later?", NewAfterLater(), "Is Jedi Survivor better now after 2 years?")
}

func TestAfterLater_AfterAYearLater(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Even after a year later, I don’t know how to get my self-love back.", NewAfterLater(), "Even a year later, I don’t know how to get my self-love back.")
}

func TestAfterLater_After22YearsLater2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "After 22 years later, my top 1 game was Zeroed", NewAfterLater(), "After 22 years, my top 1 game was Zeroed")
}
