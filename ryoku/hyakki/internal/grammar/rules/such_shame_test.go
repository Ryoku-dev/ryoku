package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSuchShame_ItsApostrope(t *testing.T) {
	linttest.AssertSuggestionResult(t, "it's such shame that for large datasets the usage of this wonderful package becomes not pracitcal", NewSuchShame(), "it's such a shame that for large datasets the usage of this wonderful package becomes not pracitcal")
}

func TestSuchShame_ItIs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It is such shame that the TV adaptation changed this fundamental part of the story for no reason", NewSuchShame(), "It is such a shame that the TV adaptation changed this fundamental part of the story for no reason")
}

func TestSuchShame_DontFlagThereWas(t *testing.T) {
	linttest.AssertNoLints(t, "but I think there was such shame in that at the time most conversations on the subject would be ...", NewSuchShame())
}

func TestSuchShame_DontFlagThereIs(t *testing.T) {
	linttest.AssertNoLints(t, "There is such shame that seems to come from not being able to handle your house.", NewSuchShame())
}

func TestSuchShame_ItWas(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It was such shame to lose such a talented composer.", NewSuchShame(), "It was such a shame to lose such a talented composer.")
}

func TestSuchShame_ItsNoApostrophe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Its such shame to because this update had such potential", NewSuchShame(), "Its such a shame to because this update had such potential")
}

func TestSuchShame_DontFlagTheirWas(t *testing.T) {
	linttest.AssertNoLints(t, "Partially because their was such shame among black people if they had darker skin. ", NewSuchShame())
}
