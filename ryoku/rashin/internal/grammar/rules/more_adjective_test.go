package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMoreAdjective_AddEr(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The red car is more fast.", NewMoreAdjective(dict.Curated()), "The red car is faster.")
}

func TestMoreAdjective_AddR(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The fluffy one is more cute.", NewMoreAdjective(dict.Curated()), "The fluffy one is cuter.")
}

func TestMoreAdjective_DoubleFinalConsonant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You'll find out when you're more big.", NewMoreAdjective(dict.Curated()), "You'll find out when you're bigger.")
}

func TestMoreAdjective_FinalY(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That one was even more smelly!", NewMoreAdjective(dict.Curated()), "That one was even smellier!")
}

func TestMoreAdjective_IrregularGood(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I bet you couldn't do more good.", NewMoreAdjective(dict.Curated()), "I bet you couldn't do better.")
}

func TestMoreAdjective_IrregularFar(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "Is it much more far?", NewMoreAdjective(dict.Curated()), []string{"Is it much further?", "Is it much farther?"}, []string{})
}

func TestMoreAdjective_Humane(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That Klingon is more humane than the humans!", NewMoreAdjective(dict.Curated()), "That Klingon is humaner than the humans!")
}

func TestMoreAdjective_DontFlagMoreTime(t *testing.T) {
	linttest.AssertNoLints(t, "I need more time.", NewMoreAdjective(dict.Curated()))
}

func TestMoreAdjective_DontFlagMoreModel(t *testing.T) {
	linttest.AssertNoLints(t, "Expanded access to more model architectures", NewMoreAdjective(dict.Curated()))
}

func TestMoreAdjective_DontFlagMoreHuman(t *testing.T) {
	linttest.AssertNoLints(t, "I am more human than machine.", NewMoreAdjective(dict.Curated()))
}

func TestMoreAdjective_DontFlagMoreBattle(t *testing.T) {
	linttest.AssertNoLints(t, "and has more battle-tested defaults", NewMoreAdjective(dict.Curated()))
}

func TestMoreAdjective_DontFlagMoreLike(t *testing.T) {
	linttest.AssertNoLints(t, "It's more like a suggestion than a mistake.", NewMoreAdjective(dict.Curated()))
}

func TestMoreAdjective_DontFlagMoreGround(t *testing.T) {
	linttest.AssertNoLints(t, "This E2E security scan covers more ground", NewMoreAdjective(dict.Curated()))
}

func TestMoreAdjective_DontFlagMoreForeign(t *testing.T) {
	linttest.AssertNoLints(t, "There are more foreign visitors this year.", NewMoreAdjective(dict.Curated()))
}

func TestMoreAdjective_DontFlagMoreSubtleThanDirect2925(t *testing.T) {
	linttest.AssertNoLints(t, "more subtle than direct", NewMoreAdjective(dict.Curated()))
}

func TestMoreAdjective_DontCorrectInMostToInnest3284(t *testing.T) {
	linttest.AssertNoLints(t, "I have spent most in my life in Florida and had never heard \"display\" with an emphasis on the first syllable.", NewMoreAdjective(dict.Curated()))
}

func TestMoreAdjective_DontCorrectMoreCutToCuter(t *testing.T) {
	t.Skip("this problem persists, even after changing the 'cut' and 'cute' annotations")
	linttest.AssertNoLints(t, "they’re more cut from “one and done” cloth", NewMoreAdjective(dict.Curated()))
}
