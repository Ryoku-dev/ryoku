package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAWaysToGo_AndHas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This project is currently under dev and has ways to go.", NewAWaysToGo(), "This project is currently under dev and has a ways to go.")
}

func TestAWaysToGo_AndHasComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's an ongoing effort and has ways to go, but please take a look and consider contributing as well.", NewAWaysToGo(), "It's an ongoing effort and has a ways to go, but please take a look and consider contributing as well.")
}

func TestAWaysToGo_HaveInOrderTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have ways to go in order to reach even close.", NewAWaysToGo(), "We have a ways to go in order to reach even close.")
}

func TestAWaysToGo_StillHas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and our darray implementation still has ways to go.", NewAWaysToGo(), "and our darray implementation still has a ways to go.")
}

func TestAWaysToGo_StillHasBefore(t *testing.T) {
	linttest.AssertSuggestionResult(t, "that thing still has ways to go before it's actually usable", NewAWaysToGo(), "that thing still has a ways to go before it's actually usable")
}

func TestAWaysToGo_StillHasToBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Currently react-native-keys still has ways to go to be free of vulnerabilities.", NewAWaysToGo(), "Currently react-native-keys still has a ways to go to be free of vulnerabilities.")
}

func TestAWaysToGo_StillHaveToWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the fact that we still have ways to go with updating the documentation", NewAWaysToGo(), "the fact that we still have a ways to go with updating the documentation")
}

func TestAWaysToGo_StillHaveThough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I still have ways to go though so it's a challenge for sure.", NewAWaysToGo(), "I still have a ways to go though so it's a challenge for sure.")
}

func TestAWaysToGo_StillHaveBefore(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but it seems I'll still have ways to go before getting a fully descriptive usable schema", NewAWaysToGo(), "but it seems I'll still have a ways to go before getting a fully descriptive usable schema")
}

func TestAWaysToGo_StillHaveBut(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Now to work on the email part still have ways to go but I'm actually liking the new logic.", NewAWaysToGo(), "Now to work on the email part still have a ways to go but I'm actually liking the new logic.")
}

func TestAWaysToGo_DontFlagWaysToGoBack(t *testing.T) {
	linttest.AssertNoLints(t, "All IDEs (and even Vim/evil) have ways to go back to the last place''", NewAWaysToGo())
}

func TestAWaysToGo_DontFlagWereFromAToB(t *testing.T) {
	linttest.AssertNoLints(t, "In every map there were ways to go from A to B by going through intermediate factories in <= time.", NewAWaysToGo())
}

func TestAWaysToGo_DontFlagToGoAbout(t *testing.T) {
	linttest.AssertNoLints(t, "Also I now have ways to go about rendering audio along with the frames", NewAWaysToGo())
}
