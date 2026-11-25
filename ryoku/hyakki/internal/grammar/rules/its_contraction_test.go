package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestItsContraction_FixHad(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its had an enormous effect.", NewItsContraction(), "It's had an enormous effect.")
}

func TestItsContraction_FixBeen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its been months since we spoke.", NewItsContraction(), "It's been months since we spoke.")
}

func TestItsContraction_FixGot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think its got nothing to do with us.", NewItsContraction(), "I think it's got nothing to do with us.")
}

func TestItsContraction_FixesItsCommon(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its common for users to get frustrated.", NewItsContraction(), "It's common for users to get frustrated.")
}

func TestItsContraction_IgnoreCorrectContraction(t *testing.T) {
	linttest.AssertLintCount(t, "It's been a long year for everyone.", NewItsContraction(), 0)
}

func TestItsContraction_IgnorePossessive(t *testing.T) {
	linttest.AssertLintCount(t, "The company revised its policies last week.", NewItsContraction(), 0)
}

func TestItsContraction_IgnoreCoroutine(t *testing.T) {
	linttest.AssertLintCount(t, "Launch each task within its own child coroutine.", NewItsContraction(), 0)
}

func TestItsContraction_Issue381(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its a nice day.", NewItsContraction(), "It's a nice day.")
}

func TestItsContraction_IgnoreNominalProgressive(t *testing.T) {
	linttest.AssertLintCount(t, "The class preserves its existing properties.", NewItsContraction(), 0)
}

func TestItsContraction_IgnoreNominalPerfect(t *testing.T) {
	t.Skip("past participles are not always adjectives ('cared' for instance)")
	linttest.AssertLintCount(t, "The robot followed its predetermined route.", NewItsContraction(), 0)
}

func TestItsContraction_IgnoreNominalLong(t *testing.T) {
	linttest.AssertLintCount(t, "I think of its exploding marvelous spectacular output.", NewItsContraction(), 0)
}

func TestItsContraction_CorrectsBecause(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its because they don't want to.", NewItsContraction(), "It's because they don't want to.")
}

func TestItsContraction_CorrectsItsHard(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its hard to believe that.", NewItsContraction(), "It's hard to believe that.")
}

func TestItsContraction_CorrectsItsEasy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its easy if you try.", NewItsContraction(), "It's easy if you try.")
}

func TestItsContraction_CorrectsItsAPicnic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its a beautiful day for a picnic", NewItsContraction(), "It's a beautiful day for a picnic")
}

func TestItsContraction_CorrectsItsMy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its my favorite song.", NewItsContraction(), "It's my favorite song.")
}

func TestItsContraction_AllowsItsNew(t *testing.T) {
	linttest.AssertNoLints(t, "The company announced its new product line. ", NewItsContraction())
}

func TestItsContraction_AllowsItsOwnCharm(t *testing.T) {
	linttest.AssertNoLints(t, "The house has its own charm. ", NewItsContraction())
}

func TestItsContraction_AllowsItsVictory(t *testing.T) {
	linttest.AssertNoLints(t, "The team celebrated its victory. ", NewItsContraction())
}

func TestItsContraction_AllowsItsHistory(t *testing.T) {
	linttest.AssertNoLints(t, "The country is proud of its history. ", NewItsContraction())
}

func TestItsContraction_AllowsItsSecrets(t *testing.T) {
	linttest.AssertNoLints(t, "The book contains its own secrets. ", NewItsContraction())
}

func TestItsContraction_CorrectsThinkGoogle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think its Google, not Microsoft.", NewItsContraction(), "I think it's Google, not Microsoft.")
}

func TestItsContraction_CorrectsHopeKatie(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I hope its Katie.", NewItsContraction(), "I hope it's Katie.")
}

func TestItsContraction_CorrectsGuessDate(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I guess its March 6.", NewItsContraction(), "I guess it's March 6.")
}

func TestItsContraction_CorrectsAssumeJohn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We assume its John.", NewItsContraction(), "We assume it's John.")
}

func TestItsContraction_CorrectsDoubtTesla(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They doubt its Tesla this year.", NewItsContraction(), "They doubt it's Tesla this year.")
}

func TestItsContraction_HandlesTwoWordName(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She thinks its New York.", NewItsContraction(), "She thinks it's New York.")
}

func TestItsContraction_IgnoresExistingContraction(t *testing.T) {
	linttest.AssertLintCount(t, "I think it's Google.", NewItsContraction(), 0)
}

func TestItsContraction_IgnoresPossessiveNounAfterName(t *testing.T) {
	linttest.AssertLintCount(t, "I think its Google product launch.", NewItsContraction(), 0)
}

func TestItsContraction_IgnoresWithoutOpinionVerb(t *testing.T) {
	linttest.AssertLintCount(t, "Its Google Pixel lineup is impressive.", NewItsContraction(), 0)
}

func TestItsContraction_IgnoresCommonNounTarget(t *testing.T) {
	linttest.AssertLintCount(t, "We hope its accuracy improves.", NewItsContraction(), 0)
}

func TestItsContraction_Issue2547(t *testing.T) {
	linttest.AssertNoLints(t, "using the foo feature and its associated parameter", NewItsContraction())
}

