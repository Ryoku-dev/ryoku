package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestABitOfPlural_Things(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Silence the test output a bit of things we ignore", NewABitOfPlural(), "Silence the test output a few things we ignore")
}

func TestABitOfPlural_GeneralTasks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it seems to be able to handle a bit of general tasks such as Generating text,", NewABitOfPlural(), "it seems to be able to handle a couple of general tasks such as Generating text,")
}

func TestABitOfPlural_Chatbots(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Repository to study a bit of chatbots with artificial intelligence using python and nltk.", NewABitOfPlural(), "Repository to study a few chatbots with artificial intelligence using python and nltk.")
}

func TestABitOfPlural_LittleBugs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have DTM-08 installed, and im experiencing a little bit of bugs.", NewABitOfPlural(), "I have DTM-08 installed, and im experiencing a few bugs.")
}

func TestABitOfPlural_SlowDays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A bit of slow days, a bit of family, and obviously… knitting.", NewABitOfPlural(), "A few slow days, a bit of family, and obviously… knitting.")
}

func TestABitOfPlural_BigThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Quite a bit of big things to load and unload so need stronger helpers.", NewABitOfPlural(), "Quite a few big things to load and unload so need stronger helpers.")
}

func TestABitOfPlural_LittleGoodThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sometimes you get a little bit of good things here and there done", NewABitOfPlural(), "Sometimes you get a couple of good things here and there done")
}

func TestABitOfPlural_LittleBadDays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I live here a lot of nice days with a little bit of bad days.", NewABitOfPlural(), "I live here a lot of nice days with a few bad days.")
}
