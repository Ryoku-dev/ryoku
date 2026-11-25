package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNorModalPronoun_FixNorICan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "i can't see the menu nor i can see the features of your app or how it looks !", NewNorModalPronoun(), "i can't see the menu nor can i see the features of your app or how it looks !")
}

func TestNorModalPronoun_FixNorICould(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but never saw any warnings nor I could read messages until I debugged", NewNorModalPronoun(), "but never saw any warnings nor could I read messages until I debugged")
}

func TestNorModalPronoun_FixNorIWill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am not the author of the plugins nor I will be updating bugged/unavailable plugins.", NewNorModalPronoun(), "I am not the author of the plugins nor will I be updating bugged/unavailable plugins.")
}

func TestNorModalPronoun_FixNorIWould(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would not like to own a Pollock, nor I would hang one of his paintings on a wall inside my home", NewNorModalPronoun(), "I would not like to own a Pollock, nor would I hang one of his paintings on a wall inside my home")
}

func TestNorModalPronoun_FixNorItCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However, since several days ago FreeTube simply doesn't open ANY videos nor it can search.", NewNorModalPronoun(), "However, since several days ago FreeTube simply doesn't open ANY videos nor can it search.")
}

func TestNorModalPronoun_FixNorItShould(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Since the code doesn't guard against it (nor it should), internalModule.stripBOM is called with an undefined", NewNorModalPronoun(), "Since the code doesn't guard against it (nor should it), internalModule.stripBOM is called with an undefined")
}

func TestNorModalPronoun_FixNorItWill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It will never \"create a table\", nor it will issue any query - it will only create the entity instance.", NewNorModalPronoun(), "It will never \"create a table\", nor will it issue any query - it will only create the entity instance.")
}

func TestNorModalPronoun_FixNorItWould(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Neither whitespace (excepting NL and CR) is special char in this sense, nor it would destroy something, if it gets \"escaped\" as variable", NewNorModalPronoun(), "Neither whitespace (excepting NL and CR) is special char in this sense, nor would it destroy something, if it gets \"escaped\" as variable")
}

func TestNorModalPronoun_FixNorTheyCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Currently these assets don't include the code provided by the submodules nor they can be disabled", NewNorModalPronoun(), "Currently these assets don't include the code provided by the submodules nor can they be disabled")
}

func TestNorModalPronoun_FixNorWeCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The NSLayoutConstraint errors are really Apple bugs, not our fault, nor we can fix them, but they are harmless.", NewNorModalPronoun(), "The NSLayoutConstraint errors are really Apple bugs, not our fault, nor can we fix them, but they are harmless.")
}

func TestNorModalPronoun_FixNorYouCan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You cannot create a view to do it, nor you can have a function to do it", NewNorModalPronoun(), "You cannot create a view to do it, nor can you have a function to do it")
}

func TestNorModalPronoun_FixNorYouShould(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I believe you cannot create two sessions through one signin, and maybe nor you should.", NewNorModalPronoun(), "I believe you cannot create two sessions through one signin, and maybe nor should you.")
}

func TestNorModalPronoun_IgnoreNeitherTheyNorICould(t *testing.T) {
	linttest.AssertNoLints(t, "One of my users was unable to install tools via mise, but neither they nor I could initially figure out why.", NewNorModalPronoun())
}

func TestNorModalPronoun_IgnoreNeitherMyToolNorIShall(t *testing.T) {
	linttest.AssertNoLints(t, "but neither my tool nor I shall feel disrespected", NewNorModalPronoun())
}
