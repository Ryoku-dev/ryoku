package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSneakedSnuck_ByDefaultOneMustBeEnabledAndOneMustBeDisabled(t *testing.T) {
	linttest.AssertLintCount(t, "I say 'snuck' but you say 'sneaked'.", NewCurated(dict.Curated(), dict.American), 1)
}
