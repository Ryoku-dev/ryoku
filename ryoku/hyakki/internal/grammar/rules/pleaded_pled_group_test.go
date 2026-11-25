package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPleadedPled_ByDefaultOneMustBeEnabledAndOneMustBeDisabled(t *testing.T) {
	lg := NewCurated(dict.Curated(), dict.American)
	lg.Config.UnsetRuleEnabled("SpellCheck")
	linttest.AssertLintCount(t, "I say 'pled' but you say 'pleaded'.", lg, 1)
}
