package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOneOfTheSingular_FixOneOfTheNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "one of the noun", NewOneOfTheSingular(dict.Curated()), "one of the nouns")
}

func TestOneOfTheSingular_FixOneOfTheNounNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "one of the car park", NewOneOfTheSingular(dict.Curated()), "one of the car parks")
}

func TestOneOfTheSingular_FixOneOfTheAdjNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "one of the best noun", NewOneOfTheSingular(dict.Curated()), "one of the best nouns")
}

func TestOneOfTheSingular_FixOneOfTheAdvAdvAdjAdjNounNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "one of the really incredibly big red rubber ball", NewOneOfTheSingular(dict.Curated()), "one of the really incredibly big red rubber balls")
}

func TestOneOfTheSingular_FixOneOfTheBestTutorial(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Bro casually dropped one of the best graphics tutorial I've ever seen and thought we wouldn't notice", NewOneOfTheSingular(dict.Curated()), "Bro casually dropped one of the best graphics tutorials I've ever seen and thought we wouldn't notice")
}

func TestOneOfTheSingular_FixOneOfTheNeatTrick(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One of the neat trick with AVX-512 is that given a mask", NewOneOfTheSingular(dict.Curated()), "One of the neat tricks with AVX-512 is that given a mask")
}

func TestOneOfTheSingular_FixOneOfTheLatestVersion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Footer line shown since one of the latest version", NewOneOfTheSingular(dict.Curated()), "Footer line shown since one of the latest versions")
}

func TestOneOfTheSingular_FixOneOfTheNode(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... noticed occasional production issue when one of the node loses connection", NewOneOfTheSingular(dict.Curated()), "... noticed occasional production issue when one of the nodes loses connection")
}

func TestOneOfTheSingular_FixOneOfTheUnstagedFile(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sublime Merge hangs if one of the unstaged file is a pretty ...", NewOneOfTheSingular(dict.Curated()), "Sublime Merge hangs if one of the unstaged files is a pretty ...")
}

func TestOneOfTheSingular_FixOneOfTheTediousThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One of the tedious thing in Stack Overflow is to grab example data provided by users", NewOneOfTheSingular(dict.Curated()), "One of the tedious things in Stack Overflow is to grab example data provided by users")
}

func TestOneOfTheSingular_FixOneOfTheBraveProcess(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One of the Brave Process is consuming almost 170%", NewOneOfTheSingular(dict.Curated()), "One of the Brave Processes is consuming almost 170%")
}

func TestOneOfTheSingular_FixOneOfTheMostCumbersomeThing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One of the most cumbersome thing to create in markdown is a table.", NewOneOfTheSingular(dict.Curated()), "One of the most cumbersome things to create in markdown is a table.")
}

func TestOneOfTheSingular_FixOneOfTheTest(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Not passing one of the test", NewOneOfTheSingular(dict.Curated()), "Not passing one of the tests")
}

func TestOneOfTheSingular_FixOneOfTheProcessMainThread(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And those threads life cycle is very long, sometimes, it will be one of the process main thread", NewOneOfTheSingular(dict.Curated()), "And those threads life cycle is very long, sometimes, it will be one of the process main threads")
}

func TestOneOfTheSingular_DontFlagBeing(t *testing.T) {
	linttest.AssertNoLints(t, "HMMs underlie the functioning of stochastic taggers and are used in various algorithms one of the most widely used being the bi-directional inference algorithm.", NewOneOfTheSingular(dict.Curated()))
}

func TestOneOfTheSingular_DontFlagOneOfTheRabbitsGloves(t *testing.T) {
	linttest.AssertNoLints(t, "As she said this she looked down at her hands, and was surprised to see that she had put on one of the Rabbit’s little white kid gloves while she was talking.", NewOneOfTheSingular(dict.Curated()))
}

func TestOneOfTheSingular_DontFlagOneOfTheLatter(t *testing.T) {
	linttest.AssertNoLints(t, "He asked a few friends and others asked him to come give talks. One of the latter was Jeanne Russell, who lives with her husband, Ron.", NewOneOfTheSingular(dict.Curated()))
}

func TestOneOfTheSingular_DontFlagOneOfTheFormer(t *testing.T) {
	linttest.AssertNoLints(t, "One of the former has to stay behind to lock up, so let them go first.", NewOneOfTheSingular(dict.Curated()))
}
