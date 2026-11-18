package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestAmountsFor_CorrectsThatAmountsForToAmountsToEntireValue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Skyler stated the car wash is worth close to $800k, that amounts for the entire value of the company", NewAmountsFor(), "Skyler stated the car wash is worth close to $800k, that amounts to the entire value of the company")
}

func TestAmountsFor_CorrectsThatAmountsForToAmountsToPercent(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Together, that amounts for 1157 calls or 60% of the failures.", NewAmountsFor(), "Together, that amounts to 1157 calls or 60% of the failures.")
}

func TestAmountsFor_CorrectsThatAmountsForToAccountsForSettingUp(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One solution to this would be to have separate controllers but the amount of code that amounts for setting up, processing and calling", NewAmountsFor(), "One solution to this would be to have separate controllers but the amount of code that accounts for setting up, processing and calling")
}

func TestAmountsFor_CorrectsWhichAmountsForToAccountsFor16k(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It has an offset of 0xC000 which amounts for the 16k.", NewAmountsFor(), "It has an offset of 0xC000 which accounts for the 16k.")
}

func TestAmountsFor_CorrectsThisAmountsForToAccountsForLargePart(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm pretty sure that this amounts for a large part of the speed I gained when typing, in addition to touch-typing.", NewAmountsFor(), "I'm pretty sure that this accounts for a large part of the speed I gained when typing, in addition to touch-typing.")
}

func TestAmountsFor_CorrectsTheyAmountForToAmountTo16kb(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it is obvious that the messages are being held \"somewhere\" until they amount for 16kB and then the whole lot come at once.", NewAmountsFor(), "it is obvious that the messages are being held \"somewhere\" until they amount to 16kB and then the whole lot come at once.")
}

func TestAmountsFor_CorrectsWhichAmountsForToAmountsTo10Minutes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "set a small TTL for your hostname (like 600 which amounts for 10 minutes).", NewAmountsFor(), "set a small TTL for your hostname (like 600 which amounts to 10 minutes).")
}

func TestAmountsFor_CorrectsItAmountsForToAmountsToRedefinition(t *testing.T) {
	linttest.AssertSuggestionResult(t, "included for convenience to get a Lorentz invariant result (it amounts for a redefinition of ap).", NewAmountsFor(), "included for convenience to get a Lorentz invariant result (it amounts to a redefinition of ap).")
}

func TestAmountsFor_CorrectsTheyAmountForToAmountToNothing(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Matter and antimatter are spread throughout the Universe, and in total, they amount for nothing", NewAmountsFor(), "Matter and antimatter are spread throughout the Universe, and in total, they amount to nothing")
}

func TestAmountsFor_WouldAmountForToAmountToApiRequests(t *testing.T) {
	linttest.AssertSuggestionResult(t, "10% of 6,782,091 would amount for 678,209 API requests", NewAmountsFor(), "10% of 6,782,091 would amount to 678,209 API requests")
}

func TestAmountsFor_WillAmountForToAmountToRelationships(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Consider this statistic from Gartner, that artificial intelligence will amount for 85% of customer relationships by 2020.", NewAmountsFor(), "Consider this statistic from Gartner, that artificial intelligence will amount to 85% of customer relationships by 2020.")
}

func TestAmountsFor_ShouldAmountForToAmountToHalfPack(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It doesn't seem realistic that this single elite should amount for half the pack", NewAmountsFor(), "It doesn't seem realistic that this single elite should amount to half the pack")
}

func TestAmountsFor_CanAmountForToAmountToDrawCalls(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That can amount for a lot of draw calls and work for the engine to cull. ", NewAmountsFor(), "That can amount to a lot of draw calls and work for the engine to cull. ")
}
