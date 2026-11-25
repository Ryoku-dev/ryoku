package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSomethingIs_FixesSomethingsGoing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Somethings going well today.", NewSomethingIs(), "Something's going well today.")
}

func TestSomethingIs_FixesAnythingsHappening(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Anythings happening tonight?", NewSomethingIs(), "Anything's happening tonight?")
}

func TestSomethingIs_FixesEverythingsWorking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Everythings working smoothly.", NewSomethingIs(), "Everything's working smoothly.")
}

func TestSomethingIs_FixesNothingsChanging(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nothings changing around here.", NewSomethingIs(), "Nothing's changing around here.")
}

func TestSomethingIs_FixesWithAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Somethings really happening now.", NewSomethingIs(), "Something's really happening now.")
}

func TestSomethingIs_FixesUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SOMETHINGS HAPPENING NOW!", NewSomethingIs(), "SOMETHING'S HAPPENING NOW!")
}

func TestSomethingIs_OffersIsExpansion(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Somethings going wrong.", NewSomethingIs(), "Something is going wrong.")
}

func TestSomethingIs_NoLintWhenContracted(t *testing.T) {
	linttest.AssertNoLints(t, "Something's going well today.", NewSomethingIs())
}

func TestSomethingIs_NoLintWhenPluralNoun(t *testing.T) {
	linttest.AssertLintCount(t, "Somethings in the attic kept us awake.", NewSomethingIs(), 0)
}

func TestSomethingIs_NoLintAtSentenceEnd(t *testing.T) {
	linttest.AssertNoLints(t, "Somethings.", NewSomethingIs())
}
