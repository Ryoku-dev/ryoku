package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAdjectiveOfA_CorrectLargeOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yeah I'm using as large of a batch size as I can on this machine", NewAdjectiveOfA(), "Yeah I'm using as large a batch size as I can on this machine")
}

func TestAdjectiveOfA_CorrectBadOfAn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "- If forking is really that bad of an option, let's first decide where to put this.", NewAdjectiveOfA(), "- If forking is really that bad an option, let's first decide where to put this.")
}

func TestAdjectiveOfA_DontFlagComparative(t *testing.T) {
	linttest.AssertLintCount(t, "I only worked with custom composer installers for the better of a day, so please excuse me if I missed a thing.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagSuperlative(t *testing.T) {
	linttest.AssertLintCount(t, "I am trying to use composites to visualize the worst of a set of metrics.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagKind(t *testing.T) {
	linttest.AssertLintCount(t, "Log.txt file automatic creation in PWD is kind of an anti-feature", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagPart(t *testing.T) {
	linttest.AssertLintCount(t, "cannot delete a food that is no longer part of a recipe", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagMuch(t *testing.T) {
	linttest.AssertLintCount(t, "How much of a performance impact when switching from rails to rails-api ?", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagPartUppercase(t *testing.T) {
	linttest.AssertLintCount(t, "Quarkus Extension as Part of a Project inside a Monorepo?", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagAllOf(t *testing.T) {
	linttest.AssertLintCount(t, "This repository is deprecated. All of its content and history has been moved.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagInside(t *testing.T) {
	linttest.AssertLintCount(t, "Michael and Brock sat inside of a diner in Brandon", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagOut(t *testing.T) {
	linttest.AssertLintCount(t, "not only would he potentially be out of a job and back to sort of poverty", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagFull(t *testing.T) {
	linttest.AssertLintCount(t, "fortunately I happen to have this Tupperware full of an unceremoniously disassembled LED Mac Mini", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagSomething(t *testing.T) {
	linttest.AssertLintCount(t, "Well its popularity seems to be taking something of a dip right now.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagShort(t *testing.T) {
	linttest.AssertLintCount(t, "I found one Youtube short of an indonesian girl.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagBottom(t *testing.T) {
	linttest.AssertLintCount(t, "When leaves are just like coming out individually from the bottom of a fruit.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagLeft(t *testing.T) {
	linttest.AssertLintCount(t, "and what is left of a 12vt coil", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagFullUppercase(t *testing.T) {
	linttest.AssertLintCount(t, "Full of a bunch varnish like we get.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagHead(t *testing.T) {
	linttest.AssertLintCount(t, "You need to get out if you're the head of an education department and you're not using AI", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagMiddle(t *testing.T) {
	linttest.AssertLintCount(t, "just to get to that part in the middle of a blizzard", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagChance(t *testing.T) {
	linttest.AssertLintCount(t, "products that you overpay for because there are subtle details in the terms and conditions that reduce the size or chance of a payout.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagPotential(t *testing.T) {
	linttest.AssertLintCount(t, "People that are happy to accept it for the potential of a reward.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagSound(t *testing.T) {
	linttest.AssertLintCount(t, "the sound of an approaching Krampus", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagRid(t *testing.T) {
	linttest.AssertLintCount(t, "I need to get rid of a problem", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagPrecision(t *testing.T) {
	linttest.AssertLintCount(t, "a man whose crew cut has the precision of a targeted drone strike", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagBack(t *testing.T) {
	linttest.AssertLintCount(t, "a man whose crew cut has the back of a targeted drone strike", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagEmblematic(t *testing.T) {
	linttest.AssertLintCount(t, "... situation was emblematic of a publication that ...", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagHalf(t *testing.T) {
	linttest.AssertLintCount(t, "And now I only have half of a CyberTruck", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagBit(t *testing.T) {
	linttest.AssertLintCount(t, "we ran into a bit of an issue", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagDream(t *testing.T) {
	linttest.AssertLintCount(t, "When the dream of a united Europe began", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagBeginning(t *testing.T) {
	linttest.AssertLintCount(t, "That's the beginning of a conversation.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagSide(t *testing.T) {
	linttest.AssertLintCount(t, "it hit the barrier on the side of a highway", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagDerivative(t *testing.T) {
	linttest.AssertLintCount(t, "Techniques for evaluating the *partial derivative of a function", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagEquivalent(t *testing.T) {
	linttest.AssertLintCount(t, "Rust's equivalent of a switch statement is a match expression", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagUp(t *testing.T) {
	linttest.AssertLintCount(t, "Yeah gas is made up of a bunch of teenytiny particles all moving around.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagEighth(t *testing.T) {
	linttest.AssertLintCount(t, "It's about an eighth of an inch or whatever", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagShy(t *testing.T) {
	linttest.AssertLintCount(t, "... or just shy of a third of the country's total trade deficit.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagFun(t *testing.T) {
	linttest.AssertLintCount(t, "Remember that $4,000 Hermes horse bag I was making fun of a little while ago.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagOff(t *testing.T) {
	linttest.AssertLintCount(t, "can't identify a person based off of an IP from 10 years ago", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagBorderlineOf(t *testing.T) {
	linttest.AssertLintCount(t, "it's very very on the borderline of a rock pop ballad", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagLight(t *testing.T) {
	linttest.AssertLintCount(t, "The light of a star.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagMultiple(t *testing.T) {
	linttest.AssertLintCount(t, "The image needs to be a multiple of a certain size.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagRed(t *testing.T) {
	linttest.AssertLintCount(t, "The red of a drop of blood.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagTop(t *testing.T) {
	linttest.AssertLintCount(t, "The top of a hill.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagSlack(t *testing.T) {
	linttest.AssertLintCount(t, "They've been picking up the slack of a federal government mostly dominated by whatever this is.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagIllustrative(t *testing.T) {
	linttest.AssertLintCount(t, "Yet, the fact that they clearly give a one-sided account of most of their case studies is illustrative of a bias.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_DontFlagPerspective(t *testing.T) {
	linttest.AssertLintCount(t, "I always assess software by looking at it from the perspective of a new user.", NewAdjectiveOfA(), 0)
}

func TestAdjectiveOfA_CorrectTooLargeOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Warn users if setting too large of a session object", NewAdjectiveOfA(), "Warn users if setting too large a session object")
}

func TestAdjectiveOfA_CorrectTooLongOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "An Org Role with Too Long of a Name Hides Delete Option", NewAdjectiveOfA(), "An Org Role with Too Long a Name Hides Delete Option")
}

func TestAdjectiveOfA_CorrectTooBigOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "StepButton has too big of a space to click", NewAdjectiveOfA(), "StepButton has too big a space to click")
}

func TestAdjectiveOfA_CorrectTooVagueOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "\"No Speech provider is registered.\" is too vague of an error", NewAdjectiveOfA(), "\"No Speech provider is registered.\" is too vague an error")
}

func TestAdjectiveOfA_CorrectTooDumbOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hopefully this isn't too dumb of a question.", NewAdjectiveOfA(), "Hopefully this isn't too dumb a question.")
}

func TestAdjectiveOfA_CorrectHowImportantOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This should tell us how important of a use case that is and how often writing a type literal in a case is deliberate.", NewAdjectiveOfA(), "This should tell us how important a use case that is and how often writing a type literal in a case is deliberate.")
}

func TestAdjectiveOfA_CorrectThatRareOfAn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "so making changes isn't that rare of an occurrence for me.", NewAdjectiveOfA(), "so making changes isn't that rare an occurrence for me.")
}

func TestAdjectiveOfA_CorrectAsImportantOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Might be nice to have it draggable from other places as well, but not as important of a bug anymore.", NewAdjectiveOfA(), "Might be nice to have it draggable from other places as well, but not as important a bug anymore.")
}

func TestAdjectiveOfA_CorrectTooShortOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I login infrequently as well and 6 months is too short of a time.", NewAdjectiveOfA(), "I login infrequently as well and 6 months is too short a time.")
}

func TestAdjectiveOfA_CorrectThatCommonOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "that common of a name for a cluster role its hard to rule out", NewAdjectiveOfA(), "that common a name for a cluster role its hard to rule out")
}

func TestAdjectiveOfA_CorrectAsGreatOfAn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the w factor into the u factor to as great of an extent as possible.", NewAdjectiveOfA(), "the w factor into the u factor to as great an extent as possible.")
}

func TestAdjectiveOfA_CorrectTooUncommonOfA(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but this is probably too uncommon of a practice to be the default", NewAdjectiveOfA(), "but this is probably too uncommon a practice to be the default")
}
