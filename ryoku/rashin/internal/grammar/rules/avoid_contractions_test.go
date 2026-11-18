package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAvoidContractions_ExpandsIsnt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This isn't necessary.", NewAvoidContractions(), "This is not necessary.")
}

func TestAvoidContractions_ExpandsWasnt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It wasn't ready.", NewAvoidContractions(), "It was not ready.")
}

func TestAvoidContractions_ExpandsArent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They aren't coming.", NewAvoidContractions(), "They are not coming.")
}

func TestAvoidContractions_ExpandsDont(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We don't need it.", NewAvoidContractions(), "We do not need it.")
}

func TestAvoidContractions_ExpandsDoesnt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She doesn't agree.", NewAvoidContractions(), "She does not agree.")
}

func TestAvoidContractions_ExpandsDidnt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He didn't answer.", NewAvoidContractions(), "He did not answer.")
}

func TestAvoidContractions_ExpandsCant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You can't go.", NewAvoidContractions(), "You cannot go.")
}

func TestAvoidContractions_ExpandsWont(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They won't stop.", NewAvoidContractions(), "They will not stop.")
}

func TestAvoidContractions_ExpandsHavent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I haven't finished.", NewAvoidContractions(), "I have not finished.")
}

func TestAvoidContractions_ExpandsIm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm ready.", NewAvoidContractions(), "I am ready.")
}

func TestAvoidContractions_ExpandsYoure(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You're right.", NewAvoidContractions(), "You are right.")
}

func TestAvoidContractions_ExpandsTheyll(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They'll arrive soon.", NewAvoidContractions(), "They will arrive soon.")
}

func TestAvoidContractions_ExpandsAdditionalUnambiguousContractions(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We could've waited.", NewAvoidContractions(), "We could have waited.")
	linttest.AssertSuggestionResult(t, "They couldn't've known.", NewAvoidContractions(), "They could not have known.")
	linttest.AssertSuggestionResult(t, "How're you doing?", NewAvoidContractions(), "How are you doing?")
	linttest.AssertSuggestionResult(t, "You mayn't agree.", NewAvoidContractions(), "You may not agree.")
	linttest.AssertSuggestionResult(t, "I might've helped.", NewAvoidContractions(), "I might have helped.")
	linttest.AssertSuggestionResult(t, "We must've missed it.", NewAvoidContractions(), "We must have missed it.")
	linttest.AssertSuggestionResult(t, "She oughtn't leave.", NewAvoidContractions(), "She ought not leave.")
	linttest.AssertSuggestionResult(t, "That'll work.", NewAvoidContractions(), "That will work.")
	linttest.AssertSuggestionResult(t, "What'll happen?", NewAvoidContractions(), "What will happen?")
	linttest.AssertSuggestionResult(t, "Who're they?", NewAvoidContractions(), "Who are they?")
}

func TestAvoidContractions_PreservesSentenceInitialCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Isn't this clear?", NewAvoidContractions(), "Is not this clear?")
}

func TestAvoidContractions_PreservesAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "WE DON'T AGREE.", NewAvoidContractions(), "WE DO NOT AGREE.")
}

func TestAvoidContractions_HandlesTypographicApostrophes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They’re prepared.", NewAvoidContractions(), "They are prepared.")
}

func TestAvoidContractions_DoesNotFlagPossessives(t *testing.T) {
	linttest.AssertNoLints(t, "Alice's book is here.", NewAvoidContractions())
}

func TestAvoidContractions_DoesNotFlagNamesWithApostrophes(t *testing.T) {
	linttest.AssertNoLints(t, "O'Connor arrived.", NewAvoidContractions())
}

func TestAvoidContractions_DoesNotFlagAmbiguousContractions(t *testing.T) {
	linttest.AssertNoLints(t, "Ain't that odd? It's done. He'd go. She's been there. Who's ready? We'd left.", NewAvoidContractions())
}

func TestAvoidContractions_DoesNotFlagNonStandardOrLexicalizedContractions(t *testing.T) {
	linttest.AssertNoLints(t, "We met y'all near the nor'easter exhibit at five o'clock.", NewAvoidContractions())
}
