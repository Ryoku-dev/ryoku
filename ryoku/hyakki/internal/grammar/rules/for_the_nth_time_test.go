package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestForTheNthTime_FixStandingUpOpencloud(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Lots of errors when standing up OpenCloud for first time.", NewForTheNthTime(), "Lots of errors when standing up OpenCloud for the first time.")
}

func TestForTheNthTime_FixForSecondTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SearchAnchor error on selecting an item for second time.", NewForTheNthTime(), "SearchAnchor error on selecting an item for the second time.")
}

func TestForTheNthTime_FixForUmpteenthTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Having just accidentally \"closed and commented\", instead of \"commented\" for umpteenth time, I would like to suggest that", NewForTheNthTime(), "Having just accidentally \"closed and commented\", instead of \"commented\" for the umpteenth time, I would like to suggest that")
}

func TestForTheNthTime_FixForNthTimeCapitalized(t *testing.T) {
	linttest.AssertSuggestionResult(t, "sometimes breaks can be very long and you just want to pass your map, since you've failed it for Nth time", NewForTheNthTime(), "sometimes breaks can be very long and you just want to pass your map, since you've failed it for the Nth time")
}

func TestForTheNthTime_FixForNthTimeLowercase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "sometimes breaks can be very long and you just want to pass your map, since you've failed it for nth time", NewForTheNthTime(), "sometimes breaks can be very long and you just want to pass your map, since you've failed it for the nth time")
}

func TestForTheNthTime_FixForMillionthTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "try fixing bin name resolve for millionth time", NewForTheNthTime(), "try fixing bin name resolve for the millionth time")
}

func TestForTheNthTime_FixForEleventhTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "DNF5 will fail after any package download fails for eleventh time", NewForTheNthTime(), "DNF5 will fail after any package download fails for the eleventh time")
}

func TestForTheNthTime_FlagNthTimeFollowedByAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am reading the schema documentation now for nth time and i do not confirm the main restriction", NewForTheNthTime(), "I am reading the schema documentation now for the nth time and i do not confirm the main restriction")
}

func TestForTheNthTime_FixSeventhAndEighthTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Press the key for seventh time and the remainder is 7. The shell will print out the acceleration value;. Press the key for eighth time and the remainder is 0.", NewForTheNthTime(), "Press the key for the seventh time and the remainder is 7. The shell will print out the acceleration value;. Press the key for the eighth time and the remainder is 0.")
}

func TestForTheNthTime_FixEleventhTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The number is cumulative, so e.g. for retries=10 , DNF5 will fail after any package download fails for eleventh time.", NewForTheNthTime(), "The number is cumulative, so e.g. for retries=10 , DNF5 will fail after any package download fails for the eleventh time.")
}

func TestForTheNthTime_FixThousandthTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Why has no one told me before i tried for thousandth time?", NewForTheNthTime(), "Why has no one told me before i tried for the thousandth time?")
}

func TestForTheNthTime_FixSixthTimeBut(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Melissa McCarthy hosts for sixth time but laughs are sparse.", NewForTheNthTime(), "Melissa McCarthy hosts for the sixth time but laughs are sparse.")
}

func TestForTheNthTime_DontFlagHyphenatedReaders(t *testing.T) {
	linttest.AssertNoLints(t, "and it felt like there is still a bit of a hurdle for first-time readers", NewForTheNthTime())
}

func TestForTheNthTime_DontFlagUnhyphenatedUsers(t *testing.T) {
	linttest.AssertNoLints(t, "but just throwing this out as something less complicated for first time users", NewForTheNthTime())
}

func TestForTheNthTime_DontFlagSecondTimeLogin(t *testing.T) {
	linttest.AssertNoLints(t, "Google consent screen popup is not opening for second time login of another account", NewForTheNthTime())
}

func TestForTheNthTime_ShouldFixFollowedByEven(t *testing.T) {
	t.Skip("not yet implemented")
	linttest.AssertSuggestionResult(t, "Appium Android driver is un installing for second time even full reset is false", NewForTheNthTime(), "Appium Android driver is un installing for the second time even full reset is false")
}

func TestForTheNthTime_ShouldFlagNthTimeFollowedByIn(t *testing.T) {
	t.Skip("not yet implemented")
	linttest.AssertSuggestionResult(t, "but somewhat very tired to emphasize for Nth time in the minimap topic", NewForTheNthTime(), "but somewhat very tired to emphasize for the Nth time in the minimap topic")
}

func TestForTheNthTime_ShouldFlagFifthTime(t *testing.T) {
	t.Skip("not yet implemented")
	linttest.AssertSuggestionResult(t, "I see with my naked eyes, no input field, but it says for fifth time there is input field, wtf, hpw to work with this shit?", NewForTheNthTime(), "I see with my naked eyes, no input field, but it says for the fifth time there is input field, wtf, hpw to work with this shit?")
}
