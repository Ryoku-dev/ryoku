package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHelpedPast_ThatIHelpedBuilt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... as I have moved from CraftCMS into a startup that I helped built", NewHelpedPast(dict.Curated()), "... as I have moved from CraftCMS into a startup that I helped build")
}

func TestHelpedPast_IHelpedBuiltSome(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Strangely, thru some form of \"magic\" I helped built some hundreds of commercial T2 targets", NewHelpedPast(dict.Curated()), "Strangely, thru some form of \"magic\" I helped build some hundreds of commercial T2 targets")
}

func TestHelpedPast_HasHelpedBuilt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Dify is an LLM application development platform that has helped built over 100,000 applications.", NewHelpedPast(dict.Curated()), "Dify is an LLM application development platform that has helped build over 100,000 applications.")
}

func TestHelpedPast_WhoHelpedMade(t *testing.T) {
	linttest.AssertSuggestionResult(t, "our many amazing contributors who helped made this possible", NewHelpedPast(dict.Curated()), "our many amazing contributors who helped make this possible")
}

func TestHelpedPast_HasHelpedMade(t *testing.T) {
	linttest.AssertSuggestionResult(t, "everyone who says that something I've worked on has helped made their lives easier", NewHelpedPast(dict.Curated()), "everyone who says that something I've worked on has helped make their lives easier")
}

func TestHelpedPast_IHelpedMade(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a a video game I helped made for a national competition in high school.", NewHelpedPast(dict.Curated()), "This is a a video game I helped make for a national competition in high school.")
}

func TestHelpedPast_HelpedMadeAtStart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Spinner Tutorial - Helped made the roulette wheel come to life.", NewHelpedPast(dict.Curated()), "Spinner Tutorial - Helped make the roulette wheel come to life.")
}

func TestHelpedPast_HelpedChose(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The 2018 analysis helped chose tires (as well as validated prior choices)", NewHelpedPast(dict.Curated()), "The 2018 analysis helped choose tires (as well as validated prior choices)")
}

func TestHelpedPast_HelpedCreated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "React helped created a light weight dynamic webpage that can be implmented in various different applications.", NewHelpedPast(dict.Curated()), "React helped create a light weight dynamic webpage that can be implmented in various different applications.")
}

func TestHelpedPast_HelpedDebugged(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Someone helped debugged this issue.", NewHelpedPast(dict.Curated()), "Someone helped debug this issue.")
}

func TestHelpedPast_HelpedDesigned(t *testing.T) {
	linttest.AssertSuggestionResult(t, "when speaking with Ka Ming who has helped designed many of the UX/UI elements for the Security Plugin", NewHelpedPast(dict.Curated()), "when speaking with Ka Ming who has helped design many of the UX/UI elements for the Security Plugin")
}

func TestHelpedPast_HelpedImplemented(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The way to do that, that I have helped implemented in some other addons already", NewHelpedPast(dict.Curated()), "The way to do that, that I have helped implement in some other addons already")
}

func TestHelpedPast_AlsoHelpedImproved(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Also helped improved and scale billing infrastructure as customer growth accelerated especially in the enterprise space.", NewHelpedPast(dict.Curated()), "Also helped improve and scale billing infrastructure as customer growth accelerated especially in the enterprise space.")
}

func TestHelpedPast_HasHelpedImproved(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thanks for making Psalm, it's a fantastic tool and has helped improved our code quality massively.", NewHelpedPast(dict.Curated()), "Thanks for making Psalm, it's a fantastic tool and has helped improve our code quality massively.")
}

func TestHelpedPast_HelpedPicked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "you helped picked the default", NewHelpedPast(dict.Curated()), "you helped pick the default")
}

func TestHelpedPast_HelpedWrote(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Disclosure: I didn't find the bugs. I helped wrote the blog post.", NewHelpedPast(dict.Curated()), "Disclosure: I didn't find the bugs. I helped write the blog post.")
}

func TestHelpedPast_DontFlagShouldveHelpedGivenThat(t *testing.T) {
	linttest.AssertNoLints(t, "However, it seems like 1 minute timeout should've helped given that default idle timeout is 10 minutes.", NewHelpedPast(dict.Curated()))
}

func TestHelpedPast_DontFlagBeHelpedGivenThe(t *testing.T) {
	linttest.AssertNoLints(t, "Major API breakages can't be helped given the nature of code generation", NewHelpedPast(dict.Curated()))
}

func TestHelpedPast_DontFlagHelpedGot(t *testing.T) {
	linttest.AssertNoLints(t, "but along the way everyone who helped got +1 from me", NewHelpedPast(dict.Curated()))
}

func TestHelpedPast_DontFlagHelpedDid(t *testing.T) {
	linttest.AssertNoLints(t, "So, that helped did it?", NewHelpedPast(dict.Curated()))
}
