package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTheProperNounPossessive_FixThePutinsWar(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The Putin's war", NewTheProperNounPossessive(), "Putin's war")
}

func TestTheProperNounPossessive_FixTheLondonsPopulation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The London's population.", NewTheProperNounPossessive(), "London's population.")
}

func TestTheProperNounPossessive_DontFlagCommonNounInTitlecase(t *testing.T) {
	linttest.AssertNoLints(t, "The Dog's Dinner", NewTheProperNounPossessive())
}

func TestTheProperNounPossessive_FixProperNounStylizedToBeginLowercase(t *testing.T) {
	t.Skip("Can't currently do this due to issue #???")
	linttest.AssertSuggestionResult(t, "The macOS's Finder", NewTheProperNounPossessive(), "macOS's Finder")
}

func TestTheProperNounPossessive_FixEvenWhenCapitalisationOmitted(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the egypt's pyramids", NewTheProperNounPossessive(), "egypt's pyramids")
}

func TestTheProperNounPossessive_DontFlagProperNounThatsAlsoCommonNoun(t *testing.T) {
	linttest.AssertNoLints(t, "the china's broken", NewTheProperNounPossessive())
}

func TestTheProperNounPossessive_DontFlagTheInternets(t *testing.T) {
	linttest.AssertNoLints(t, "The internet's most popular icon toolkit has been redesigned", NewTheProperNounPossessive())
}

func TestTheProperNounPossessive_DontFlagTheInternetsCurlyApostrophe(t *testing.T) {
	linttest.AssertNoLints(t, "The internet’s most popular icon toolkit has been redesigned", NewTheProperNounPossessive())
}
