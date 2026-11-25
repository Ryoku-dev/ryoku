package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRightClick_HyphenatesBasicCommand(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Right click the icon.", NewRightClick(), "Right-click the icon.")
}

func TestRightClick_HyphenatesWithPreposition(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Please right click on the link.", NewRightClick(), "Please right-click on the link.")
}

func TestRightClick_HyphenatesPastTense(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They right clicked the submit button.", NewRightClick(), "They right-clicked the submit button.")
}

func TestRightClick_HyphenatesGerund(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Right clicking the item highlights it.", NewRightClick(), "Right-clicking the item highlights it.")
}

func TestRightClick_HyphenatesPluralNoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Right clicks are tracked in the log.", NewRightClick(), "Right-clicks are tracked in the log.")
}

func TestRightClick_HyphenatesAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He RIGHT CLICKED the file.", NewRightClick(), "He RIGHT-CLICKED the file.")
}

func TestRightClick_HyphenatesLeftClick(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Left click the checkbox.", NewRightClick(), "Left-click the checkbox.")
}

func TestRightClick_HyphenatesMiddleClick(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Middle click to open in a new tab.", NewRightClick(), "Middle-click to open in a new tab.")
}

func TestRightClick_AllowsHyphenatedForm(t *testing.T) {
	linttest.AssertLintCount(t, "Right-click the icon.", NewRightClick(), 0)
}

func TestRightClick_IgnoresUnrelatedRightAndClick(t *testing.T) {
	linttest.AssertLintCount(t, "Click the right button to continue.", NewRightClick(), 0)
}
