package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRespond_FixesWillResponse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He will response soon.", NewRespond(), "He will respond soon.")
}

func TestRespond_FixesCanResponse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They can response to the survey.", NewRespond(), "They can respond to the survey.")
}

func TestRespond_FixesDidNotResponse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I did not response yesterday.", NewRespond(), "I did not respond yesterday.")
}

func TestRespond_FixesMightQuicklyResponse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "She might quickly response to feedback.", NewRespond(), "She might quickly respond to feedback.")
}

func TestRespond_FixesWontResponse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They won't response in time.", NewRespond(), "They won't respond in time.")
}

func TestRespond_FixesWouldResponse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We would response if we could.", NewRespond(), "We would respond if we could.")
}

func TestRespond_FixesShouldResponse(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You should response politely.", NewRespond(), "You should respond politely.")
}

func TestRespond_DoesNotFlagCorrectRespond(t *testing.T) {
	linttest.AssertNoLints(t, "Please respond when you can.", NewRespond())
}

func TestRespond_DoesNotFlagNounUse(t *testing.T) {
	linttest.AssertNoLints(t, "The response time was great.", NewRespond())
}

func TestRespond_DoesNotFlagQuestionSubject(t *testing.T) {
	linttest.AssertLintCount(t, "Should response times be logged?", NewRespond(), 0)
}

func TestRespond_DoesNotFlagResponseAsObject(t *testing.T) {
	linttest.AssertNoLints(t, "I have no response for that.", NewRespond())
}
