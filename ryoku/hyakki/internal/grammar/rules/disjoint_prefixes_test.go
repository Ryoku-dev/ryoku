package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDisjointPrefixes_FixHyphenatedToJoined(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Download pre-built binaries or build from source.", NewDisjointPrefixes(dict.Curated()), "Download prebuilt binaries or build from source.")
}

func TestDisjointPrefixes_FixOpenToJoined(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Advanced Nginx configuration available for super users", NewDisjointPrefixes(dict.Curated()), "Advanced Nginx configuration available for superusers")
}

func TestDisjointPrefixes_DontJoinContractionSuffixAfterAsciiApostrophe(t *testing.T) {
	linttest.AssertNoLints(t, "you 're fresh so you don't neglect them", NewDisjointPrefixes(dict.Curated()))
}

func TestDisjointPrefixes_DontJoinContractionSuffixAfterTypographicApostrophe(t *testing.T) {
	linttest.AssertNoLints(t, "you ’re fresh so you don’t neglect them", NewDisjointPrefixes(dict.Curated()))
}

func TestDisjointPrefixes_DontJoinOpenCoOp(t *testing.T) {
	linttest.AssertNoLints(t, "They are cheaper at the co op.", NewDisjointPrefixes(dict.Curated()))
}

func TestDisjointPrefixes_DontJoinHyphenatedCoOp(t *testing.T) {
	linttest.AssertNoLints(t, "Almost everything is cheaper at the co-op.", NewDisjointPrefixes(dict.Curated()))
}

func TestDisjointPrefixes_FixOpenToHyphenated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My hobby is de extinction of the dinosaurs.", NewDisjointPrefixes(dict.Curated()), "My hobby is de-extinction of the dinosaurs.")
}

func TestDisjointPrefixes_DontJoinHyphenatedReExecute3954(t *testing.T) {
	linttest.AssertNoLints(t, "I saw re-execute flagged with DisjointPrefixes.", NewDisjointPrefixes(dict.Curated()))
}

func TestDisjointPrefixes_DontJoinOutGuesses4853(t *testing.T) {
	linttest.AssertNoLints(t, "Leave out guesses about how people felt.", NewDisjointPrefixes(dict.Curated()))
}

func TestDisjointPrefixes_FixTitleCaseHyphenatedToJoined4381(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Pre-Requisite", NewDisjointPrefixes(dict.Curated()), "Prerequisite")
}

func TestDisjointPrefixes_FixAllCapsHyphenatedToJoined(t *testing.T) {
	linttest.AssertSuggestionResult(t, "PRE-REQUISITE", NewDisjointPrefixes(dict.Curated()), "PREREQUISITE")
}
