package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRegularIrregulars_FixIrregulars(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Womans and childs first", NewRegularIrregulars(dict.Curated()), "Women and children first")
}

func TestRegularIrregulars_FixYsAndFs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Kittys playing on the shelfs.", NewRegularIrregulars(dict.Curated()), "Kitties playing on the shelves.")
}

func TestRegularIrregulars_FixOsAndOes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The heros climb the volcanos", NewRegularIrregulars(dict.Curated()), "The heroes climb the volcanoes")
}

func TestRegularIrregulars_FixOxenAndMeatloaves(t *testing.T) {
	linttest.AssertSuggestionResult(t, "These meatloafs are made out of oxes.", NewRegularIrregulars(dict.Curated()), "These meatloaves are made out of oxen.")
}

func TestRegularIrregulars_FixIrregularPastVerb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I eated the banana.", NewRegularIrregulars(dict.Curated()), "I ate the banana.")
}

func TestRegularIrregulars_FixReaded(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He readed the newspaper", NewRegularIrregulars(dict.Curated()), "He read the newspaper")
}

func TestRegularIrregulars_FixWrited(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She writed many lines of code.", NewRegularIrregulars(dict.Curated()), "She wrote many lines of code.")
}

func TestRegularIrregulars_FixRunned(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I runned faster than ever!", NewRegularIrregulars(dict.Curated()), "I ran faster than ever!")
}

func TestRegularIrregulars_FixResetted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I resetted the phone to factory settings.", NewRegularIrregulars(dict.Curated()), "I reset the phone to factory settings.")
}

func TestRegularIrregulars_FixEatDrinkSleep(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I eated and drinked too much but I sleeped good.", NewRegularIrregulars(dict.Curated()), "I ate and drank too much but I slept good.")
}

func TestRegularIrregulars_FixDigged(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I digged a bit deeper.", NewRegularIrregulars(dict.Curated()), "I dug a bit deeper.")
}

func TestRegularIrregulars_FixAdjectives(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "This way is farer.", NewRegularIrregulars(dict.Curated()), []string{"This way is farther.", "This way is further."}, []string{})
}
