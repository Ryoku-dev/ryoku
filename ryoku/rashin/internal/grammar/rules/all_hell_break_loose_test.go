package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAllHellBreakLoose_FixBreak(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Just run around planting satchels charges while you're alive and let all hell break out when you die.", NewAllHellBreakLoose(), "Just run around planting satchels charges while you're alive and let all hell break loose when you die.")
}

func TestAllHellBreakLoose_FixBreaks(t *testing.T) {
	linttest.AssertSuggestionResult(t, "we upgraded 2 months ago, and now we upgrade prod and all hell breaks out", NewAllHellBreakLoose(), "we upgraded 2 months ago, and now we upgrade prod and all hell breaks loose")
}

func TestAllHellBreakLoose_FixBreaking(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Next scene, all hell breaking out!", NewAllHellBreakLoose(), "Next scene, all hell breaking loose!")
}

func TestAllHellBreakLoose_FixBroke(t *testing.T) {
	linttest.AssertSuggestionResult(t, "this time going from 1.3.4 to 1.4.2 and all hell broke out", NewAllHellBreakLoose(), "this time going from 1.3.4 to 1.4.2 and all hell broke loose")
}

func TestAllHellBreakLoose_FixBroken(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I’m using silenced weapons but as soon as I fire it’s all hell broken out.", NewAllHellBreakLoose(), "I’m using silenced weapons but as soon as I fire it’s all hell broken loose.")
}
