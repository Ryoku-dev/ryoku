package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestProgressiveNeedsBe_SuggestsImLooking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've looking into it.", NewProgressiveNeedsBe(), "I'm looking into it.")
}

func TestProgressiveNeedsBe_CorrectsBasicIm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've looking into it.", NewProgressiveNeedsBe(), "I'm looking into it.")
}

func TestProgressiveNeedsBe_OffersBothSuggestions(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I've looking into it.", NewProgressiveNeedsBe(), []string{"I'm looking into it.", "I've been looking into it."}, []string{})
}

func TestProgressiveNeedsBe_AllowsIveLooked(t *testing.T) {
	linttest.AssertLintCount(t, "I've looked into it.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_AllowsIveBeenLooking(t *testing.T) {
	linttest.AssertLintCount(t, "I've been looking into it.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_AllowsIveSeen(t *testing.T) {
	linttest.AssertLintCount(t, "I've seen the results.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_AllowsIveLongBeenLooking(t *testing.T) {
	linttest.AssertLintCount(t, "I've long been looking into it.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_NoMatchWithPunctuationBetween(t *testing.T) {
	linttest.AssertLintCount(t, "I've, looking into it.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_HandlesNewlineWhitespace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've\nlooking into it.", NewProgressiveNeedsBe(), "I'm\nlooking into it.")
}

func TestProgressiveNeedsBe_CapitalizationAllCapsBase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'VE looking into it.", NewProgressiveNeedsBe(), "I'M looking into it.")
}

func TestProgressiveNeedsBe_WorksForWeve(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We've looking into it.", NewProgressiveNeedsBe(), "We're looking into it.")
}

func TestProgressiveNeedsBe_SuggestsImLookingNonContracted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have looking into it.", NewProgressiveNeedsBe(), "I'm looking into it.")
}

func TestProgressiveNeedsBe_OffersBothSuggestionsNonContracted(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "They have looking into it.", NewProgressiveNeedsBe(), []string{
		"They're looking into it.",
		"They have been looking into it.",
	}, []string{})
}

func TestProgressiveNeedsBe_AllowsIHaveBeenLooking(t *testing.T) {
	linttest.AssertLintCount(t, "I have been looking into it.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_AllowsIHaveLooked(t *testing.T) {
	linttest.AssertLintCount(t, "I have looked into it.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_IveWorking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've working on it today.", NewProgressiveNeedsBe(), "I'm working on it today.")
}

func TestProgressiveNeedsBe_WeveWorking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We've working on it today.", NewProgressiveNeedsBe(), "We're working on it today.")
}

func TestProgressiveNeedsBe_YouveWorking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You've working on it today.", NewProgressiveNeedsBe(), "You're working on it today.")
}

func TestProgressiveNeedsBe_TheyveWorking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They've working on it today.", NewProgressiveNeedsBe(), "They're working on it today.")
}

func TestProgressiveNeedsBe_IveEating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've eating it today.", NewProgressiveNeedsBe(), "I'm eating it today.")
}

func TestProgressiveNeedsBe_WeveEating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We've eating it today.", NewProgressiveNeedsBe(), "We're eating it today.")
}

func TestProgressiveNeedsBe_YouveEating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You've eating it today.", NewProgressiveNeedsBe(), "You're eating it today.")
}

func TestProgressiveNeedsBe_TheyveEating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They've eating it today.", NewProgressiveNeedsBe(), "They're eating it today.")
}

func TestProgressiveNeedsBe_IveReading(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've reading it today.", NewProgressiveNeedsBe(), "I'm reading it today.")
}

func TestProgressiveNeedsBe_WeveReading(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We've reading it today.", NewProgressiveNeedsBe(), "We're reading it today.")
}

func TestProgressiveNeedsBe_YouveReading(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You've reading it today.", NewProgressiveNeedsBe(), "You're reading it today.")
}

func TestProgressiveNeedsBe_TheyveReading(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They've reading it today.", NewProgressiveNeedsBe(), "They're reading it today.")
}

func TestProgressiveNeedsBe_IveWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've writing it today.", NewProgressiveNeedsBe(), "I'm writing it today.")
}

func TestProgressiveNeedsBe_WeveWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We've writing it today.", NewProgressiveNeedsBe(), "We're writing it today.")
}

func TestProgressiveNeedsBe_YouveWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You've writing it today.", NewProgressiveNeedsBe(), "You're writing it today.")
}

func TestProgressiveNeedsBe_TheyveWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They've writing it today.", NewProgressiveNeedsBe(), "They're writing it today.")
}

func TestProgressiveNeedsBe_IveSpeaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've speaking about it today.", NewProgressiveNeedsBe(), "I'm speaking about it today.")
}

func TestProgressiveNeedsBe_WeveSpeaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We've speaking about it today.", NewProgressiveNeedsBe(), "We're speaking about it today.")
}

func TestProgressiveNeedsBe_YouveSpeaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You've speaking about it today.", NewProgressiveNeedsBe(), "You're speaking about it today.")
}

