package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestGetPassGoPass_FixGetPass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can't get pass the provider confirmation in the exchange feature", NewGetPassGoPass(), "Can't get past the provider confirmation in the exchange feature")
}

func TestGetPassGoPass_FixGetsPass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But once rewritten in block code it gets pass the type check just fine", NewGetPassGoPass(), "But once rewritten in block code it gets past the type check just fine")
}

func TestGetPassGoPass_FixGettingPass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am not getting pass the initial \"Control the game using WASD keys\" message.", NewGetPassGoPass(), "I am not getting past the initial \"Control the game using WASD keys\" message.")
}

func TestGetPassGoPass_FixGoPass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "the X axis movement will go pass the collider", NewGetPassGoPass(), "the X axis movement will go past the collider")
}

func TestGetPassGoPass_FixGoesPass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "return false if it goes pass the number of questions", NewGetPassGoPass(), "return false if it goes past the number of questions")
}

func TestGetPassGoPass_FixGoingPass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Why is it not going pass the first batch?", NewGetPassGoPass(), "Why is it not going past the first batch?")
}

func TestGetPassGoPass_FixGonePass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Make sure we haven't gone pass the node count limit.", NewGetPassGoPass(), "Make sure we haven't gone past the node count limit.")
}

func TestGetPassGoPass_FixGotPass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Ok I got pass the first issue.", NewGetPassGoPass(), "Ok I got past the first issue.")
}

func TestGetPassGoPass_FixGottenPass(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I have now gotten pass the show stoppers.", NewGetPassGoPass(), "I have now gotten past the show stoppers.")
}
