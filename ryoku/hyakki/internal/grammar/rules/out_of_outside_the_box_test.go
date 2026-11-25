package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOutOfOutsideTheBox_ThinkOutOfTheBox(t *testing.T) {
	linttest.AssertSuggestionResult(t, "think out of the box", NewOutOfOutsideTheBox(), "think outside the box")
}

func TestOutOfOutsideTheBox_RightOutsideTheBox(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it works right outside the box", NewOutOfOutsideTheBox(), "it works right out of the box")
}

func TestOutOfOutsideTheBox_RightOutSideTheBox(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it works right out side the box", NewOutOfOutsideTheBox(), "it works right out of the box")
}

func TestOutOfOutsideTheBox_RightOutsideOfTheBox(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it works right outside of the box", NewOutOfOutsideTheBox(), "it works right out of the box")
}

func TestOutOfOutsideTheBox_RightOutSideOfTheBox(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it works right out side of the box", NewOutOfOutsideTheBox(), "it works right out of the box")
}
