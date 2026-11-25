package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAsToInterrogative_FixCuriousHow(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would be curious to how you dealt with the issues I mentioned above", NewAsToInterrogative(), "I would be curious as to how you dealt with the issues I mentioned above")
}

func TestAsToInterrogative_FixCuriousWhat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm curious to what @Katzmann1983 thinks on this topic.", NewAsToInterrogative(), "I'm curious as to what @Katzmann1983 thinks on this topic.")
}

func TestAsToInterrogative_DontFlagCuriousWhatExtent(t *testing.T) {
	linttest.AssertNoLints(t, "I really just want to ask these questions / am curious to what extent anything like this is in place or will be put in place.", NewAsToInterrogative())
}

func TestAsToInterrogative_FixCuriousWhen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So I am curious to when you should return an array to destructure", NewAsToInterrogative(), "So I am curious as to when you should return an array to destructure")
}

func TestAsToInterrogative_FixCuriousWhere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Curious to where you ended up.", NewAsToInterrogative(), "Curious as to where you ended up.")
}

func TestAsToInterrogative_FixCuriousWhether(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have never used them so was curious to whether I could improve my workflow even more", NewAsToInterrogative(), "I have never used them so was curious as to whether I could improve my workflow even more")
}

func TestAsToInterrogative_FixCuriousWhich(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm curious to which country you're assigning these noble ideals.", NewAsToInterrogative(), "I'm curious as to which country you're assigning these noble ideals.")
}

func TestAsToInterrogative_FixCuriousWho(t *testing.T) {
	linttest.AssertSuggestionResult(t, "New to climbing, curious to who create the problems/leads in the Olympics and how/by who are they tested?", NewAsToInterrogative(), "New to climbing, curious as to who create the problems/leads in the Olympics and how/by who are they tested?")
}

func TestAsToInterrogative_FixCuriousWhy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am curious to why the below weird_query does not yield a query with the parent relationship loaded", NewAsToInterrogative(), "I am curious as to why the below weird_query does not yield a query with the parent relationship loaded")
}

func TestAsToInterrogative_FixUnsureWhat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you are unsure to what BurpSuite is, or how to set it up please complete our BurpSuite room first.", NewAsToInterrogative(), "If you are unsure as to what BurpSuite is, or how to set it up please complete our BurpSuite room first.")
}

func TestAsToInterrogative_DontFlagUnsureWhatDegree(t *testing.T) {
	linttest.AssertNoLints(t, "and I'm unsure to what degree these have already been done", NewAsToInterrogative())
}

func TestAsToInterrogative_DontFlagUnsureWhichGo(t *testing.T) {
	t.Skip("This edge case with 'go' is tricky to handle")
	linttest.AssertNoLints(t, "Two classical music performances on the same day, unsure to which I should go.", NewAsToInterrogative())
}

func TestAsToInterrogative_FixUnsureWhy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am unsure to why this code is wrong.", NewAsToInterrogative(), "I am unsure as to why this code is wrong.")
}
