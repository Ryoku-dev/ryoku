package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNoFrenchSpaces_FixesBasic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a short sentence.  This is another short sentence.", NewNoFrenchSpaces(), "This is a short sentence. This is another short sentence.")
}
