package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestFallBelow_Fix40Feet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "However, there wasn't, causing Gurman to fall 40 feet below.", NewFallBelow(), "However, there wasn't, causing Gurman to fall 40 feet.")
}

func TestFallBelow_Fix130Feet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Woman who died in Brazil Bungee jump tragedy was alive after falling 130 feet below", NewFallBelow(), "Woman who died in Brazil Bungee jump tragedy was alive after falling 130 feet")
}

func TestFallBelow_Fix300Feet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The young trekker was stuck in Bramhagiri Rocks in Nandi Hills after slipping and falling 300 feet below, said PRO Defence.", NewFallBelow(), "The young trekker was stuck in Bramhagiri Rocks in Nandi Hills after slipping and falling 300 feet, said PRO Defence.")
}

func TestFallBelow_FixFell20Feet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But while jumping, unfortunately, the rope broke and he fell 20 feet below.", NewFallBelow(), "But while jumping, unfortunately, the rope broke and he fell 20 feet.")
}

func TestFallBelow_Fix30Feet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "“Mike and Annie call for the lift to stop, but the chair did not stop. Instead it carried her up, and Annie fell 30 feet below, landing on her skis,” Miller’s attorney, Bruce Braley, said.", NewFallBelow(), "“Mike and Annie call for the lift to stop, but the chair did not stop. Instead it carried her up, and Annie fell 30 feet, landing on her skis,” Miller’s attorney, Bruce Braley, said.")
}

func TestFallBelow_Fix50Feet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SUV crashes through bridge guardrail, falls 50 feet below.", NewFallBelow(), "SUV crashes through bridge guardrail, falls 50 feet.")
}

func TestFallBelow_FixFalls20Feet(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Motorcyclist crashes off edge of Ballard Bridge, falls 20 feet below", NewFallBelow(), "Motorcyclist crashes off edge of Ballard Bridge, falls 20 feet")
}

func TestFallBelow_FixFell20MetersAnd(t *testing.T) {
	linttest.AssertSuggestionResult(t, "On the 18th, a man fell 20 meters below and died in Songnisan National Park.", NewFallBelow(), "On the 18th, a man fell 20 meters and died in Songnisan National Park.")
}

func TestFallBelow_CantFixSeveralMetresBelow(t *testing.T) {
	t.Skip("We can't detect when the chunk does not end after 'below'")
	linttest.AssertSuggestionResult(t, "A large billboard on Krystal Point building fell several metres below damaging several vehicles.", NewFallBelow(), "A large billboard on Krystal Point building fell several metres damaging several vehicles.")
}

func TestFallBelow_CantFixFall40Feet(t *testing.T) {
	t.Skip("We can't detect when the chunk does not end after 'below'")
	linttest.AssertSuggestionResult(t, "Speeding Bike Collides with Railing, 3 Riders Fall 40 Feet Below in Visakhapatnam.", NewFallBelow(), "Speeding Bike Collides with Railing, 3 Riders Fall 40 Feet in Visakhapatnam.")
}
