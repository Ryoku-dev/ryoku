package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMissingTo_InsertsToAfterMeant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I meant call you last night.", NewMissingTo(), "I meant to call you last night.")
}

func TestMissingTo_InsertsToAfterWants(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She wants finish early.", NewMissingTo(), "She wants to finish early.")
}

func TestMissingTo_InsertsToAfterNeed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We need talk about pricing.", NewMissingTo(), "We need to talk about pricing.")
}

func TestMissingTo_InsertsToAfterAgreed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They agreed meet at dawn.", NewMissingTo(), "They agreed to meet at dawn.")
}

func TestMissingTo_InsertsToAfterForgot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He forgot send the file.", NewMissingTo(), "He forgot to send the file.")
}

func TestMissingTo_InsertsToAfterTrying(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm trying get better at chess.", NewMissingTo(), "I'm trying to get better at chess.")
}

func TestMissingTo_InsertsToAfterRefused(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She refused answer the question.", NewMissingTo(), "She refused to answer the question.")
}

func TestMissingTo_InsertsToAfterReady(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We're ready start the meeting.", NewMissingTo(), "We're ready to start the meeting.")
}

func TestMissingTo_InsertsToAfterEager(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm eager see the results.", NewMissingTo(), "I'm eager to see the results.")
}

func TestMissingTo_InsertsToAfterInclined(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm inclined believe you.", NewMissingTo(), "I'm inclined to believe you.")
}

func TestMissingTo_InsertsToAfterResolved(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She resolved solve the case.", NewMissingTo(), "She resolved to solve the case.")
}

func TestMissingTo_Issue3951PossessiveNoun(t *testing.T) {
	linttest.AssertNoLints(t, "This Article's aims are both theoretical and historical.", NewMissingTo())
}

func TestMissingTo_NoLintWhenToPresent(t *testing.T) {
	linttest.AssertNoLints(t, "She wants to finish early.", NewMissingTo())
}

func TestMissingTo_NoLintWithNounAfterController(t *testing.T) {
	linttest.AssertNoLints(t, "They arranged a meeting at noon.", NewMissingTo())
}

func TestMissingTo_NoLintNeedsFollowUpAppointments(t *testing.T) {
	linttest.AssertNoLints(t, "Gus is recovering well, though he needs follow-up appointments.", NewMissingTo())
}

func TestMissingTo_NoLintDelaysMeantDecisions(t *testing.T) {
	linttest.AssertNoLints(t, "The delays meant decisions were often made on outdated information, hindering agility and potentially impacting return on investment.", NewMissingTo())
}

func TestMissingTo_NoLintReducedRelativeClauseAfterParticiple(t *testing.T) {
	linttest.AssertNoLints(t, "The techniques learned would probably not change much with resolution so 480i would seem almost as usable for educational use as 8K video.", NewMissingTo())
}

func TestMissingTo_NoLintBouquetOfRoses(t *testing.T) {
	linttest.AssertNoLints(t, "I made a note to request a small bouquet of roses for his room, a simple gesture that I hoped would bring a moment of solace.", NewMissingTo())
}

func TestMissingTo_NoLintForIntendedWordPhrase(t *testing.T) {
	linttest.AssertNoLints(t, "Detects incorrect usage of `peak` when the intended word is `pique`.", NewMissingTo())
}

func TestMissingTo_NoLintLongPassage(t *testing.T) {
	linttest.AssertNoLints(t, "Before her was another long passage illuminated by lamps.", NewMissingTo())
}

func TestMissingTo_NoLintLongIslandSound(t *testing.T) {
	linttest.AssertNoLints(t, "The sailboat drifted along Long Island Sound at sunrise.", NewMissingTo())
}

func TestMissingTo_NoLintLearnTagProbabilities(t *testing.T) {
	linttest.AssertNoLints(t, "These models learn tag probabilities from annotated corpora.", NewMissingTo())
}

func TestMissingTo_NoLintStandardFeatureNominalPhrase(t *testing.T) {
	linttest.AssertNoLints(t, "This is a standard and expected feature for any e-commerce site selling visually-driven products.", NewMissingTo())
}

func TestMissingTo_NoLintMixingBowlNominalPhrase(t *testing.T) {
	linttest.AssertNoLints(t, "This is a 2-quart mixing bowl, ideal for everything from whipping cream to preparing cake batter.", NewMissingTo())
}

func TestMissingTo_NoLintTryAndSay(t *testing.T) {
	linttest.AssertNoLints(t, "I'll try and say hello before I leave.", NewMissingTo())
}

