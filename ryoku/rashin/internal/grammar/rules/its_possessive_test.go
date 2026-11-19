package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestItsPossessive_CorrectsItsVarious(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I like it's various colors.", NewItsPossessive(), "I like its various colors.")
}

func TestItsPossessive_FixesInspiration(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would just put `Orthography` and it's various function implementations in their own `orthography.rs` file.", NewItsPossessive(), "I would just put `Orthography` and its various function implementations in their own `orthography.rs` file.")
}

func TestItsPossessive_EngineLostItsCompression(t *testing.T) {
	linttest.AssertLintCount(t, "The engine lost it's compression.", NewItsPossessive(), 1)
}

func TestItsPossessive_AdmiredSculptureForItsIntricacy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I admired the sculpture for it's intricacy.", NewItsPossessive(), "I admired the sculpture for its intricacy.")
}

func TestItsPossessive_ParisIsKnownForItsArchitecture(t *testing.T) {
	linttest.AssertLintCount(t, "Paris is known for it's architecture.", NewItsPossessive(), 1)
}

func TestItsPossessive_PlainSentenceWithApostropheS(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's benefits are numerous.", NewItsPossessive(), "Its benefits are numerous.")
}

func TestItsPossessive_FixesItsAncestor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's ancestor is still around.", NewItsPossessive(), "Its ancestor is still around.")
}

func TestItsPossessive_DeviceReachedIts100thCycle(t *testing.T) {
	linttest.AssertLintCount(t, "The device reached it's 100th cycle.", NewItsPossessive(), 1)
}

func TestItsPossessive_OddlyItsWheelsMisaligned(t *testing.T) {
	linttest.AssertLintCount(t, "Oddly, it's wheels were misaligned.", NewItsPossessive(), 1)
}

func TestItsPossessive_LeakingOilConstantIssue(t *testing.T) {
	linttest.AssertLintCount(t, "It's leaking oil constantly.", NewItsPossessive(), 0)
}

func TestItsPossessive_FiftythAnniversary(t *testing.T) {
	linttest.AssertLintCount(t, "The company celebrated it's 50th anniversary.", NewItsPossessive(), 1)
}

func TestItsPossessive_SecondAttempt(t *testing.T) {
	linttest.AssertLintCount(t, "He failed it's 2nd attempt.", NewItsPossessive(), 1)
}

func TestItsPossessive_ThirdIteration(t *testing.T) {
	linttest.AssertLintCount(t, "The program finished it's 3rd iteration.", NewItsPossessive(), 1)
}

func TestItsPossessive_TenthMilestone(t *testing.T) {
	linttest.AssertLintCount(t, "They reached it's 10th milestone.", NewItsPossessive(), 1)
}

func TestItsPossessive_SeventhChapter(t *testing.T) {
	linttest.AssertLintCount(t, "The novel lost it's 7th chapter.", NewItsPossessive(), 1)
}

func TestItsPossessive_FifthVersion(t *testing.T) {
	linttest.AssertLintCount(t, "Software updated to it's 5th version.", NewItsPossessive(), 1)
}

func TestItsPossessive_EighthFloor(t *testing.T) {
	linttest.AssertLintCount(t, "Elevator stopped at it's 8th floor.", NewItsPossessive(), 1)
}

func TestItsPossessive_TwelfthEpisode(t *testing.T) {
	linttest.AssertLintCount(t, "Series ended it's 12th episode.", NewItsPossessive(), 1)
}

func TestItsPossessive_FourthDraft(t *testing.T) {
	linttest.AssertLintCount(t, "He completed it's 4th draft.", NewItsPossessive(), 1)
}

func TestItsPossessive_NinthRevision(t *testing.T) {
	linttest.AssertLintCount(t, "The report saved it's 9th revision.", NewItsPossessive(), 1)
}

func TestItsPossessive_AllowsHardToTell(t *testing.T) {
	linttest.AssertNoLints(t, "It's hard to tell from here.", NewItsPossessive())
}

func TestItsPossessive_AllowsIllegible(t *testing.T) {
	linttest.AssertNoLints(t, "When you write in cursive, its illegible", NewItsPossessive())
}

func TestItsPossessive_AllowsGoodPractice(t *testing.T) {
	linttest.AssertNoLints(t, "it's good practice to review the general settings", NewItsPossessive())
}

func TestItsPossessive_AllowsUnderstandable(t *testing.T) {
	linttest.AssertNoLints(t, "It's understandable that you'd feel the weight of responsibility.", NewItsPossessive())
}

func TestItsPossessive_AllowsInsincere(t *testing.T) {
	linttest.AssertNoLints(t, "But feel free to omit it if you feel it's insincere.", NewItsPossessive())
}

func TestItsPossessive_AllowsItsPossible(t *testing.T) {
	linttest.AssertNoLints(t, "It's possible that a record was improperly handled. ", NewItsPossessive())
}

func TestItsPossessive_AllowsManyTimesHarder(t *testing.T) {
	linttest.AssertNoLints(t, "It's many times harder to do this than that.", NewItsPossessive())
}

func TestItsPossessive_AllowIssue1658(t *testing.T) {
	linttest.AssertNoLints(t, "It's kind of a nuisance, but it will work.", NewItsPossessive())
}

func TestItsPossessive_AllowIssue2001(t *testing.T) {
	linttest.AssertNoLints(t, "It's worth highlighting that while using a fork instead of a spoon is easy, it sometimes isn't.", NewItsPossessive())
}

func TestItsPossessive_DontFlagIssue1722ItsWhatsAccessible(t *testing.T) {
	linttest.AssertNoLints(t, "The base execution context is the global execution context: it's what's accessible everywhere in your code.", NewItsPossessive())
}

func TestItsPossessive_DontFlagIssue1722ItsEarlyAnd(t *testing.T) {
	linttest.AssertNoLints(t, "it's early and there's plenty of room for novel and potentially", NewItsPossessive())
}

func TestItsPossessive_DontFlagIssue1722ItsBigEnough(t *testing.T) {
	linttest.AssertNoLints(t, "It's big enough.", NewItsPossessive())
}

func TestItsPossessive_AllowsItsAwesome(t *testing.T) {
	linttest.AssertNoLints(t, "It's awesome.", NewItsPossessive())
}

func TestItsPossessive_FlagsAllItsPossessiveInList(t *testing.T) {
	linttest.AssertLintCount(t, "Understand it's code, it's values, and it's purpose.", NewItsPossessive(), 3)
}

func TestItsPossessive_FixesAllItsPossessiveInList(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Understand it's code, it's values, and it's purpose.", NewItsPossessive(), "Understand its code, its values, and its purpose.")
}
