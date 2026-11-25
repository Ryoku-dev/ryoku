package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFedUpWith_CorrectFedUpOfInUsEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am fed up of Bugzilla reports being ignored.", NewFedUpWith(dict.American), "I am fed up with Bugzilla reports being ignored.")
}

func TestFedUpWith_CorrectFedUpOfInCanadianEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Fed up of long links ??? Use ✨ Linsh ✨, a CLI tool to shorten links.", NewFedUpWith(dict.Canadian), "Fed up with long links ??? Use ✨ Linsh ✨, a CLI tool to shorten links.")
}

func TestFedUpWith_CorrectFedUpOfInAusEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Fed up of the lack of Twitter embedded timeline styling options?", NewFedUpWith(dict.Australian), "Fed up with the lack of Twitter embedded timeline styling options?")
}

func TestFedUpWith_CorrectFedUpOfInIndianEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I got fed up of finding my IP (v4) address in the big pile of text that ifconfig outputs on OS X.", NewFedUpWith(dict.Indian), "I got fed up with finding my IP (v4) address in the big pile of text that ifconfig outputs on OS X.")
}

func TestFedUpWith_DontFlagFedUpOfInBritishEnglish(t *testing.T) {
	linttest.AssertNoLints(t, "Fed up of having to repeat the same actions for installing webmin so here's a script for 16.04+", NewFedUpWith(dict.British))
}
