package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOnTheFence_OnTheFenceEndNoPunc(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm on a fence", NewOnTheFence(), "I'm on the fence")
}

func TestOnTheFence_OnTheFenceEndPeriod(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You're on a fence.", NewOnTheFence(), "You're on the fence.")
}

func TestOnTheFence_OnTheFenceEndSpacePeriod(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He is on a fence .", NewOnTheFence(), "He is on the fence .")
}

func TestOnTheFence_FixAnyoneIsOnAFence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If anyone is still on a fence, the new Joyride update changes a lot.", NewOnTheFence(), "If anyone is still on the fence, the new Joyride update changes a lot.")
}

func TestOnTheFence_FixIAmAboutThis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am on a fence about this.", NewOnTheFence(), "I am on the fence about this.")
}

func TestOnTheFence_FixIAmAlso(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am also on a fence, especially b/c the morning check-in might end up being a duplicate of previous EOD meeting", NewOnTheFence(), "I am also on the fence, especially b/c the morning check-in might end up being a duplicate of previous EOD meeting")
}

func TestOnTheFence_FixImOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm on a fence of scripting it myself for good or improving the initial experience for everyone :)", NewOnTheFence(), "I'm on the fence of scripting it myself for good or improving the initial experience for everyone :)")
}

func TestOnTheFence_FixImStillWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm still on a fence with Rust at this point.", NewOnTheFence(), "I'm still on the fence with Rust at this point.")
}

func TestOnTheFence_FixYouAreOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have a one month contract so if you are on a fence of trying them out, that's a great starter option. ", NewOnTheFence(), "They have a one month contract so if you are on the fence of trying them out, that's a great starter option. ")
}

func TestOnTheFence_DontFlagIt(t *testing.T) {
	linttest.AssertNoLints(t, "I would like to remove it but it is on a fence between the neighbors yard and mine.", NewOnTheFence())
}

func TestOnTheFence_DontFlagPosted(t *testing.T) {
	linttest.AssertNoLints(t, "A private property sign is posted on a fence.", NewOnTheFence())
}

func TestOnTheFence_DontFlagScrolling(t *testing.T) {
	linttest.AssertNoLints(t, "My only issue is scrolling on a fence is a bit slow and laggy.", NewOnTheFence())
}

func TestOnTheFence_DontFlagVerse(t *testing.T) {
	linttest.AssertNoLints(t, "My daddy is a dollar / I wrote it on a fence / My daddy is a dollar / not worth a hundred cents.", NewOnTheFence())
}
