package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestChickenAndEgg_FixChickenHyphenEgg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but yep, can confirm the chicken-egg problem and drives me nuts", NewChickenAndEgg(), "but yep, can confirm the chicken-and-egg problem and drives me nuts")
}

func TestChickenAndEgg_DontFlagHyphenatedAnd(t *testing.T) {
	linttest.AssertNoLints(t, "Solving a chicken-and-egg problem when using Flux", NewChickenAndEgg())
}

func TestChickenAndEgg_DontFlagTitleCaseSpacesAmpersand(t *testing.T) {
	linttest.AssertNoLints(t, "How to solve Chicken & Egg problem", NewChickenAndEgg())
}

func TestChickenAndEgg_FixAnProblem(t *testing.T) {
	linttest.AssertSuggestionResult(t, "suspect it's a chicken an egg problem because peerings like subnets are not resources in their own right", NewChickenAndEgg(), "suspect it's a chicken and egg problem because peerings like subnets are not resources in their own right")
}

func TestChickenAndEgg_FixAnSituation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However, this is a chicken an egg situation in that ruby won't be installed until puppet is", NewChickenAndEgg(), "However, this is a chicken and egg situation in that ruby won't be installed until puppet is")
}

func TestChickenAndEgg_FixAnArgument(t *testing.T) {
	linttest.AssertSuggestionResult(t, "an explanation of what the whole chicken an egg argument is all about", NewChickenAndEgg(), "an explanation of what the whole chicken and egg argument is all about")
}

func TestChickenAndEgg_FixAnInDoubleQuotes(t *testing.T) {
	t.Skip("currently requires 'problems' to immediately follow the idiom")
	linttest.AssertSuggestionResult(t, "took a crack at bootstrapping UML 2 but I found there were \"chicken an egg\" problems", NewChickenAndEgg(), "took a crack at bootstrapping UML 2 but I found there were \"chicken and egg\" problems")
}

func TestChickenAndEgg_FixAnInSingleQuotes(t *testing.T) {
	t.Skip("currently requires 'scenario' to immediately follow the idiom")
	linttest.AssertSuggestionResult(t, "It's an unfalsifiable 'chicken-an-egg' scenario.", NewChickenAndEgg(), "It's an unfalsifiable 'chicken-and-egg' scenario.")
}

func TestChickenAndEgg_FixVsInDoubleQuotes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "0.12upgrade and init have a \"chicken vs egg problem\"", NewChickenAndEgg(), "0.12upgrade and init have a \"chicken vs egg problem\"")
}

func TestChickenAndEgg_FixOr(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's kind of a chicken or egg situation LOL.", NewChickenAndEgg(), "It's kind of a chicken and egg situation LOL.")
}

func TestChickenAndEgg_FixSlash(t *testing.T) {
	linttest.AssertSuggestionResult(t, "build on payloads, today, to break the chicken/egg situation", NewChickenAndEgg(), "build on payloads, today, to break the chicken & egg situation")
}

func TestChickenAndEgg_FixOrHyhens(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How to solve this chicken-or-egg conundrum?", NewChickenAndEgg(), "How to solve this chicken-and-egg conundrum?")
}

func TestChickenAndEgg_FixAndThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I worked around the chicken and the egg problem by starting kubelet-rubber-stamp outside a container.", NewChickenAndEgg(), "I worked around the chicken and egg problem by starting kubelet-rubber-stamp outside a container.")
}

func TestChickenAndEgg_FixOrTheEgg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "consul/terraform - chicken or the egg dilemma", NewChickenAndEgg(), "consul/terraform - chicken and egg dilemma")
}
