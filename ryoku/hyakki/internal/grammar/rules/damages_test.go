package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestDamages_FixRobustAgainstDamagesByPrevPreposition(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Flow networks robust against damages are simple model networks described in a series of publications by Kaluza et al.", NewDamages(), "Flow networks robust against damage are simple model networks described in a series of publications by Kaluza et al.")
}

func TestDamages_FixVehicleDamagesOnACarByFallThrough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "POC to select vehicle damages on a car and mark the severity - sudheeshcm/vehicle-damage-selector.", NewDamages(), "POC to select vehicle damage on a car and mark the severity - sudheeshcm/vehicle-damage-selector.")
}

func TestDamages_FixDamagesOnMangoes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a web application that detects damages on mangoes using a TensorFlow model with Django as the frontend framework", NewDamages(), "This is a web application that detects damage on mangoes using a TensorFlow model with Django as the frontend framework")
}

func TestDamages_FixTypesOfDamagesOfRoads(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Detecting different types of damages of roads like cracks and potholes for the given image/video of the road.", NewDamages(), "Detecting different types of damage of roads like cracks and potholes for the given image/video of the road.")
}

func TestDamages_IgnoreDamagesInLostChanceCases(t *testing.T) {
	linttest.AssertNoLints(t, "Code used for calculating damages in lost chance cases.", NewDamages())
}

func TestDamages_IgnoreClaimForDamages(t *testing.T) {
	linttest.AssertNoLints(t, "Where the dispute involves a claim for damages in respect of a motor accident for cost of rental of a replacement vehicle", NewDamages())
}

func TestDamages_IgnorePayDamages(t *testing.T) {
	linttest.AssertNoLints(t, `Under this section, the Commercial Contributor would have to
            defend claims against the other Contributors related to those
            performance claims and warranties, and if a court requires any other
            Contributor to pay any damages as a result, the Commercial Contributor
            must pay those damages.`, NewDamages())
}

func TestDamages_IgnoreDamagesThem(t *testing.T) {
	linttest.AssertNoLints(t, "Profiles pb's and damages them when their runtime goes over a set value - sirhamsteralot/HaE-PBLimiter.", NewDamages())
}

func TestDamages_IgnoreDamagesFirefox(t *testing.T) {
	linttest.AssertNoLints(t, "Opening Wayland-native terminal damages Firefox", NewDamages())
}

func TestDamages_IgnoreDamagesUnderlayingWindows(t *testing.T) {
	linttest.AssertNoLints(t, "Open File Requester damages underlaying windows when moved", NewDamages())
}

func TestDamages_IgnoreEstimateTheDamagesAndTheDamagesResult(t *testing.T) {
	t.Skip("too close to call for now")
	linttest.AssertNoLints(t, "The goal is to estimate the damages of each link in the Graph object using the Damages result (estimating the damages for each segment of a Network).", NewDamages())
}

func TestDamages_IgnoreDamagesInference(t *testing.T) {
	t.Skip("too close to call for now")
	linttest.AssertNoLints(t, "This repository contains code to conduct statistical inference in cartel damages estimation. It will be updated to include a Stata .do file which approximates the standard error of total damages from a fixed effects panel data model, using the delta method.", NewDamages())
}

func TestDamages_IgnoreReceivedErrors(t *testing.T) {
	t.Skip("too close to call for now")
	linttest.AssertNoLints(t, "Financial damages caused by received errors $$$$.", NewDamages())
}

func TestDamages_IgnoreAssetLevelDamages(t *testing.T) {
	t.Skip("too close to call for now")
	linttest.AssertNoLints(t, "It would be useful to be able to see asset-level damages after running FDA 2.0.", NewDamages())
}

func TestDamages_IgnoreItDamagesTheEnvironment(t *testing.T) {
	linttest.AssertNoLints(t, "it damages the environment", NewDamages())
}
