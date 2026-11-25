package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSomewhatSomething_Issue414(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This may be somewhat of a surprise.", NewSomewhatSomething(), "This may be something of a surprise.")
}

func TestSomewhatSomething_FlagThese(t *testing.T) {
	linttest.AssertSuggestionResult(t, "These are somewhat of a cult data structure.", NewSomewhatSomething(), "These are something of a cult data structure.")
}
