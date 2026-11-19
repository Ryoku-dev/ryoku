package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestShowCase_FixShowCaseNounTitleCase(t *testing.T) {
	t.Skip("Ignored upstream: Fails due to `replace_with_match_case_str` copying case based on character index")
	linttest.AssertSuggestionResult(t, "Progressive Web Apps (PWA) Show Case.", NewShowCase(), "Progressive Web Apps (PWA) Showcase.")
}

func TestShowCase_FixShowCaseNounLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hello,How to run the show case in release mode.", NewShowCase(), "Hello,How to run the showcase in release mode.")
}

func TestShowCase_DontFlagCaseInsensitive(t *testing.T) {
	linttest.AssertNoLints(t, "Show case insensitive results when searching for a branch.", NewShowCase())
}

func TestShowCase_DontFlagCaseSensitive(t *testing.T) {
	linttest.AssertNoLints(t, "starship shows case-sensitive behavior on case-insensitive file system (macOS)", NewShowCase())
}

func TestShowCase_FixVerb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Some stories don't show case how the component should be used.", NewShowCase(), "Some stories don't showcase how the component should be used.")
}

func TestShowCase_FixVerbHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It's pretty simple to show-case a feature in your App by using BubbleShowCase framework.", NewShowCase(), "It's pretty simple to showcase a feature in your App by using BubbleShowCase framework.")
}
