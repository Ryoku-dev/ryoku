package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestTryOnesHandAt_FixTriedMy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I tried my hands at a little test to see how different parameters I get for the same deck and the same material.", NewTryOnesHandAt(), "I tried my hand at a little test to see how different parameters I get for the same deck and the same material.")
}

func TestTryOnesHandAt_FixTriedTheir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "If there isn't any obvious reason why no one has tried their hands at it yet, I might try implementing it.", NewTryOnesHandAt(), "If there isn't any obvious reason why no one has tried their hand at it yet, I might try implementing it.")
}

func TestTryOnesHandAt_FixTriesHis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A fellow programmer from India who tries his hands at everything he can.", NewTryOnesHandAt(), "A fellow programmer from India who tries his hand at everything he can.")
}

func TestTryOnesHandAt_FixTryMy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am happy to try my hands at implementing one, but not being that proficient in C/C++ need some guidance on where to start.", NewTryOnesHandAt(), "I am happy to try my hand at implementing one, but not being that proficient in C/C++ need some guidance on where to start.")
}

func TestTryOnesHandAt_FixTryOur(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One way to make some of the requirements for this more concrete is to try our hands at implementing a language server.", NewTryOnesHandAt(), "One way to make some of the requirements for this more concrete is to try our hand at implementing a language server.")
}

func TestTryOnesHandAt_FixTryTheir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "At the end the user will be able to create a list of decimal numbers to try their hands at the Diagonal Argument on their own.", NewTryOnesHandAt(), "At the end the user will be able to create a list of decimal numbers to try their hand at the Diagonal Argument on their own.")
}

func TestTryOnesHandAt_FixTryYour(t *testing.T) {
	linttest.AssertSuggestionResult(t, "You'll likely need to try your hands at a bit of Lua to make it work.", NewTryOnesHandAt(), "You'll likely need to try your hand at a bit of Lua to make it work.")
}

func TestTryOnesHandAt_FixTryingMy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I wouldn't mind trying my hands at a PR if the solution would be accepted.", NewTryOnesHandAt(), "I wouldn't mind trying my hand at a PR if the solution would be accepted.")
}
