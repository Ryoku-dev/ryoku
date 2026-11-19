package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestPayForPrice_PayForFees(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Accounts having BNB accounts use this to pay for fees", NewPayForPrice(), "Accounts having BNB accounts use this to pay fees")
}

func TestPayForPrice_PayForABill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "being instructed to either pay for a bill, an expense, or a gift", NewPayForPrice(), "being instructed to either pay a bill, an expense, or a gift")
}

func TestPayForPrice_PayForACheck(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Having to pay for a check is 'paying for what you use'.", NewPayForPrice(), "Having to pay a check is 'paying for what you use'.")
}

func TestPayForPrice_PayForAFee(t *testing.T) {
	linttest.AssertSuggestionResult(t, "then the account needs to pay for a fee of X", NewPayForPrice(), "then the account needs to pay a fee of X")
}

func TestPayForPrice_PayForTheBill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Cannot pay for the bill and cannot delete the bill", NewPayForPrice(), "Cannot pay the bill and cannot delete the bill")
}

func TestPayForPrice_PayForTheCheck(t *testing.T) {
	linttest.AssertSuggestionResult(t, "should be able to ascertain if he will pay for the check or not", NewPayForPrice(), "should be able to ascertain if he will pay the check or not")
}

func TestPayForPrice_PayingForTheBill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Maybe some people in his family weren't paying for the bill out of their own account", NewPayForPrice(), "Maybe some people in his family weren't paying the bill out of their own account")
}

func TestPayForPrice_PayingForTheCheck(t *testing.T) {
	linttest.AssertSuggestionResult(t, "how does everyone deal with paying for the check or the bill of any activity", NewPayForPrice(), "how does everyone deal with paying the check or the bill of any activity")
}

func TestPayForPrice_PaysForTheBill(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I now understand that when a man pays for the bill, he has clear intentions", NewPayForPrice(), "I now understand that when a man pays the bill, he has clear intentions")
}

func TestPayForPrice_DontFlagPayingForFees(t *testing.T) {
	t.Skip("Detecting this pattery will require more thought")
	linttest.AssertNoLints(t, "it will specify how much you need and how much you're paying for fees", NewPayForPrice())
}

func TestPayForPrice_DontFlagPayForRateLimitIncrease(t *testing.T) {
	linttest.AssertNoLints(t, "Can we pay for a rate limit increase?", NewPayForPrice())
}

func TestPayForPrice_DontFlagPayForCheckInOrOut(t *testing.T) {
	linttest.AssertNoLints(t, "Most normally, tenants pay for a check-in and landlords pay for the check-out.", NewPayForPrice())
}

func TestPayForPrice_DontFlagPayForThePriceIncrease(t *testing.T) {
	linttest.AssertNoLints(t, "Unless you want to pay for the price increase and have no issue with the owner, then go for signing up for a sourcehut account.", NewPayForPrice())
}

func TestPayForPrice_DontFlagPayingForCommaPrices(t *testing.T) {
	linttest.AssertNoLints(t, "If everyone used all of the tokens they thought they were paying for, prices would explode.", NewPayForPrice())
}
