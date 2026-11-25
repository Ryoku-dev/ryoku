package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTakeMedicine_SwapsAteAntibiotics(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I ate antibiotics for a week.", NewTakeMedicine(), "I took antibiotics for a week.")
}

func TestTakeMedicine_SwapsEatMedicine(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You should eat the medicine now.", NewTakeMedicine(), "You should take the medicine now.")
}

func TestTakeMedicine_SwapsEatsMedication(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She eats medication daily.", NewTakeMedicine(), "She takes medication daily.")
}

func TestTakeMedicine_SwapsEatingMedicines(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Are you eating medicines for that illness?", NewTakeMedicine(), "Are you taking medicines for that illness?")
}

func TestTakeMedicine_SwapsEatenMedication(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He has eaten medication already.", NewTakeMedicine(), "He has taken medication already.")
}

func TestTakeMedicine_SwapsEatPills(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He ate the pills without water.", NewTakeMedicine(), "He took the pills without water.")
}

func TestTakeMedicine_SwapsEatingParacetamol(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She is eating paracetamol for her headache.", NewTakeMedicine(), "She is taking paracetamol for her headache.")
}

func TestTakeMedicine_HandlesPossessiveModifier(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Please eat my antibiotics.", NewTakeMedicine(), "Please take my antibiotics.")
}

func TestTakeMedicine_HandlesAdjectives(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They ate the prescribed antibiotics.", NewTakeMedicine(), "They took the prescribed antibiotics.")
}

func TestTakeMedicine_SupportsUppercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Eat antibiotics with water.", NewTakeMedicine(), "Take antibiotics with water.")
}

func TestTakeMedicine_OffersSwallowAlternative(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He ate the medication without water.", NewTakeMedicine(), "He swallowed the medication without water.")
}

func TestTakeMedicine_IgnoresCorrectUsage(t *testing.T) {
	linttest.AssertLintCount(t, "She took antibiotics last winter.", NewTakeMedicine(), 0)
}

func TestTakeMedicine_IgnoresUnrelatedEat(t *testing.T) {
	linttest.AssertLintCount(t, "They ate dinner after taking medicine.", NewTakeMedicine(), 0)
}
