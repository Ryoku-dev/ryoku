package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLittleToNo_HyphenHyphen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "parents who allow their 15 and 16 year olds to drive on the same streets with no-to-little training", NewLittleToNo(), "parents who allow their 15 and 16 year olds to drive on the same streets with little-to-no training")
}

func TestLittleToNo_SpaceSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There was no to little industrialisation of India.", NewLittleToNo(), "There was little to no industrialisation of India.")
}

func TestLittleToNo_HyphenSpaceHyphen(t *testing.T) {
	t.Skip("this is surely too rare and quirky to support")
	linttest.AssertSuggestionResult(t, "The reflecter supports reflection utilities for node.js with no- to little impact on runtime performance.", NewLittleToNo(), "The reflecter supports reflection utilities for node.js with little- to no impact on runtime performance.")
}

func TestLittleToNo_StartOfSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No to little increase in the number of metrics collected.", NewLittleToNo(), "Little to no increase in the number of metrics collected.")
}