func TestProgressiveNeedsBe_TheyveSpeaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They've speaking about it today.", NewProgressiveNeedsBe(), "They're speaking about it today.")
}

func TestProgressiveNeedsBe_IveStudying(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've studying it today.", NewProgressiveNeedsBe(), "I'm studying it today.")
}

func TestProgressiveNeedsBe_WeveStudying(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We've studying it today.", NewProgressiveNeedsBe(), "We're studying it today.")
}

func TestProgressiveNeedsBe_YouveStudying(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You've studying it today.", NewProgressiveNeedsBe(), "You're studying it today.")
}

func TestProgressiveNeedsBe_TheyveStudying(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They've studying it today.", NewProgressiveNeedsBe(), "They're studying it today.")
}

func TestProgressiveNeedsBe_IveTesting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've testing it today.", NewProgressiveNeedsBe(), "I'm testing it today.")
}

func TestProgressiveNeedsBe_WeveTesting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We've testing it today.", NewProgressiveNeedsBe(), "We're testing it today.")
}

func TestProgressiveNeedsBe_YouveTesting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You've testing it today.", NewProgressiveNeedsBe(), "You're testing it today.")
}

func TestProgressiveNeedsBe_TheyveTesting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They've testing it today.", NewProgressiveNeedsBe(), "They're testing it today.")
}

func TestProgressiveNeedsBe_IveUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've using it today.", NewProgressiveNeedsBe(), "I'm using it today.")
}

func TestProgressiveNeedsBe_WeveUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We've using it today.", NewProgressiveNeedsBe(), "We're using it today.")
}

func TestProgressiveNeedsBe_YouveUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You've using it today.", NewProgressiveNeedsBe(), "You're using it today.")
}

func TestProgressiveNeedsBe_TheyveUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They've using it today.", NewProgressiveNeedsBe(), "They're using it today.")
}

func TestProgressiveNeedsBe_IHaveWorking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have working on it today.", NewProgressiveNeedsBe(), "I'm working on it today.")
}

func TestProgressiveNeedsBe_WeHaveWorking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have working on it today.", NewProgressiveNeedsBe(), "We're working on it today.")
}

func TestProgressiveNeedsBe_YouHaveWorking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You have working on it today.", NewProgressiveNeedsBe(), "You're working on it today.")
}

func TestProgressiveNeedsBe_TheyHaveWorking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have working on it today.", NewProgressiveNeedsBe(), "They're working on it today.")
}

func TestProgressiveNeedsBe_IHaveEating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have eating it today.", NewProgressiveNeedsBe(), "I'm eating it today.")
}

func TestProgressiveNeedsBe_WeHaveEating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have eating it today.", NewProgressiveNeedsBe(), "We're eating it today.")
}

func TestProgressiveNeedsBe_YouHaveEating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You have eating it today.", NewProgressiveNeedsBe(), "You're eating it today.")
}

func TestProgressiveNeedsBe_TheyHaveEating(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have eating it today.", NewProgressiveNeedsBe(), "They're eating it today.")
}

func TestProgressiveNeedsBe_IHaveReading(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have reading it today.", NewProgressiveNeedsBe(), "I'm reading it today.")
}

func TestProgressiveNeedsBe_WeHaveReading(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have reading it today.", NewProgressiveNeedsBe(), "We're reading it today.")
}

func TestProgressiveNeedsBe_YouHaveReading(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You have reading it today.", NewProgressiveNeedsBe(), "You're reading it today.")
}

func TestProgressiveNeedsBe_TheyHaveReading(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have reading it today.", NewProgressiveNeedsBe(), "They're reading it today.")
}

func TestProgressiveNeedsBe_IHaveWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have writing it today.", NewProgressiveNeedsBe(), "I'm writing it today.")
}

func TestProgressiveNeedsBe_WeHaveWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have writing it today.", NewProgressiveNeedsBe(), "We're writing it today.")
}

func TestProgressiveNeedsBe_YouHaveWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You have writing it today.", NewProgressiveNeedsBe(), "You're writing it today.")
}

func TestProgressiveNeedsBe_TheyHaveWriting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have writing it today.", NewProgressiveNeedsBe(), "They're writing it today.")
}

func TestProgressiveNeedsBe_IHaveSpeaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have speaking about it today.", NewProgressiveNeedsBe(), "I'm speaking about it today.")
}

func TestProgressiveNeedsBe_WeHaveSpeaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have speaking about it today.", NewProgressiveNeedsBe(), "We're speaking about it today.")
}

func TestProgressiveNeedsBe_YouHaveSpeaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You have speaking about it today.", NewProgressiveNeedsBe(), "You're speaking about it today.")
}

func TestProgressiveNeedsBe_TheyHaveSpeaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have speaking about it today.", NewProgressiveNeedsBe(), "They're speaking about it today.")
}

func TestProgressiveNeedsBe_IHaveStudying(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have studying it today.", NewProgressiveNeedsBe(), "I'm studying it today.")
}

func TestProgressiveNeedsBe_WeHaveStudying(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have studying it today.", NewProgressiveNeedsBe(), "We're studying it today.")
}

