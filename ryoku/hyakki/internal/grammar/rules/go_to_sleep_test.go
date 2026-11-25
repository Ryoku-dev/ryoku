package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestGoToSleep_FixGoToSleep(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I changed sleep mode to s3 and now my computer cant go into sleep, it just shut down.", NewGoToSleep(), "I changed sleep mode to s3 and now my computer cant go to sleep, it just shut down.")
}

func TestGoToSleep_FixGoesIntoSleep(t *testing.T) {
	linttest.AssertSuggestionResult(t, "On macbook with working Internet, close the lid so that mac goes into sleep.", NewGoToSleep(), "On macbook with working Internet, close the lid so that mac goes to sleep.")
}

func TestGoToSleep_FixGoingIntoSleep(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Either pressing the power button on top of the Deck or going into Sleep from the Launcher freezes the Deck", NewGoToSleep(), "Either pressing the power button on top of the Deck or going to Sleep from the Launcher freezes the Deck")
}

func TestGoToSleep_FixGoneIntoSleep(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The last time this happened, it was after the computer had gone into sleep when I was away from my desk", NewGoToSleep(), "The last time this happened, it was after the computer had gone to sleep when I was away from my desk")
}

func TestGoToSleep_FixWentIntoSleep(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Broken UI rendering after macOS went into sleep", NewGoToSleep(), "Broken UI rendering after macOS went to sleep")
}

func TestGoToSleep_DontFlagGoIntoSleepState(t *testing.T) {
	linttest.AssertNoLints(t, "When using Jobber, they either go into sleep state while still retaining memory", NewGoToSleep())
}

func TestGoToSleep_DontFlagGoesIntoSleepMode(t *testing.T) {
	linttest.AssertNoLints(t, "When the computer goes into sleep mode or the screen turns off, the program reports an error and closes.", NewGoToSleep())
}

func TestGoToSleep_DontFlagWentIntoSleepSuspend(t *testing.T) {
	linttest.AssertNoLints(t, "The power LED remains ON even when it presumably went into sleep/suspend mode", NewGoToSleep())
}

func TestGoToSleep_DontFlagGoesIntoSleepSlashHibernate(t *testing.T) {
	linttest.AssertNoLints(t, "When the laptop goes into sleep/hibernate mode and they come back to it, it shows Netbird is connected", NewGoToSleep())
}

func TestGoToSleep_DontFlagGoesIntoSleepSlashHibernation(t *testing.T) {
	linttest.AssertNoLints(t, "Whenever system goes into sleep/hibernation and comes back, sometimes the driver will get \"stuck\"", NewGoToSleep())
}
