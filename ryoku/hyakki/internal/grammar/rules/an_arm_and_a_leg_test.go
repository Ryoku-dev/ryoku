package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAnArmAndALeg_PayingALegAndAnArm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am sick and tired of paying a leg and an arm in NYC taxes.", NewAnArmAndALeg(), "I am sick and tired of paying an arm and a leg in NYC taxes.")
}

func TestAnArmAndALeg_ChargeLegAndArm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "they just “fix things” but charge leg and arm for their services", NewAnArmAndALeg(), "they just “fix things” but charge an arm and a leg for their services")
}

func TestAnArmAndALeg_ChargingArmAndLeg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "higher education institute charging arm and leg for people to network", NewAnArmAndALeg(), "higher education institute charging an arm and a leg for people to network")
}

func TestAnArmAndALeg_CostAnArmAndLeg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Professional workstations have always cost an arm and leg - both arms and both legs usually", NewAnArmAndALeg(), "Professional workstations have always cost an arm and a leg - both arms and both legs usually")
}

func TestAnArmAndALeg_PayArmAndALeg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "than continuing to pay arm and a leg for eternity", NewAnArmAndALeg(), "than continuing to pay an arm and a leg for eternity")
}

func TestAnArmAndALeg_CostsArmAndLeg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Using your own analogy car repair costs arm and leg with the dealer vs diy", NewAnArmAndALeg(), "Using your own analogy car repair costs an arm and a leg with the dealer vs diy")
}

func TestAnArmAndALeg_ChargeAnArmAndLeg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They manage to charge an arm and leg for regular things that a lot of the world makes", NewAnArmAndALeg(), "They manage to charge an arm and a leg for regular things that a lot of the world makes")
}

func TestAnArmAndALeg_DontFlagRemoval(t *testing.T) {
	linttest.AssertNoLints(t, "As in, they require removal of an arm and a leg.", NewAnArmAndALeg())
}
