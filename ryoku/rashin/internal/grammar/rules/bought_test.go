package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestBought_CorrectsHeBough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He bough a laptop yesterday.", NewBought(), "He bought a laptop yesterday.")
}

func TestBought_CorrectsSheNeverBough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She never bough fresh herbs there.", NewBought(), "She never bought fresh herbs there.")
}

func TestBought_CorrectsTheyAlreadyBough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They already bough the train tickets.", NewBought(), "They already bought the train tickets.")
}

func TestBought_CorrectsWeHaveBough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have bough extra paint.", NewBought(), "We have bought extra paint.")
}

func TestBought_CorrectsTheyHaveNeverBough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have never bough theatre seats online.", NewBought(), "They have never bought theatre seats online.")
}

func TestBought_CorrectsIveBough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've bough the ingredients already.", NewBought(), "I've bought the ingredients already.")
}

func TestBought_CorrectsWedBough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We'd bough snacks before the film.", NewBought(), "We'd bought snacks before the film.")
}

func TestBought_NoLintForTreeBough(t *testing.T) {
	linttest.AssertNoLints(t, "The heavy bough cracked under the snow.", NewBought())
}

func TestBought_NoLintForHeBought(t *testing.T) {
	linttest.AssertNoLints(t, "He bought a laptop yesterday.", NewBought())
}

func TestBought_NoLintForPluralBoughs(t *testing.T) {
	linttest.AssertNoLints(t, "Boughs swayed in the evening breeze.", NewBought())
}