func TestMissingTo_NoLintFailedEditAttempts(t *testing.T) {
	linttest.AssertNoLints(t, "failed edit attempts", NewMissingTo())
}

func TestMissingTo_NoLintReadyWork(t *testing.T) {
	linttest.AssertNoLints(t, "ready work", NewMissingTo())
}

func TestMissingTo_NoLintBadAtManagingSideEffects(t *testing.T) {
	linttest.AssertNoLints(t, "Bad at managing side-effects", NewMissingTo())
}

func TestMissingTo_NoLintAFullyResolvedConflict(t *testing.T) {
	linttest.AssertNoLints(t, "a fully resolved conflict", NewMissingTo())
}

func TestMissingTo_NoLintAResolvedConfiguration(t *testing.T) {
	linttest.AssertNoLints(t, "A resolved configuration", NewMissingTo())
}

func TestMissingTo_NoLintAFullyResolvedConfiguration(t *testing.T) {
	linttest.AssertNoLints(t, "A fully resolved configuration", NewMissingTo())
}

func TestMissingTo_NoLintAResolvedSetOfConfiguration(t *testing.T) {
	linttest.AssertNoLints(t, "A resolved set of configuration", NewMissingTo())
}

func TestMissingTo_NoLintAFullyResolvedSetOfConfiguration(t *testing.T) {
	linttest.AssertNoLints(t, "A fully resolved set of configuration", NewMissingTo())
}

func TestMissingTo_NoLintSystemProducedAFullyResolvedSetOfDependencies(t *testing.T) {
	linttest.AssertNoLints(t, "System produced a fully resolved set of dependencies", NewMissingTo())
}

func TestMissingTo_NoLintAResolvedListOfParameters(t *testing.T) {
	linttest.AssertNoLints(t, "A resolved list of parameters", NewMissingTo())
}

func TestMissingTo_NoLintAFullyResolvedListOfParameters(t *testing.T) {
	linttest.AssertNoLints(t, "A fully resolved list of parameters", NewMissingTo())
}

func TestMissingTo_NoLintAPreparedStranger(t *testing.T) {
	linttest.AssertNoLints(t, "A prepared stranger", NewMissingTo())
}

func TestMissingTo_NoLintAFullyPreparedStranger(t *testing.T) {
	linttest.AssertNoLints(t, "A fully prepared stranger", NewMissingTo())
}

func TestMissingTo_NoLintAPreparedGroupOfStrangers(t *testing.T) {
	linttest.AssertNoLints(t, "A prepared group of strangers", NewMissingTo())
}

func TestMissingTo_NoLintAFullyPreparedGroupOfStrangers(t *testing.T) {
	linttest.AssertNoLints(t, "A fully prepared group of strangers", NewMissingTo())
}

func TestMissingTo_NoLintANicelyArrangedSetOfFlowers(t *testing.T) {
	linttest.AssertNoLints(t, "A nicely arranged bunch of flowers", NewMissingTo())
}

func TestMissingTo_NoLintARecentlyForgottenListOfNames(t *testing.T) {
	linttest.AssertNoLints(t, "A recently forgotten list of names", NewMissingTo())
}

func TestMissingTo_April163188(t *testing.T) {
	linttest.AssertNoLints(t, "I want people like you in my life.", NewMissingTo())
}

func TestMissingTo_June133188(t *testing.T) {
	linttest.AssertNoLints(t, "Sometimes too much or too little to do. If too much, might be serious about wanting distance from others.", NewMissingTo())
}

func TestMissingTo_NoLintAttemptedMurder(t *testing.T) {
	linttest.AssertNoLints(t, "You’re under arrest for racketeering and attempted murder of Alek Suvor.", NewMissingTo())
}

func TestMissingTo_NoLintAttemptedRobbery(t *testing.T) {
	linttest.AssertNoLints(t, "He was convicted of attempted robbery.", NewMissingTo())
}

func TestMissingTo_InsertsToAfterAttempted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They attempted solve the problem without assistance.", NewMissingTo(), "They attempted to solve the problem without assistance.")
}

func TestMissingTo_InsertsToAfterAttemptedWithPrecedingThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He found the thing and attempted solve it.", NewMissingTo(), "He found the thing and attempted to solve it.")
}

func TestMissingTo_AllowsTryingStuffIssue4579(t *testing.T) {
	linttest.AssertNoLints(t, "I was just trying stuff out.", NewMissingTo())
}

func TestMissingTo_AllowsTryWithANounObject(t *testing.T) {
	linttest.AssertNoLints(t, "After trying stuff for hours, we tried things the docs suggested.", NewMissingTo())
}
