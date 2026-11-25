package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestMostOfTheTimes_HangsForever(t *testing.T) {
	linttest.AssertSuggestionResult(t, "restic backup hangs forever most of the times · Issue #2834", NewMostOfTheTimes(), "restic backup hangs forever most of the time · Issue #2834")
}

func TestMostOfTheTimes_OptionsAreIgnored(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but other options like device and options are ignored most of the times", NewMostOfTheTimes(), "but other options like device and options are ignored most of the time")
}

func TestMostOfTheTimes_Parenthesized(t *testing.T) {
	linttest.AssertSuggestionResult(t, "prompted html code gets (most of the times) read by copilot but is not displayed.", NewMostOfTheTimes(), "prompted html code gets (most of the time) read by copilot but is not displayed.")
}

func TestMostOfTheTimes_ICantPlay(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I cannot get the version 1.0 without c so I cant play a lot of the times with other people", NewMostOfTheTimes(), "I cannot get the version 1.0 without c so I cant play a lot of the time with other people")
}
