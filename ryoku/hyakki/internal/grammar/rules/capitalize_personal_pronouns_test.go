package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCapitalizePersonalPronouns_Start(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i am hungry", NewCapitalizePersonalPronouns(), "I am hungry")
}

func TestCapitalizePersonalPronouns_End(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There is no one stronger than i", NewCapitalizePersonalPronouns(), "There is no one stronger than I")
}

func TestCapitalizePersonalPronouns_Middle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "First of all, i am not happy with this.", NewCapitalizePersonalPronouns(), "First of all, I am not happy with this.")
}

func TestCapitalizePersonalPronouns_Issue365(t *testing.T) {
	linttest.AssertLintCount(t, "access will succeed, unlike with UDEREF/i386.", NewCapitalizePersonalPronouns(), 0)
}

func TestCapitalizePersonalPronouns_CorrectsId(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i'd", NewCapitalizePersonalPronouns(), "I'd")
}

func TestCapitalizePersonalPronouns_CorrectRealWorldId(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Personal Homebrew tap with tools i'd like to use", NewCapitalizePersonalPronouns(), "Personal Homebrew tap with tools I'd like to use")
}

func TestCapitalizePersonalPronouns_CorrectsIdve(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i'd've", NewCapitalizePersonalPronouns(), "I'd've")
}

func TestCapitalizePersonalPronouns_CorrectRealWorldIdve(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... i'd've loved this even more twice length , but let not get greedy", NewCapitalizePersonalPronouns(), "... I'd've loved this even more twice length , but let not get greedy")
}

func TestCapitalizePersonalPronouns_CorrectsIll(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i'll", NewCapitalizePersonalPronouns(), "I'll")
}

func TestCapitalizePersonalPronouns_CorrectRealWorldIll(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hey i deploy my contract it give me error and i'll match with the script file both are same if someone have idea how i slove this please ...", NewCapitalizePersonalPronouns(), "Hey I deploy my contract it give me error and I'll match with the script file both are same if someone have idea how I slove this please ...")
}

func TestCapitalizePersonalPronouns_CorrectsIm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i'm", NewCapitalizePersonalPronouns(), "I'm")
}

func TestCapitalizePersonalPronouns_CorrectRealWorldIm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Grid view not working, i'm not using any template", NewCapitalizePersonalPronouns(), "Grid view not working, I'm not using any template")
}

func TestCapitalizePersonalPronouns_CorrectsIve(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i've", NewCapitalizePersonalPronouns(), "I've")
}

func TestCapitalizePersonalPronouns_CorrectRealWorldIve(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can't use Github Pro although i've verified for student pack", NewCapitalizePersonalPronouns(), "Can't use Github Pro although I've verified for student pack")
}
