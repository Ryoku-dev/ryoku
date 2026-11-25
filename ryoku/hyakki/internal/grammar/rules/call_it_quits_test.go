package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCallItQuits_FixCantNow(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can't call it quit now, you need to see it through !!", NewCallItQuits(), "You can't call it quits now, you need to see it through !!")
}

func TestCallItQuits_FixForTheEvening(t *testing.T) {
	linttest.AssertSuggestionResult(t, "do some maintenance and logistics before you call it quit for the evening", NewCallItQuits(), "do some maintenance and logistics before you call it quits for the evening")
}

func TestCallItQuits_FixBeforeCalling(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you may like to put a large number for TIMEOUT, so that before calling it quit, xstress will wait to see", NewCallItQuits(), "you may like to put a large number for TIMEOUT, so that before calling it quits, xstress will wait to see")
}

func TestCallItQuits_FixMightNeedTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But I feel like I might need to call it quit and calibrate/find parameters myself and ignore the data provided/used inside OpenVR.", NewCallItQuits(), "But I feel like I might need to call it quits and calibrate/find parameters myself and ignore the data provided/used inside OpenVR.")
}

func TestCallItQuits_FixFinally(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I was about to call it quit but I tried a second time this way and it finally worked", NewCallItQuits(), "I was about to call it quits but I tried a second time this way and it finally worked")
}

func TestCallItQuits_FixCtf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "everyone got tired, we called it quit and the CTF ended", NewCallItQuits(), "everyone got tired, we called it quits and the CTF ended")
}

func TestCallItQuits_AvoidSomeCallIt(t *testing.T) {
	linttest.AssertNoLints(t, "Some call it Quit, some Exit and some Close, There's no issue to fix.", NewCallItQuits())
}

func TestCallItQuits_AvoidLetsCallIt(t *testing.T) {
	linttest.AssertNoLints(t, "and i need a new UiMode variant let's call it Quit.", NewCallItQuits())
}

func TestCallItQuits_DontFlgWitherInterveninPunctuation(t *testing.T) {
	linttest.AssertNoLints(t, "I try it, but I can't find function of dispose called. it quit immediately.", NewCallItQuits())
}

func TestCallItQuits_AvoidFlaggingAllCapsPossibleFalsePositives(t *testing.T) {
	linttest.AssertNoLints(t, "Just call it QUIT and you're done.", NewCallItQuits())
}

func TestCallItQuits_AvoidFlaggingFunctionNames(t *testing.T) {
	linttest.AssertNoLints(t, "call it soloSplit() , call it quit() or whatever else would be nice", NewCallItQuits())
}

func TestCallItQuits_AvoidFlaggingStopButton(t *testing.T) {
	linttest.AssertNoLints(t, "The Stop button (or maybe call it Quit) should stop just like the Pause button does", NewCallItQuits())
}
