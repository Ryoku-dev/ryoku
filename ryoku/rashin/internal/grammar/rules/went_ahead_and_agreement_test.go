package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWentAheadAndAgreement_WentBan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "He went ahead and spam the Discord so I've gone ahead and ban him.", NewWentAheadAndAgreement(dict.Curated()), "He went ahead and spammed the Discord so I've gone ahead and banned him.")
}

func TestWentAheadAndAgreement_WentAdd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I went ahead and add a note to it's javadocs that the reason argument doesn't impact it's equality", NewWentAheadAndAgreement(dict.Curated()), "I went ahead and added a note to it's javadocs that the reason argument doesn't impact it's equality")
}

func TestWentAheadAndAgreement_WentBuild(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I went ahead and build out creating a shiny input from a json schema as as separate package using reactR and react-jsonschema-form", NewWentAheadAndAgreement(dict.Curated()), "I went ahead and build out creating a shiny input from a json schema as as separate package using reactR and react-jsonschema-form")
}

func TestWentAheadAndAgreement_WentChange(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So I went ahead and change the behavior to explicitly fills the default domain into the domain field if no domain is specified.", NewWentAheadAndAgreement(dict.Curated()), "So I went ahead and changed the behavior to explicitly fills the default domain into the domain field if no domain is specified.")
}

func TestWentAheadAndAgreement_WentDo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "compiler automatically identified vectorization opportunities and went ahead and do vectorization", NewWentAheadAndAgreement(dict.Curated()), "compiler automatically identified vectorization opportunities and went ahead and did vectorization")
}

func TestWentAheadAndAgreement_WentEnable(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I went ahead and enable it for those systems and fixed the resulting errors that were previously unsurfaced.", NewWentAheadAndAgreement(dict.Curated()), "I went ahead and enabled it for those systems and fixed the resulting errors that were previously unsurfaced.")
}

func TestWentAheadAndAgreement_GoneMake(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hi - I've gone ahead and make a conda package for UMICollapse", NewWentAheadAndAgreement(dict.Curated()), "Hi - I've gone ahead and made a conda package for UMICollapse")
}

func TestWentAheadAndAgreement_GoneOpen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would've gone ahead and open a PR in this project for the style guide", NewWentAheadAndAgreement(dict.Curated()), "I would've gone ahead and opened a PR in this project for the style guide")
}

func TestWentAheadAndAgreement_DontFlagHaveGoneAheadAndHaveAdded(t *testing.T) {
	t.Skip("Upstream ignored: we're aware of this edge case but we don't handle it yet")
	linttest.AssertNoLints(t, "I have gone ahead and have added a +1 and have added your case to the request in support of it.", NewWentAheadAndAgreement(dict.Curated()))
}
