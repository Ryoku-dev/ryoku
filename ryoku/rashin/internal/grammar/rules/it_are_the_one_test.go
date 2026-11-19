package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestItAreTheOne_AreTheOneWho(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Amateurs are the one who argue about syntax.", NewItAreTheOne(), "Amateurs are the ones who argue about syntax.")
}

func TestItAreTheOne_AreTheOnlyOneWho(t *testing.T) {
	linttest.AssertSuggestionResult(t, "those approved addressed are the only one who can change the rate of the token", NewItAreTheOne(), "those approved addressed are the only ones who can change the rate of the token")
}

func TestItAreTheOne_AreTheOneWhich(t *testing.T) {
	linttest.AssertSuggestionResult(t, "graphics and gfx folder are the one which have different svgs and jpg files", NewItAreTheOne(), "graphics and gfx folder are the ones which have different svgs and jpg files")
}

func TestItAreTheOne_WereTheOnlyOneThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "SGN were the only one that actually sent me a sensible map", NewItAreTheOne(), "SGN were the only ones that actually sent me a sensible map")
}

func TestItAreTheOne_WeAreTheOneWho(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They call us Dremers, but we are the one who don't sleep.", NewItAreTheOne(), "They call us Dremers, but we are the ones who don't sleep.")
}

func TestItAreTheOne_AreTheOneThat(t *testing.T) {
	linttest.AssertSuggestionResult(t, "things you don't want to share are the one that would interest your friends the most", NewItAreTheOne(), "things you don't want to share are the ones that would interest your friends the most")
}
