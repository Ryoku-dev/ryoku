package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFindFine_Issue2115(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I was using oil.nvim from an year and everything was find for me but I was missing a very key feature", NewFindFine(), "I was using oil.nvim from an year and everything was fine for me but I was missing a very key feature")
	linttest.AssertSuggestionResult(t, "I made several observations throughout the evening and everything was find.", NewFindFine(), "I made several observations throughout the evening and everything was fine.")
	linttest.AssertSuggestionResult(t, "I am find not using GPU at all for open3d.", NewFindFine(), "I am fine not using GPU at all for open3d.")
}

func TestFindFine_DontFlagIsFindSomeone3255(t *testing.T) {
	linttest.AssertLintCount(t, "Generally speaking what you want to try and do is find someone to replace you before you leave.", NewFindFine(), 0)
}