func TestProgressiveNeedsBe_YouHaveStudying(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You have studying it today.", NewProgressiveNeedsBe(), "You're studying it today.")
}

func TestProgressiveNeedsBe_TheyHaveStudying(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have studying it today.", NewProgressiveNeedsBe(), "They're studying it today.")
}

func TestProgressiveNeedsBe_IHaveTesting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have testing it today.", NewProgressiveNeedsBe(), "I'm testing it today.")
}

func TestProgressiveNeedsBe_WeHaveTesting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have testing it today.", NewProgressiveNeedsBe(), "We're testing it today.")
}

func TestProgressiveNeedsBe_YouHaveTesting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You have testing it today.", NewProgressiveNeedsBe(), "You're testing it today.")
}

func TestProgressiveNeedsBe_TheyHaveTesting(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have testing it today.", NewProgressiveNeedsBe(), "They're testing it today.")
}

func TestProgressiveNeedsBe_IHaveUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have using it today.", NewProgressiveNeedsBe(), "I'm using it today.")
}

func TestProgressiveNeedsBe_WeHaveUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have using it today.", NewProgressiveNeedsBe(), "We're using it today.")
}

func TestProgressiveNeedsBe_YouHaveUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You have using it today.", NewProgressiveNeedsBe(), "You're using it today.")
}

func TestProgressiveNeedsBe_TheyHaveUsing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They have using it today.", NewProgressiveNeedsBe(), "They're using it today.")
}

func TestProgressiveNeedsBe_BothSuggestionsIveWorking(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "I've working today.", NewProgressiveNeedsBe(), []string{"I'm working today.", "I've been working today."}, []string{})
}

func TestProgressiveNeedsBe_BothSuggestionsWeHaveReading(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "We have reading it today.", NewProgressiveNeedsBe(), []string{"We're reading it today.", "We have been reading it today."}, []string{})
}

func TestProgressiveNeedsBe_BothSuggestionsYouveReading(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "You've reading today.", NewProgressiveNeedsBe(), []string{"You're reading today.", "You've been reading today."}, []string{})
}

func TestProgressiveNeedsBe_BothSuggestionsTheyHaveWriting(t *testing.T) {
	linttest.AssertGoodAndBadSuggestions(t, "They have writing today.", NewProgressiveNeedsBe(), []string{"They're writing today.", "They have been writing today."}, []string{})
}

func TestProgressiveNeedsBe_NoMatchPunctuationNonContracted(t *testing.T) {
	linttest.AssertLintCount(t, "I have, working today.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_NoMatchAdverbInterruption(t *testing.T) {
	linttest.AssertLintCount(t, "I have quickly working today.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_AllowedContractedHaveBeen(t *testing.T) {
	linttest.AssertLintCount(t, "You've been studying today.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_AllowedNonContractedHaveBeen(t *testing.T) {
	linttest.AssertLintCount(t, "You have been studying today.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_AllowedTheyHaveBeen(t *testing.T) {
	linttest.AssertLintCount(t, "They have been testing today.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_AllowedTheyveBeen(t *testing.T) {
	linttest.AssertLintCount(t, "They've been testing today.", NewProgressiveNeedsBe(), 0)
}

func TestProgressiveNeedsBe_CapitalizationVariantsNonContracted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "WE HAVE working today.", NewProgressiveNeedsBe(), "WE'RE working today.")
}

func TestProgressiveNeedsBe_NewlineVariantsNonContracted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We have\nworking on it today.", NewProgressiveNeedsBe(), "We're\nworking on it today.")
}

func TestProgressiveNeedsBe_TestIveBeing(t *testing.T) {
	t.Skip("Upstream ignored: Handling the progressive `being` will need a special case")
	linttest.AssertGoodAndBadSuggestions(t, "I've being playing with languages.toml", NewProgressiveNeedsBe(), []string{
		"I've been playing with languages.toml",
		"I'm playing with languages.toml",
	}, []string{})
}

func TestProgressiveNeedsBe_TestIveDoingNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ive always seen the variables and debug into it, and thats what ive doing.", NewProgressiveNeedsBe(), "Ive always seen the variables and debug into it, and thats what i'm doing.")
}

func TestProgressiveNeedsBe_TestIveLookingNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ive looking for a way to get temperature and humidity for all of our rooms within for a reasonable price in Germany.", NewProgressiveNeedsBe(), "I'm looking for a way to get temperature and humidity for all of our rooms within for a reasonable price in Germany.")
}

func TestProgressiveNeedsBe_TestYouveBeing(t *testing.T) {
	t.Skip("Upstream ignored: Handling the progressive `being` will need a special case")
	linttest.AssertSuggestionResult(t, "Thanks for all the work you've being doing for this project btw!", NewProgressiveNeedsBe(), "Thanks for all the work you're doing for this project btw!")
}

func TestProgressiveNeedsBe_TestTheyveDoing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it’s also kind of implied users read the documentation or generally have a sense of what they’ve doing and what could go wrong", NewProgressiveNeedsBe(), "it’s also kind of implied users read the documentation or generally have a sense of what they're doing and what could go wrong")
}
