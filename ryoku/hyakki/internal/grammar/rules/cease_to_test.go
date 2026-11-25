package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCeaseTo_SeizeToExist(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The moment you add webbrowser control, all other controls on the Page seize to exist.", NewCeaseTo(), "The moment you add webbrowser control, all other controls on the Page cease to exist.")
}

func TestCeaseTo_SeizeToImpress(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Considering the project is considered feature complete, you seize to impress me with new functionality, that just works.", NewCeaseTo(), "Considering the project is considered feature complete, you cease to impress me with new functionality, that just works.")
}

func TestCeaseTo_SiezeToBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "When do the photons emerging from the Eiffel tower sieze to be copyrighted?", NewCeaseTo(), "When do the photons emerging from the Eiffel tower cease to be copyrighted?")
}

func TestCeaseTo_SeizedToBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Though I love Chrome as a browser, at this particular point GWT Plugin / Chrome combo seized to be an asset and became a liability :(.", NewCeaseTo(), "Though I love Chrome as a browser, at this particular point GWT Plugin / Chrome combo ceased to be an asset and became a liability :(.")
}

func TestCeaseTo_SeizedToFunction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "upgrading Apache and PHP to the latest versions, PHPMyAdmin seized to function", NewCeaseTo(), "upgrading Apache and PHP to the latest versions, PHPMyAdmin ceased to function")
}

func TestCeaseTo_SeizesToBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sure, the sandbox cannot be super restrictive, otherwise the terminal seizes to be useful.", NewCeaseTo(), "Sure, the sandbox cannot be super restrictive, otherwise the terminal ceases to be useful.")
}

func TestCeaseTo_SeizesToWork(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Recently, my Azure OpenAI configuartion seizes to work.", NewCeaseTo(), "Recently, my Azure OpenAI configuartion ceases to work.")
}

func TestCeaseTo_NeverSiezesToAmaze(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Number of ppl in our industry who don’t seem to be aware of the Brooks’ Law never siezes to amaze me.", NewCeaseTo(), "Number of ppl in our industry who don’t seem to be aware of the Brooks’ Law never ceases to amaze me.")
}

func TestCeaseTo_SeizingToBe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "sharing much of their genetics, and seizing to be very similar genetically", NewCeaseTo(), "sharing much of their genetics, and ceasing to be very similar genetically")
}

func TestCeaseTo_SeizingToFunction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Task feature timeout_seconds seizing to function as intended after update", NewCeaseTo(), "Task feature timeout_seconds ceasing to function as intended after update")
}

func TestCeaseTo_SiezingToRespond(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and now i cant save without the game siezing to respond after 30 seconds", NewCeaseTo(), "and now i cant save without the game ceasing to respond after 30 seconds")
}

func TestCeaseTo_DontFlagSeizedToBeMade(t *testing.T) {
	t.Skip("can't clock legit verb yet")
	linttest.AssertNoLints(t, "In order to accomplish this the tasks have their ownership seized to be made the property of the user running the script.", NewCeaseTo())
}

func TestCeaseTo_DontFlagCollateralSeizedToCoverADebt(t *testing.T) {
	t.Skip("can't clock legit verb yet")
	linttest.AssertNoLints(t, "When a borrower's collateral on Chain A is seized to cover a debt on Chain B, Chain A's CrossChainRouter sends a LiquidationSuccess message", NewCeaseTo())
}

func TestCeaseTo_DontFlagWealthSiezedToSavePpl(t *testing.T) {
	t.Skip("can't clock legit verb yet")
	linttest.AssertNoLints(t, "software devlopers, who make vastly more than the average person should have their wealth siezed to save those people?", NewCeaseTo())
}

func TestCeaseTo_DontFlagNutSiezingToTheScrew(t *testing.T) {
	linttest.AssertNoLints(t, "Most of the problem is the nut siezing to the screw.", NewCeaseTo())
}

func TestCeaseTo_TypoForSizes(t *testing.T) {
	t.Skip("can't clock typo")
	linttest.AssertNoLints(t, "Would work, if the body width is already set at 99% of screen size, which makes all possible seizes to be limited to the 99% of screen width", NewCeaseTo())
}

func TestCeaseTo_DontFlagMapsSeizeTo(t *testing.T) {
	linttest.AssertNoLints(t, "Win32ControlUnitMgr maps Seize to SeizeInput(hwnd_, false)", NewCeaseTo())
}

func TestCeaseTo_ExecutesSeizeToTake(t *testing.T) {
	t.Skip("can't clock seize used as noun yet")
	linttest.AssertNoLints(t, "Lender-miner immediately executes seize to take the borrower's collateral.", NewCeaseTo())
}

func TestCeaseTo_CantGrokMaySeizeToAny(t *testing.T) {
	linttest.AssertNoLints(t, "defaults to always-allow when unset, so an unconfigured token may seize to any destination", NewCeaseTo())
}
