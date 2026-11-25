package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestEverEvery_FixCanIEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Odd, how can i every become negative in that case?", NewEverEvery(), "Odd, how can i ever become negative in that case?")
}

func TestEverEvery_FixCanTheyEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if each component has its own instance of NameService, how can they every share state?", NewEverEvery(), "if each component has its own instance of NameService, how can they ever share state?")
}

func TestEverEvery_FixCanWeEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "can we every have a good dev UX?", NewEverEvery(), "can we ever have a good dev UX?")
}

func TestEverEvery_FixDidWeEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Did we every fix that?", NewEverEvery(), "Did we ever fix that?")
}

func TestEverEvery_FixDidYouEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Did you every get vtsls working properly?", NewEverEvery(), "Did you ever get vtsls working properly?")
}

func TestEverEvery_FixDoIEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Rarely do I every look forward to the new ui.", NewEverEvery(), "Rarely do I ever look forward to the new ui.")
}

func TestEverEvery_FixDoWeEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "do we every stop learning new things?", NewEverEvery(), "do we ever stop learning new things?")
}

func TestEverEvery_FixDoYouEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Do you every faced the issue or have any idea why this could happen?", NewEverEvery(), "Do you ever faced the issue or have any idea why this could happen?")
}

func TestEverEvery_FixDontIEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "WHY DONT I EVERY SEE OR HEAR ABOUT THINGS HAPPENING IN SOUTHPORT?", NewEverEvery(), "WHY DONT I EVER SEE OR HEAR ABOUT THINGS HAPPENING IN SOUTHPORT?")
}

func TestEverEvery_FixDontTheyEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "And why dont they every smile first?", NewEverEvery(), "And why dont they ever smile first?")
}

func TestEverEvery_FixDontYouEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Dont you every forget this and believe nothing else.", NewEverEvery(), "Dont you ever forget this and believe nothing else.")
}

func TestEverEvery_FixHaveYouEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Have you every wanted to generate geometric structures from data.frames", NewEverEvery(), "Have you ever wanted to generate geometric structures from data.frames")
}

func TestEverEvery_FixShouldIEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I.e. why would I every use deepcopy ?", NewEverEvery(), "I.e. why would I ever use deepcopy ?")
}

func TestEverEvery_FixShouldWeEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Should we every meet, I'll get you a beverage of your choosing!", NewEverEvery(), "Should we ever meet, I'll get you a beverage of your choosing!")
}

func TestEverEvery_FixShouldYouEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but you will always have a place in his home should you every truly desire it", NewEverEvery(), "but you will always have a place in his home should you ever truly desire it")
}

func TestEverEvery_FixWouldIEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Why would I every do that?", NewEverEvery(), "Why would I ever do that?")
}

func TestEverEvery_FixWouldTheyEvery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Would they every be installed together?", NewEverEvery(), "Would they ever be installed together?")
}

func TestEverEvery_DontFlagShouldWeEvery(t *testing.T) {
	t.Skip("unusual but not wrong position of time phrase, maybe should have commas?")
	linttest.AssertNoLints(t, "MM: should we every month or two have a roundup of what's been happening in WGSL", NewEverEvery())
}
