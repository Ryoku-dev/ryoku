package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestLeavingInDroves_Jobs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Jobs are leaving in drones out of the country.", NewLeavingInDroves(), "Jobs are leaving in droves out of the country.")
}

func TestLeavingInDroves_Wealthy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yet they are amongst some of the loudest promoters of the notion that the wealthy will leave in drones.", NewLeavingInDroves(), "Yet they are amongst some of the loudest promoters of the notion that the wealthy will leave in droves.")
}

func TestLeavingInDroves_Employees(t *testing.T) {
	linttest.AssertSuggestionResult(t, "consider the damage that will be caused when employees start leaving in drones as if we didn't have enough people leave already in the past 2 years.", NewLeavingInDroves(), "consider the damage that will be caused when employees start leaving in droves as if we didn't have enough people leave already in the past 2 years.")
}

func TestLeavingInDroves_Pastors(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Why Are Nigerian Pastors Leaving In Drones?", NewLeavingInDroves(), "Why Are Nigerian Pastors Leaving In Droves?")
}

func TestLeavingInDroves_Left(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have remained at the place while my coworkers have left in drones.", NewLeavingInDroves(), "I have remained at the place while my coworkers have left in droves.")
}
