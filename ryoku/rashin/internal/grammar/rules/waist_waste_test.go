package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWaistWaste_ItIsAWaistOfSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The problem is, I think it is a waist of space pushing to remote all the software just because I have changed some files.", NewWaistWaste(), "The problem is, I think it is a waste of space pushing to remote all the software just because I have changed some files.")
}

func TestWaistWaste_ThisIsAWaistOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This is a waist of Bs resources.", NewWaistWaste(), "This is a waste of Bs resources.")
}

func TestWaistWaste_IsAWaistOfSpace(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The whole height takes only 19 rows, I think this is a waist of space", NewWaistWaste(), "The whole height takes only 19 rows, I think this is a waste of space")
}

func TestWaistWaste_IsAWaistOfTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Digging a hole with a toothpick is a waist of time.", NewWaistWaste(), "Digging a hole with a toothpick is a waste of time.")
}

func TestWaistWaste_DontFlagAtStart(t *testing.T) {
	linttest.AssertNoLints(t, "Is a waist of 46 inches around the belly button unhealthy??", NewWaistWaste())
}

func TestWaistWaste_WaistOfMoney(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Copilot ==Waist of money. It's full of bugs", NewWaistWaste(), "Copilot ==Waste of money. It's full of bugs")
}

func TestWaistWaste_HugeWaistOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A huge waist of time. If the goal cannot be fullfulled, the model will not fallback to debugging", NewWaistWaste(), "A huge waste of time. If the goal cannot be fullfulled, the model will not fallback to debugging")
}

func TestWaistWaste_CompleteWaist(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but what a complete waist of time to be stuck in a job for 8 hours a day", NewWaistWaste(), "but what a complete waste of time to be stuck in a job for 8 hours a day")
}

func TestWaistWaste_TotalWaist(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Total Waist Of Money & Time !! NOT RECOMMENDED !!!", NewWaistWaste(), "Total Waste Of Money & Time !! NOT RECOMMENDED !!!")
}

func TestWaistWaste_AWaistOfYourTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nevertheless I learned some things form your answer so it wasn't a waist of your time.", NewWaistWaste(), "Nevertheless I learned some things form your answer so it wasn't a waste of your time.")
}

func TestWaistWaste_JustAWaistOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The \"temporary\" bitmap is just a waist of CPU resources.", NewWaistWaste(), "The \"temporary\" bitmap is just a waste of CPU resources.")
}
