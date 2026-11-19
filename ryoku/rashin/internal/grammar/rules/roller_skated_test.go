package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRollerSkated_CorrectsBasicSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He roller skated down the hill.", NewRollerSkated(), "He roller-skated down the hill.")
}

func TestRollerSkated_CorrectsWithAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They roller skated quickly around the rink.", NewRollerSkated(), "They roller-skated quickly around the rink.")
}

func TestRollerSkated_CorrectsWithAuxiliary(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She had roller skated there before.", NewRollerSkated(), "She had roller-skated there before.")
}

func TestRollerSkated_CorrectsWithContraction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They'd roller skated all night.", NewRollerSkated(), "They'd roller-skated all night.")
}

func TestRollerSkated_CorrectsCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They ROLLER SKATED yesterday.", NewRollerSkated(), "They ROLLER-SKATED yesterday.")
}

func TestRollerSkated_CorrectsInQuotes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "\"We roller skated together,\" she said.", NewRollerSkated(), "\"We roller-skated together,\" she said.")
}

func TestRollerSkated_CorrectsAcrossLineBreak(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We\nroller skated whenever we could.", NewRollerSkated(), "We\nroller-skated whenever we could.")
}

func TestRollerSkated_CorrectsWithTrailingPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He roller skated, laughed, and waved.", NewRollerSkated(), "He roller-skated, laughed, and waved.")
}

func TestRollerSkated_CorrectsWithoutSpaceAfterPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He roller skated,laughed, and waved.", NewRollerSkated(), "He roller-skated,laughed, and waved.")
}

func TestRollerSkated_AllowsHyphenatedForm(t *testing.T) {
	linttest.AssertLintCount(t, "They roller-skated yesterday.", NewRollerSkated(), 0)
}

func TestRollerSkated_AllowsSubjectNamedRoller(t *testing.T) {
	linttest.AssertLintCount(t, "The roller skated across the stage.", NewRollerSkated(), 0)
}

func TestRollerSkated_AllowsOtherCompounds(t *testing.T) {
	linttest.AssertLintCount(t, "Their roller skating routine impressed everyone.", NewRollerSkated(), 0)
}
