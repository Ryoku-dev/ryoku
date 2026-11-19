package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestNailInCoffin_FixFinalThe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Maybe this will be the final nail on the coffin for the console.", NewNailInCoffin(), "Maybe this will be the final nail in the coffin for the console.")
}

func TestNailInCoffin_FixFinalHis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A single hit whether blocked or not is going to put the final nail on his coffin.", NewNailInCoffin(), "A single hit whether blocked or not is going to put the final nail in his coffin.")
}

func TestNailInCoffin_FixLastHis(t *testing.T) {
	linttest.AssertSuggestionResult(t, "and as the last nail on his coffin", NewNailInCoffin(), "and as the last nail in his coffin")
}

func TestNailInCoffin_FixFinalHer(t *testing.T) {
	linttest.AssertSuggestionResult(t, "That's literally the final nail on her coffin since she's the dead ass only one that can't use that card.", NewNailInCoffin(), "That's literally the final nail in her coffin since she's the dead ass only one that can't use that card.")
}

func TestNailInCoffin_FixLastTheir(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So I just sent Khalida to put the last nail on their coffin", NewNailInCoffin(), "So I just sent Khalida to put the last nail in their coffin")
}
