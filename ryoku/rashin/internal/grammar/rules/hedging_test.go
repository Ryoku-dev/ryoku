package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHedging_DetectsHedgingPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "I would argue that this is correct.", NewHedging(), 1)
}

func TestHedging_DoesNotFlagCleanText(t *testing.T) {
	linttest.AssertLintCount(t, "This is clear and direct.", NewHedging(), 0)
}

func TestHedging_LowercaseHedging(t *testing.T) {
	linttest.AssertLintCount(t, "i would argue that the outcome is uncertain.", NewHedging(), 1)
}

func TestHedging_IncompletePhraseNotFlagged(t *testing.T) {
	linttest.AssertLintCount(t, "I would argue the data is clear.", NewHedging(), 0)
}

func TestHedging_PhraseWithTrailingComma(t *testing.T) {
	text := "I would argue that, this method works."
	linttest.AssertLintCount(t, text, NewHedging(), 1)
}

func TestHedging_PhraseWithExtraWhitespace(t *testing.T) {
	linttest.AssertLintCount(t, "to   a   certain   degree the results are ambiguous.", NewHedging(), 1)
}

func TestHedging_DoesNotFlagSimilarButIncorrectPhrase(t *testing.T) {
	linttest.AssertLintCount(t, "He spoke so to speakingly about the event.", NewHedging(), 0)
}

func TestHedging_PhraseSplitByLineBreak(t *testing.T) {
	linttest.AssertLintCount(t, "I would argue\nthat this approach fails.", NewHedging(), 1)
}
