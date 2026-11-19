package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestArriveTo_FixWithDestination(t *testing.T) {
	linttest.AssertSuggestionResult(t, "arrive to the destination", NewArriveTo(), "arrive at the destination")
}

func TestArriveTo_FixWithoutDestination(t *testing.T) {
	linttest.AssertSuggestionResult(t, "arrive to", NewArriveTo(), "arrive at")
}

func TestArriveTo_DontFlagWithVerbLemma(t *testing.T) {
	linttest.AssertNoLints(t, "I arrived to find something was wrong.", NewArriveTo())
}

func TestArriveTo_FixFlightsExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Number of flights departing from and arriving to every airport.", NewArriveTo(), "Number of flights departing from and arriving at every airport.")
}

func TestArriveTo_FixVancouverExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We had several days vacation in Vancouver prior to arriving to Seattle", NewArriveTo(), "We had several days vacation in Vancouver prior to arriving in Seattle")
}

func TestArriveTo_FixPacketExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The packet arrives to the client socket correctly as well, according to Wireshark.", NewArriveTo(), "The packet arrives at the client socket correctly as well, according to Wireshark.")
}

func TestArriveTo_FixEmailExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Okay, so, add whatever email address the emails arrive to originally as a System Email under Admin Panel", NewArriveTo(), "Okay, so, add whatever email address the emails arrive at originally as a System Email under Admin Panel")
}

func TestArriveTo_FixGameMenuExample(t *testing.T) {
	linttest.AssertSuggestionResult(t, "God Of War Collection: White screen before arrive to the game menu", NewArriveTo(), "God Of War Collection: White screen before arrive at the game menu")
}

func TestArriveTo_DontFlagCustomerArrives(t *testing.T) {
	linttest.AssertNoLints(t, "whenever a new customer arrives to determine which set they will belong to", NewArriveTo())
}

func TestArriveTo_DontFlagLuxuryBusArrived(t *testing.T) {
	linttest.AssertNoLints(t, "Another luxury bus has arrived to collect arrested protesters chilling on the guardrail.", NewArriveTo())
}

func TestArriveTo_DontFlagSovietParatroopersArrived(t *testing.T) {
	linttest.AssertNoLints(t, "On the morning of 28 December Soviet paratroopers arrived to protect the hotel where he was staying.", NewArriveTo())
}

func TestArriveTo_DontFlagNegotiatorArrives(t *testing.T) {
	linttest.AssertNoLints(t, "US negotiator with North Korea arrives to begin talks.", NewArriveTo())
}
