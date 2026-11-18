package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHaveAHardTime_Had(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I also had hard time to investigate why debug(error) only shows stack traces, but no other visible properties", NewHaveAHardTime(dict.American), "I also had a hard time to investigate why debug(error) only shows stack traces, but no other visible properties")
}

func TestHaveAHardTime_Has(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Typescript has hard time finding declaration file for petite-vue", NewHaveAHardTime(dict.American), "Typescript has a hard time finding declaration file for petite-vue")
}

func TestHaveAHardTime_Have(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have hard time running getMeasuringContext(text, optionsopt) function.", NewHaveAHardTime(dict.American), "I have a hard time running getMeasuringContext(text, optionsopt) function.")
}

func TestHaveAHardTime_Having(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Instructions for those who are having hard time to install from scratch.", NewHaveAHardTime(dict.American), "Instructions for those who are having a hard time to install from scratch.")
}

func TestHaveAHardTime_HavingExtremely(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Bug: having extremely hard time converting html", NewHaveAHardTime(dict.American), "Bug: having an extremely hard time converting html")
}

func TestHaveAHardTime_HavingReal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am having real hard time integrating kalidokit (face+body tracking)", NewHaveAHardTime(dict.American), "I am having a real hard time integrating kalidokit (face+body tracking)")
}

func TestHaveAHardTime_DontFlagAnHardTime(t *testing.T) {
	linttest.AssertNoLints(t, "This feature will have an hard time working well with the other ...", NewHaveAHardTime(dict.American))
}

func TestHaveAHardTime_DontFlagSomeHardTime(t *testing.T) {
	linttest.AssertNoLints(t, "we had some hard time finding the right tk because 8.6 has a \"t\" suffix.", NewHaveAHardTime(dict.American))
}
