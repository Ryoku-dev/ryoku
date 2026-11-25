package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFillerWords_RemoveUh(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Let's remove all the uh filler words.", NewFillerWords(), "Let's remove all the filler words.")
}

func TestFillerWords_RemoveUmStStart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Um but I'll just add some context for this.", NewFillerWords(), "but I'll just add some context for this.")
}
