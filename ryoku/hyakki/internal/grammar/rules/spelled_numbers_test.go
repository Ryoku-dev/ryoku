package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSpelledNumbers_ProducesZero(t *testing.T) {
	if got, ok := spelledNumbersSpellOutNumber(0); !ok || got != "zero" {
		t.Fatalf("got %q, %v", got, ok)
	}
}

func TestSpelledNumbers_ProducesEightyTwo(t *testing.T) {
	if got, ok := spelledNumbersSpellOutNumber(82); !ok || got != "eighty-two" {
		t.Fatalf("got %q, %v", got, ok)
	}
}

func TestSpelledNumbers_ProducesNineHundredNinetyNine(t *testing.T) {
	if got, ok := spelledNumbersSpellOutNumber(999); !ok || got != "nine hundred ninety-nine" {
		t.Fatalf("got %q, %v", got, ok)
	}
}

func TestSpelledNumbers_CorrectsNine(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There are 9 pigs.", NewSpelledNumbers(), "There are nine pigs.")
}

func TestSpelledNumbers_DoesNotCorrectTen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "There are 10 pigs.", NewSpelledNumbers(), "There are 10 pigs.")
}

func TestSpelledNumbers_ServicesRange(t *testing.T) {
	for i := uint64(0); i < 1000; i++ {
		if _, ok := spelledNumbersSpellOutNumber(i); !ok {
			t.Fatalf("could not spell %d", i)
		}
	}
}
