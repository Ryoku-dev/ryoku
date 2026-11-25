package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSpaces_DetectsSpaceBeforePeriod(t *testing.T) {
	source := "There is a space at the end of this sentence ."
	linttest.AssertLintCount(t, source, NewSpaces(), 1)
}

func TestSpaces_AllowsPeriodWithoutSpace(t *testing.T) {
	source := "There isn't a space at the end of this sentence."
	linttest.AssertLintCount(t, source, NewSpaces(), 0)
}

func TestSpaces_IgnoresFrenchSpacing(t *testing.T) {
	linttest.AssertNoLints(t, "This is a short sentence.  This is another short sentence.", NewSpaces())
}
