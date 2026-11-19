package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSeemsLikeAdjective_Appears(t *testing.T) {
	linttest.AssertSuggestionResult(t, "appears like awesome", NewSeemsLikeAdjective(), "appears awesome")
}

func TestSeemsLikeAdjective_Feels(t *testing.T) {
	linttest.AssertSuggestionResult(t, "feels like lumpy", NewSeemsLikeAdjective(), "feels lumpy")
}

func TestSeemsLikeAdjective_Looks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "looks like handsome", NewSeemsLikeAdjective(), "looks handsome")
}

func TestSeemsLikeAdjective_Seems(t *testing.T) {
	linttest.AssertSuggestionResult(t, "seems like shabby", NewSeemsLikeAdjective(), "seems shabby")
}

func TestSeemsLikeAdjective_Sounds(t *testing.T) {
	linttest.AssertSuggestionResult(t, "sounds like loud", NewSeemsLikeAdjective(), "sounds loud")
}

func TestSeemsLikeAdjective_Smells(t *testing.T) {
	linttest.AssertSuggestionResult(t, "smells like unpleasant", NewSeemsLikeAdjective(), "smells unpleasant")
}

func TestSeemsLikeAdjective_Tastes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "tastes like yucky", NewSeemsLikeAdjective(), "tastes yucky")
}
