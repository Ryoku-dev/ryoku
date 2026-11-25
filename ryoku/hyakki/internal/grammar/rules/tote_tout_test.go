package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestToteTout_FixTotedAsATool(t *testing.T) {
	linttest.AssertSuggestionResult(t, "remember cloudformation is toted as a tool for resource creation, not orchestration", NewToteTout(), "remember cloudformation is touted as a tool for resource creation, not orchestration")
}

func TestToteTout_FixOftenTotedAsTheWorst(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He is often toted as the worst killer in history", NewToteTout(), "He is often touted as the worst killer in history")
}

func TestToteTout_FixGunToutingSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Batman doesn't use guns now is because his back-story includes a gun touting coward stealing away his parents", NewToteTout(), "Batman doesn't use guns now is because his back-story includes a gun toting coward stealing away his parents")
}

func TestToteTout_FixMuchTotedSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I realize it's much toted for having a brilliant API, but unfortunately that's not sufficient", NewToteTout(), "I realize it's much touted for having a brilliant API, but unfortunately that's not sufficient")
}

func TestToteTout_FixOftenToted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The NN being often toted as a black box (even some say model free) model or function of its input vector.", NewToteTout(), "The NN being often touted as a black box (even some say model free) model or function of its input vector.")
}

func TestToteTout_FixMuchTotedHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The much-toted \"93% hire rate\" is certainly not true for my class.", NewToteTout(), "The much-touted \"93% hire rate\" is certainly not true for my class.")
}

func TestToteTout_DontFlagAdToting(t *testing.T) {
	linttest.AssertNoLints(t, "Uber's Ad-Toting Drones Are Heckling Drivers Stuck in Traffic.", NewToteTout())
}

func TestToteTout_DontFlagPlentyOfToutsAround(t *testing.T) {
	linttest.AssertNoLints(t, "there are plenty of pathetic touts around the hotel", NewToteTout())
}

func TestToteTout_DontFlagToutingBox(t *testing.T) {
	linttest.AssertNoLints(t, "distracting from the dysfunction by touting box office revenue as the solution for paltry royalty earnings", NewToteTout())
}

func TestToteTout_FixWidelyToted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is widely toted as the smart, capable and economically insightful newspaper for the white, liberal elite.", NewToteTout(), "It is widely touted as the smart, capable and economically insightful newspaper for the white, liberal elite.")
}
