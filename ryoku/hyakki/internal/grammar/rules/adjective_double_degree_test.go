package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAdjectiveDoubleDegree_FixDoubleRegularSuperlative(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The most easiest to use, self-service open BI reporting and BI dashboard and BI monitor screen platform.", NewAdjectiveDoubleDegree(), "The easiest to use, self-service open BI reporting and BI dashboard and BI monitor screen platform.")
}

func TestAdjectiveDoubleDegree_FixDoubleRegularComparative(t *testing.T) {
	linttest.AssertSuggestionResult(t, "how can make docx gennerate more faster?", NewAdjectiveDoubleDegree(), "how can make docx gennerate faster?")
}

func TestAdjectiveDoubleDegree_FixDoubleIrregularComparative(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Find alternative product name more better than age .", NewAdjectiveDoubleDegree(), "Find alternative product name better than age .")
}

func TestAdjectiveDoubleDegree_FixDoubleIrregularSuperlative(t *testing.T) {
	linttest.AssertSuggestionResult(t, "how can i get a most best quality file", NewAdjectiveDoubleDegree(), "how can i get a best quality file")
}

func TestAdjectiveDoubleDegree_ConflictingMosterOffersTwoSuggestions(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "application which students to learn most faster in efficient way.", NewAdjectiveDoubleDegree(), []string{
		"application which students to learn faster in efficient way.",
		"application which students to learn fastest in efficient way.",
	}, []string{})
}

func TestAdjectiveDoubleDegree_ConflictingMorestOffersTwoSuggestions(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I suggest migrating to vite that more flexible and more fastest.", NewAdjectiveDoubleDegree(), []string{
		"I suggest migrating to vite that more flexible and faster.",
		"I suggest migrating to vite that more flexible and fastest.",
	}, []string{})
}

func TestAdjectiveDoubleDegree_ConflictingMostBetterOffersTwoSuggestions(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "But first logo is most better for me.", NewAdjectiveDoubleDegree(), []string{
		"But first logo is better for me.",
		"But first logo is best for me.",
	}, []string{})
}

func TestAdjectiveDoubleDegree_ConflictingMostWorseOffersTwoSuggestions(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "We also see the need of a generic solution built-in in Pimcore, but currently it's probably the most worse time to implement a new solution.", NewAdjectiveDoubleDegree(), []string{

		"We also see the need of a generic solution built-in in Pimcore, but currently it's probably the worse time to implement a new solution.",
		"We also see the need of a generic solution built-in in Pimcore, but currently it's probably the worst time to implement a new solution.",
	}, []string{})
}
