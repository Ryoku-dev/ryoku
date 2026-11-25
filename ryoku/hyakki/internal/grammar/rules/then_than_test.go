package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThenThan_AllowsBackThen(t *testing.T) {
	linttest.AssertLintCount(t, "I was a gross kid back then.", NewThenThan(), 0)
}

func TestThenThan_CatchesShorterThen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One was shorter then the other.", NewThenThan(), "One was shorter than the other.")
}

func TestThenThan_CatchesBetterThen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One was better then the other.", NewThenThan(), "One was better than the other.")
}

func TestThenThan_CatchesLongerThen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One was longer then the other.", NewThenThan(), "One was longer than the other.")
}

func TestThenThan_CatchesLessThen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I eat less then you.", NewThenThan(), "I eat less than you.")
}

func TestThenThan_CatchesMoreThen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I eat more then you.", NewThenThan(), "I eat more than you.")
}

func TestThenThan_StrongerShouldChange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "a chain is no stronger then its weakest link", NewThenThan(), "a chain is no stronger than its weakest link")
}

func TestThenThan_HalfALoafShouldChange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "half a loaf is better then no bread", NewThenThan(), "half a loaf is better than no bread")
}

func TestThenThan_ThenEveryoneClappedShouldBeAllowed(t *testing.T) {
	linttest.AssertLintCount(t, "and then everyone clapped", NewThenThan(), 0)
}

func TestThenThan_CrazierThanRatShouldChange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "crazier then a shithouse rat", NewThenThan(), "crazier than a shithouse rat")
}

func TestThenThan_PokeInEyeShouldChange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "better then a poke in the eye with a sharp stick", NewThenThan(), "better than a poke in the eye with a sharp stick")
}

func TestThenThan_OtherThenShouldChange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There was no one other then us at the campsite.", NewThenThan(), "There was no one other than us at the campsite.")
}

func TestThenThan_AllowsAndThen(t *testing.T) {
	linttest.AssertLintCount(t, "And then we left.", NewThenThan(), 0)
}

func TestThenThan_AllowsThisThen(t *testing.T) {
	linttest.AssertLintCount(t, "Do this then that.", NewThenThan(), 0)
}

func TestThenThan_AllowsIssue720(t *testing.T) {
	linttest.AssertLintCount(t, "And if just one of those is set incorrectly or it has the tiniest bit of dirt inside then that will wreak havoc with the engine's running ability.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "So let's check it out then.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "And if just the tiniest bit of dirt gets inside then that will wreak havoc.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "He was always a top student in school but then his argument is that grades don't define intelligence.", NewThenThan(), 0)
}

func TestThenThan_AllowsIssue744(t *testing.T) {
	linttest.AssertLintCount(t, "So then after talking about how he would, he didn't.", NewThenThan(), 0)
}

func TestThenThan_Issue720SchoolButThenHis(t *testing.T) {
	linttest.AssertLintCount(t, "She loved the atmosphere of the school but then his argument is that it lacks proper resources for students.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "The teacher praised the efforts of the school but then his argument is that the curriculum needs to be updated.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "They were excited about the new program at school but then his argument is that it won't be effective without proper training.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "The community supported the school but then his argument is that funding is still a major issue.", NewThenThan(), 0)
}

func TestThenThan_Issue720SoThenTheseResistors(t *testing.T) {
	linttest.AssertLintCount(t, "So then these resistors are connected up in parallel to reduce the overall resistance.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "So then these resistors are connected up to ensure the current flows properly.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "So then these resistors are connected up to achieve the desired voltage drop.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "So then these resistors are connected up to demonstrate the principles of series and parallel circuits.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "So then these resistors are connected up to optimize the circuit's performance.", NewThenThan(), 0)
}

func TestThenThan_Issue720YesSoThenSorry(t *testing.T) {
	linttest.AssertLintCount(t, "Yes so then sorry you didn't receive the memo about the meeting changes.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "Yes so then sorry you had to wait so long for a response from our team.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "Yes so then sorry you felt left out during the discussion; we value your input.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "Yes so then sorry you missed the deadline; we can discuss an extension.", NewThenThan(), 0)
	linttest.AssertLintCount(t, "Yes so then sorry you encountered issues with the software; let me help you troubleshoot.", NewThenThan(), 0)
}

func TestThenThan_MoreTalentedThenHerIssue720(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He was more talented then her at writing code.", NewThenThan(), "He was more talented than her at writing code.")
}

func TestThenThan_SimplerThenHersIssue720(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The design was simpler then hers in layout and color scheme.", NewThenThan(), "The design was simpler than hers in layout and color scheme.")
}

func TestThenThan_EarlierThenHimIssue720(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We arrived earlier then him at the event.", NewThenThan(), "We arrived earlier than him at the event.")
}

func TestThenThan_MoreRobustThenHisIssue720(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This approach is more robust then his for handling edge cases.", NewThenThan(), "This approach is more robust than his for handling edge cases.")
}

func TestThenThan_PatchMoreRecentlyThenLastWeekIssue720(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We submitted the patch more recently then last week, so they should have it already.", NewThenThan(), "We submitted the patch more recently than last week, so they should have it already.")
}

func TestThenThan_AllowsWellThen(t *testing.T) {
	linttest.AssertLintCount(t, "Well then we're just going to raise all of these taxes", NewThenThan(), 0)
}

func TestThenThan_AllowsNervousThen(t *testing.T) {
	linttest.AssertLintCount(t, "I think both of us were getting nervous then because the system would have automatically aborted.", NewThenThan(), 0)
}

func TestThenThan_FlagsStupiderThenAndMoreAndLessStupidThen(t *testing.T) {
	linttest.AssertLintCount(t, "He was stupider then her but she was more stupid then some. Then again he was less stupid then some too.", NewThenThan(), 3)
}

func TestThenThan_PatchWorseThen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He was worse then her at writing code.", NewThenThan(), "He was worse than her at writing code.")
}

func TestThenThan_PatchRatherThen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If copy-paste has to be prevented, I'd prefer it if paste rather then copy would be disabled", NewThenThan(), "If copy-paste has to be prevented, I'd prefer it if paste rather than copy would be disabled")
}

func TestThenThan_PatchEasierSaidThenDone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is currently easier said then done because you cannot press Ctrl+A in the debug console", NewThenThan(), "This is currently easier said than done because you cannot press Ctrl+A in the debug console")
}

func TestThenThan_PatchEveryNowAndThan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I was testing every now and than after an upgrade on the home assistant plugin.", NewThenThan(), "I was testing every now and then after an upgrade on the home assistant plugin.")
}

func TestThenThan_PatchUntilThan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For the case anyone else ever hits this and the problem is not solved until than, this is a working workaround for the problem", NewThenThan(), "For the case anyone else ever hits this and the problem is not solved until then, this is a working workaround for the problem")
}

func TestThenThan_PatchNowAndThan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "sounds good if golang-set becomes an issue between now and than…just let me know!", NewThenThan(), "sounds good if golang-set becomes an issue between now and then…just let me know!")
}
