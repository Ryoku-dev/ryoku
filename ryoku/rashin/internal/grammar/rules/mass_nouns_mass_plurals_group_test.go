package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMassNounsMassPlurals_FixKindOfAdvices(t *testing.T) {
	linttest.AssertSuggestionResult(t, "IMO these kind of advices never matters.", NewCurated(dict.Curated(), dict.American), "IMO these kind of advice never matters.")
}
