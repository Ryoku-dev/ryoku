package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCloseTightKnit_FixTightNitSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To keep components theme-able and tight nit, an ideal scenario", NewCloseTightKnit(), "To keep components theme-able and tight knit, an ideal scenario")
}

func TestCloseTightKnit_FixTightNitHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We're a small, tight-nit, experienced team from Google, Optimizely, Uber, & DraftKings", NewCloseTightKnit(), "We're a small, tight-knit, experienced team from Google, Optimizely, Uber, & DraftKings")
}

func TestCloseTightKnit_FixCloseNitSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I get its all a close nit set of Github folks driving both projects but...", NewCloseTightKnit(), "I get its all a close knit set of Github folks driving both projects but...")
}

func TestCloseTightKnit_FixCloseNitHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Within a group of more senior engineers working on close-nit teams (like MX) there's a lot of trust in the author during the review process.", NewCloseTightKnit(), "Within a group of more senior engineers working on close-knit teams (like MX) there's a lot of trust in the author during the review process.")
}

func TestCloseTightKnit_FixTightlyNitSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Initially public keys and aliases are tightly nit, changes in key values break expectations of EVM authorization checks", NewCloseTightKnit(), "Initially public keys and aliases are tightly knit, changes in key values break expectations of EVM authorization checks")
}

func TestCloseTightKnit_FixTightlyNitHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... will result, in my opinion and from what I have witnessed, in a more tightly-nit community", NewCloseTightKnit(), "... will result, in my opinion and from what I have witnessed, in a more tightly-knit community")
}

func TestCloseTightKnit_FixCloselyNitSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "For a small family, the boys have a nice network of closely nit family members.", NewCloseTightKnit(), "For a small family, the boys have a nice network of closely knit family members.")
}

func TestCloseTightKnit_FixCloselyNitHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However this time around we are aiming for a much smaller more \"closely-nit\" town.", NewCloseTightKnit(), "However this time around we are aiming for a much smaller more \"closely-knit\" town.")
}
