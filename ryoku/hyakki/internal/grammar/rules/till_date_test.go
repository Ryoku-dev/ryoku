package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTillDate_FixTillDateDoubleL(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This repository contains all my REXX codes till date.", NewTillDate(dict.American), "This repository contains all my REXX codes to date.")
}

func TestTillDate_FixTillDateSingleL(t *testing.T) {
	linttest.AssertSuggestionResult(t, "streams recorded on the platform from 1930 til date", NewTillDate(dict.American), "streams recorded on the platform from 1930 to date")
}

func TestTillDate_DontFlagTilDateApostropheBeforeSingleL(t *testing.T) {
	linttest.AssertNoLints(t, "A good 'til date (GTD) order is an instruction to buy or sell an asset that remains open until a future date specified by the trader.", NewTillDate(dict.American))
}

func TestTillDate_DontFlagGoodTilDateApostropheAfterSingleL(t *testing.T) {
	linttest.AssertNoLints(t, "Can someone give me some information as to what the Good Til' date means in the context of buying shares?", NewTillDate(dict.American))
}

func TestTillDate_DontFlagGoodTillDateDoubleLAllCaps(t *testing.T) {
	linttest.AssertNoLints(t, "TERMS AND CONDITIONS FOR GOOD TILL DATE (GTD) ORDERS.", NewTillDate(dict.American))
}

func TestTillDate_DontFlagValidTillDate(t *testing.T) {
	linttest.AssertNoLints(t, "The quotation states: \"Good till date (GTD) orders are valid until a specified date.\"", NewTillDate(dict.American))
}
