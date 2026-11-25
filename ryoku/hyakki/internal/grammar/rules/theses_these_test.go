package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestThesesThese_CorrectsThesesScenes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Are theses scenes from a novel?", NewThesesThese(), "Are these scenes from a novel?")
}

func TestThesesThese_CorrectsThesesDays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That's why the two countries look as they do theses days.", NewThesesThese(), "That's why the two countries look as they do these days.")
}

func TestThesesThese_AllowsCorrectTheses(t *testing.T) {
	linttest.AssertNoLints(t, "There are universities that are dedicated just to this field, thousands of people doing theses on Picasso, for example.", NewThesesThese())
}

func TestThesesThese_AllowsThesesFollowedByVerb(t *testing.T) {
	linttest.AssertNoLints(t, "Theses are the times that try men's souls.", NewThesesThese())
}

func TestThesesThese_WorksWithCapitalization(t *testing.T) {
	linttest.AssertSuggestionResult(t, "THESES BOOKS ARE GREAT.", NewThesesThese(), "THESE BOOKS ARE GREAT.")
}

func TestThesesThese_WorksWithMixedCapitalization(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Theses Books Are My Favorite.", NewThesesThese(), "These Books Are My Favorite.")
}

func TestThesesThese_SimpleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I like theses apples.", NewThesesThese(), "I like these apples.")
}

func TestThesesThese_WithPunctuation(t *testing.T) {
	linttest.AssertNoLints(t, "Are theses, books good?", NewThesesThese())
}

func TestThesesThese_InTheMiddleOfSentence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I saw theses movies yesterday.", NewThesesThese(), "I saw these movies yesterday.")
}

func TestThesesThese_AnotherExample(t *testing.T) {
	linttest.AssertLintCount(t, "I have theses books.", NewThesesThese(), 1)
}

func TestThesesThese_AllowsBandName(t *testing.T) {
	linttest.AssertNoLints(t, "Theses are a great band.", NewThesesThese())
}

func TestThesesThese_DoesNotCorrectValidTheses(t *testing.T) {
	linttest.AssertNoLints(t, "She wrote multiple theses on the topic.", NewThesesThese())
}
