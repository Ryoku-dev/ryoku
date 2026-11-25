package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestComplainAsNoun_GotAComplain(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Got a complain abount 'direct reference to field' even using the method", NewComplainAsNoun(), "Got a complaint abount 'direct reference to field' even using the method")
}

func TestComplainAsNoun_LodgeAComplainAndHisComplain(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To solve most of these things, requires the one to lodge a complain as well as follow up on his complain to ensure it has been resolved.", NewComplainAsNoun(), "To solve most of these things, requires the one to lodge a complaint as well as follow up on his complaint to ensure it has been resolved.")
}

func TestComplainAsNoun_TheComplains(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This platform provides a all in one solution for the complaining and solving of the complains through public forums.", NewComplainAsNoun(), "This platform provides a all in one solution for the complaining and solving of the complaints through public forums.")
}

func TestComplainAsNoun_MyComplain(t *testing.T) {
	linttest.AssertSuggestionResult(t, "My complain about: esp-idf/examples/peripherals/lcd/i2c_oled", NewComplainAsNoun(), "My complaint about: esp-idf/examples/peripherals/lcd/i2c_oled")
}

func TestComplainAsNoun_YourComplain(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In a customer, you just have to submit your complain.", NewComplainAsNoun(), "In a customer, you just have to submit your complaint.")
}

func TestComplainAsNoun_RaiseAComplain(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I raise a complain about the documentation.", NewComplainAsNoun(), "I raise a complaint about the documentation.")
}

func TestComplainAsNoun_ManyComplains(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Many complains were raised about the documentation.", NewComplainAsNoun(), "Many complaints were raised about the documentation.")
}

func TestComplainAsNoun_DontFlagTheComplainFlag(t *testing.T) {
	linttest.AssertNoLints(t, "The idea is to only have the complain flag defined in the manifest", NewComplainAsNoun())
}
