package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFirstAidKit_CorrectsFirstAidKid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A first aid kid is a collection of medical supplies.", NewFirstAidKit(), "A first aid kit is a collection of medical supplies.")
}

func TestFirstAidKit_CorrectsStarterKid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Check the starter kid before proceeding.", NewFirstAidKit(), "Check the starter kit before proceeding.")
}

func TestFirstAidKit_CorrectsTravelKid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Pack your travel kid for the trip.", NewFirstAidKit(), "Pack your travel kit for the trip.")
}

func TestFirstAidKit_CorrectsToolKid(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Don't forget the tool kid for assembly.", NewFirstAidKit(), "Don't forget the tool kit for assembly.")
}

func TestFirstAidKit_DoesNotFlagKidInOtherContexts(t *testing.T) {
	linttest.AssertLintCount(t, "The kid ran through the aid station.", NewFirstAidKit(), 0)
}
