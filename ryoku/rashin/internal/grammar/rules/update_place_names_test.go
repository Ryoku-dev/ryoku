package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestUpdatePlaceNames_UpdateSingleWordNameAlone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Bombay", NewUpdatePlaceNames(), "Mumbai")
}

func TestUpdatePlaceNames_UpdateSingleWordNameAfterSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, " Bombay", NewUpdatePlaceNames(), " Mumbai")
}

func TestUpdatePlaceNames_UpdateSingleWordNameAfterPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, ";Bombay", NewUpdatePlaceNames(), ";Mumbai")
}

func TestUpdatePlaceNames_UpdateTwoWordNameToSingleWordAlone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ayers Rock", NewUpdatePlaceNames(), "Uluru")
}

func TestUpdatePlaceNames_UpdateTwoWordNameToSingleWordAfterSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, " Ayers Rock", NewUpdatePlaceNames(), " Uluru")
}

func TestUpdatePlaceNames_UpdateTwoWordNameToSingleWordAfterPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, ";Ayers Rock", NewUpdatePlaceNames(), ";Uluru")
}

func TestUpdatePlaceNames_UpdateSingleWordNameToMultiWordNameAlone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Saigon", NewUpdatePlaceNames(), "Ho Chi Minh City")
}

func TestUpdatePlaceNames_UpdateTwoWordNameToTwoWordNameAlone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Olgas", NewUpdatePlaceNames(), "Kata Tjuta")
}

func TestUpdatePlaceNames_DontFlagMultiwordNameWithNonSpace(t *testing.T) {
	linttest.AssertLintCount(t, "The, Olgas", NewUpdatePlaceNames(), 0)
}

func TestUpdatePlaceNames_DontFlagMultiwordNameWithHyphen(t *testing.T) {
	linttest.AssertLintCount(t, "The-Olgas", NewUpdatePlaceNames(), 0)
}

func TestUpdatePlaceNames_FlagMultiwordNameWithTabs(t *testing.T) {
	t.Skip("tabs not supported as whitespace?")
	linttest.AssertLintCount(t, "The\tOlgas", NewUpdatePlaceNames(), 1)
}

func TestUpdatePlaceNames_FlagMultiwordNameWithNewline(t *testing.T) {
	t.Skip("newlines not supported as whitespace?")
	linttest.AssertLintCount(t, "The\nOlgas", NewUpdatePlaceNames(), 1)
}

func TestUpdatePlaceNames_UpdateTwoWordNameToSingleWordAtEndOfSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's dangerous to climb Ayers Rock.", NewUpdatePlaceNames(), "It's dangerous to climb Uluru.")
}

func TestUpdatePlaceNames_UpdateTwoWordNameToSingleWordAtStartOfSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ayers Rock is dangerous to climb.", NewUpdatePlaceNames(), "Uluru is dangerous to climb.")
}

func TestUpdatePlaceNames_UpdateFirstOldName(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Leningrad", NewUpdatePlaceNames(), "Saint Petersburg")
}

func TestUpdatePlaceNames_UpdateSecondOldName(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Have you ever been to Petrograd before?", NewUpdatePlaceNames(), "Have you ever been to Saint Petersburg before?")
}

func TestUpdatePlaceNames_UpdateTwoWordNameWithTwoWordName(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Upper Volta is in Africa.", NewUpdatePlaceNames(), "Burkina Faso is in Africa.")
}

func TestUpdatePlaceNames_DontFlagCzechRepublic(t *testing.T) {
	linttest.AssertLintCount(t, "The Czech Republic is in Europe.", NewUpdatePlaceNames(), 0)
}

func TestUpdatePlaceNames_UpdateToNameWithPunctuation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've never been to Ivory Coast.", NewUpdatePlaceNames(), "I've never been to Côte d'Ivoire.")
}
