package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThrowRubbish_AllowThrowAwayRubbish(t *testing.T) {
	linttest.AssertLintCount(t, "Throw away the rubbish", NewThrowRubbish(), 0)
}

func TestThrowRubbish_AllowThrowsGarbageAway(t *testing.T) {
	linttest.AssertLintCount(t, "He throws garbage away", NewThrowRubbish(), 0)
}

func TestThrowRubbish_AllowThrewOutTrash(t *testing.T) {
	linttest.AssertLintCount(t, "I threw out the trash", NewThrowRubbish(), 0)
}

func TestThrowRubbish_AllowThrowJunkInto(t *testing.T) {
	linttest.AssertLintCount(t, "Throw that junk into the bin!", NewThrowRubbish(), 0)
}

func TestThrowRubbish_RejectThrowGarbage(t *testing.T) {
	linttest.AssertLintCount(t, "You should throw garbage", NewThrowRubbish(), 1)
}

func TestThrowRubbish_RejectThrowingRubbish(t *testing.T) {
	linttest.AssertLintCount(t, "Throwing rubbish is not a good idea", NewThrowRubbish(), 1)
}

func TestThrowRubbish_CorrectThrownSomeTrash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've thrown some trash", NewThrowRubbish(), "I've thrown some trash away")
}

func TestThrowRubbish_CorrectThrowsGarbage(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That guy just throws his garbage", NewThrowRubbish(), "That guy just throws out his garbage")
}

func TestThrowRubbish_IgnoreThrowBall(t *testing.T) {
	linttest.AssertLintCount(t, "Can you throw the ball?", NewThrowRubbish(), 0)
}

func TestThrowRubbish_CorrectComeCloseToThrowTrash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Smart Dustbin is a trash bin that automatically opens when you come close to throw trash.", NewThrowRubbish(), "Smart Dustbin is a trash bin that automatically opens when you come close to throw away trash.")
}

func TestThrowRubbish_CorrectThrownRubbish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Add a script that draws the bin behind thrown rubbish.", NewThrowRubbish(), "Add a script that draws the bin behind thrown away rubbish.")
}

func TestThrowRubbish_CorrectEncouragePeopleToThrowTrash(t *testing.T) {
	t.Skip("`on` doesn't go with `throw` but with `daily basis`")
	linttest.AssertSuggestionResult(t, "The app main goal is to encourage people to throw trash they can found on a daily basis.", NewThrowRubbish(), "The app main goal is to encourage people to throw away trash they can found on a daily basis.")
}

func TestThrowRubbish_CorrectAPersonThrowingTrash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think personally the icons look okay, aside from the clear prompt one, as it's currently accented on a person throwing trash.", NewThrowRubbish(), "I think personally the icons look okay, aside from the clear prompt one, as it's currently accented on a person throwing away trash.")
}

func TestThrowRubbish_AllowAtAndBack(t *testing.T) {
	linttest.AssertLintCount(t, "Throw garbage at a program, it will throw garbage back.", NewThrowRubbish(), 0)
}

func TestThrowRubbish_CorrectResponsiblyThrowTrash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Reward system for people responsibly throwing trash saving the environment.", NewThrowRubbish(), "Reward system for people responsibly throwing away trash saving the environment.")
}

func TestThrowRubbish_DontFlagThrowsJunkErrors(t *testing.T) {
	linttest.AssertLintCount(t, "Experimental init throws junk errors, Ignore.", NewThrowRubbish(), 0)
}

func TestThrowRubbish_DontFlagThrowingGarbageData(t *testing.T) {
	linttest.AssertLintCount(t, "I can resolve this in various ways, such as by not throwing garbage data at Typesense", NewThrowRubbish(), 0)
}

func TestThrowRubbish_DontFlagThrowingGarbageValue(t *testing.T) {
	linttest.AssertLintCount(t, "Fix Spill tree Throwing garbage value", NewThrowRubbish(), 0)
}

func TestThrowRubbish_CorrectThrewTrashProperly(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we want to know which student threw trash properly so that we can reward that student", NewThrowRubbish(), "we want to know which student threw away trash properly so that we can reward that student")
}

func TestThrowRubbish_DontFlagThrowJunkBytes(t *testing.T) {
	linttest.AssertLintCount(t, "the most efficient way to enforce the buffer size and throw junk bytes is to have a local (to the reception function) buffer", NewThrowRubbish(), 0)
}

func TestThrowRubbish_DontFlagThrowTrashEverywhere(t *testing.T) {
	linttest.AssertLintCount(t, "People throw trash everywhere and this tendency is very harmful.", NewThrowRubbish(), 0)
}

func TestThrowRubbish_DontFlagThrowsGarbageComments(t *testing.T) {
	linttest.AssertLintCount(t, "We dont need guys that throws garbage comments based on theyr lack of knowledge.", NewThrowRubbish(), 0)
}

func TestThrowRubbish_DontFlagTrashCanBeThrownIntoTheTrash(t *testing.T) {
	linttest.AssertLintCount(t, "Trash balls generated during cutting can be thrown into the trash", NewThrowRubbish(), 0)
}

func TestThrowRubbish_CorrectThrowingRubbish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Admiring paintings, throwing rubbish, greeting.", NewThrowRubbish(), "Admiring paintings, throwing away rubbish, greeting.")
}
