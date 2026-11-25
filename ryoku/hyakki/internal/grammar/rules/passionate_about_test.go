package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPassionateAbout_FixPassionateOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "if a human being is writing about something you enjoy or are passionate of, you firstly have a community and are able to talk with a actual person ...", NewPassionateAbout(), "if a human being is writing about something you enjoy or are passionate about, you firstly have a community and are able to talk with a actual person ...")
}

func TestPassionateAbout_FixOfWhichWe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... and become a thing of which we are passionate and which, through misapplication ...", NewPassionateAbout(), "... and become a thing about which we are passionate and which, through misapplication ...")
}

func TestPassionateAbout_FixPassionateOfEmbodiedAi(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Passionate of Embodied AI from Poland.", NewPassionateAbout(), "Passionate about Embodied AI from Poland.")
}

func TestPassionateAbout_FixOfWhichThey(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... part of organizations and causes on campus, of which they are passionate.", NewPassionateAbout(), "... part of organizations and causes on campus, about which they are passionate.")
}

func TestPassionateAbout_FixPassionateOfTechAiMathFinance(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Computer scientist, passionate of tech, AI, math finance.", NewPassionateAbout(), "Computer scientist, passionate about tech, AI, math finance.")
}

func TestPassionateAbout_FixPassionateOfWebMapping(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A geographer turned software engineer, passionate of web mapping, data engineering, analytics, and visualization.", NewPassionateAbout(), "A geographer turned software engineer, passionate about web mapping, data engineering, analytics, and visualization.")
}

func TestPassionateAbout_FixPassionateOfAutomation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am passionate of automation because: I like to make my life easier; I like to make my work easier. Examples of projects in IT field, of automations I ...Read more", NewPassionateAbout(), "I am passionate about automation because: I like to make my life easier; I like to make my work easier. Examples of projects in IT field, of automations I ...Read more")
}

func TestPassionateAbout_DontFlagBothOfWhichHeWasPassionateAbout(t *testing.T) {
	linttest.AssertNoLints(t, "... give up his research on glioma and being a neurosurgeon both of which he was passionate about to go into the black hole that was researching a disease ...", NewPassionateAbout())
}

func TestPassionateAbout_FixPassionateAboutInParentheses(t *testing.T) {
	t.Skip("Both suggestions are broken due to #3741")
	linttest.AssertGoodAndBadSuggestions(t, "Watching/ reading documentaries; anything history-related, especially about Richard III (of which I am passionate about) and ...", NewPassionateAbout(), []string{
		"Watching/ reading documentaries; anything history-related, especially about Richard III (about which I am passionate) and ...",
		"Watching/ reading documentaries; anything history-related, especially about Richard III (which I am passionate about) and ...",
	}, []string{})
}

func TestPassionateAbout_DontFlagAllOfWhichHeWasPassionateAbout(t *testing.T) {
	linttest.AssertNoLints(t, "he could be found spending time on the pontoon boat at Little Sebago Lake, all of which he was passionate about.", NewPassionateAbout())
}

func TestPassionateAbout_FixDoingSomething(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If I was going to go out and compete with my brothers, I wanted to make sure I was doing something of which I was passionate.", NewPassionateAbout(), "If I was going to go out and compete with my brothers, I wanted to make sure I was doing something about which I was passionate.")
}

func TestPassionateAbout_FixEnvironmetalFields(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I choose EEP because I wanted to learn how to better advocate for and make changes in the marine and environmental fields of which I am passionate.", NewPassionateAbout(), "I choose EEP because I wanted to learn how to better advocate for and make changes in the marine and environmental fields about which I am passionate.")
}
