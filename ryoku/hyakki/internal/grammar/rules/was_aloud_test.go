package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWasAloud_CorrectsWasAloud(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He was aloud to enter the room.", NewWasAloud(), "He was allowed to enter the room.")
}

func TestWasAloud_CorrectsWereAloud(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They were aloud to participate.", NewWasAloud(), "They were allowed to participate.")
}

func TestWasAloud_DoesNotCorrectProperUseOfAloud(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She read the passage aloud to the class.", NewWasAloud(), "She read the passage aloud to the class.")
}

func TestWasAloud_DoesNotFlagUnrelatedText(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The concert was loud and exciting.", NewWasAloud(), "The concert was loud and exciting.")
}

func TestWasAloud_BeAloud(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You may be aloud to enter the room.", NewWasAloud(), "You may be allowed to enter the room.")
}

func TestWasAloud_BeenAloud(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If I had been aloud to enter I would've jumped at the chance.", NewWasAloud(), "If I had been allowed to enter I would've jumped at the chance.")
}
