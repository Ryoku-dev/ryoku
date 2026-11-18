package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOrthographicConsistency_LowercaseBbcBeatsSpellCheck(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The bbc reported it.", NewCurated(dict.Curated(), dict.American), "The BBC reported it.")
}

func TestOrthographicConsistency_TitleCaseBbcBeatsSpellCheck(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Bbc reported it.", NewCurated(dict.Curated(), dict.American), "The BBC reported it.")
}

func TestOrthographicConsistency_LowercaseHtmlBeatsSpellCheck(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Validate the html first.", NewCurated(dict.Curated(), dict.American), "Validate the HTML first.")
}

func TestOrthographicConsistency_LowercaseNhsBeatsSpellCheck(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The nhs is very busy.", NewCurated(dict.Curated(), dict.American), "The NHS is very busy.")
}

func TestOrthographicConsistency_TwoLetterTypoStaysWithSpellCheck(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We went ot the shop.", NewCurated(dict.Curated(), dict.American), "We went out the shop.")
}