func TestItsContraction_Issue2609(t *testing.T) {
	linttest.AssertNoLints(t, "One such aspect is how the standard library takes great care to make working with its built-in collections as efficient as possible.", NewItsContraction())
}

func TestItsContraction_IgnorePastParticipleNounPhrase(t *testing.T) {
	linttest.AssertNoLints(t, "using the foo feature and its abetted parameter", NewItsContraction())
}

func TestItsContraction_CorrectsPredicativeCalled(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its called recursion.", NewItsContraction(), "It's called recursion.")
}

func TestItsContraction_CorrectsPredicativeNamed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its named Manhattan.", NewItsContraction(), "It's named Manhattan.")
}

func TestItsContraction_AllowsPossessiveGeneratedCode(t *testing.T) {
	linttest.AssertNoLints(t, "The compiler emits its generated code.", NewItsContraction())
}

func TestItsContraction_CorrectsItsAnybody(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its anybody who volunteers should speak up.", NewItsContraction(), "It's anybody who volunteers should speak up.")
}

func TestItsContraction_CorrectsItsAnyone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its anyone you ping will be looped in automatically.", NewItsContraction(), "It's anyone you ping will be looped in automatically.")
}

func TestItsContraction_CorrectsItsEveryone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its everyone on the thread noticed the spike.", NewItsContraction(), "It's everyone on the thread noticed the spike.")
}

func TestItsContraction_CorrectsItsEverything(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its everything we collect ends up in the archive.", NewItsContraction(), "It's everything we collect ends up in the archive.")
}

func TestItsContraction_CorrectsItsSomebody(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its somebody on call right now.", NewItsContraction(), "It's somebody on call right now.")
}

func TestItsContraction_CorrectsItsSomewhere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its somewhere safe to stash the nightly dump.", NewItsContraction(), "It's somewhere safe to stash the nightly dump.")
}

func TestItsContraction_CorrectsItsNobody(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its nobody left who can approve this.", NewItsContraction(), "It's nobody left who can approve this.")
}

func TestItsContraction_CorrectsItsAnywhere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its anywhere the monitors blink red.", NewItsContraction(), "It's anywhere the monitors blink red.")
}

func TestItsContraction_CorrectsItsAnything(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its anything worth keeping should be archived.", NewItsContraction(), "It's anything worth keeping should be archived.")
}

func TestItsContraction_CorrectsItsSomething(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its something that keeps restarting the job.", NewItsContraction(), "It's something that keeps restarting the job.")
}

func TestItsContraction_AllowsItsTail(t *testing.T) {
	linttest.AssertNoLints(t, "Its tail twitches every time I call it.", NewItsContraction())
}

func TestItsContraction_AllowsItsReleasePipeline(t *testing.T) {
	linttest.AssertNoLints(t, "Its release pipeline is stable in production.", NewItsContraction())
}

func TestItsContraction_AllowsItsNodes(t *testing.T) {
	linttest.AssertNoLints(t, "The cluster updates its nodes nightly.", NewItsContraction())
}

func TestItsContraction_AllowsItsCoverPage(t *testing.T) {
	linttest.AssertNoLints(t, "Its cover page mentions all contributors.", NewItsContraction())
}

func TestItsContraction_AllowsItsNotAnybody(t *testing.T) {
	linttest.AssertNoLints(t, "Its not anybody to blame despite the outage.", NewItsContraction())
}

func TestItsContraction_AllowsItsNeverAnywhere(t *testing.T) {
	linttest.AssertNoLints(t, "Its never anywhere near the ideal timing we hoped for.", NewItsContraction())
}

func TestItsContraction_AllowsAtItsHighest(t *testing.T) {
	linttest.AssertNoLints(t, "Curiosity about Gatsby was at its highest that night.", NewItsContraction())
}

func TestItsContraction_AllowsItsPullIs(t *testing.T) {
	linttest.AssertNoLints(t, "The Earth is huge, so its pull is super strong.", NewItsContraction())
}

func TestItsContraction_AllowsItsUnforeseenConsequences(t *testing.T) {
	linttest.AssertNoLints(t, "The experiment continued despite its unforeseen consequences.", NewItsContraction())
}

func TestItsContraction_AllowsItsPotentialToBenefit(t *testing.T) {
	linttest.AssertNoLints(t, "The proposal emphasized its potential to benefit local businesses.", NewItsContraction())
}

func TestItsContraction_AllowsItsCoverWas(t *testing.T) {
	linttest.AssertNoLints(t, "Its cover was intricately engraved with floral patterns.", NewItsContraction())
}

func TestItsContraction_AllowsItsStartingLevel(t *testing.T) {
	linttest.AssertNoLints(t, "Reduce the tracker to its starting level.", NewItsContraction())
}

func TestItsContraction_Issue4435(t *testing.T) {
	linttest.AssertNoLints(t, "I spent more effort in its reading than he did in its creation.", NewItsContraction())
}

func TestItsContraction_AllowsItsGerundAfterPreposition(t *testing.T) {
	linttest.AssertNoLints(t, "The value comes from its measuring of the output.", NewItsContraction())
}

func TestItsContraction_AllowsItsGerundAfterUnlistedPreposition(t *testing.T) {
	linttest.AssertNoLints(t, "I spent more effort during its reading than he did during its creation.", NewItsContraction())
}
