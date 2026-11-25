package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestWithOpenArms_FixWelcome(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Contributions to this project are welcome with open arms!", NewWithOpenArms(), "Contributions to this project are welcomed with open arms!")
}

func TestWithOpenArms_FixOpened(t *testing.T) {
	linttest.AssertSuggestionResult(t, "This PR will be welcomed with opened arms <3.", NewWithOpenArms(), "This PR will be welcomed with open arms <3.")
}

func TestWithOpenArms_FixArm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "there are no maxis and Vitalk is welcomed with open arm in to building on top of Bitcoin", NewWithOpenArms(), "there are no maxis and Vitalk is welcomed with open arms in to building on top of Bitcoin")
}

func TestWithOpenArms_FixGreetAndArm(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm excited to be greet with open arm and the same eagerness and affection that has always awaited me", NewWithOpenArms(), "I'm excited to be greeted with open arms and the same eagerness and affection that has always awaited me")
}
