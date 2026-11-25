package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestRackOnesBrain_WrackingTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wracking my brain", NewRackOnesBrain(), "Racking my brain")
}

func TestRackOnesBrain_WreckingTitlecase(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Wrecking my brain", NewRackOnesBrain(), "Racking my brain")
}

func TestRackOnesBrain_WrackedAllcaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "WRACKED MY BRAIN", NewRackOnesBrain(), "RACKED MY BRAIN")
}

func TestRackOnesBrain_WreckedAllcaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "WRECKED MY BRAIN", NewRackOnesBrain(), "RACKED MY BRAIN")
}

func TestRackOnesBrain_WrackLower(t *testing.T) {
	linttest.AssertSuggestionResult(t, "wrack my brain", NewRackOnesBrain(), "rack my brain")
}

func TestRackOnesBrain_WreckLower(t *testing.T) {
	linttest.AssertSuggestionResult(t, "wreck my brain", NewRackOnesBrain(), "rack my brain")
}

func TestRackOnesBrain_WrackingMyAdjSg(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I've been wracking my tiny brain re: Parcelables, ContentValues, and inserting into SQLite", NewRackOnesBrain(), "I've been racking my tiny brain re: Parcelables, ContentValues, and inserting into SQLite")
}

func TestRackOnesBrain_WrackingMyPl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I write code and enjoy wracking my brains on research problems.", NewRackOnesBrain(), "I write code and enjoy racking my brains on research problems.")
}
