package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDotInitialisms_MatchesEg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Some text here (eg. more text).", NewDotInitialisms(), "Some text here (e.g. more text).")
}
