package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSafeToSave_CorrectsCouldSafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He could safe my life.", NewSafeToSave(), "He could save my life.")
}

func TestSafeToSave_CorrectsShouldSafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You should safe your work frequently.", NewSafeToSave(), "You should save your work frequently.")
}

func TestSafeToSave_CorrectsWillSafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This will safe you time.", NewSafeToSave(), "This will save you time.")
}

func TestSafeToSave_CorrectsWouldSafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It would safe us money.", NewSafeToSave(), "It would save us money.")
}

func TestSafeToSave_CorrectsCanSafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can safe the document now.", NewSafeToSave(), "You can save the document now.")
}

func TestSafeToSave_CorrectsMightSafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This might safe the company.", NewSafeToSave(), "This might save the company.")
}

func TestSafeToSave_CorrectsMustSafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We must safe our resources.", NewSafeToSave(), "We must save our resources.")
}

func TestSafeToSave_CorrectsMaySafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You may safe your progress here.", NewSafeToSave(), "You may save your progress here.")
}

func TestSafeToSave_CorrectsWithAdverb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You should definitely safe your changes.", NewSafeToSave(), "You should definitely save your changes.")
}

func TestSafeToSave_CorrectsShallSafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We shall safe the nation.", NewSafeToSave(), "We shall save the nation.")
}

func TestSafeToSave_CorrectsCouldntSafe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I couldn't safe the file.", NewSafeToSave(), "I couldn't save the file.")
}

func TestSafeToSave_AllowsSafeToVerb(t *testing.T) {
	linttest.AssertNoLints(t, "It is safe to assume.", NewSafeToSave())
}

func TestSafeToSave_AllowsSafeNoun(t *testing.T) {
	linttest.AssertNoLints(t, "Put the money in the safe today.", NewSafeToSave())
}

func TestSafeToSave_AllowsCorrectSave(t *testing.T) {
	linttest.AssertNoLints(t, "You should save your work.", NewSafeToSave())
}
