package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRedundantAlmostNearly_AlmostNearly(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Sub-word encoding models are almost nearly identical to the Character encoding models.", NewRedundantAlmostNearly(), []string{
		"Sub-word encoding models are almost identical to the Character encoding models.",
		"Sub-word encoding models are nearly identical to the Character encoding models.",
	}, []string{})
}

func TestRedundantAlmostNearly_NearlyAlmost(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Running Openshift 3.11 inside WSL2. This is not running yet, but nearly almost.", NewRedundantAlmostNearly(), []string{
		"Running Openshift 3.11 inside WSL2. This is not running yet, but almost.",
		"Running Openshift 3.11 inside WSL2. This is not running yet, but nearly.",
	}, []string{})
}
