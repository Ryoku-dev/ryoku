package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWouldNeverHave_FixCouldHaveNeverBeen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Having a conversation would have never been easier with Ramen!", NewWouldNeverHave(), "Having a conversation never would have been easier with Ramen!")
}

func TestWouldNeverHave_FixWouldHaveNeverCome(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This would have never come about without the help and encouragement of many people, too numerous to mention here.", NewWouldNeverHave(), "This never would have come about without the help and encouragement of many people, too numerous to mention here.")
}

func TestWouldNeverHave_FixWouldHaveNeverFind(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Thanks for the help, think I would have never find it out alone.", NewWouldNeverHave(), "Thanks for the help, think I never would have find it out alone.")
}

func TestWouldNeverHave_FixAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I WOULD'VE NEVER THOUGHT TO TEST ALL CAPS.", NewWouldNeverHave(), "I NEVER WOULD'VE THOUGHT TO TEST ALL CAPS.")
}

func TestWouldNeverHave_FixTitleCase(t *testing.T) {
	t.Skip("Upstream ignored: Fails due to the strange way replace_with_match_case works")
	linttest.AssertSuggestionResult(t, "I Would Never Have Thought To Test Title Case English.", NewWouldNeverHave(), "I Never Would Have Thought To Test Title Case English.")
}

func TestWouldNeverHave_FixCouldNeverHaveWorked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "ft_quantile_discretizer could never have worked", NewWouldNeverHave(), "ft_quantile_discretizer never could have worked")
}

func TestWouldNeverHave_FixWouldNeverHaveThoughtOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We discover security flaws that your team would never have thought of.", NewWouldNeverHave(), "We discover security flaws that your team never would have thought of.")
}

func TestWouldNeverHave_FixWouldveNeverKnownMissingApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We wouldve never known from the current api docs", NewWouldNeverHave(), "We never would've known from the current api docs")
}

func TestWouldNeverHave_FixWouldveNeverGrokked(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would've never grokked that it's an issue in rollup.", NewWouldNeverHave(), "I never would've grokked that it's an issue in rollup.")
}

func TestWouldNeverHave_FixCouldveNeverDesigned(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Without my subscription I could've never designed this in such little time without it.", NewWouldNeverHave(), "Without my subscription I never could've designed this in such little time without it.")
}
