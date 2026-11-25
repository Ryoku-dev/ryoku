package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestInOnesDefense_FixToMyDefenseFromIssue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I did try to drive this thing. And to my defense, um, I bought this before I had other financial and time responsibilities.", NewInOnesDefense(), "I did try to drive this thing. And in my defense, um, I bought this before I had other financial and time responsibilities.")
}

func TestInOnesDefense_FixSentenceInitialToTheirDefence(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To their defence, the docs never mentioned it.", NewInOnesDefense(), "In their defence, the docs never mentioned it.")
}

func TestInOnesDefense_FixToOnesDefense(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Everyone has something to say to one's defense.", NewInOnesDefense(), "Everyone has something to say in one's defense.")
}

func TestInOnesDefense_AllowsRushedToMyDefense(t *testing.T) {
	linttest.AssertNoLints(t, "My friends rushed to my defense when the reviewer complained.", NewInOnesDefense())
}

func TestInOnesDefense_AllowsCameToHerDefence(t *testing.T) {
	linttest.AssertNoLints(t, "Nobody came to her defence after the outage.", NewInOnesDefense())
}

func TestInOnesDefense_AllowsInMyDefense(t *testing.T) {
	linttest.AssertNoLints(t, "In my defense, the tests were already failing.", NewInOnesDefense())
}
