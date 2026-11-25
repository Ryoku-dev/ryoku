package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestToTo_ShouldBeToDoWithSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I need to add that to my to to list first.", NewToTo(), "I need to add that to my to do list first.")
}

func TestToTo_ShouldBeToDoWithHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I need to add that to my to-to list first.", NewToTo(), "I need to add that to my to-do list first.")
}

func TestToTo_TripleToShouldBeToDoTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a little example of what you'd need to to to create and resolve a promise off the main thread", NewToTo(), "a little example of what you'd need to do to create and resolve a promise off the main thread")
}

func TestToTo_ShouldBeSingleTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What is the modern way to to do XML transforms in Python?", NewToTo(), "What is the modern way to do XML transforms in Python?")
}

func TestToTo_DontFlagAppliedToToStandOut(t *testing.T) {
	linttest.AssertNoLints(t, "Indeed says to store messages to jobs I've applied to to stand out", NewToTo())
}

func TestToTo_DontFlagApplyToToBe(t *testing.T) {
	linttest.AssertNoLints(t, "I'm expecting the company that I'm applying to to be efficient, to assign me tasks that aren't a waste of time", NewToTo())
}

func TestToTo_DontFlagConnectedToToAllow(t *testing.T) {
	linttest.AssertNoLints(t, "Create headless \"VS Code Server\" that can be connected to to allow editing remote code within it's environment", NewToTo())
}

func TestToTo_DontFlagHaveToToTake(t *testing.T) {
	linttest.AssertNoLints(t, "Do what you have to to take care of yourself and get what you need.", NewToTo())
}

func TestToTo_DontFlagNeedToToIncrease(t *testing.T) {
	linttest.AssertNoLints(t, "something that they're wasting their time doing, so adjust as you need to to increase their productivity", NewToTo())
}

func TestToTo_DontFlagPrayToToFind(t *testing.T) {
	linttest.AssertNoLints(t, "what dieties would you pray to to find lost things and for a housing opportunity?", NewToTo())
}

func TestToTo_DontFlagReferredToToExist(t *testing.T) {
	linttest.AssertNoLints(t, "IsValid requires a drive being referred to to exist", NewToTo())
}

func TestToTo_DontFlagReferredToToWrite(t *testing.T) {
	linttest.AssertNoLints(t, "Could you tell me which paper you referred to to write this algorithm?", NewToTo())
}

func TestToTo_DontFlagReferToToCreate(t *testing.T) {
	linttest.AssertNoLints(t, "Which Packages do I need to refer to to create my own NUnit console runner?", NewToTo())
}

func TestToTo_DontFlagNeedToToStay(t *testing.T) {
	linttest.AssertNoLints(t, "Yes, if I fail of course I'll do whatever I need to to stay healthy", NewToTo())
}

func TestToTo_DontFlagReferToToEnsure(t *testing.T) {
	linttest.AssertNoLints(t, "It could be a convenience list that one could refer to to ensure that they're not about to say a banned word.", NewToTo())
}

func TestToTo_FixAccordingToTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this URL is not formatted strictly according to to RFC2396", NewToTo(), "this URL is not formatted strictly according to RFC2396")
}

func TestToTo_FixAppliedToTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "TG_IR.corr(xlim) is not applied to to correction graphs", NewToTo(), "TG_IR.corr(xlim) is not applied to correction graphs")
}

func TestToTo_FixReferReferTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but there may be a chance to to refer non existing actor", NewToTo(), "but there may be a chance to refer non existing actor")
}

func TestToTo_FixRefersToTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Getting \"{{component}} refers to to a value, but is being used as a type\" error when attemping to identify an argument that refers to a Vue Component", NewToTo(), "Getting \"{{component}} refers to a value, but is being used as a type\" error when attemping to identify an argument that refers to a Vue Component")
}
