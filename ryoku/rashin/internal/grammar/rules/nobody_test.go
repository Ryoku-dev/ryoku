package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNobody_BothValidAndInvalid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "No body told me. I have a head but no body.", NewNobody(), "Nobody told me. I have a head but no body.")
}

func TestNobody_IgnoresNoBodyWasFound(t *testing.T) {
	linttest.AssertLintCount(t, "No body was found after the search.", NewNobody(), 0)
}

func TestNobody_IgnoresNoBodyComma(t *testing.T) {
	linttest.AssertLintCount(t, "No body, no signs of a struggle, no answers.", NewNobody(), 0)
}
