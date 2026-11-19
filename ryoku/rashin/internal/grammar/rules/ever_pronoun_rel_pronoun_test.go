package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestEverPronounRelPronoun_WhateverThatShe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She had no evidence of colitis flare up, meaning, whatever that she has now, is what she had at the first  emergency room visit.", NewEverPronounRelPronoun(), "She had no evidence of colitis flare up, meaning, whatever she has now, is what she had at the first  emergency room visit.")
}

func TestEverPronounRelPronoun_WhateverThatI(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Whatever that I have already tried gives back attribute errors etc.", NewEverPronounRelPronoun(), "Whatever I have already tried gives back attribute errors etc.")
}

func TestEverPronounRelPronoun_WhateverThatIveTranscribed(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the history in the handy app gets updated with whatever that I've transcribed", NewEverPronounRelPronoun(), "the history in the handy app gets updated with whatever I've transcribed")
}

func TestEverPronounRelPronoun_WhateverThatIExperience(t *testing.T) {
	linttest.AssertSuggestionResult(t, "whatever that I experience is, I want it to \"just work\" with VS Code", NewEverPronounRelPronoun(), "whatever I experience is, I want it to \"just work\" with VS Code")
}

func TestEverPronounRelPronoun_UseWhateverWhich(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The git hooks would use whatever which dvc is set on the shell.", NewEverPronounRelPronoun(), "The git hooks would use whatever dvc is set on the shell.")
}

func TestEverPronounRelPronoun_DoWhateverWhich(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When I load my page I have an auto focus and I can type and do whatever which works fine.", NewEverPronounRelPronoun(), "When I load my page I have an auto focus and I can type and do whatever works fine.")
}

func TestEverPronounRelPronoun_WhateverWho(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can use cURL for that (or whatever who can make a HTTP request).", NewEverPronounRelPronoun(), "You can use cURL for that (or whatever can make a HTTP request).")
}

func TestEverPronounRelPronoun_WheneverThatIveCreatedReference(t *testing.T) {
	linttest.AssertSuggestionResult(t, "or whenever that i create a reference to a class and that Class adds tons of stacks", NewEverPronounRelPronoun(), "or whenever i create a reference to a class and that Class adds tons of stacks")
}

func TestEverPronounRelPronoun_WheneverThatIWant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Any special scenario, or like JSON whenever that I want to transport data?", NewEverPronounRelPronoun(), "Any special scenario, or like JSON whenever I want to transport data?")
}

func TestEverPronounRelPronoun_WhoeverWhoNeeds(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A public motivation site for whoever who needs inspiration to get some PIZZA!!!", NewEverPronounRelPronoun(), "A public motivation site for whoever needs inspiration to get some PIZZA!!!")
}

func TestEverPronounRelPronoun_WhoeverWhoOwns(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We'd need permission from whoever who owns the legal/moral rights to the Python logo if we want to use it", NewEverPronounRelPronoun(), "We'd need permission from whoever owns the legal/moral rights to the Python logo if we want to use it")
}

func TestEverPronounRelPronoun_WhoeverWhoRead(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hi, I'm asking to whoever who read this for help.", NewEverPronounRelPronoun(), "Hi, I'm asking to whoever read this for help.")
}

func TestEverPronounRelPronoun_DontFlagWhateverThat(t *testing.T) {
	t.Skip("'whatever' means 'etc.', hopefully rare")
	linttest.AssertNoLints(t, "My actual goal is to have a handful of names, locations or whatever that I have to type", NewEverPronounRelPronoun())
}

func TestEverPronounRelPronoun_WhateverWhichManager(t *testing.T) {
	t.Skip("does the writer mean 'no matter which'?")
	linttest.AssertNoLints(t, "We don't need something more complex but the behavior should be the same wherever which manager extract the dependency.", NewEverPronounRelPronoun())
}

func TestEverPronounRelPronoun_WhateverWhoYouAre(t *testing.T) {
	t.Skip("probably needs to change 'whatever who' to 'whoever'")
	linttest.AssertNoLints(t, "Hi friends, Whatever who you are and what your title is if you're reading this it means the internal infrastructure of your company is fully or partially broken.", NewEverPronounRelPronoun())
}
