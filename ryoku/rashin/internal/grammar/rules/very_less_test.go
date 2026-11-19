package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestVeryLess_VeryToMuch(t *testing.T) {
	linttest.AssertSuggestionResult(t, "here is a simple way to do it with very less coding ... ;)", NewVeryLess(), "here is a simple way to do it with much less coding ... ;)")
}

func TestVeryLess_VeryToFar(t *testing.T) {
	linttest.AssertSuggestionResult(t, "algorithm for processing large datasets with very less pre-configuration", NewVeryLess(), "algorithm for processing large datasets with far less pre-configuration")
}

func TestVeryLess_VeryToALot(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Also the gpu memory usage is very less.", NewVeryLess(), "Also the gpu memory usage is a lot less.")
}

func TestVeryLess_TooLess(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Too much code for too less, maybe too many requests for only 10$.. who knows?!", NewVeryLess(), "Too much code for too little, maybe too many requests for only 10$.. who knows?!")
}

func TestVeryLess_TooLessInfo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "codex showing too less information when he is think chain.", NewVeryLess(), "codex showing too little information when he is think chain.")
}

func TestVeryLess_VeryLessTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Connection also takes very less time.", NewVeryLess(), "Connection also takes a lot less time.")
}

func TestVeryLess_TooLessSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Auto-abbreviate text if there is too less space", NewVeryLess(), "Auto-abbreviate text if there is too little space")
}

func TestVeryLess_CantFixIsQuiteLess(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Based on the result, the number of genes in the braker.gtf from braker3 is quite less (25943) than braker2 (48396).", NewVeryLess(), "Based on the result, the number of genes in the braker.gtf from braker3 is quite a lot less (25943) than braker2 (48396).")
}

func TestVeryLess_CantFixBecomeQuiteLess(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But those things became quite less important compared to having a consistent formatting style across a team", NewVeryLess(), "But those things became quite a lot less important compared to having a consistent formatting style across a team")
}

func TestVeryLess_DontFlagComparablyLess(t *testing.T) {
	linttest.AssertNoLints(t, "This is comparably less retraining, but the incorrect keypresses are the bigger issue.", NewVeryLess())
}

func TestVeryLess_CantFixVeryLessCcr(t *testing.T) {
	t.Skip("May be beyond the typical Germlish usage? Requires 'very low/small' etc.")
	linttest.AssertSuggestionResult(t, "[Bug]: Disabled primary button having very less color contrast ratio", NewVeryLess(), "[Bug]: Disabled primary button having very low color contrast ratio")
}

func TestVeryLess_CantFixLogTooLess(t *testing.T) {
	t.Skip("Too hard to parse. Does it mean the log is too small?")
	linttest.AssertNoLints(t, "build error log too less to find the key position", NewVeryLess())
}

func TestVeryLess_CantFixPrettyLess(t *testing.T) {
	t.Skip("Proper fix would be something like 'pretty few items' or 'a pretty small number of items', etc.")
	linttest.AssertSuggestionResult(t, "I have pretty less number of items in x-axis", NewVeryLess(), "I have pretty few items in x-axis")
}
