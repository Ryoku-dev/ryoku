package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestQuantifierNumeralConflict_FlagSeveralTwo(t *testing.T) {
	linttest.AssertLintCount(t, "A few minutes ago, there was an outage due to several two hosts being down at the same time.", NewQuantifierNumeralConflict(), 1)
}

func TestQuantifierNumeralConflict_DontFlagAtLeast(t *testing.T) {
	linttest.AssertNoLints(t, "Serving a company that encourages the \"996\" work schedule usually means working for at least 60 hours per week.", NewQuantifierNumeralConflict())
}

func TestQuantifierNumeralConflict_DontFlagAtMost(t *testing.T) {
	linttest.AssertNoLints(t, "But don't worry, the second machine takes at most 3 years.", NewQuantifierNumeralConflict())
}

func TestQuantifierNumeralConflict_DontFlagBoth32BitAnd64Bit(t *testing.T) {
	linttest.AssertNoLints(t, "Both 32 bit and 64 bit architectures are supported.", NewQuantifierNumeralConflict())
}

func TestQuantifierNumeralConflict_DontFlagMore1Click(t *testing.T) {
	linttest.AssertNoLints(t, "For more 1-click cloud deployments, see [Cloud Deployment", NewQuantifierNumeralConflict())
}

func TestQuantifierNumeralConflict_CorrectEach2(t *testing.T) {
	linttest.AssertSuggestionResult(t, "OSSEC by default run rootkit check each 2 hours.", NewQuantifierNumeralConflict(), "OSSEC by default run rootkit check every 2 hours.")
}

func TestQuantifierNumeralConflict_IgnoreNoTwoAdjacentCharacters2486(t *testing.T) {
	linttest.AssertNoLints(t, "No two adjacent characters are the same.", NewQuantifierNumeralConflict())
}
