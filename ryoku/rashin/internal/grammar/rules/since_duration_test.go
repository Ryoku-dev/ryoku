package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSinceDuration_CatchesSpelled(t *testing.T) {
	linttest.AssertLintCount(t, "I have been waiting since two hours.", NewSinceDuration(), 1)
}

func TestSinceDuration_PermitsSpelledWithAgo(t *testing.T) {
	linttest.AssertNoLints(t, "I have been waiting since two hours ago.", NewSinceDuration())
}

func TestSinceDuration_CatchesNumerals(t *testing.T) {
	linttest.AssertLintCount(t, "I have been waiting since 2 hours.", NewSinceDuration(), 1)
}

func TestSinceDuration_PermitsNumeralsWithAgo(t *testing.T) {
	linttest.AssertNoLints(t, "I have been waiting since 2 hours ago.", NewSinceDuration())
}

func TestSinceDuration_CorrectWithoutIssues(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm running v2.2.1 on bare metal (no docker, vm) since two weeks without issues.", NewSinceDuration(), "I'm running v2.2.1 on bare metal (no docker, vm) for two weeks without issues.")
}

func TestSinceDuration_CorrectAnythingBack(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have not heard anything back since three months.", NewSinceDuration(), "I have not heard anything back for three months.")
}

func TestSinceDuration_CorrectGetDone(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am trying to get this done since two days, someone please help.", NewSinceDuration(), "I am trying to get this done for two days, someone please help.")
}

func TestSinceDuration_CorrectDeprecated(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This project is now officially deprecated, since I worked with virtualabs on the next version of Mirage since three years now: an ecosystem of tools named WHAD.", NewSinceDuration(), "This project is now officially deprecated, since I worked with virtualabs on the next version of Mirage for three years now: an ecosystem of tools named WHAD.")
}

func TestSinceDuration_CorrectSame(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Same! Since two days.", NewSinceDuration(), "Same! For two days.")
}

func TestSinceDuration_CorrectWhatChanged(t *testing.T) {
	linttest.AssertSuggestionResult(t, "What changed since two weeks?", NewSinceDuration(), "What changed since two weeks ago?")
}

func TestSinceDuration_CorrectWithPeriod(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have been waiting since two hours.", NewSinceDuration(), "I have been waiting since two hours ago.")
}

func TestSinceDuration_CorrectWithExclamation(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have been waiting since two hours!", NewSinceDuration(), "I have been waiting since two hours ago!")
}

func TestSinceDuration_CorrectWithQuestionMark(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Have you been waiting since two hours?", NewSinceDuration(), "Have you been waiting for two hours?")
}

func TestSinceDuration_CorrectWithComma(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Since two days, I have been trying to get this done.", NewSinceDuration(), "For two days, I have been trying to get this done.")
}

func TestSinceDuration_CorrectForTitleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Since 45 Minutes I See The Following Picture In The Terminal.", NewSinceDuration(), "For 45 Minutes I See The Following Picture In The Terminal.")
}

func TestSinceDuration_CorrectForAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "STOPPED SINCE 12 HOURS WITH EXIT CODE 0", NewSinceDuration(), "STOPPED FOR 12 HOURS WITH EXIT CODE 0")
}

func TestSinceDuration_CorrectAgoTitleCase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It Is In Development Since Two Years.", NewSinceDuration(), "It Is In Development Since Two Years Ago.")
}

func TestSinceDuration_CorrectAgoAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "BUG: SINCE 6 MONTHS UNLOAD CHECKPOINT", NewSinceDuration(), "BUG: SINCE 6 MONTHS AGO UNLOAD CHECKPOINT")
}

func TestSinceDuration_NotYetHandled(t *testing.T) {
	t.Skip("We can't yet handle modifiers like 'over'. Plus it doesn't work with 'ago'.")
	linttest.AssertSuggestionResult(t, "It's an asked feature since over 9 years", NewSinceDuration(), "It's an asked feature for over 9 years.")
}

func TestSinceDuration_NotYetHandled2(t *testing.T) {
	t.Skip("We can't yet handle modifiers like 'more than'. Plus it doesn't work with 'ago'.")
	linttest.AssertSuggestionResult(t, "It's an asked feature since more than 9 years", NewSinceDuration(), "It's an asked feature for more than 9 years.")
}

func TestSinceDuration_NotYetHandled3(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I use a Wacom Cintiq 27QHDT since several years on Linux", NewSinceDuration(), "I use a Wacom Cintiq 27QHDT for several years on Linux")
}

func TestSinceDuration_IgnoreSinceYearsOld(t *testing.T) {
	linttest.AssertNoLints(t, "I've been coding since 11 years old and I'm now 57", NewSinceDuration())
}
