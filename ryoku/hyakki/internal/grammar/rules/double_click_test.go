package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDoubleClick_CorrectsBasicCommand(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Double click the icon.", NewDoubleClick(), "Double-click the icon.")
}

func TestDoubleClick_CorrectsWithPreposition(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Please double click on the link.", NewDoubleClick(), "Please double-click on the link.")
}

func TestDoubleClick_CorrectsWithPronoun(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You should double click it to open.", NewDoubleClick(), "You should double-click it to open.")
}

func TestDoubleClick_CorrectsPluralForm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Double clicks are recorded in the log.", NewDoubleClick(), "Double-clicks are recorded in the log.")
}

func TestDoubleClick_CorrectsPastTense(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They double clicked the submit button.", NewDoubleClick(), "They double-clicked the submit button.")
}

func TestDoubleClick_CorrectsGerund(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Double clicking the item highlights it.", NewDoubleClick(), "Double-clicking the item highlights it.")
}

func TestDoubleClick_CorrectsWithCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He DOUBLE CLICKED the file.", NewDoubleClick(), "He DOUBLE-CLICKED the file.")
}

func TestDoubleClick_CorrectsMultiline(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Double\nclick the checkbox.", NewDoubleClick(), "Double-click the checkbox.")
}

func TestDoubleClick_CorrectsAtSentenceEnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just double click.", NewDoubleClick(), "Just double-click.")
}

func TestDoubleClick_AllowsHyphenatedForm(t *testing.T) {
	linttest.AssertLintCount(t, "Double-click the icon.", NewDoubleClick(), 0)
}

func TestDoubleClick_IgnoresOtherDoubleWords(t *testing.T) {
	linttest.AssertLintCount(t, "She said the double rainbow was beautiful.", NewDoubleClick(), 0)
}
