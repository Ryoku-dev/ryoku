package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestJealousOf_ReplacesBasicFrom(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She was jealous from her sister's success.", NewJealousOf(), "She was jealous of her sister's success.")
}

func TestJealousOf_HandlesOptionalDeterminer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He grew jealous from the attention.", NewJealousOf(), "He grew jealous of the attention.")
}

func TestJealousOf_FixesPronounObject(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They became jealous from him.", NewJealousOf(), "They became jealous of him.")
}

func TestJealousOf_AllowsOovTarget(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm jealous from Zybrix.", NewJealousOf(), "I'm jealous of Zybrix.")
}

func TestJealousOf_CorrectsUppercasePreposition(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Jealous FROM his fame.", NewJealousOf(), "Jealous OF his fame.")
}

func TestJealousOf_FixesLongerPhrase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They felt jealous from the sudden praise she received.", NewJealousOf(), "They felt jealous of the sudden praise she received.")
}

func TestJealousOf_FixesMinimalPhrase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "jealous from success", NewJealousOf(), "jealous of success")
}

func TestJealousOf_DoesNotFlagCorrectUsage(t *testing.T) {
	linttest.AssertLintCount(t, "She was jealous of her sister's success.", NewJealousOf(), 0)
}

func TestJealousOf_DoesNotFlagOtherPrepositionSequence(t *testing.T) {
	linttest.AssertLintCount(t, "They stayed jealous from within the fortress.", NewJealousOf(), 0)
}

func TestJealousOf_FixesFollowingGerund(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He was jealous from being ignored.", NewJealousOf(), "He was jealous of being ignored.")
}

func TestJealousOf_IgnoresNumbersAfterFrom(t *testing.T) {
	linttest.AssertLintCount(t, "She remained jealous from 2010 through 2015.", NewJealousOf(), 0)
}
