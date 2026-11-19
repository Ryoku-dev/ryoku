package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTakeALookTo_TakeALookToANew(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hello, I am Drago and in this video we're going to take a look to a new AI CLI and VS Code extension tool", NewTakeALookTo(), "Hello, I am Drago and in this video we're going to take a look at a new AI CLI and VS Code extension tool")
}

func TestTakeALookTo_HaveALookToUrl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If you haven't yet, please have a look to https://docs.conan.io/2/devops/devops_local_recipes_index.html", NewTakeALookTo(), "If you haven't yet, please have a look at https://docs.conan.io/2/devops/devops_local_recipes_index.html")
}

func TestTakeALookTo_HavingALookToMode(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Having a look to mode and overScaleMode , I see they are scriptable", NewTakeALookTo(), "Having a look at mode and overScaleMode , I see they are scriptable")
}

func TestTakeALookTo_TakingALookToThis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "after taking a look to this issue and making some test I figure out that it likely to be an error", NewTakeALookTo(), "after taking a look at this issue and making some test I figure out that it likely to be an error")
}

func TestTakeALookTo_HaveHadALookToYour(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have had a look to your conanfile.py and it is strange that it fails.", NewTakeALookTo(), "I have had a look at your conanfile.py and it is strange that it fails.")
}

func TestTakeALookTo_TookALookToBoth(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Since I have some knowledge in programing I took a look to both codes (LK and XCS)", NewTakeALookTo(), "Since I have some knowledge in programing I took a look at both codes (LK and XCS)")
}

func TestTakeALookTo_TakenALookToThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yeah I've taken a look to that, but I really need to use classes on this one", NewTakeALookTo(), "Yeah I've taken a look at that, but I really need to use classes on this one")
}

func TestTakeALookTo_TakesALookToThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "basically, it takes a look to the signing request", NewTakeALookTo(), "basically, it takes a look at the signing request")
}

func TestTakeALookTo_DontFlagHaveALookToSeeIf(t *testing.T) {
	linttest.AssertNoLints(t, "@budarin can you have a look to see if it addresses your concerns?", NewTakeALookTo())
}

func TestTakeALookTo_DontFlagTakingALookToDecide(t *testing.T) {
	linttest.AssertNoLints(t, "Would be worth taking a look to decide which way to go.", NewTakeALookTo())
}

func TestTakeALookTo_DontFlagTakesALookToSee(t *testing.T) {
	linttest.AssertNoLints(t, "It attempts to open the URL in a new window and then after 2s it takes a look to see if it can read the location.", NewTakeALookTo())
}

func TestTakeALookTo_DontFlagHasALookToIt(t *testing.T) {
	linttest.AssertNoLints(t, "The ecosystem's UI certainly has a look to it but inside of your app you could implement a different look as long as it's consistent.", NewTakeALookTo())
}

func TestTakeALookTo_ButDontIgnoreTakesALookToIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When he gets back I hope he takes a look to it", NewTakeALookTo(), "When he gets back I hope he takes a look at it")
}
