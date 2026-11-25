package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWereWhere_FixTheyWhere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They where going to the store.", NewWereWhere(), "They were going to the store.")
}

func TestWereWhere_FixWeWhere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We where right about that.", NewWereWhere(), "We were right about that.")
}

func TestWereWhere_FixTheyWhereHappy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They where happy with the result.", NewWereWhere(), "They were happy with the result.")
}

func TestWereWhere_FixYouWhereGoing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you where going in the right direction.", NewWereWhere(), "you were going in the right direction.")
}

func TestWereWhere_FixYouWhereRight(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you where right about that.", NewWereWhere(), "you were right about that.")
}

func TestWereWhere_FixKnowWereThey(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Do you know were they went?", NewWereWhere(), "Do you know where they went?")
}

func TestWereWhere_FixForgotWereI(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I forgot were I put my keys.", NewWereWhere(), "I forgot where I put my keys.")
}

func TestWereWhere_FixFoundWereThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I found were the book was.", NewWereWhere(), "I found where the book was.")
}

func TestWereWhere_FixGoWereThey(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Go were they tell you.", NewWereWhere(), "Go where they tell you.")
}

func TestWereWhere_FixWeWhereAlmostDone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We where almost done with the task.", NewWereWhere(), "We were almost done with the task.")
}

func TestWereWhere_FixTheyWhereAble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They where able to fix the issue in time.", NewWereWhere(), "They were able to fix the issue in time.")
}

func TestWereWhere_FixWeWhereTold(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We where told about the change last week.", NewWereWhere(), "We were told about the change last week.")
}

func TestWereWhere_FixTheyWhereSupposed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They where supposed to be here by now.", NewWereWhere(), "They were supposed to be here by now.")
}

func TestWereWhere_FixYouWhereSupposed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You where supposed to call me.", NewWereWhere(), "You were supposed to call me.")
}

func TestWereWhere_FixYouWhereAsked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you where asked to leave the room.", NewWereWhere(), "you were asked to leave the room.")
}

func TestWereWhere_FixWhereYouAble(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Where you able to make forward progress here?", NewWereWhere(), "Were you able to make forward progress here?")
}

func TestWereWhere_FixRememberWereI(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Do you remember were I left the keys?", NewWereWhere(), "Do you remember where I left the keys?")
}

func TestWereWhere_FixCheckWereThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Check were the error occurred.", NewWereWhere(), "Check where the error occurred.")
}

func TestWereWhere_FixAskedWereHe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She asked were he lived.", NewWereWhere(), "She asked where he lived.")
}

func TestWereWhere_FixKnowWereTheBug(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I know were the bug is.", NewWereWhere(), "I know where the bug is.")
}

func TestWereWhere_FixFindWereIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Find were it crashed.", NewWereWhere(), "Find where it crashed.")
}

func TestWereWhere_NoFlagWhereTheyAre(t *testing.T) {
	linttest.AssertNoLints(t, "Do you know where they are going?", NewWereWhere())
}

func TestWereWhere_NoFlagTheyWereGoing(t *testing.T) {
	linttest.AssertNoLints(t, "They were going to the store.", NewWereWhere())
}

func TestWereWhere_NoFlagWeWereRight(t *testing.T) {
	linttest.AssertNoLints(t, "We were right about that.", NewWereWhere())
}

func TestWereWhere_NoFlagShowYouWhere(t *testing.T) {
	linttest.AssertNoLints(t, "I'll show you where to go.", NewWereWhere())
}

func TestWereWhere_NoFlagTellYouWhereThe(t *testing.T) {
	linttest.AssertNoLints(t, "I'll tell you where the exit is.", NewWereWhere())
}

func TestWereWhere_NoFlagTheyWereWrong(t *testing.T) {
	linttest.AssertNoLints(t, "I think they were wrong.", NewWereWhere())
}

func TestWereWhere_NoFlagConfirmedTheyWere(t *testing.T) {
	linttest.AssertNoLints(t, "I confirmed they were correct.", NewWereWhere())
}

func TestWereWhere_NoFlagFoundTheyWere(t *testing.T) {
	linttest.AssertNoLints(t, "He found they were missing.", NewWereWhere())
}

func TestWereWhere_NoFlagWhereWereThey(t *testing.T) {
	linttest.AssertNoLints(t, "Where were they going?", NewWereWhere())
}

func TestWereWhere_NoFlagShowedMeWhere(t *testing.T) {
	linttest.AssertNoLints(t, "He showed me where the exit was.", NewWereWhere())
}

func TestWereWhere_NoFlagWhereYouGo(t *testing.T) {
	linttest.AssertNoLints(t, "I wonder where you go from here.", NewWereWhere())
}

func TestWereWhere_NoFlagWhereYouCanCustomize(t *testing.T) {
	linttest.AssertNoLints(t, "Click the menu item where you can customize the settings.", NewWereWhere())
}

func TestWereWhere_NoFlagWhereYouAllocate(t *testing.T) {
	linttest.AssertNoLints(t, "Use the panel where you allocate resources for the task.", NewWereWhere())
}

func TestWereWhere_FixYouWhereTheOnlyOne(t *testing.T) {
	t.Skip("Upstream ignored: limitation: 'you where' followed by DET is not flagged; would need DET in the following-word set")
	t.Skip("limitation: 'you where' followed by DET is not flagged; would need DET in the following-word set")
	linttest.AssertSuggestionResult(t, "you where the only one there.", NewWereWhere(), "you were the only one there.")
}

func TestWereWhere_FixWhereTheyGoingSentenceStart(t *testing.T) {
	t.Skip("Upstream ignored: limitation: sentence-initial 'Where' as typo for 'Were' is not handled")
	t.Skip("limitation: sentence-initial 'Where' as typo for 'Were' is not handled")
	linttest.AssertSuggestionResult(t, "Where they going to the party?", NewWereWhere(), "Were they going to the party?")
}

func TestWereWhere_FixShowedMeWere(t *testing.T) {
	t.Skip("Upstream ignored: limitation: indirect object between verb and 'were' is not detected")
	t.Skip("limitation: indirect object between verb and 'were' is not detected")
	linttest.AssertSuggestionResult(t, "He showed me were the exit was.", NewWereWhere(), "He showed me where the exit was.")
}

func TestWereWhere_FixIfIWhereTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If I where to have raw image data would I be able to convert that", NewWereWhere(), "If I were to have raw image data would I be able to convert that")
}

func TestWereWhere_FixIfWeWhereTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If we where to choose GraphQL, i don't think we'd be even near finished yet.", NewWereWhere(), "If we were to choose GraphQL, i don't think we'd be even near finished yet.")
}

func TestWereWhere_FixIfYouWhereTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you where to change jobs, what kind of company would you rather work for", NewWereWhere(), "If you were to change jobs, what kind of company would you rather work for")
}

func TestWereWhere_FixIfItWhereTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If it where to return after the P-statement, the value of the semaphore does not get decreased", NewWereWhere(), "If it were to return after the P-statement, the value of the semaphore does not get decreased")
}
