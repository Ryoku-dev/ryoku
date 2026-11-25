package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOxfordCommas_ByDefaultOneMustBeEnabledAndOneMustBeDisabled(t *testing.T) {
	linttest.AssertLintCount(t, "One, two and three. But four, five, and six.", NewCurated(dict.Curated(), dict.American), 1)
}
