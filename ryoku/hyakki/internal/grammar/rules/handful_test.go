package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestHandful_SuggestsPlainSpacing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Her basket held a hand full of berries.", NewHandful(), "Her basket held a handful of berries.")
}

func TestHandful_SuggestsCapitalizedForm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hand full of tales lined the shelf.", NewHandful(), "Handful of tales lined the shelf.")
}

func TestHandful_SuggestsHyphenatedForm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A hand-full of marbles scattered across the floor.", NewHandful(), "A handful of marbles scattered across the floor.")
}

func TestHandful_SuggestsSpaceHyphenCombo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A hand - full of seeds spilled on the workbench.", NewHandful(), "A handful of seeds spilled on the workbench.")
}

func TestHandful_SuggestsInitialHyphenVariants(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hand-Full of furniture, the cart creaked slowly.", NewHandful(), "Handful of furniture, the cart creaked slowly.")
}

func TestHandful_FlagsMultipleInstances(t *testing.T) {
	linttest.AssertLintCount(t, "She carried a hand full of carrots and a hand full of radishes.", NewHandful(), 2)
}

func TestHandful_AllowsCorrectHandful(t *testing.T) {
	linttest.AssertNoLints(t, "A handful of volunteers arrived in time.", NewHandful())
}

func TestHandful_AllowsParentheticalHand(t *testing.T) {
	linttest.AssertNoLints(t, "His hand, full of ink, kept writing without pause.", NewHandful())
}

func TestHandful_AllowsHandIsFull(t *testing.T) {
	linttest.AssertNoLints(t, "The hand is full of water.", NewHandful())
}

func TestHandful_AllowsHandfullTypo(t *testing.T) {
	linttest.AssertNoLints(t, "The word handfull is an incorrect spelling.", NewHandful())
}
