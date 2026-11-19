package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWhereas_WhereAs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Dogs love playing fetch, where as cats are more independent creatures.", NewWhereas(), "Dogs love playing fetch, whereas cats are more independent creatures.")
}
