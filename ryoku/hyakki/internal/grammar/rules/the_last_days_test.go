package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTheLastDays_FixInTheLastMinutes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Required Files changed in the last minutes.", NewTheLastDays(), "Required Files changed in the last few minutes.")
}

func TestTheLastDays_FixForTheLastDays(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The nightly docs CI job and the job on the individual commits has been failing for the last days", NewTheLastDays(), "The nightly docs CI job and the job on the individual commits has been failing for the last few days")
}

func TestTheLastDays_FixDuringTheLastWeeks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "no, i was not in germany during the last weeks!", NewTheLastDays(), "no, i was not in germany during the last few weeks!")
}

func TestTheLastDays_FixDuringTheLastMonths(t *testing.T) {
	linttest.AssertSuggestionResult(t, "GroupDocs.Viewer.dll increasing rapidly in size during the last months", NewTheLastDays(), "GroupDocs.Viewer.dll increasing rapidly in size during the last few months")
}

func TestTheLastDays_FixInTheLastYears(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the last years I've been asked multiple times about the comparison between raylib and SDL libraries.", NewTheLastDays(), "In the last few years I've been asked multiple times about the comparison between raylib and SDL libraries.")
}

func TestTheLastDays_FixDuringTheLastDecades(t *testing.T) {
	linttest.AssertSuggestionResult(t, "During the last decades they have been used successfully as digital music archive management tools", NewTheLastDays(), "During the last few decades they have been used successfully as digital music archive management tools")
}

func TestTheLastDays_DontFlagSecondsWithoutPreposition(t *testing.T) {
	linttest.AssertNoLints(t, "It grabs the last seconds from the song and continues the song only from that data.", NewTheLastDays())
}

func TestTheLastDays_DontFlagMinutesWithoutPreposition(t *testing.T) {
	linttest.AssertNoLints(t, "so the tracker can count the last minutes it was seeded", NewTheLastDays())
}

func TestTheLastDays_DontFlagDaysWithoutPreposition(t *testing.T) {
	linttest.AssertNoLints(t, "Wrong date being returned during the last days of year", NewTheLastDays())
}

func TestTheLastDays_FixSpentTheLastHours(t *testing.T) {
	t.Skip("not after a preposition but is a mistake...or maybe ambiguous")
	linttest.AssertSuggestionResult(t, "And from what I spent the last hours investigating there's no way to change this behavior", NewTheLastDays(), "And from what I spent the last few hours investigating there's no way to change this behavior")
}

func TestTheLastDays_Fix4TimesTheLastMonths(t *testing.T) {
	t.Skip("not after a preposition but is a mistake")
	linttest.AssertSuggestionResult(t, "We have been affected 4 different times the last months due to upgrades coming from \"async\" mode", NewTheLastDays(), "We have been affected 4 different times the last months due to upgrades coming from \"async\" mode")
}

func TestTheLastDays_DontFlagLastSecondsOf(t *testing.T) {
	linttest.AssertNoLints(t, "But if you have specific trouble at the last seconds of the stream, try this PR", NewTheLastDays())
}
