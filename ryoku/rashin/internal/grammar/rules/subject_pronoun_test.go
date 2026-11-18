package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSubjectPronoun_AlexLadder(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Alex carried the huge ladder.", NewSubjectPronoun(), "Alex and I carried the huge ladder.")
}

func TestSubjectPronoun_JordanLamp(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Jordan fixed the broken lamp.", NewSubjectPronoun(), "Jordan and I fixed the broken lamp.")
}

func TestSubjectPronoun_TaylorCrate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Taylor opened the dusty crate.", NewSubjectPronoun(), "Taylor and I opened the dusty crate.")
}

func TestSubjectPronoun_KaylaDog(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Kayla chased the noisy dog.", NewSubjectPronoun(), "Kayla and I chased the noisy dog.")
}

func TestSubjectPronoun_MadisonYard(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Madison painted the small yard shed.", NewSubjectPronoun(), "Madison and I painted the small yard shed.")
}

func TestSubjectPronoun_AveryTree(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Avery climbed the old tree.", NewSubjectPronoun(), "Avery and I climbed the old tree.")
}

func TestSubjectPronoun_BlakeRoom(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Blake cleaned the crowded room.", NewSubjectPronoun(), "Blake and I cleaned the crowded room.")
}

func TestSubjectPronoun_RileyTrain(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Riley watched the slow train go by.", NewSubjectPronoun(), "Riley and I watched the slow train go by.")
}

func TestSubjectPronoun_CameronDoor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Cameron fixed the loose door hinge.", NewSubjectPronoun(), "Cameron and I fixed the loose door hinge.")
}

func TestSubjectPronoun_JamieBag(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Me and Jamie carried the heavy shopping bag.", NewSubjectPronoun(), "Jamie and I carried the heavy shopping bag.")
}
